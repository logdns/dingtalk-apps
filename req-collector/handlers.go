package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed static
var staticFS embed.FS

const staticVer = "20261013a"

// 下拉选项全部来自 config.json 的 options 段，未配置时用通用默认值（见 config.go）

// ---- 页面模板（占位符同 Flask 版） ----

const headHTML = `<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/static/arco.min.css?v=` + staticVer + `">
<script src="/static/react.production.min.js?v=` + staticVer + `"></script>
<script src="/static/react-dom.production.min.js?v=` + staticVer + `"></script>
<script src="/static/arco.min.js?v=` + staticVer + `"></script>
<script src="/static/arco-icon.min.js?v=` + staticVer + `"></script>
<script src="/static/dingtalk.js?v=` + staticVer + `"></script>
<script src="/static/auth.js?v=` + staticVer + `"></script>`

const formPage = `<!doctype html><html lang="zh"><head>` + headHTML + `
<title>__TITLE__</title>
<style>
  body { background:#f2f3f5; }
  .wrap { max-width:720px; margin:28px auto; padding:0 16px 40px; }
  .topbar { display:flex; justify-content:space-between; align-items:center; margin-bottom:16px; }
</style></head>
<body><div id="root"></div>
<script>
window.APP_CONFIG = {
  app_title: __TITLE_JSON__,
  departments: __DEPARTMENTS__,
  req_types: __REQ_TYPES__,
  urgencies: __URGENCIES__,
  bills: __BILLS__,
};
</script>
<script src="/static/form.js?v=` + staticVer + `"></script>
</body></html>`

const listPage = `<!doctype html><html lang="zh"><head>` + headHTML + `
<title>需求列表 - __TITLE__</title>
<style>
  body { background:#f2f3f5; }
  .wrap { max-width:1360px; margin:28px auto; padding:0 16px 40px; }
  .topbar { display:flex; justify-content:space-between; align-items:center; margin-bottom:16px; }
</style></head>
<body><div id="root"></div>
<script>
window.APP_CONFIG = {
  app_title: __TITLE_JSON__,
  departments: __DEPARTMENTS__,
  req_types: __REQ_TYPES__,
  urgencies: __URGENCIES__,
  bills: __BILLS__,
  feasibilities: __FEASIBILITIES__,
};
</script>
<script src="/static/list.js?v=` + staticVer + `"></script>
</body></html>`

const adminPage = `<!doctype html><html lang="zh"><head>` + headHTML + `
<title>管理面板 - __TITLE__</title>
<style>
  body { background:#f2f3f5; }
  .wrap { max-width:1360px; margin:28px auto; padding:0 16px 40px; }
  .login-wrap { max-width:420px; margin:12vh auto; }
</style></head>
<body><div id="root"></div>
<script>
window.APP_CONFIG = {
  app_title: __TITLE_JSON__,
  departments: __DEPARTMENTS__,
  req_types: __REQ_TYPES__,
  urgencies: __URGENCIES__,
  bills: __BILLS__,
  feasibilities: __FEASIBILITIES__,
};
</script>
<script src="/static/admin.js?v=` + staticVer + `"></script>
</body></html>`

func renderPage(tpl string, mapping map[string]string) string {
	s := tpl
	for k, v := range mapping {
		s = strings.ReplaceAll(s, "__"+k+"__", v)
	}
	return s
}

// jsonSlice 用 json.Marshal 生成，避免选项里出现引号/尖括号时破坏页面 JS（模板注入）
func jsonSlice(items []string) string {
	if items == nil {
		items = []string{}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	// < > & 转成 \u003c 等，防止提前闭合 <script>
	s := string(b)
	s = strings.ReplaceAll(s, "<", "\\u003c")
	s = strings.ReplaceAll(s, ">", "\\u003e")
	s = strings.ReplaceAll(s, "&", "\\u0026")
	return s
}

// jsonString 生成带引号的 JSON 字符串（同上，做安全转义）
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	out := string(b)
	out = strings.ReplaceAll(out, "<", "\\u003c")
	out = strings.ReplaceAll(out, ">", "\\u003e")
	out = strings.ReplaceAll(out, "&", "\\u0026")
	return out
}

// internalErr 服务端记录详情，客户端只看到泛化错误（不泄露数据库/内部信息）
func internalErr(w http.ResponseWriter, what string, err error) {
	log.Printf("%s 失败: %v", what, err)
	errJSON(w, 500, "服务器内部错误，详情见服务端日志")
}

// ---- 路由 ----

func NewRouter() *http.ServeMux {
	mux := http.NewServeMux()

	// 页面
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		writeHTML(w, renderPage(formPage, pageVars(false)))
	})
	mux.HandleFunc("GET /list", func(w http.ResponseWriter, r *http.Request) {
		writeHTML(w, renderPage(listPage, pageVars(true)))
	})
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		writeHTML(w, renderPage(adminPage, pageVars(true)))
	})

	// 静态资源（embed，去掉 static/ 前缀）
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// 公开 API
	mux.HandleFunc("GET /api/dingtalk/config", hDingConfig)
	mux.HandleFunc("GET /api/me", hMe)
	mux.HandleFunc("POST /api/logout", hLogout)
	mux.HandleFunc("POST /api/dingtalk/login", hDingLogin)
	mux.HandleFunc("POST /api/submit", hSubmit)
	mux.HandleFunc("GET /api/requirements", hRequirements)
	mux.HandleFunc("POST /api/feasibility", hFeasibility)

	// 附件
	mux.HandleFunc("POST /api/attachment/upload", hAttachmentUpload)
	mux.HandleFunc("GET /api/requirements/attachments", hReqAttachments)
	mux.HandleFunc("GET /api/attachment/download", hAttachmentDownload)

	// 管理 API
	mux.HandleFunc("POST /api/admin/login", hAdminLogin)
	mux.HandleFunc("GET /api/admin/status", hAdminStatus)
	mux.HandleFunc("POST /api/admin/logout", hAdminLogout)
	mux.HandleFunc("POST /api/admin/change_password", adminAPI(hChangePassword))
	mux.HandleFunc("POST /api/admin/requirement/update", hReqUpdate)
	mux.HandleFunc("POST /api/admin/requirement/delete", hReqDelete)
	mux.HandleFunc("GET /api/admin/admins", adminAPI(hListAdmins))
	mux.HandleFunc("POST /api/admin/add_admin", adminAPI(hAddAdmin))
	mux.HandleFunc("POST /api/admin/remove_admin", adminAPI(hRemoveAdmin))
	mux.HandleFunc("GET /api/admin/dd_users", adminAPI(hDDUsers))
	mux.HandleFunc("POST /api/admin/attachment/retry", adminAPI(hAttachmentRetry))
	mux.HandleFunc("POST /api/admin/attachment/delete", adminAPI(hAttachmentDelete))
	mux.HandleFunc("GET /api/admin/logs", adminAPI(hAdminLogs))
	mux.HandleFunc("POST /api/admin/logs/clear", adminAPI(hAdminLogsClear))

	return mux
}

func pageVars(withFeas bool) map[string]string {
	v := map[string]string{
		"HEAD":          headHTML,
		"TITLE":         html.EscapeString(AppTitleOrDefault()),
		"TITLE_JSON":    jsonString(AppTitleOrDefault()),
		"DEPARTMENTS":   jsonSlice(OptDepartments()),
		"REQ_TYPES":     jsonSlice(OptReqTypes()),
		"URGENCIES":     jsonSlice(OptUrgencies()),
		"BILLS":         jsonSlice(OptBills()),
		"FEASIBILITIES": jsonSlice(OptFeasibilities()),
	}
	if !withFeas {
		delete(v, "FEASIBILITIES")
	}
	return v
}

func writeHTML(w http.ResponseWriter, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

// adminAPI 包装管理员权限校验
func adminAPI(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAdminSession(r) {
			errJSON(w, http.StatusForbidden, "仅管理人员可操作")
			return
		}
		next(w, r)
	}
}

// ---- 公开 API ----

func hDingConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"enabled":  cfg.DingTalk.CorpID != "",
		"corpId":   cfg.DingTalk.CorpID,
		"clientId": cfg.DingTalk.AppKey,
		"agentId":  cfg.DingTalk.AgentID,
	})
}

func hMe(w http.ResponseWriter, r *http.Request) {
	s := tokenRow(r)
	// 部门兜底回填：老会话可能缺部门，从通讯录补齐并写回会话
	if s != nil && s.UserID != "" && s.Department == "" {
		dept := backfillDepartment(s.UserID)
		if dept != "" {
			s.Department = dept
			_, _ = db.Exec("UPDATE app_sessions SET department=? WHERE token=?", dept, s.Token)
			log.Printf("会话部门回填: %s(%s) → %s", s.Name, s.UserID, dept)
		}
	}
	resp := map[string]any{"logged": false, "token_kind": "none"}
	if s != nil && s.UserID != "" {
		resp = map[string]any{
			"logged":     true,
			"name":       s.Name,
			"userid":     s.UserID,
			"department": s.Department,
			"is_admin":   isAdminSession(r),
		}
		if s.PanelAdmin {
			resp["panel_admin"] = true
		}
		resp["token_kind"] = "token"
	} else if s != nil {
		resp["token_kind"] = "token"
		resp["panel_admin"] = s.PanelAdmin
	}
	writeJSON(w, 200, resp)
}

func hLogout(w http.ResponseWriter, r *http.Request) {
	dropToken(bearerToken(r))
	okJSON(w, nil)
}

// backfillDepartment 从通讯录（在线缓存 → 落库缓存）按 userid 反查部门名
func backfillDepartment(userid string) string {
	if users, err := ddFetchAllUsers(); err == nil {
		for _, u := range users {
			if u.Userid == userid {
				return u.Dept
			}
		}
		return ""
	}
	// 在线拉取失败 → 用落库缓存兜底
	if users, _ := ddCachedUsers(); users != nil {
		for _, u := range users {
			if u.Userid == userid {
				return u.Dept
			}
		}
	}
	return ""
}

func hDingLogin(w http.ResponseWriter, r *http.Request) {
	body := readJSON(r)
	code := strTrim(body, "code")
	if code == "" {
		code = strTrim(body, "authCode")
	}
	if code == "" {
		errJSON(w, 400, "缺少免登授权码")
		return
	}
	if cfg.DingTalk.CorpID == "" {
		errJSON(w, 500, "服务端未配置钉钉企业 CorpId")
		return
	}
	userid, errMsg := ddCodeToUserid(code)
	if userid == "" {
		log.Printf("免登失败: %s", errMsg)
		errJSON(w, 401, "免登失败，请重新从钉钉工作台进入应用")
		return
	}
	name, department := ddUserDetail(userid)
	// 部门兜底：详情接口没拿到时，从通讯录（含缓存）按 userid 反查
	if department == "" {
		department = backfillDepartment(userid)
	}
	log.Printf("免登成功: %s(%s) 部门=%s", name, userid, department)
	// 部门取一级部门名，格式同 Flask 版（仅一个部门名）
	tk, err := issueToken(Session{UserID: userid, Name: name, Department: department})
	if err != nil {
		internalErr(w, "签发会话", err)
		return
	}
	okJSON(w, map[string]any{"token": tk, "name": name, "department": department, "userid": userid,
		"is_admin": isAdminSession(r)})
}

func hSubmit(w http.ResponseWriter, r *http.Request) {
	f := readJSON(r)
	ddUser := currentUser(r)
	department := strTrim(f, "department")
	requester := strTrim(f, "requester")
	if ddUser != nil {
		if ddUser.Department != "" {
			department = ddUser.Department
		} else if dept := backfillDepartment(ddUser.UserID); dept != "" {
			department = dept
			_, _ = db.Exec("UPDATE app_sessions SET department=? WHERE token=?", dept, ddUser.Token)
		}
		requester = ddUser.Name
	}
	description := strTrim(f, "description")
	billName := strTrim(f, "bill_name")
	reqType := strTrim(f, "req_type")
	urgency := str(f, "urgency")
	if urgency == "" {
		urgency = "中"
	}
	valid := false
	for _, u := range OptUrgencies() {
		if u == urgency {
			valid = true
		}
	}
	if !valid {
		urgency = "中"
	}
	if department == "" || description == "" || billName == "" || reqType == "" {
		errJSON(w, 400, "必填项不完整")
		return
	}
	if ddUser == nil && requester == "" {
		errJSON(w, 400, "请填写姓名")
		return
	}
	// 长度上限：与编辑接口保持一致，避免超长文本灌库并塞进钉钉通知
	limits := map[string]int{"description": 2000, "scenario": 2000, "remark": 2000, "bill_name": 100, "requester": 50, "department": 50, "req_type": 50}
	for k, max := range limits {
		if len([]rune(strTrim(f, k))) > max {
			errJSON(w, 400, fmt.Sprintf("字段 %s 超过 %d 字上限", k, max))
			return
		}
	}
	var username any
	if ddUser != nil {
		username = ddUser.UserID
	}
	res, err := db.Exec(
		"INSERT INTO req_items (department, requester, username, description, scenario, bill_name, req_type, urgency, remark) VALUES (?,?,?,?,?,?,?,?,?)",
		department, requester, username, description, strTrim(f, "scenario"), billName, reqType, urgency, strTrim(f, "remark"))
	if err != nil {
		internalErr(w, "提交需求", err)
		return
	}
	reqID, _ := res.LastInsertId()
	// 提交时声明了附件数量 → 通知里带上（附件在提交成功后立即上传）
	attCount, _ := intVal(f, "attachment_count")
	notifyAdminsNewRequirement(map[string]string{
		"department": department, "requester": requester, "description": description,
		"bill_name": billName, "req_type": reqType, "urgency": urgency,
		"attachments": fmt.Sprint(attCount),
	})
	okJSON(w, map[string]any{"id": reqID})
}

func hRequirements(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(
		"SELECT id, department, requester, username, description, scenario, bill_name, req_type, urgency, feasibility, remark, created_at " +
			"FROM req_items ORDER BY id DESC")
	if err != nil {
		internalErr(w, "查询需求列表", err)
		return
	}
	defer rows.Close()
	type Row struct {
		ID          int    `json:"id"`
		Department  string `json:"department"`
		Requester   string `json:"requester"`
		Username    any    `json:"username"`
		Description string `json:"description"`
		Scenario    any    `json:"scenario"`
		BillName    string `json:"bill_name"`
		ReqType     string `json:"req_type"`
		Urgency     string `json:"urgency"`
		Feasibility string `json:"feasibility"`
		Remark      any    `json:"remark"`
		CreatedAt   string `json:"created_at"`
	}
	var out []Row
	for rows.Next() {
		var o Row
		var username, scenario, remark any
		var createdAt time.Time
		if err := rows.Scan(&o.ID, &o.Department, &o.Requester, &username, &o.Description, &scenario,
			&o.BillName, &o.ReqType, &o.Urgency, &o.Feasibility, &remark, &createdAt); err == nil {
			o.Username = nullStr(username)
			o.Scenario = nullStr(scenario)
			o.Remark = nullStr(remark)
			o.CreatedAt = createdAt.Format("2006-01-02 15:04")
			out = append(out, o)
		}
	}
	if out == nil {
		out = []Row{}
	}
	writeJSON(w, 200, map[string]any{"rows": out})
}

func nullStr(v any) any {
	if s, ok := v.(string); ok {
		return s
	}
	return nil
}

func hFeasibility(w http.ResponseWriter, r *http.Request) {
	f := readJSON(r)
	rid, hasID := intVal(f, "id")
	feas := str(f, "feasibility")
	valid := false
	for _, x := range OptFeasibilities() {
		if x == feas {
			valid = true
		}
	}
	if !hasID || !valid {
		errJSON(w, 400, "参数错误")
		return
	}
	_, isAdmin, allowed := rowOwnerOrAdmin(r, rid)
	if !allowed {
		errJSON(w, 403, "无权限")
		return
	}
	// 「能否实现」只能由管理员判定，提交人不可自评
	if !isAdmin {
		errJSON(w, 403, "仅管理员可判定能否实现")
		return
	}
	if _, err := db.Exec("UPDATE req_items SET feasibility=? WHERE id=?", feas, rid); err != nil {
		internalErr(w, "更新能否实现", err)
		return
	}
	// 管理员判定「能否实现」→ 通知需求提交人
	if isAdmin {
		var owner sql.NullString
		var description string
		if err := db.QueryRow("SELECT username, description FROM req_items WHERE id=?", rid).
			Scan(&owner, &description); err != nil {
			log.Printf("结果通知查询失败(记录 %d): %v", rid, err)
		} else if !owner.Valid || owner.String == "" {
			log.Printf("结果通知跳过(记录 %d): 提交人为手动填写姓名，无钉钉账号可推送", rid)
		} else {
			notifySubmitterFeasibility(owner.String, feas, description)
		}
	}
	okJSON(w, nil)
}

// ---- 附件 API ----

// formInt 从表单/查询参数取正整数
func formInt(v string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// hAttachmentUpload 提交成功后上传附件：本地留档 + 后台异步推送钉盘
func hAttachmentUpload(w http.ResponseWriter, r *http.Request) {
	sess := currentUser(r)
	if sess == nil {
		errJSON(w, 401, "请先登录")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(cfg.MaxUploadMB+2)<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		errJSON(w, 400, fmt.Sprintf("上传解析失败（单个附件上限 %dMB）", cfg.MaxUploadMB))
		return
	}
	rid, hasID := formInt(r.FormValue("req_id"))
	if !hasID {
		errJSON(w, 400, "缺少需求编号")
		return
	}
	_, _, allowed := rowOwnerOrAdmin(r, rid)
	if !allowed {
		errJSON(w, 403, "无权限：只能给自己的需求上传附件")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		errJSON(w, 400, "缺少文件")
		return
	}
	defer file.Close()
	if header.Size > int64(cfg.MaxUploadMB)<<20 {
		errJSON(w, 400, fmt.Sprintf("「%s」超过 %dMB 上限", header.Filename, cfg.MaxUploadMB))
		return
	}
	content, err := readAllLimited(file, int64(cfg.MaxUploadMB)<<20)
	if err != nil {
		errJSON(w, 400, fmt.Sprintf("「%s」超过 %dMB 上限", header.Filename, cfg.MaxUploadMB))
		return
	}

	// 1) 本地留档（钉盘推送失败时管理面板仍可下载）
	localPath, err := saveLocal(rid, header.Filename, content)
	if err != nil {
		internalErr(w, "附件保存", err)
		return
	}

	// 2) 记录数据库
	fname := sanitizeFileName(header.Filename)
	res, err := db.Exec(
		`INSERT INTO req_files (req_id, file_name, file_size, local_path, drive_status, uploaded_by, uploaded_by_name)
		 VALUES (?,?,?,?,?,?,?)`,
		rid, fname, header.Size, localPath, "pending", sess.UserID, sess.Name)
	if err != nil {
		internalErr(w, "附件记录", err)
		return
	}
	fid, _ := res.LastInsertId()

	// 3) 先立即响应，钉盘推送放后台异步执行（避免前端长时间等待/卡住）
	//    本地留档已就绪，管理面板随时可下载；钉盘状态随后更新
	operator := sess.UserID
	if operator == "" {
		_ = db.QueryRow("SELECT IFNULL(username,'') FROM req_items WHERE id=?", rid).Scan(&operator)
	}
	log.Printf("附件已接收: 需求 %d ← %s（%d 字节），开始后台推送钉盘", rid, fname, header.Size)
	okJSON(w, map[string]any{
		"file_id": int(fid), "file_name": fname, "drive_status": "pending",
	})
	if operator != "" {
		go pushFileToDrive(int(fid), rid, fname, operator, content)
	} else {
		updateFileDriveStatus(int(fid), "failed", "", "", "无钉钉操作人身份，跳过钉盘上传")
	}
}

// readAllLimited 读取内容并限制大小
func readAllLimited(f io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("too large")
	}
	return b, nil
}

// hReqAttachments 查看某条需求的附件（本人或管理员）
func hReqAttachments(w http.ResponseWriter, r *http.Request) {
	rid, hasID := formInt(r.URL.Query().Get("req_id"))
	if !hasID {
		errJSON(w, 400, "缺少需求编号")
		return
	}
	_, _, allowed := rowOwnerOrAdmin(r, rid)
	if !allowed {
		errJSON(w, 403, "无权限查看该需求的附件")
		return
	}
	files := listFilesByReq(rid)
	if files == nil {
		files = []ReqFile{}
	}
	okJSON(w, map[string]any{"files": files, "count": len(files)})
}

// hAttachmentDownload 下载附件：本地存在直接下发，否则走钉盘临时链接代理
func hAttachmentDownload(w http.ResponseWriter, r *http.Request) {
	fid, hasID := formInt(r.URL.Query().Get("id"))
	if !hasID {
		errJSON(w, 400, "缺少附件编号")
		return
	}
	f := fileByID(fid)
	if f == nil {
		errJSON(w, 404, "附件不存在")
		return
	}
	if _, _, allowed := rowOwnerOrAdmin(r, f.ReqID); !allowed {
		errJSON(w, 403, "无权限下载该附件")
		return
	}
	if f.LocalPath != "" && fileExists(f.LocalPath) {
		w.Header().Set("Content-Type", localContentType(f.LocalPath))
		w.Header().Set("Content-Disposition",
			mime.FormatMediaType("attachment", map[string]string{"filename": f.FileName}))
		http.ServeFile(w, r, f.LocalPath)
		return
	}
	// 本地缺失 → 钉盘临时下载链接（由管理员/提交人身份换取）
	operator := ""
	if s := currentUser(r); s != nil {
		operator = s.UserID
	}
	if operator == "" {
		operator = f.UploadedBy
	}
	if f.DriveSpaceID == "" || f.DriveFileID == "" || operator == "" {
		errJSON(w, 404, "附件文件已丢失（本地与钉盘均不可用）")
		return
	}
	dlURL, hdrs, err := driveDownloadURL(operator, f.DriveSpaceID, f.DriveFileID)
	if err != nil {
		log.Printf("钉盘下载链接获取失败(附件 %d): %v", fid, err)
		errJSON(w, 502, "获取钉盘下载链接失败，请稍后重试")
		return
	}
	req, err := http.NewRequest("GET", dlURL, nil)
	if err != nil {
		errJSON(w, 500, "下载请求构建失败")
		return
	}
	for k, v := range hdrs {
		req.Header.Set(k, v)
	}
	resp, err := driveClient.Do(req)
	if err != nil {
		log.Printf("从钉盘下载失败(附件 %d): %v", fid, err)
		errJSON(w, 502, "从钉盘下载失败，请稍后重试")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		io.Copy(io.Discard, resp.Body)
		errJSON(w, 502, fmt.Sprintf("从钉盘下载失败: HTTP %d", resp.StatusCode))
		return
	}
	w.Header().Set("Content-Type", localContentType(f.FileName))
	w.Header().Set("Content-Disposition",
		mime.FormatMediaType("attachment", map[string]string{"filename": f.FileName}))
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// hAttachmentRetry 管理员：把「仅本地」的附件重新推送到钉盘（权限补齐后补救用）
func hAttachmentRetry(w http.ResponseWriter, r *http.Request) {
	fid, hasID := intVal(readJSON(r), "id")
	if !hasID {
		if v, ok := formInt(r.URL.Query().Get("id")); ok && v > 0 {
			fid, hasID = v, true
		}
	}
	if !hasID || fid <= 0 {
		errJSON(w, 400, "缺少附件编号")
		return
	}
	f := fileByID(fid)
	if f == nil {
		errJSON(w, 404, "附件不存在")
		return
	}
	if f.DriveStatus == "uploaded" {
		errJSON(w, 400, "该附件已在钉盘中，无需同步")
		return
	}
	if f.LocalPath == "" || !fileExists(f.LocalPath) {
		errJSON(w, 404, "本地副本已丢失，无法同步（请在填写页重新上传附件）")
		return
	}
	operator := f.UploadedBy
	if s := currentUser(r); s != nil && s.UserID != "" {
		operator = s.UserID
	}
	if operator == "" {
		errJSON(w, 400, "缺少钉钉操作人身份，无法同步")
		return
	}
	go func() {
		spaceID, driveFID, err := retryFileToDrive(f.ID, f.LocalPath, f.FileName, operator)
		if err != nil {
			updateFileDriveStatus(f.ID, "failed", "", "", err.Error())
			log.Printf("管理员重新同步钉盘失败(附件 %d, %s): %s", f.ID, f.FileName, err.Error())
			return
		}
		updateFileDriveStatus(f.ID, "uploaded", spaceID, driveFID, "")
		log.Printf("管理员重新同步钉盘成功(附件 %d, %s)", f.ID, f.FileName)
	}()
	okJSON(w, map[string]any{"message": "已在后台同步，稍后刷新查看状态"})
}

// hAttachmentDelete 管理员：删除已丢失的附件记录（本地与钉盘都取不到时清理脏数据）
func hAttachmentDelete(w http.ResponseWriter, r *http.Request) {
	fid, hasID := intVal(readJSON(r), "id")
	if !hasID {
		errJSON(w, 400, "缺少附件编号")
		return
	}
	f := fileByID(fid)
	if f == nil {
		errJSON(w, 404, "附件不存在")
		return
	}
	if f.LocalPath != "" && fileExists(f.LocalPath) {
		_ = os.Remove(f.LocalPath)
	}
	if _, err := db.Exec("DELETE FROM req_files WHERE id=?", fid); err != nil {
		internalErr(w, "删除附件", err)
		return
	}
	log.Printf("管理员删除附件记录: id=%d（需求 %d, %s）", fid, f.ReqID, f.FileName)
	okJSON(w, nil)
}

// ---- 管理 API ----

// ---- 管理面板登录限流（防暴力破解） ----
// 同一 IP 连续失败 maxLoginFails 次 → 锁定 loginBlockTTL；成功一次即清零。
const (
	maxLoginFails = 5
	loginBlockTTL = 5 * time.Minute
)

var (
	loginMu      sync.Mutex
	loginFails   = map[string]int{}
	loginBlocked = map[string]time.Time{}
)

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// 仅当请求来自本机反向代理（Nginx 等）时才信任 X-Forwarded-For，
	// 否则攻击者可用该头伪造来源 IP 绕过登录限流。
	if host == "127.0.0.1" || host == "::1" {
		if x := r.Header.Get("X-Forwarded-For"); x != "" {
			return strings.TrimSpace(strings.Split(x, ",")[0])
		}
	}
	return host
}

// loginBlockLeft 返回还需等待的秒数与是否被锁定
func loginBlockLeft(ip string) (int, bool) {
	loginMu.Lock()
	defer loginMu.Unlock()
	if t, ok := loginBlocked[ip]; ok {
		if left := time.Until(t); left > 0 {
			return int(left.Seconds()) + 1, true
		}
		delete(loginBlocked, ip)
		delete(loginFails, ip)
	}
	return 0, false
}

func loginNoteFail(ip string) (remain int) {
	loginMu.Lock()
	defer loginMu.Unlock()
	loginFails[ip]++
	remain = maxLoginFails - loginFails[ip]
	if remain <= 0 {
		loginBlocked[ip] = time.Now().Add(loginBlockTTL)
		remain = 0
	}
	log.Printf("管理面板登录失败 1 次（来源 IP %s，剩余尝试 %d 次）", ip, remain)
	return remain
}

func loginNoteSuccess(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginFails, ip)
	delete(loginBlocked, ip)
}

func hAdminLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if secs, blocked := loginBlockLeft(ip); blocked {
		errJSON(w, 429, fmt.Sprintf("密码错误次数过多，请 %d 秒后再试", secs))
		return
	}
	password := str(readJSON(r), "password")
	stored, err := getSetting("admin_password")
	if err != nil || stored == "" || !checkPassword(stored, password) {
		remain := loginNoteFail(ip)
		msg := "管理密码错误"
		if remain > 0 {
			msg = fmt.Sprintf("管理密码错误，还可尝试 %d 次", remain)
		} else {
			msg = "密码错误次数过多，已锁定 5 分钟"
		}
		errJSON(w, 401, msg)
		return
	}
	loginNoteSuccess(ip)
	// 已有钉钉身份令牌 → 升级为管理令牌
	if s := tokenRow(r); s != nil {
		if _, err := db.Exec("UPDATE app_sessions SET panel_admin=1 WHERE token=?", s.Token); err == nil {
			okJSON(w, map[string]any{"token": s.Token})
			return
		}
	}
	tk, err := issueToken(Session{Name: "管理面板", PanelAdmin: true})
	if err != nil {
		internalErr(w, "更新需求", err)
		return
	}
	okJSON(w, map[string]any{"token": tk})
}

func hAdminStatus(w http.ResponseWriter, r *http.Request) {
	s := tokenRow(r)
	writeJSON(w, 200, map[string]any{
		"logged":      s != nil && isAdminSession(r),
		"panel_admin": s != nil && s.PanelAdmin,
	})
}

func hAdminLogout(w http.ResponseWriter, r *http.Request) {
	s := tokenRow(r)
	if s != nil {
		if s.UserID != "" { // 钉钉身份令牌只降级
			_, _ = db.Exec("UPDATE app_sessions SET panel_admin=0 WHERE token=?", s.Token)
		} else {
			dropToken(s.Token)
		}
	}
	okJSON(w, nil)
}

func hChangePassword(w http.ResponseWriter, r *http.Request) {
	f := readJSON(r)
	oldPw, newPw := str(f, "old_password"), str(f, "new_password")
	stored, err := getSetting("admin_password")
	if err != nil || stored == "" || !checkPassword(stored, oldPw) {
		errJSON(w, 400, "原密码错误")
		return
	}
	if len([]rune(newPw)) < 6 {
		errJSON(w, 400, "新密码至少 6 位")
		return
	}
	if _, err := db.Exec("UPDATE req_settings SET v=? WHERE k='admin_password'", hashPassword(newPw)); err != nil {
		internalErr(w, "修改管理密码", err)
		return
	}
	// 改密后强制所有管理会话重新登录，避免旧令牌继续可用
	if _, err := db.Exec("DELETE FROM app_sessions WHERE panel_admin=1"); err != nil {
		log.Printf("清理管理会话失败: %v", err)
	}
	log.Printf("管理面板密码已修改，全部管理会话已失效")
	okJSON(w, nil)
}

var editableFields = []struct {
	Name   string
	MaxLen int
}{
	{"department", 50}, {"requester", 50}, {"description", 2000}, {"scenario", 2000},
	{"bill_name", 100}, {"req_type", 50}, {"urgency", 10}, {"feasibility", 20}, {"remark", 500},
}

func hReqUpdate(w http.ResponseWriter, r *http.Request) {
	f := readJSON(r)
	rid, hasID := intVal(f, "id")
	if !hasID {
		errJSON(w, 400, "参数错误")
		return
	}
	_, isAdmin, allowed := rowOwnerOrAdmin(r, rid)
	if !allowed {
		errJSON(w, 403, "无权限：只能编辑自己提交的需求")
		return
	}
	// 编辑前快照：用于判断「能否实现」是否被管理员改动（改动则通知提交人）
	var oldFeas, newFeas, owner, description string
	feasChanged := false
	if isAdmin {
		var oFeas, oOwner sql.NullString
		if err := db.QueryRow("SELECT feasibility, username, description FROM req_items WHERE id=?", rid).
			Scan(&oFeas, &oOwner, &description); err != nil {
			log.Printf("编辑前查询记录 %d 失败: %v", rid, err)
		} else {
			oldFeas = oFeas.String
			owner = oOwner.String
		}
	}
	var sets []string
	var params []any
	for _, field := range editableFields {
		if !isAdmin && field.Name == "feasibility" {
			continue
		}
		val, ok := f[field.Name]
		if !ok {
			continue
		}
		s := strTrim(map[string]any{"x": val}, "x")
		if len([]rune(s)) > field.MaxLen {
			s = string([]rune(s)[:field.MaxLen])
		}
		if field.Name == "urgency" && !inSlice(s, OptUrgencies()) {
			continue
		}
		if field.Name == "feasibility" && !inSlice(s, OptFeasibilities()) {
			continue
		}
		if field.Name == "feasibility" {
			newFeas = s
			feasChanged = s != oldFeas
		}
		if field.Name == "description" && s == "" {
			continue
		}
		sets = append(sets, field.Name+"=?")
		params = append(params, s)
	}
	if len(sets) == 0 {
		errJSON(w, 400, "无可更新字段")
		return
	}
	params = append(params, rid)
	q := fmt.Sprintf("UPDATE req_items SET %s WHERE id=?", strings.Join(sets, ", "))
	if _, err := db.Exec(q, params...); err != nil {
		internalErr(w, "删除需求", err)
		return
	}
	// 管理员在编辑弹窗里改了「能否实现」→ 同样通知需求提交人
	if isAdmin && feasChanged {
		if owner == "" {
			log.Printf("结果通知跳过(记录 %d): 提交人为手动填写姓名，无钉钉账号可推送", rid)
		} else {
			log.Printf("管理员编辑将记录 %d 能否实现由「%s」改为「%s」，准备通知提交人 %s", rid, oldFeas, newFeas, owner)
			notifySubmitterFeasibility(owner, newFeas, description)
		}
	}
	okJSON(w, nil)
}

func hReqDelete(w http.ResponseWriter, r *http.Request) {
	rid, hasID := intVal(readJSON(r), "id")
	if !hasID {
		errJSON(w, 400, "参数错误")
		return
	}
	if _, _, allowed := rowOwnerOrAdmin(r, rid); !allowed {
		errJSON(w, 403, "无权限：只能删除自己提交的需求")
		return
	}
	// 联动清理附件记录与本地文件
	if files := listFilesByReq(rid); files != nil {
		for _, f := range files {
			if f.LocalPath != "" && fileExists(f.LocalPath) {
				_ = os.Remove(f.LocalPath)
			}
		}
	}
	_, _ = db.Exec("DELETE FROM req_files WHERE req_id=?", rid)
	if _, err := db.Exec("DELETE FROM req_items WHERE id=?", rid); err != nil {
		internalErr(w, "删除需求", err)
		return
	}
	okJSON(w, nil)
}

func hListAdmins(w http.ResponseWriter, r *http.Request) {
	type Admin struct {
		ID        int    `json:"id"`
		Userid    string `json:"userid"`
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}
	type Sub struct {
		Userid string `json:"userid"`
		Name   string `json:"name"`
	}
	var admins []Admin
	rows, err := db.Query("SELECT id, userid, name, created_at FROM req_admins ORDER BY id")
	if err == nil {
		for rows.Next() {
			var a Admin
			var createdAt time.Time
			if rows.Scan(&a.ID, &a.Userid, &a.Name, &createdAt) == nil {
				a.CreatedAt = createdAt.Format("2006-01-02 15:04")
				admins = append(admins, a)
			}
		}
		rows.Close()
	}
	if admins == nil {
		admins = []Admin{}
	}
	var submitters []Sub
	rows2, err := db.Query("SELECT DISTINCT username, requester FROM req_items WHERE username IS NOT NULL AND username != ''")
	if err == nil {
		for rows2.Next() {
			var s Sub
			if rows2.Scan(&s.Userid, &s.Name) == nil {
				submitters = append(submitters, s)
			}
		}
		rows2.Close()
	}
	if submitters == nil {
		submitters = []Sub{}
	}
	writeJSON(w, 200, map[string]any{"admins": admins, "submitters": submitters})
}

func hAddAdmin(w http.ResponseWriter, r *http.Request) {
	f := readJSON(r)
	userid, name := strTrim(f, "userid"), strTrim(f, "name")
	if userid == "" || name == "" {
		errJSON(w, 400, "参数不完整")
		return
	}
	_, err := db.Exec("INSERT INTO req_admins (userid, name) VALUES (?,?)", userid, name)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			errJSON(w, 400, "该用户已是管理员")
			return
		}
		internalErr(w, "添加管理员", err)
		return
	}
	okJSON(w, nil)
}

func hRemoveAdmin(w http.ResponseWriter, r *http.Request) {
	rid, hasID := intVal(readJSON(r), "id")
	if !hasID {
		errJSON(w, 400, "参数错误")
		return
	}
	if _, err := db.Exec("DELETE FROM req_admins WHERE id=?", rid); err != nil {
		internalErr(w, "移除管理员", err)
		return
	}
	okJSON(w, nil)
}

// hAdminLogs 在线日志（管理面板「运行日志」页签）
func hAdminLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"ok": true, "lines": appLog.Lines(300)})
}

func hAdminLogsClear(w http.ResponseWriter, r *http.Request) {
	appLog.Clear()
	okJSON(w, nil)
}

func hDDUsers(w http.ResponseWriter, r *http.Request) {
	keyword := strings.ToLower(trimSpace(r.URL.Query().Get("name")))
	if keyword == "" {
		errJSON(w, 400, "请输入姓名关键字")
		return
	}
	users, err := ddFetchAllUsers()
	source, ts := "live", ""
	if err != nil {
		log.Printf("通讯录拉取失败，尝试缓存: %v", err)
		users2, ts2 := ddCachedUsers()
		if users2 == nil {
			internalErr(w, "拉取钉钉通讯录", err)
			return
		}
		users, ts, source = users2, ts2, "cache"
	}
	var matched []DDUser
	seen := map[string]bool{}
	for _, u := range users {
		if strings.Contains(strings.ToLower(u.Name), keyword) && !seen[u.Userid] {
			seen[u.Userid] = true
			matched = append(matched, u)
			if len(matched) >= 50 {
				break
			}
		}
	}
	if matched == nil {
		matched = []DDUser{}
	}
	extra := map[string]any{"users": matched, "source": source}
	if source == "cache" {
		extra["cached_at"] = ts
	} else {
		extra["cached_at"] = nil
	}
	okJSON(w, extra)
}

func inSlice(s string, list []string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func trimSpace(s string) string { return strings.TrimSpace(s) }

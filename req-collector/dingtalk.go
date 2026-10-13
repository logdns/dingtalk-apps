package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var ding *DingCfg
var baseURL string // 对外访问地址（通知卡片跳转用）
var httpClient = &http.Client{Timeout: 15 * time.Second}

func InitDingTalk(c *Config) {
	ding = &c.DingTalk
	baseURL = c.BaseURL
}

// ---------------- access_token（带缓存） ----------------

var (
	ddTokenMu    sync.Mutex
	ddTokenVal   string
	ddTokenUntil time.Time
)

func ddAccessToken() (string, error) {
	ddTokenMu.Lock()
	defer ddTokenMu.Unlock()
	if ddTokenVal != "" && time.Now().Before(ddTokenUntil) {
		return ddTokenVal, nil
	}
	body, _ := json.Marshal(map[string]string{"appKey": ding.AppKey, "appSecret": ding.AppSecret})
	resp, err := httpClient.Post("https://api.dingtalk.com/v1.0/oauth2/accessToken", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int64  `json:"expireIn"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.AccessToken == "" {
		return "", fmt.Errorf("钉钉返回空 accessToken")
	}
	ddTokenVal = r.AccessToken
	ddTokenUntil = time.Now().Add(time.Duration(r.ExpireIn-300) * time.Second)
	return ddTokenVal, nil
}

// ddPost 调用 oapi 旧版接口（POST JSON）
func ddPost(path string, payload map[string]any) map[string]any {
	token, err := ddAccessToken()
	if err != nil {
		return map[string]any{"errcode": -1, "errmsg": "获取钉钉凭证失败: " + err.Error()}
	}
	return ddPostRaw("https://oapi.dingtalk.com"+path+"?access_token="+token, payload)
}

func ddPostRaw(url string, payload map[string]any) map[string]any {
	body, _ := json.Marshal(payload)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return map[string]any{"errcode": -2, "errmsg": err.Error()}
	}
	defer resp.Body.Close()
	var r map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return map[string]any{"errcode": -3, "errmsg": err.Error()}
	}
	return r
}

func rStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func rInt(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

// ---------------- 免登码换 userid（兼容新旧码） ----------------

// ddCodeToUserid 旧码走 v2/v1 getuserinfo；新码（OAuth2）走 userAccessToken → users/me → getbyunionid
func ddCodeToUserid(code string) (string, string) {
	legacyErr := ""
	// 1) 旧版免登码（v1 接口已被钉钉下线，优先 v2）
	token, err := ddAccessToken()
	if err == nil {
		for _, path := range []string{"/topapi/v2/user/getuserinfo", "/topapi/user/getuserinfo"} {
			data := ddPostRaw("https://oapi.dingtalk.com"+path+"?access_token="+token, map[string]any{"code": code})
			if rInt(data, "errcode") == 0 {
				result, _ := data["result"].(map[string]any)
				if uid := rStr(result, "userid"); uid != "" {
					return uid, ""
				}
			}
			legacyErr = fmt.Sprintf("%s: %s (errcode=%v)", path, rStr(data, "errmsg"), data["errcode"])
			ec := rInt(data, "errcode")
			if ec != 15 && ec != 60020 {
				break
			}
		}
	} else {
		legacyErr = "获取 access_token 失败: " + err.Error()
	}

	// 2) 新版 OAuth2 免登码
	body, _ := json.Marshal(map[string]string{
		"clientId": ding.AppKey, "clientSecret": ding.AppSecret,
		"code": code, "grantType": "authorization_code",
	})
	resp, err := httpClient.Post("https://api.dingtalk.com/v1.0/oauth2/userAccessToken", "application/json", bytes.NewReader(body))
	if err != nil {
		// 明细只进服务端日志，不回显给客户端（避免泄露内部接口与令牌信息）
		log.Printf("钉钉免登失败（新码请求异常）: %v | 旧码链路: %s", err, legacyErr)
		return "", "免登失败，请重新从钉钉工作台进入应用"
	}
	var r struct {
		Result struct {
			AccessToken string `json:"accessToken"`
			UnionID     string `json:"unionId"`
		} `json:"result"`
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&r)
	resp.Body.Close()
	if r.Result.AccessToken == "" {
		// 注意：不要序列化整个响应（含 accessToken）回传给客户端
		log.Printf("钉钉免登失败（新码换取失败）: code=%s message=%s | 旧码链路: %s", r.Code, r.Message, legacyErr)
		return "", "免登失败，请重新从钉钉工作台进入应用"
	}
	union := r.Result.UnionID
	// users/me 取 userId / unionId
	req, _ := http.NewRequest("GET", "https://api.dingtalk.com/v1.0/contact/users/me", nil)
	req.Header.Set("x-acs-dingtalk-access-token", r.Result.AccessToken)
	if resp2, err := httpClient.Do(req); err == nil {
		var me map[string]any
		_ = json.NewDecoder(resp2.Body).Decode(&me)
		resp2.Body.Close()
		if uid, _ := me["userId"].(string); uid != "" {
			return uid, ""
		}
		if u, _ := me["unionId"].(string); u != "" {
			union = u
		}
	}
	if union != "" {
		token, _ = ddAccessToken()
		data := ddPostRaw("https://oapi.dingtalk.com/topapi/user/getbyunionid?access_token="+token,
			map[string]any{"unionid": union})
		result, _ := data["result"].(map[string]any)
		if uid := rStr(result, "userid"); uid != "" {
			return uid, ""
		}
		return "", "unionid 换 userid 失败"
	}
	return "", "新码换取成功但未获取到用户标识"
}

// ddUserDetail userid → 姓名 / 部门名
func ddUserDetail(userid string) (name, department string) {
	token, err := ddAccessToken()
	if err != nil {
		return userid, ""
	}
	data := ddPostRaw("https://oapi.dingtalk.com/topapi/v2/user/get?access_token="+token,
		map[string]any{"userid": userid})
	result, _ := data["result"].(map[string]any)
	name = rStr(result, "name")
	if name == "" {
		name = userid
	}
	if deptList, ok := result["dept_id_list"].([]any); ok && len(deptList) > 0 {
		if d, ok := deptList[0].(map[string]any); ok {
			if deptID := rInt(d, "dept_id"); deptID != 0 {
				department = ddDeptName(token, deptID)
			}
		}
	}
	return name, department
}

func ddDeptName(token string, deptID int) string {
	for _, req := range []struct {
		path    string
		payload map[string]any
	}{
		{"/topapi/v2/department/get", map[string]any{"dept_id": deptID}},
		{"/topapi/department/get", map[string]any{"id": deptID}},
	} {
		data := ddPostRaw("https://oapi.dingtalk.com"+req.path+"?access_token="+token, req.payload)
		if rInt(data, "errcode") == 0 {
			result, _ := data["result"].(map[string]any)
			if nm := rStr(result, "name"); nm != "" {
				return nm
			}
		}
	}
	return ""
}

// ---------------- 通讯录全量拉取（内存缓存 10 分钟 + 落库兜底） ----------------

type DDUser struct {
	Userid string `json:"userid"`
	Name   string `json:"name"`
	Dept   string `json:"dept"`
}

var (
	ddUsersMu    sync.Mutex
	ddUsersCache []DDUser
	ddUsersUntil time.Time
)

func WarmupDirectory() {
	if _, err := ddFetchAllUsers(); err != nil {
		log.Printf("通讯录预热失败: %v", err)
	} else {
		log.Printf("通讯录预热完成")
	}
}

func ddFetchAllUsers() ([]DDUser, error) {
	ddUsersMu.Lock()
	defer ddUsersMu.Unlock()
	if ddUsersCache != nil && time.Now().Before(ddUsersUntil) {
		return ddUsersCache, nil
	}
	token, err := ddAccessToken()
	if err != nil {
		return nil, fmt.Errorf("获取钉钉 access_token 失败，请检查应用凭证")
	}

	// 1) 遍历部门树（根部门 dept_id=1）
	deptNames := map[int]string{1: "总部"}
	queue := []int{1}
	visited := map[int]bool{}
	for len(queue) > 0 && len(visited) < 200 {
		deptID := queue[0]
		queue = queue[1:]
		if visited[deptID] {
			continue
		}
		visited[deptID] = true
		data := ddPostRaw("https://oapi.dingtalk.com/topapi/v2/department/listsub?access_token="+token,
			map[string]any{"dept_id": deptID})
		if rInt(data, "errcode") != 0 {
			if deptID == 1 {
				return nil, fmt.Errorf("获取部门列表失败：%s（请确认已开通「通讯录部门信息读权限」「通讯录部门成员读权限」）", rStr(data, "errmsg"))
			}
			continue
		}
		list, _ := data["result"].([]any)
		for _, item := range list {
			d, _ := item.(map[string]any)
			id := rInt(d, "dept_id")
			deptNames[id] = rStr(d, "name")
			queue = append(queue, id)
		}
	}

	// 2) 拉取每个部门的成员
	var users []DDUser
	for deptID, deptName := range deptNames {
		cursor := 0
		for i := 0; i < 20; i++ {
			data := ddPostRaw("https://oapi.dingtalk.com/topapi/v2/user/list?access_token="+token,
				map[string]any{"dept_id": deptID, "cursor": cursor, "size": 100})
			if rInt(data, "errcode") != 0 {
				break
			}
			result, _ := data["result"].(map[string]any)
			list, _ := result["list"].([]any)
			for _, item := range list {
				u, _ := item.(map[string]any)
				users = append(users, DDUser{Userid: rStr(u, "userid"), Name: rStr(u, "name"), Dept: deptName})
			}
			if hasMore, _ := result["has_more"].(bool); !hasMore {
				break
			}
			cursor = rInt(result, "next_cursor")
		}
		if len(users) > 20000 {
			break
		}
	}

	ddUsersCache = users
	ddUsersUntil = time.Now().Add(10 * time.Minute)
	ddPersistUsers(users)
	return users, nil
}

func ddPersistUsers(users []DDUser) {
	b, _ := json.Marshal(map[string]any{"users": users, "ts": time.Now().Format("2006-01-02 15:04")})
	_ = setSetting("dd_users_cache", string(b))
}

func ddCachedUsers() ([]DDUser, string) {
	v, err := getSetting("dd_users_cache")
	if err != nil || v == "" {
		return nil, ""
	}
	var d struct {
		Users []DDUser `json:"users"`
		Ts    string   `json:"ts"`
	}
	if err := json.Unmarshal([]byte(v), &d); err != nil {
		return nil, ""
	}
	return d.Users, d.Ts
}

// ---------------- 钉钉工作通知（卡片式，markdown 排版 + 横排按钮） ----------------

// urgencyEmoji 紧急程度 → 彩色圆点
func urgencyEmoji(u string) string {
	switch u {
	case "紧急":
		return "🔴"
	case "高":
		return "🟠"
	case "低":
		return "🟢"
	default:
		return "🟡"
	}
}

// ddNotify 发送 action_card 工作通知；buttons 为空时退化为纯文本
// buttons: [][2]string{{按钮文字, 跳转地址}, ...}，多于 1 个时横排显示
func ddNotify(userids []string, title, markdown string, buttons [][2]string) error {
	if len(userids) == 0 {
		return fmt.Errorf("无接收人")
	}
	var msg map[string]any
	if len(buttons) > 0 && buttons[0][1] != "" {
		card := map[string]any{"title": title, "markdown": markdown}
		if len(buttons) == 1 {
			card["single_title"] = buttons[0][0]
			card["single_url"] = buttons[0][1]
		} else {
			var bl []map[string]string
			for _, b := range buttons {
				bl = append(bl, map[string]string{"title": b[0], "action_url": b[1]})
			}
			card["btn_json_list"] = bl
			card["btn_orientation"] = "1" // 1 = 按钮横排
		}
		msg = map[string]any{"msgtype": "action_card", "action_card": card}
	} else {
		msg = map[string]any{
			"msgtype": "text",
			"text":    map[string]string{"content": title + "\n----------------\n" + markdown},
		}
	}
	data := ddPost("/topapi/message/corpconversation/asyncsend_v2", map[string]any{
		"agent_id":    ding.AgentID,
		"userid_list": strings.Join(userids, ","),
		"msg":         msg,
	})
	if rInt(data, "errcode") != 0 {
		b, _ := json.Marshal(data)
		return fmt.Errorf("钉钉返回: %s", string(b))
	}
	// 异步回查投递结果：把无效用户/被禁止用户写进日志，避免"假成功"
	taskID := int64(rInt(data, "task_id"))
	go func() {
		defer func() { recover() }()
		time.Sleep(4 * time.Second)
		p := ddPost("/topapi/message/corpconversation/getsendresult", map[string]any{
			"agent_id": ding.AgentID, "task_id": taskID,
		})
		if rInt(p, "errcode") != 0 {
			return
		}
		res, _ := p["send_result"].(map[string]any)
		if res == nil {
			return
		}
		var bad []string
		for _, key := range []string{"invalid_user_id_list", "failed_user_id_list", "forbidden_list"} {
			if list, ok := res[key].([]any); ok && len(list) > 0 {
				b, _ := json.Marshal(list)
				bad = append(bad, key+"="+string(b))
			}
		}
		if len(bad) > 0 {
			log.Printf("通知投递异常(task %d): %s", taskID, strings.Join(bad, " "))
		} else {
			log.Printf("通知投递确认(task %d): 已送达 %d 人", taskID, len(userids))
		}
	}()
	return nil
}

// notifyAdminsNewRequirement 新需求 → 通知全部管理员（卡片可点击进管理面板）
func notifyAdminsNewRequirement(info map[string]string) {
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("新需求通知 panic: %v", rec)
			}
		}()
		rows, err := db.Query("SELECT userid FROM req_admins")
		if err != nil {
			log.Printf("新需求通知查询失败: %v", err)
			return
		}
		var uids []string
		for rows.Next() {
			var userid string
			if err := rows.Scan(&userid); err == nil && userid != "" {
				uids = append(uids, userid)
			}
		}
		rows.Close()
		base := strings.TrimRight(baseURL, "/")
		attLine := ""
		if n := info["attachments"]; n != "" && n != "0" {
			attLine = fmt.Sprintf("\n\n**附件**：📎 %s 个（管理面板可下载）", n)
		}
		markdown := fmt.Sprintf(
			"### 📋 收到新需求\n\n"+
				"**部　门**：%s\n\n"+
				"**需求人**：%s\n\n"+
				"**需求描述**\n\n%s\n\n"+
				"---\n\n"+
				"**单据/类型**：%s（%s）\n\n"+
				"**紧急程度**：%s %s%s",
			info["department"], info["requester"], info["description"],
			info["bill_name"], info["req_type"], urgencyEmoji(info["urgency"]), info["urgency"], attLine)
		var btns [][2]string
		if base != "" {
			btns = [][2]string{
				{"打开管理面板处理", base + "/admin"},
				{"查看需求列表", base + "/list"},
			}
		}
		err = ddNotify(uids, "【"+AppTitleOrDefault()+"】收到新需求", markdown, btns)
		if err != nil {
			log.Printf("新需求通知失败: %v", err)
		} else {
			log.Printf("已通知 %d 位管理员（新需求：%s）", len(uids), info["requester"])
		}
	}()
}

// notifySubmitterFeasibility 管理员更新「能否实现」→ 通知需求提交人（卡片可点击进需求列表）
func notifySubmitterFeasibility(ownerUserid, feasibility, description string) {
	if ownerUserid == "" {
		return
	}
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("结果通知 panic: %v", rec)
			}
		}()
		short := description
		if len(short) > 60 {
			short = short[:60] + "…"
		}
		base := strings.TrimRight(baseURL, "/")
		feasEmoji := "🟡"
		if strings.Contains(feasibility, "能") && !strings.Contains(feasibility, "不") {
			feasEmoji = "🟢"
		} else if strings.Contains(feasibility, "不") {
			feasEmoji = "🔴"
		}
		markdown := fmt.Sprintf(
			"### ✅ 您的需求有了处理结果\n\n"+
				"**需求描述**\n\n%s\n\n"+
				"---\n\n"+
				"**能否实现**：%s %s\n\n"+
				"如对结果有疑问，请联系管理员。",
			short, feasEmoji, feasibility)
		var btns [][2]string
		if base != "" {
			btns = [][2]string{
				{"查看需求列表", base + "/list"},
				{"提交新需求", base + "/"},
			}
		}
		err := ddNotify([]string{ownerUserid}, "【"+AppTitleOrDefault()+"】处理结果通知", markdown, btns)
		if err != nil {
			log.Printf("结果通知失败(%s): %v", ownerUserid, err)
		} else {
			log.Printf("已通知需求提交人 %s（能否实现：%s）", ownerUserid, feasibility)
		}
	}()
}

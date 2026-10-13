package main

// 钉钉企业存储（钉盘）附件：服务端代传 → OSS 带签直传 → commit 挂载 → 临时下载链接
// 走新版 storage 接口（v1.0/storage，对应 Storage.* 权限点）：
//   POST /v1.0/storage/spaces/{spaceId}/files/uploadInfos/query   → Storage.UploadInfo.Read
//   PUT  OSS resourceUrls[0]（带 headerSignatureInfo.headers）
//   POST /v1.0/storage/spaces/{spaceId}/files/commit             → Storage.File.Write
//   POST /v1.0/storage/spaces/{spaceId}/dentries/{id}/downloadInfos/query → Storage.DownloadInfo.Read
// 权限未开通时自动降级：附件仅保存在服务器本地，管理面板仍可下载。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var driveClient = &http.Client{Timeout: 120 * time.Second} // 大文件直传 OSS 用长超时

// driveAPI 调用 api.dingtalk.com 新版接口（GET/POST JSON）
func driveAPI(method, path string, query url.Values, body any) (int, map[string]any, error) {
	token, err := ddAccessToken()
	if err != nil {
		return 0, nil, fmt.Errorf("获取钉钉凭证失败: %w", err)
	}
	full := "https://api.dingtalk.com" + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req, err = http.NewRequest(method, full, bytes.NewReader(b))
	} else {
		req, err = http.NewRequest(method, full, nil)
	}
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("x-acs-dingtalk-access-token", token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := driveClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil && err != io.EOF {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, m, nil
}

// permissionHint 权限报错时给出可点击的申请链接
func permissionHint(m map[string]any) string {
	code, _ := m["code"].(string)
	if !strings.Contains(code, "PermissionDenied") {
		return ""
	}
	scopes := "未知权限"
	if d, ok := m["accessdenieddetail"].(map[string]any); ok {
		if list, ok := d["requiredScopes"].([]any); ok && len(list) > 0 {
			var ss []string
			for _, s := range list {
				ss = append(ss, fmt.Sprint(s))
			}
			scopes = strings.Join(ss, " / ")
		}
	}
	return fmt.Sprintf("钉钉应用缺少权限点「%s」，请在开发者后台开通（应用权限-钉盘）后重试", scopes)
}

// ---------------- unionId 缓存（userid → unionId，24h） ----------------

var (
	uidMapMu  sync.Mutex
	uidMap    = map[string]string{}
	uidMapExp = map[string]time.Time{}
)

func driveUnionID(userid string) (string, error) {
	uidMapMu.Lock()
	defer uidMapMu.Unlock()
	if v, ok := uidMap[userid]; ok && time.Now().Before(uidMapExp[userid]) {
		return v, nil
	}
	token, err := ddAccessToken()
	if err != nil {
		return "", err
	}
	data := ddPostRaw("https://oapi.dingtalk.com/topapi/v2/user/get?access_token="+token,
		map[string]any{"userid": userid})
	if rInt(data, "errcode") != 0 {
		return "", fmt.Errorf("获取用户 %s unionId 失败: %s", userid, rStr(data, "errmsg"))
	}
	result, _ := data["result"].(map[string]any)
	union := rStr(result, "unionid")
	if union == "" {
		return "", fmt.Errorf("用户 %s 无 unionId", userid)
	}
	uidMap[userid] = union
	uidMapExp[userid] = time.Now().Add(24 * time.Hour)
	return union, nil
}

// ---------------- 钉盘空间（缓存 10 分钟） ----------------

var (
	driveSpaceMu    sync.Mutex
	driveSpaceVal   string
	driveSpaceUntil time.Time
)

func driveSpaceID(unionID string) (string, error) {
	driveSpaceMu.Lock()
	defer driveSpaceMu.Unlock()
	// 1) 优先用配置里手工指定的 space_id（企业自定义空间等）；配置固定 → 可缓存
	if cfg.DriveSpaceID != "" {
		if driveSpaceVal != "" && time.Now().Before(driveSpaceUntil) {
			return driveSpaceVal, nil
		}
		driveSpaceVal = cfg.DriveSpaceID
		driveSpaceUntil = time.Now().Add(10 * time.Minute)
		return driveSpaceVal, nil
	}
	// 2) 兜底：上传人自己的钉盘空间（mySpaces，按操作人不同，不缓存）
	status, m, err := driveAPI("GET", "/v1.0/drive/mySpaces",
		url.Values{"unionId": {unionID}}, nil)
	if err != nil {
		return "", err
	}
	if status != 200 {
		if hint := permissionHint(m); hint != "" {
			return "", fmt.Errorf("%s（详情: %s）", hint, rStr(m, "message"))
		}
		b, _ := json.Marshal(m)
		return "", fmt.Errorf("获取钉盘空间失败: %s", string(b))
	}
	pick := func(sp map[string]any) string {
		if id := rStr(sp, "spaceId"); id != "" {
			log.Printf("已选定钉盘空间: %s (%s, type=%s)", rStr(sp, "spaceName"), id, rStr(sp, "spaceType"))
			return id
		}
		return ""
	}
	// 返回可能是单对象或列表
	if id := pick(m); id != "" {
		return id, nil
	}
	if list, ok := m["result"].([]any); ok {
		for _, item := range list {
			if sp, ok := item.(map[string]any); ok {
				if id := pick(sp); id != "" {
					return id, nil
				}
			}
		}
	}
	if list, ok := m["spaces"].([]any); ok {
		for _, item := range list {
			if sp, ok := item.(map[string]any); ok {
				if id := pick(sp); id != "" {
					return id, nil
				}
			}
		}
	}
	return "", fmt.Errorf("未找到可用的钉盘空间，可在 config.json 的 drive_space_id 中指定企业自定义空间 ID")
}

// ---------------- 上传（获取凭证 → OSS 直传 → 挂载） ----------------

// driveUpload 把文件传到钉盘，返回 (spaceId, fileId, error)
func driveUpload(operatorUserid, fileName string, size int64, content []byte) (string, string, error) {
	unionID, err := driveUnionID(operatorUserid)
	if err != nil {
		return "", "", err
	}
	spaceID, err := driveSpaceID(unionID)
	if err != nil {
		return "", "", err
	}

	// 1) 获取上传凭证（parentId=0 表示空间根目录，HEADER_SIGNATURE = 头部签名直传）
	status, m, err := driveAPI("POST", "/v1.0/storage/spaces/"+spaceID+"/files/uploadInfos/query",
		url.Values{"unionId": {unionID}},
		map[string]any{
			"name":      fileName,
			"size":      size,
			"parentId":  "0",
			"protocol":  "HEADER_SIGNATURE",
			"multipart": false,
		})
	if err != nil {
		return "", "", fmt.Errorf("获取上传凭证失败: %w", err)
	}
	if status != 200 {
		if hint := permissionHint(m); hint != "" {
			return "", "", fmt.Errorf("%s（详情: %s）", hint, rStr(m, "message"))
		}
		b, _ := json.Marshal(m)
		return "", "", fmt.Errorf("获取上传凭证失败: %s", string(b))
	}
	uploadKey := rStr(m, "uploadKey")
	sig, _ := m["headerSignatureInfo"].(map[string]any)
	if uploadKey == "" || sig == nil {
		b, _ := json.Marshal(m)
		return "", "", fmt.Errorf("上传凭证返回异常: %s", string(b))
	}
	uploadURL := firstString(sig, "resourceUrls")
	if uploadURL == "" {
		uploadURL = rStr(sig, "resourceUrl")
	}
	if uploadURL == "" {
		uploadURL = firstString(m, "resourceUrls")
	}
	headers, _ := sig["headers"].(map[string]any)

	// 2) OSS 带签直传
	req, err := http.NewRequest("PUT", uploadURL, bytes.NewReader(content))
	if err != nil {
		return "", "", err
	}
	for k, v := range headers {
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Host") {
			continue // Go 自动计算，签名头部多余的会被服务端忽略
		}
		if s, ok := v.(string); ok {
			req.Header.Set(k, s)
		}
	}
	resp, err := driveClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("文件直传失败: %w", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("文件直传失败: OSS 返回 %d", resp.StatusCode)
	}

	// 3) 提交（挂载）到空间根目录，同名自动改名避免覆盖
	status, m, err = driveAPI("POST", "/v1.0/storage/spaces/"+spaceID+"/files/commit",
		url.Values{"unionId": {unionID}},
		map[string]any{
			"name":      fileName,
			"parentId":  "0",
			"size":      size,
			"uploadKey": uploadKey,
			"option":    map[string]any{"conflictStrategy": "AUTO_RENAME"},
		})
	if err != nil {
		return "", "", err
	}
	if status != 200 {
		if hint := permissionHint(m); hint != "" {
			return "", "", fmt.Errorf("%s（详情: %s）", hint, rStr(m, "message"))
		}
		b, _ := json.Marshal(m)
		return "", "", fmt.Errorf("钉盘提交文件失败: %s", string(b))
	}
	// 优先取 dentry.id（数字型文件 ID，下载接口要求数字 ID）
	if d, ok := m["dentry"].(map[string]any); ok {
		if id := rStr(d, "id"); id != "" {
			return spaceID, id, nil
		}
	}
	for _, k := range []string{"dentryId", "id"} {
		if id := rStr(m, k); id != "" {
			return spaceID, id, nil
		}
	}
	b, _ := json.Marshal(m)
	return "", "", fmt.Errorf("钉盘提交文件返回异常: %s", string(b))
}

// firstString 从 map 的字符串数组字段里取第一个非空值
func firstString(m map[string]any, key string) string {
	list, _ := m[key].([]any)
	for _, item := range list {
		if s, ok := item.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// driveDownloadURL 取文件临时下载地址（含签名请求头，30 分钟有效）
func driveDownloadURL(operatorUserid, spaceID, fileID string) (string, map[string]string, error) {
	unionID, err := driveUnionID(operatorUserid)
	if err != nil {
		return "", nil, err
	}
	status, m, err := driveAPI("POST",
		"/v1.0/storage/spaces/"+spaceID+"/dentries/"+fileID+"/downloadInfos/query",
		url.Values{"unionId": {unionID}}, map[string]any{})
	if err != nil {
		return "", nil, err
	}
	if status != 200 {
		if hint := permissionHint(m); hint != "" {
			return "", nil, fmt.Errorf("%s（详情: %s）", hint, rStr(m, "message"))
		}
		b, _ := json.Marshal(m)
		return "", nil, fmt.Errorf("获取下载链接失败: %s", string(b))
	}
	var hdrs map[string]string
	collect := func(sig map[string]any) string {
		if hs, ok := sig["headers"].(map[string]any); ok {
			hdrs = map[string]string{}
			for k, v := range hs {
				if s, ok := v.(string); ok {
					hdrs[k] = s
				}
			}
		}
		if u := firstString(sig, "resourceUrls"); u != "" {
			return u
		}
		return rStr(sig, "resourceUrl")
	}
	if sig, ok := m["headerSignatureInfo"].(map[string]any); ok {
		if u := collect(sig); u != "" {
			return u, hdrs, nil
		}
	}
	if sig, ok := m["downloadInfo"].(map[string]any); ok {
		if u := collect(sig); u != "" {
			return u, hdrs, nil
		}
	}
	if u := firstString(m, "resourceUrls"); u != "" {
		return u, nil, nil
	}
	if u := rStr(m, "resourceUrl"); u != "" {
		return u, nil, nil
	}
	return "", nil, fmt.Errorf("下载链接返回异常")
}

// pushFileToDrive 后台把已本地留档的附件推送到钉盘，并回写状态
func pushFileToDrive(fileID, reqID int, fileName, operatorUserid string, content []byte) {
	spaceID, driveFID, err := driveUpload(operatorUserid, fileName, int64(len(content)), content)
	if err != nil {
		updateFileDriveStatus(fileID, "failed", "", "", err.Error())
		log.Printf("附件钉盘上传失败(需求 %d, %s): %s", reqID, fileName, err.Error())
		return
	}
	updateFileDriveStatus(fileID, "uploaded", spaceID, driveFID, "")
	log.Printf("附件已同步钉盘: 需求 %d ← %s（%d 字节）", reqID, fileName, len(content))
}

// retryFileToDrive 重新把本地副本推送到钉盘（权限补齐后补救用）
func retryFileToDrive(fileID int, localPath, fileName, operatorUserid string) (string, string, error) {
	content, err := os.ReadFile(localPath)
	if err != nil {
		return "", "", fmt.Errorf("本地文件不可用: %w", err)
	}
	return driveUpload(operatorUserid, fileName, int64(len(content)), content)
}

// ---------------- 本地存储 ----------------

func uploadsDir() string {
	if cfg.UploadsDir != "" {
		return cfg.UploadsDir
	}
	return "uploads"
}

// saveLocal 保存到本地 uploads 目录，返回相对路径
func saveLocal(reqID int, fileName string, content []byte) (string, error) {
	dir := uploadsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	safe := sanitizeFileName(fileName)
	name := fmt.Sprintf("%d_%d_%s", reqID, time.Now().UnixMilli()%100000, safe)
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		return "", err
	}
	return p, nil
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		name = "attachment"
	}
	return name
}

var uploadContentTypes = map[string]string{
	".pdf": "application/pdf", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".webp": "image/webp", ".txt": "text/plain; charset=utf-8", ".csv": "text/csv",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".zip":  "application/zip", ".rar": "application/vnd.rar", ".7z": "application/x-7z-compressed",
}

func localContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ct, ok := uploadContentTypes[ext]; ok {
		return ct
	}
	return mime.TypeByExtension(ext)
}

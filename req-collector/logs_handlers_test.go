package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestLogFile 把全局 fileLog 指向临时目录，返回清理函数
func setupTestLogFile(t *testing.T) func() {
	t.Helper()
	dir := t.TempDir()
	w, err := NewRotatingFileWriter(filepath.Join(dir, "server.log"), 20, 30)
	if err != nil {
		t.Fatal(err)
	}
	prev := fileLog
	fileLog = w
	// 造两个归档文件
	_ = os.WriteFile(filepath.Join(dir, "server-2026-10-11.log"), []byte("old a\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "server-2026-10-12.log"), []byte("old b\n"), 0644)
	return func() {
		fileLog = prev
		_ = w.Close()
	}
}

func TestAdminLogsFiles(t *testing.T) {
	cleanup := setupTestLogFile(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	hAdminLogsFiles(rec, httptest.NewRequest("GET", "/api/admin/logs/files", nil))
	if rec.Code != 200 {
		t.Fatalf("状态码 = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		OK      bool `json:"ok"`
		Enabled bool `json:"enabled"`
		Path    string
		Files   []struct {
			Name   string `json:"name"`
			Active bool   `json:"active"`
			Size   int64  `json:"size"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK || !resp.Enabled {
		t.Fatalf("响应异常: %s", rec.Body.String())
	}
	if len(resp.Files) != 3 {
		t.Fatalf("期望 3 个日志文件（1 活动 + 2 归档），实际 %d: %+v", len(resp.Files), resp.Files)
	}
	// 第一个必须是活动文件
	if resp.Files[0].Name != "server.log" || !resp.Files[0].Active {
		t.Fatalf("首个应为活动文件 server.log，实际 %+v", resp.Files[0])
	}
	// 归档按时间倒序
	if resp.Files[1].Name != "server-2026-10-12.log" || resp.Files[2].Name != "server-2026-10-11.log" {
		t.Fatalf("归档排序错误: %+v", resp.Files)
	}
}

func TestAdminLogsDownloadActive(t *testing.T) {
	cleanup := setupTestLogFile(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	hAdminLogsDownload(rec, httptest.NewRequest("GET", "/api/admin/logs/download", nil))
	if rec.Code != 200 {
		t.Fatalf("状态码 = %d, body=%s", rec.Code, rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "server.log") {
		t.Fatalf("Content-Disposition 错误: %q", cd)
	}
}

func TestAdminLogsDownloadArchive(t *testing.T) {
	cleanup := setupTestLogFile(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	hAdminLogsDownload(rec, httptest.NewRequest("GET", "/api/admin/logs/download?file=server-2026-10-12.log", nil))
	if rec.Code != 200 {
		t.Fatalf("状态码 = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "old b") {
		t.Fatalf("归档内容错误: %q", rec.Body.String())
	}
}

func TestAdminLogsDownloadRejectsTraversal(t *testing.T) {
	cleanup := setupTestLogFile(t)
	defer cleanup()

	for _, bad := range []string{
		"../config.json",
		"/etc/passwd",
		"server.log",               // 活动文件只能不带 file 参数下载
		"other-2026-10-12.log",     // 前缀不符
		"server-2026-10-12.txt",    // 后缀不符
		"sub/server-2026-10-1.log", // 带路径分隔符
	} {
		rec := httptest.NewRecorder()
		hAdminLogsDownload(rec, httptest.NewRequest("GET", "/api/admin/logs/download?file="+bad, nil))
		if rec.Code == 200 {
			t.Fatalf("非法文件名 %q 不应返回 200，body=%s", bad, rec.Body.String())
		}
	}
}

func TestAdminLogsIncludesFilePath(t *testing.T) {
	cleanup := setupTestLogFile(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	hAdminLogs(rec, httptest.NewRequest("GET", "/api/admin/logs", nil))
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if p, _ := resp["file"].(string); !strings.HasSuffix(p, "server.log") {
		t.Fatalf("响应应包含日志文件路径，实际: %v", resp)
	}
	_ = http.StatusOK
}

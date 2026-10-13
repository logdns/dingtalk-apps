package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRotateBySize(t *testing.T) {
	dir := t.TempDir()
	w, err := NewRotatingFileWriter(filepath.Join(dir, "server.log"), 1, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	// maxBytes=1MB；写入 1.2MB 触发一次轮转
	chunk := []byte(strings.Repeat("x", 1024) + "\n")
	for i := 0; i < 1200; i++ {
		if _, err := w.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	var archives int
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "server-") {
			archives++
		}
	}
	if archives == 0 {
		t.Fatalf("期望产生归档文件，实际没有；目录: %v", entries)
	}
	if !fileExists(filepath.Join(dir, "server.log")) {
		t.Fatal("活动日志文件应始终存在")
	}
}

func TestRotateByDay(t *testing.T) {
	dir := t.TempDir()
	w, err := NewRotatingFileWriter(filepath.Join(dir, "server.log"), 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err := w.Write([]byte("first\n")); err != nil {
		t.Fatal(err)
	}
	w.day = "2000-01-01" // 伪造跨天
	if _, err := w.Write([]byte("second\n")); err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(dir, "server-2000-01-01.log")) {
		t.Fatal("跨天应生成 server-2000-01-01.log")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "server.log"))
	if string(data) != "second\n" {
		t.Fatalf("活动文件内容错误: %q", data)
	}
}

func TestCleanupOldArchives(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "server-2020-01-01.log")
	if err := os.WriteFile(old, []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().AddDate(0, 0, -10)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	w, err := NewRotatingFileWriter(filepath.Join(dir, "server.log"), 20, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if fileExists(old) {
		t.Fatal("超过保留期的归档应被清理")
	}
}

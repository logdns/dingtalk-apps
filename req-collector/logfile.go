package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RotatingFileWriter 日志落盘：按天 / 按大小轮转，自动清理过期归档
//
// 活动文件始终是 path（如 server.log），轮转时改名为
// server-2026-10-13.log（同一天多次轮转则 server-2026-10-13.1.log …）。
// 所有写入无缓冲，进程异常退出也不会丢日志。
type RotatingFileWriter struct {
	mu         sync.Mutex
	path       string // 活动日志文件路径
	maxBytes   int64  // 单文件上限（字节），<=0 表示不按大小轮转
	retainDays int    // 归档保留天数，<=0 表示不清理
	file       *os.File
	size       int64
	day        string // 当前文件所属日期 YYYY-MM-DD
}

// NewRotatingFileWriter 创建并打开日志文件（自动建目录）
func NewRotatingFileWriter(path string, maxMB, retainDays int) (*RotatingFileWriter, error) {
	if maxMB <= 0 {
		maxMB = 20
	}
	if retainDays <= 0 {
		retainDays = 30
	}
	w := &RotatingFileWriter{
		path:       path,
		maxBytes:   int64(maxMB) * 1024 * 1024,
		retainDays: retainDays,
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录 %s: %w", dir, err)
		}
	}
	if err := w.open(); err != nil {
		return nil, err
	}
	w.cleanup()
	return w, nil
}

func (w *RotatingFileWriter) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file = f
	w.size = st.Size()
	w.day = time.Now().Format("2006-01-02")
	return nil
}

// Write 实现 io.Writer；必要时先轮转
func (w *RotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		// 文件不可用时不阻塞业务，直接吞掉
		return len(p), nil
	}
	day := time.Now().Format("2006-01-02")
	if day != w.day || (w.maxBytes > 0 && w.size+int64(len(p)) > w.maxBytes) {
		w.rotate()
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// rotate 关闭当前文件 → 改名归档 → 重新打开活动文件 → 清理过期归档
func (w *RotatingFileWriter) rotate() {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	if fileExists(w.path) {
		ext := filepath.Ext(w.path)
		base := strings.TrimSuffix(w.path, ext)
		target := fmt.Sprintf("%s-%s%s", base, w.day, ext)
		for i := 1; fileExists(target); i++ {
			target = fmt.Sprintf("%s-%s.%d%s", base, w.day, i, ext)
		}
		_ = os.Rename(w.path, target)
	}
	if err := w.open(); err != nil {
		fmt.Fprintf(os.Stderr, "日志文件打开失败 %s: %v\n", w.path, err)
	}
	w.cleanup()
}

// cleanup 删除超过保留期的归档日志（不含活动文件）
func (w *RotatingFileWriter) cleanup() {
	if w.retainDays <= 0 {
		return
	}
	dir := filepath.Dir(w.path)
	base := strings.TrimSuffix(filepath.Base(w.path), filepath.Ext(w.path))
	ext := filepath.Ext(w.path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -w.retainDays)
	var archives []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == filepath.Base(w.path) {
			continue
		}
		if strings.HasPrefix(name, base+"-") && strings.HasSuffix(name, ext) {
			archives = append(archives, name)
		}
	}
	sort.Strings(archives)
	for _, name := range archives {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && st.ModTime().Before(cutoff) {
			_ = os.Remove(p)
		}
	}
}

// Close 关闭活动文件
func (w *RotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

// FilePath 返回活动日志文件路径
func (w *RotatingFileWriter) FilePath() string { return w.path }

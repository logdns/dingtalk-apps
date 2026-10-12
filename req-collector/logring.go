package main

import (
	"strings"
	"sync"
)

// LogRing 内存日志环形缓冲（供管理面板在线查看）
type LogRing struct {
	mu      sync.Mutex
	buf     []string
	partial string
	max     int
}

func NewLogRing(max int) *LogRing {
	return &LogRing{max: max, buf: make([]string, 0, max)}
}

// Write 实现 io.Writer，按行切分存入环形缓冲
func (l *LogRing) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.partial += string(p)
	for {
		i := strings.Index(l.partial, "\n")
		if i < 0 {
			break
		}
		line := strings.TrimRight(l.partial[:i], "\r")
		if line != "" {
			l.buf = append(l.buf, line)
			if len(l.buf) > l.max {
				l.buf = l.buf[len(l.buf)-l.max:]
			}
		}
		l.partial = l.partial[i+1:]
	}
	return len(p), nil
}

// Lines 返回最近 n 行（时间正序）
func (l *LogRing) Lines(n int) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n <= 0 || n > len(l.buf) {
		n = len(l.buf)
	}
	out := make([]string, n)
	copy(out, l.buf[len(l.buf)-n:])
	return out
}

func (l *LogRing) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf = l.buf[:0]
}

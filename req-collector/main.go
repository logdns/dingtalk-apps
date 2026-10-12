// 钉钉需求收集应用服务端（Go 版）
// 配置 → 数据库 → 钉钉 → HTTP 服务
//
// 单二进制：前端（React + Arco Design）已 embed 进可执行文件，
// 部署只需「1 个二进制 + 1 个 config.json」。
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"
)

// fatal 出错退出；Windows 下（双击运行）暂停等回车，让用户能看到错误
func fatal(format string, v ...any) {
	log.Printf(format, v...)
	if runtime.GOOS == "windows" {
		fmt.Println("\n发生错误，按回车键退出...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	os.Exit(1)
}

// configTemplate 缺少配置文件时自动生成的模板
const configTemplate = `{
  "host": "0.0.0.0",
  "port": 1450,
  "base_url": "http://服务器IP:1450",
  "app_title": "需求收集",
  "mysql": {
    "host": "127.0.0.1",
    "port": 3306,
    "user": "root",
    "password": "改成数据库密码",
    "database": "dingtalk_apps"
  },
  "dingtalk": {
    "app_key": "改成AppKey",
    "app_secret": "改成AppSecret",
    "agent_id": "改成AgentId",
    "corp_id": "改成CorpId"
  },
  "admin_password": "改成管理面板初始密码",
  "session_days": 30,
  "options": {
    "departments": ["研发部", "产品部", "销售部", "财务部", "人事部", "其他"],
    "req_types": ["功能需求", "字段需求", "报表需求", "流程优化", "接口集成", "其他"],
    "urgencies": ["紧急", "高", "中", "低"],
    "bills": [],
    "feasibilities": ["待评估", "可实现", "需二开", "不可实现"]
  }
}
`

var appLog = NewLogRing(2000) // 管理面板在线日志（最近 2000 行）

func main() {
	log.SetOutput(io.MultiWriter(os.Stderr, appLog))
	configPath := flag.String("config", "config.json", "配置文件路径")
	showVersion := flag.Bool("version", false, "打印版本号")
	flag.Parse()

	if *showVersion {
		fmt.Println(AppVersion)
		return
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		// 配置文件不存在 → 自动生成模板，提示编辑后重启
		if errors.Is(err, os.ErrNotExist) {
			// 0600：配置文件含数据库口令与钉钉密钥，避免同机其他用户读取
			_ = os.WriteFile(*configPath, []byte(configTemplate), 0600)
			fatal("未找到配置文件，已在同目录生成模板 %s\n请用记事本打开，填好 MySQL 和钉钉信息后保存，再重新启动本程序。", *configPath)
		}
		fatal("加载配置失败: %v", err)
	}

	if err := InitStore(cfg); err != nil {
		fatal("初始化数据库失败: %v", err)
	}
	log.Printf("数据库连接成功: %s:%d/%s", cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database)

	InitDingTalk(cfg)
	if cfg.DingTalk.CorpID != "" {
		go WarmupDirectory() // 后台预热通讯录缓存
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           secureHeaders(limitBody(NewRouter())),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Printf("%s 服务 %s 已启动: http://%s:%d (监听 %s)", AppTitleOrDefault(), AppVersion, cfg.Host, cfg.Port, addr)
	if err := srv.ListenAndServe(); err != nil {
		fatal("服务启动失败: %v", err)
	}
}

// limitBody 限制请求体大小（防超大 body 打爆内存）
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
		}
		next.ServeHTTP(w, r)
	})
}

// secureHeaders 基本安全响应头（缓解 XSS / 嗅探 / 点击劫持）
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

// JSON 响应辅助
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func okJSON(w http.ResponseWriter, extra map[string]any) {
	m := map[string]any{"ok": true}
	for k, v := range extra {
		m[k] = v
	}
	writeJSON(w, http.StatusOK, m)
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": msg})
}

// 读请求体 JSON
func readJSON(r *http.Request) map[string]any {
	var m map[string]any
	_ = json.NewDecoder(r.Body).Decode(&m)
	if m == nil {
		m = map[string]any{}
	}
	return m
}

func str(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}

func strTrim(m map[string]any, key string) string { return trimSpace(str(m, key)) }

func intVal(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

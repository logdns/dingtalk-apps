package main

import (
	"encoding/json"
	"fmt"
	"os"
)

const AppVersion = "1.2.0"

// 全局配置由 store.InitStore 赋值（见 store.go 的 cfg），页面标题/下拉选项等都从它读取

// Options 页面下拉选项，全部可在 config.json 中覆盖，留空则用下面的默认值
type Options struct {
	Departments   []string `json:"departments"`   // 部门（留空则由钉钉免登自动带出）
	ReqTypes      []string `json:"req_types"`     // 需求类型
	Urgencies     []string `json:"urgencies"`     // 紧急程度
	Bills         []string `json:"bills"`         // 相关单据/模块（留空则自由输入）
	Feasibilities []string `json:"feasibilities"` // 能否实现（管理员判定用）
}

// 通用默认值（与具体业务无关）
var defaultOptions = Options{
	Departments:   []string{"研发部", "产品部", "销售部", "财务部", "人事部", "其他"},
	ReqTypes:      []string{"功能需求", "字段需求", "报表需求", "流程优化", "接口集成", "其他"},
	Urgencies:     []string{"紧急", "高", "中", "低"},
	Bills:         []string{},
	Feasibilities: []string{"待评估", "可实现", "需二开", "不可实现"},
}

// Config 服务端配置（config.json）
type Config struct {
	Host          string   `json:"host"`      // 监听地址，0.0.0.0 = 所有网卡
	Port          int      `json:"port"`      // 监听端口
	BaseURL       string   `json:"base_url"`  // 对外访问地址（钉钉工作台首页地址需与此一致，通知卡片按钮跳转用）
	AppTitle      string   `json:"app_title"` // 应用标题（页面标题、通知标题，默认「需求收集」）
	MySQL         MySQLCfg `json:"mysql"`
	DingTalk      DingCfg  `json:"dingtalk"`
	AdminPassword string   `json:"admin_password"`  // 管理面板初始密码（数据库已有 kr1$ 格式密码时不再覆盖）
	SessionDays   int      `json:"session_days"`    // 登录令牌有效期（天）
	UploadsDir    string   `json:"uploads_dir"`     // 附件本地保存目录（默认 ./uploads）
	DriveSpaceID  string   `json:"drive_space_id"`  // 可选：钉盘空间 ID（留空自动取上传人自己的钉盘空间）
	MaxUploadMB   int      `json:"max_upload_mb"`   // 单个附件上限（MB，默认 20，封顶 100）
	LogDir        string   `json:"log_dir"`         // 日志目录（默认 ./logs）
	LogMaxMB      int      `json:"log_max_mb"`      // 单个日志文件上限（MB，默认 20）
	LogRetainDays int      `json:"log_retain_days"` // 归档日志保留天数（默认 30）
	Options       Options  `json:"options"`         // 可选：业务下拉项
}

// AppTitleOrDefault 页面/通知标题
func AppTitleOrDefault() string {
	if cfg != nil && cfg.AppTitle != "" {
		return cfg.AppTitle
	}
	return "需求收集"
}

// OptDepartments 等：未配置时回落到通用默认值
func OptDepartments() []string { return optOr(cfg.Options.Departments, defaultOptions.Departments) }
func OptReqTypes() []string    { return optOr(cfg.Options.ReqTypes, defaultOptions.ReqTypes) }
func OptUrgencies() []string   { return optOr(cfg.Options.Urgencies, defaultOptions.Urgencies) }
func OptBills() []string       { return optOr(cfg.Options.Bills, defaultOptions.Bills) }
func OptFeasibilities() []string {
	return optOr(cfg.Options.Feasibilities, defaultOptions.Feasibilities)
}

func optOr(v, def []string) []string {
	if cfg == nil || len(v) == 0 {
		return def
	}
	return v
}

type MySQLCfg struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type DingCfg struct {
	AppKey    string `json:"app_key"`
	AppSecret string `json:"app_secret"`
	AgentID   string `json:"agent_id"`
	CorpID    string `json:"corp_id"`
}

// LoadConfig 读取配置文件；session_days 等缺省值兜底
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 %s: %w", path, err)
	}
	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析 %s: %w", path, err)
	}
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	if cfg.Port == 0 {
		cfg.Port = 1450
	}
	if cfg.SessionDays <= 0 {
		cfg.SessionDays = 30
	}
	if cfg.MySQL.Port == 0 {
		cfg.MySQL.Port = 3306
	}
	if cfg.MySQL.Host == "" || cfg.MySQL.User == "" || cfg.MySQL.Database == "" {
		return nil, fmt.Errorf("config.json 缺少 mysql.host / mysql.user / mysql.database")
	}
	if cfg.MaxUploadMB <= 0 {
		cfg.MaxUploadMB = 20
	}
	if cfg.MaxUploadMB > 100 {
		cfg.MaxUploadMB = 100
	}
	if cfg.LogDir == "" {
		cfg.LogDir = "logs"
	}
	if cfg.LogMaxMB <= 0 {
		cfg.LogMaxMB = 20
	}
	if cfg.LogRetainDays <= 0 {
		cfg.LogRetainDays = 30
	}
	return cfg, nil
}

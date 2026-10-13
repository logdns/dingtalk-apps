package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB
var cfg *Config

// InitStore 打开数据库连接并自动建表
func InitStore(c *Config) error {
	cfg = c
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		c.MySQL.User, c.MySQL.Password, c.MySQL.Host, c.MySQL.Port, c.MySQL.Database)
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		return err
	}
	return migrate()
}

func migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS req_items (
			id INT NOT NULL AUTO_INCREMENT,
			department VARCHAR(50) NOT NULL,
			requester VARCHAR(50) NOT NULL,
			username VARCHAR(50) DEFAULT NULL,
			description TEXT NOT NULL,
			scenario TEXT,
			bill_name VARCHAR(100) NOT NULL,
			req_type VARCHAR(50) NOT NULL,
			urgency VARCHAR(10) NOT NULL DEFAULT '中',
			feasibility VARCHAR(20) NOT NULL DEFAULT '待评估',
			remark VARCHAR(500) DEFAULT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS req_admins (
			id INT NOT NULL AUTO_INCREMENT,
			userid VARCHAR(64) NOT NULL,
			name VARCHAR(50) NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uk_req_admins_userid (userid)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS req_settings (
			k VARCHAR(50) NOT NULL,
			v TEXT NOT NULL,
			PRIMARY KEY (k)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS req_files (
			id INT NOT NULL AUTO_INCREMENT,
			req_id INT NOT NULL,
			file_name VARCHAR(255) NOT NULL,
			file_size BIGINT NOT NULL DEFAULT 0,
			local_path VARCHAR(500) DEFAULT '',
			drive_space_id VARCHAR(64) DEFAULT '',
			drive_file_id VARCHAR(64) DEFAULT '',
			drive_status VARCHAR(20) NOT NULL DEFAULT 'pending',
			drive_message VARCHAR(500) DEFAULT '',
			uploaded_by VARCHAR(64) DEFAULT '',
			uploaded_by_name VARCHAR(50) DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			KEY idx_req_files_req (req_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS app_sessions (
			token VARCHAR(64) NOT NULL,
			userid VARCHAR(64) DEFAULT NULL,
			name VARCHAR(50) DEFAULT NULL,
			department VARCHAR(120) DEFAULT NULL,
			panel_admin TINYINT NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME DEFAULT NULL,
			PRIMARY KEY (token)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("建表失败: %w", err)
		}
	}
	// 老库平滑升级：附件记录新增失败原因字段
	ensureColumn("req_files", "drive_message", "VARCHAR(500) DEFAULT ''")
	// 管理面板密码初始化/兼容
	return initAdminPassword()
}

// ---------- 管理面板密码（kr1$ 格式 = sha256(salt+password)） ----------

func hashPassword(pw string) string {
	salt := randHex(8)
	h := sha256.Sum256([]byte(salt + pw))
	return "kr1$" + salt + "$" + hex.EncodeToString(h[:])
}

func checkPassword(stored, pw string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 3 || parts[0] != "kr1" {
		return false
	}
	h := sha256.Sum256([]byte(parts[1] + pw))
	return hex.EncodeToString(h[:]) == parts[2]
}

// resolveAdminPassword 决定初始管理密码：
//   - config.json 配了 admin_password → 用它；
//   - 没配 → 随机生成 16 位强密码，只打印到启动控制台（stderr），
//     绝不写进运行日志（管理面板的「运行日志」页签会展示日志缓冲）。
func resolveAdminPassword() (pw string, generated bool) {
	if cfg != nil && cfg.AdminPassword != "" {
		return cfg.AdminPassword, false
	}
	return randHex(8), true // 16 个十六进制字符
}

func initAdminPassword() error {
	var stored sql.NullString
	err := db.QueryRow("SELECT v FROM req_settings WHERE k='admin_password'").Scan(&stored)
	if err == sql.ErrNoRows {
		pw, generated := resolveAdminPassword()
		_, err = db.Exec("INSERT INTO req_settings (k,v) VALUES ('admin_password',?)", hashPassword(pw))
		if generated {
			fmt.Fprintf(os.Stderr, "\n===== 重要 =====\n管理面板初始密码已随机生成：%s\n该密码只显示这一次，且不会出现在运行日志中，请立即登录 /admin 并修改。\n================\n\n", pw)
			log.Printf("已随机生成管理面板初始密码（仅打印在启动控制台），请尽快修改")
		} else {
			log.Printf("已按 config.json 的 admin_password 初始化管理面板密码，请尽快修改")
		}
		return err
	}
	if err != nil {
		return err
	}
	// 旧版（Python/werkzeug scrypt）密码哈希无法在 Go 中校验 → 重置为配置文件密码
	if !strings.HasPrefix(stored.String, "kr1$") {
		pw, generated := resolveAdminPassword()
		_, err = db.Exec("UPDATE req_settings SET v=? WHERE k='admin_password'", hashPassword(pw))
		if generated {
			fmt.Fprintf(os.Stderr, "\n===== 重要 =====\n检测到旧版密码哈希，管理面板密码已重置为随机密码：%s\n该密码只显示这一次，请立即登录 /admin 并修改。\n================\n\n", pw)
			log.Printf("检测到旧版密码哈希，已随机重置管理面板密码（仅打印在启动控制台）")
		} else {
			log.Printf("检测到旧版密码哈希，已按 config.json 的 admin_password 重置管理面板密码")
		}
		return err
	}
	return nil
}

func getSetting(key string) (string, error) {
	var v sql.NullString
	err := db.QueryRow("SELECT v FROM req_settings WHERE k=?", key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v.String, nil
}

func setSetting(key, val string) error {
	_, err := db.Exec("INSERT INTO req_settings (k,v) VALUES (?,?) ON DUPLICATE KEY UPDATE v=VALUES(v)", key, val)
	return err
}

// ---------- 会话令牌 ----------

type Session struct {
	Token      string
	UserID     string
	Name       string
	Department string
	PanelAdmin bool
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func newToken() string { return randHex(32) }

func issueToken(sess Session) (string, error) {
	if sess.Token == "" {
		sess.Token = newToken()
	}
	_, err := db.Exec(
		"INSERT INTO app_sessions (token,userid,name,department,panel_admin,expires_at) VALUES (?,?,?,?,?,DATE_ADD(NOW(), INTERVAL ? DAY))",
		sess.Token, sess.UserID, sess.Name, sess.Department, boolInt(sess.PanelAdmin), cfg.SessionDays)
	if err != nil {
		return "", err
	}
	return sess.Token, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// tokenRow 取会话（过期自动删除），未登录返回 nil
func tokenRow(r *http.Request) *Session {
	t := bearerToken(r)
	if t == "" {
		return nil
	}
	var s Session
	var expires sql.NullTime
	var userid, name, dept sql.NullString
	var panel int
	err := db.QueryRow("SELECT token,userid,name,department,panel_admin,expires_at FROM app_sessions WHERE token=?", t).
		Scan(&s.Token, &userid, &name, &dept, &panel, &expires)
	if err != nil {
		return nil
	}
	if expires.Valid && time.Now().After(expires.Time) {
		_, _ = db.Exec("DELETE FROM app_sessions WHERE token=?", t)
		return nil
	}
	s.UserID, s.Name, s.Department = userid.String, name.String, dept.String
	s.PanelAdmin = panel == 1
	return &s
}

func dropToken(token string) {
	if token != "" {
		_, _ = db.Exec("DELETE FROM app_sessions WHERE token=?", token)
	}
}

func bearerToken(r *http.Request) string {
	t := trimSpace(r.Header.Get("X-Auth-Token"))
	if t != "" {
		return t
	}
	return trimSpace(r.URL.Query().Get("token"))
}

// currentUser 当前钉钉用户；未登录返回 nil
func currentUser(r *http.Request) *Session {
	s := tokenRow(r)
	if s != nil && s.UserID != "" {
		return s
	}
	return nil
}

// isAdminSession 管理员 = 面板管理令牌 或 钉钉用户在 req_admins 名单
func isAdminSession(r *http.Request) bool {
	s := tokenRow(r)
	if s == nil {
		return false
	}
	if s.PanelAdmin {
		return true
	}
	if s.UserID == "" {
		return false
	}
	var one string
	err := db.QueryRow("SELECT 1 FROM req_admins WHERE userid=? LIMIT 1", s.UserID).Scan(&one)
	return err == nil
}

// rowOwnerOrAdmin 返回 (row存在的username, 是否管理员, 是否有权限)
func rowOwnerOrAdmin(r *http.Request, rid int) (string, bool, bool) {
	admin := isAdminSession(r)
	ddUser := currentUser(r)
	if !admin && ddUser == nil {
		return "", false, false
	}
	var username sql.NullString
	err := db.QueryRow("SELECT username FROM req_items WHERE id=?", rid).Scan(&username)
	if err != nil {
		return "", admin, false
	}
	if admin {
		return username.String, true, true
	}
	if ddUser != nil && username.String == ddUser.UserID {
		return username.String, false, true
	}
	return "", false, false
}

// ---------- 需求附件 ----------

type ReqFile struct {
	ID             int    `json:"id"`
	ReqID          int    `json:"req_id"`
	FileName       string `json:"file_name"`
	FileSize       int64  `json:"file_size"`
	LocalPath      string `json:"-"`
	DriveSpaceID   string `json:"-"`
	DriveFileID    string `json:"-"`
	DriveStatus    string `json:"drive_status"`
	UploadedBy     string `json:"uploaded_by"`
	UploadedByName string `json:"uploaded_by_name"`
	CreatedAt      string `json:"created_at"`
	// 前端展示用：本地副本是否缺失、是否可从钉盘/本地取到
	LocalMissing bool `json:"local_missing"`
	Available    bool `json:"available"`
}

func listFilesByReq(reqID int) []ReqFile {
	rows, err := db.Query(
		`SELECT id, req_id, file_name, file_size, local_path, drive_space_id, drive_file_id,
		        drive_status, uploaded_by, uploaded_by_name, created_at
		 FROM req_files WHERE req_id=? ORDER BY id`, reqID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []ReqFile
	for rows.Next() {
		var f ReqFile
		var lp, ds, df, ub sql.NullString
		var createdAt time.Time
		if err := rows.Scan(&f.ID, &f.ReqID, &f.FileName, &f.FileSize, &lp, &ds, &df,
			&f.DriveStatus, &ub, &f.UploadedByName, &createdAt); err != nil {
			continue
		}
		f.LocalPath, f.DriveSpaceID, f.DriveFileID, f.UploadedBy = lp.String, ds.String, df.String, ub.String
		f.CreatedAt = createdAt.Format("2006-01-02 15:04")
		f.LocalMissing = f.LocalPath == "" || !fileExists(f.LocalPath)
		f.Available = !f.LocalMissing || (f.DriveSpaceID != "" && f.DriveFileID != "")
		out = append(out, f)
	}
	return out
}

func fileByID(fid int) *ReqFile {
	var f ReqFile
	var lp, ds, df, ub sql.NullString
	var createdAt time.Time
	err := db.QueryRow(
		`SELECT id, req_id, file_name, file_size, local_path, drive_space_id, drive_file_id,
		        drive_status, uploaded_by, uploaded_by_name, created_at
		 FROM req_files WHERE id=?`, fid).
		Scan(&f.ID, &f.ReqID, &f.FileName, &f.FileSize, &lp, &ds, &df,
			&f.DriveStatus, &ub, &f.UploadedByName, &createdAt)
	if err != nil {
		return nil
	}
	f.LocalPath, f.DriveSpaceID, f.DriveFileID, f.UploadedBy = lp.String, ds.String, df.String, ub.String
	f.CreatedAt = createdAt.Format("2006-01-02 15:04")
	f.LocalMissing = f.LocalPath == "" || !fileExists(f.LocalPath)
	f.Available = !f.LocalMissing || (f.DriveSpaceID != "" && f.DriveFileID != "")
	return &f
}

func updateFileDriveStatus(fid int, status, spaceID, fileID, message string) {
	_, _ = db.Exec("UPDATE req_files SET drive_status=?, drive_space_id=?, drive_file_id=?, drive_message=? WHERE id=?",
		status, spaceID, fileID, message, fid)
}

// ensureColumn 老库平滑升级：列不存在时自动补上
func ensureColumn(table, column, ddl string) {
	var cnt int
	_ = db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?`, table, column).Scan(&cnt)
	if cnt == 0 {
		if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, ddl)); err == nil {
			log.Printf("已为表 %s 补充列 %s", table, column)
		}
	}
}

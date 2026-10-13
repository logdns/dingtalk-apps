# 钉钉需求收集 · Req Collector

企业内部的需求收集应用：员工在钉钉工作台里打开就能提需求（自动带出姓名/部门），管理员在后台评审并判定「能否实现」，结论以钉钉工作通知卡片回推给提交人。

**单二进制部署**：前端（React 18 + Arco Design）已编译并 `embed` 进可执行文件，目标机器只需「1 个二进制 + 1 个 config.json」，不需要 Node、不需要额外的静态资源目录。

- 版本：1.1.0
- 依赖：仅 MySQL 5.7+ / 8.0（数据表自动创建）
- 平台：Linux amd64/arm64、macOS arm64、Windows amd64
- 许可：MIT

---

## 一、它解决什么问题

```
员工（钉钉内）          管理员（后台）                钉钉通知
   │                        │                          │
   ├─ 打开应用，免登 ──────►│                          │
   ├─ 填需求（部门/姓名自动带出）                       │
   ├─ 提交 ─────────────────────────────────────────► 管理员收到卡片通知
   │                        ├─ 后台评审，判定「能否实现」 │
   │  ◄───────────────────────── 提交人收到结果卡片 ────┤
   └─ 随时在 /list 查看自己的需求进展
```

- **免登**：钉钉内打开自动识别身份，姓名与部门自动填好，不需要注册账号。
- **卡片通知**：提交时通知管理员，评审结论通知提交人，卡片带按钮可直接跳到管理面板 / 需求列表。
- **权限**：提交人只能改自己提交的需求；管理员可增删改全部需求、授权其他管理员、查看运行日志。
- **业务可配**：部门、需求类型、紧急程度、判定结论等下拉项都在 `config.json` 里，换一套业务不用改代码。

---

## 二、快速开始

### 1. 准备数据库

```sql
CREATE DATABASE dingtalk_apps DEFAULT CHARSET utf8mb4;
```

首次启动会自动建表（`req_items`、`req_admins`、`req_settings`、`app_sessions`）。

### 2. 配置

```bash
cp config.example.json config.json
chmod 600 config.json     # 含数据库口令与钉钉密钥
vi config.json
```

最小必填：`mysql.*` 四项与 `dingtalk.*` 四项；`base_url` 要填成实际的对外访问地址（钉钉工作台首页地址必须与它一致，否则卡片按钮跳错）。

### 3. 启动

```bash
go build -o dingtalk-req-collector .          # 或 ./scripts/build_release.sh 打全平台包
./dingtalk-req-collector -config config.json
```

访问：

| 路径 | 说明 |
|---|---|
| `/` | 填写需求（员工入口） |
| `/list` | 需求列表 |
| `/admin` | 管理面板（密码登录） |

> 首次启动若 `config.json` 未填 `admin_password`，会**随机生成**管理面板密码并打印在启动控制台（只显示一次，不会写进运行日志）。若你在配置文件里指定了密码，请务必改成强口令。

### 4. 钉钉后台配置

1. 钉钉开放平台 → 企业内部应用 → 创建 **H5 微应用**。
2. **应用凭证**：把 `AppKey`、`AppSecret`、`AgentId`、`CorpId` 填进 `config.json` 的 `dingtalk` 段。
3. **权限**：开通「通讯录个人信息读权限」「企业内部应用消息发送（工作通知）」等，拉取姓名/部门需要通讯录相关权限。
4. **工作台首页地址**：填 `base_url`（如 `http://192.168.1.100:1450`），与配置保持一致。
5. **服务器出口 IP 白名单**：把部署服务器的公网出口 IP 加进应用白名单，否则钉钉接口会拒签。
6. **（可选）附件上传到钉盘**：走新版**企业存储（storage）**接口，需在「权限管理 → 企业存储」开通
   `Storage.UploadInfo.Read`（上传凭证）、`Storage.File.Write`（写入文件）、`Storage.DownloadInfo.Read`（下载链接）等权限点。
   权限名在后台搜不到时，直接用钉钉报错信息里的申请链接（`open-dev.dingtalk.com/appscope/apply?content=<AppKey>%23<权限名>`）逐个开通。
   未开通时应用自动降级：附件仅保存在服务器本地（`uploads_dir`），管理面板仍可下载，运行日志会提示缺哪个权限点。
7. 发布新版本后让员工重新从工作台进入应用（钉钉 webview 会缓存静态资源，改前端后要升 `handlers.go` 的 `staticVer`）。

---

## 三、配置说明（config.json）

| 字段 | 说明 | 示例 |
|---|---|---|
| `host` | 监听地址，`0.0.0.0` = 所有网卡（前置反向代理时可改 `127.0.0.1`） | `0.0.0.0` |
| `port` | 监听端口 | `1450` |
| `base_url` | 对外访问地址（通知卡片按钮跳转用，需与钉钉工作台首页地址一致） | `http://192.168.1.100:1450` |
| `app_title` | 应用标题，用于页面标题与通知标题 | `需求收集` |
| `mysql.*` | 数据库连接（host / port / user / password / database） | — |
| `dingtalk.*` | `app_key` / `app_secret` / `agent_id` / `corp_id` | — |
| `admin_password` | 管理面板**初始**密码；留空则随机生成。数据库已有密码后该值不再生效 | — |
| `session_days` | 登录令牌有效期（天） | `30` |
| `uploads_dir` | 附件本地保存目录（相对二进制工作目录） | `uploads` |
| `drive_space_id` | 钉盘空间 ID（留空 = 自动用上传人自己的钉盘空间） | — |
| `max_upload_mb` | 单个附件上限（MB，默认 20，最大 100） | `20` |
| `options.departments` | 部门下拉项；留空则用默认值，钉钉免登时会自动带出真实部门 | — |
| `options.req_types` | 需求类型下拉项 | — |
| `options.urgencies` | 紧急程度下拉项 | — |
| `options.bills` | 相关单据/模块建议项；留空则为自由输入 | — |
| `options.feasibilities` | 管理员判定结论下拉项 | — |

> ⚠️ `config.json` 含密钥，**不要提交进 Git**（仓库已用 `.gitignore` 排除，只提供 `config.example.json`）。

---

## 四、部署

### systemd（Linux，推荐）

```bash
sudo cp deploy/dingtalk-req-collector.service /etc/systemd/system/
sudo vi /etc/systemd/system/dingtalk-req-collector.service   # 改 WorkingDirectory / ExecStart 路径
sudo systemctl daemon-reload
sudo systemctl enable --now dingtalk-req-collector
sudo systemctl status dingtalk-req-collector
```

### Windows

1. 解压发布包到固定目录（如 `C:\dingtalk-req\`）。
2. 编辑同目录 `config.json`（记事本即可）。
3. 双击 `start-windows.bat` 启动（比直接双击 exe 好：报错时窗口不会一闪而过）。
4. 开机自启：运行 `deploy\install-autostart.bat`（在当前用户「启动」文件夹放快捷方式，登录后自动运行）；
   需要未登录也运行、或崩溃自动拉起，改用任务计划程序 / NSSM；卸载用 `deploy\uninstall-autostart.bat`。
5. 放行防火墙（管理员 CMD）：
   ```bat
   netsh advfirewall firewall add rule name="dingtalk-req" dir=in action=allow protocol=TCP localport=1450
   ```

### 前置 HTTPS（生产建议）

服务本身是 HTTP。对外暴露时建议用 Nginx 终止 TLS 后反代到 `127.0.0.1:1450`，此时把 `host` 改成 `127.0.0.1`、`base_url` 改成 `https://域名`。

---

## 五、二次开发

### 目录结构

```
req-collector/
├── main.go        # 入口：配置加载、数据库初始化、HTTP 服务、安全中间件
├── config.go      # 配置结构 + 页面下拉选项（含通用默认值）
├── store.go       # MySQL：自动建表、会话令牌、密码哈希、附件记录
├── dingtalk.go    # 钉钉：免登码换 userid（新旧码兼容）、通讯录、工作通知卡片
├── attachment.go  # 附件：本地留档 + 钉盘上传/下载（OSS 带签直传）
├── handlers.go    # 全部 HTTP 路由与页面模板（embed 前端资源）
├── logring.go     # 环形日志缓冲（管理面板「运行日志」页签）
├── src/           # 前端源码（JSX，唯一真源）
│   ├── form.jsx   #   填写页
│   ├── list.jsx   #   需求列表页
│   ├── admin.jsx  #   管理面板
│   └── auth.js    #   登录态/请求封装（纯 JS，无需编译）
├── static/        # 前端产物（由 src/ 编译生成，embed 进二进制）
├── scripts/
│   ├── build_frontend.js  # src/*.jsx -> static/*.js（Babel classic runtime）
│   └── build_release.sh   # 一键：编译前端 + 四平台交叉编译 + 打包 dist/
└── deploy/        # systemd 服务文件、Windows 启停/自启脚本
```

### 改前端

浏览器端没有打包器，页面直接 `<script src="/static/xxx.js">`，所以 JSX 要先预编译：

```bash
node scripts/build_frontend.js   # src/*.jsx -> static/*.js；src/auth.js 直接复制
```

然后 **更新 `handlers.go` 里的 `staticVer` 常量**（用于打破钉钉 webview 缓存），再重新编译二进制。

### 构建发布包

```bash
export GOPROXY=https://goproxy.cn,direct   # 国内代理（首次）
./scripts/build_release.sh                 # 版本号取自 config.go 的 AppVersion
./scripts/build_release.sh 1.0.1           # 指定版本（记得同步改 AppVersion）
```

产物在 `dist/`：`linux-amd64` / `linux-arm64` / `macos-arm64` 的 `.tar.gz` 与 `windows` 的 `.zip`，包内含二进制、`config.json`、`README.md`、`deploy/`。
默认打进去的是**不含密钥**的 `config.example.json`；需要把本机真实凭证一起打包时用 `WITH_SECRETS=1 ./scripts/build_release.sh`（注意分发范围）。

---

## 六、API 一览

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/dingtalk/config` | 公开 | 免登配置（corpId / clientId / agentId） |
| GET | `/api/me` | 公开 | 当前登录态（并回填缺失的部门） |
| POST | `/api/dingtalk/login` | 公开 | `{code}` → 免登换 token（兼容新旧免登码） |
| POST | `/api/logout` | 公开 | 退出登录 |
| POST | `/api/submit` | 公开（免登用户自动带身份） | 提交需求，成功后通知管理员 |
| GET | `/api/requirements` | 公开 | 需求列表 |
| POST | `/api/attachment/upload` | 管理员/本人 | 上传附件（multipart：`req_id` + `file`），本地留档 + 后台推送钉盘 |
| GET | `/api/requirements/attachments?req_id=` | 管理员/本人 | 某条需求的附件列表 |
| GET | `/api/attachment/download?id=` | 管理员/本人 | 下载附件（本地优先，缺失时走钉盘临时链接） |
| POST | `/api/feasibility` | **管理员** | 更新「能否实现」并通知提交人 |
| POST | `/api/admin/login` | 公开（有限流） | 面板密码登录 → token |
| GET | `/api/admin/status` | 公开 | 面板登录态 |
| POST | `/api/admin/logout` | 登录 | 退出（钉钉身份只降级） |
| POST | `/api/admin/change_password` | 管理员 | 修改面板密码（会踢掉全部管理会话） |
| POST | `/api/admin/requirement/update` | 管理员/本人 | 编辑需求（本人不可改「能否实现」） |
| POST | `/api/admin/requirement/delete` | 管理员/本人 | 删除需求 |
| GET | `/api/admin/admins` | 管理员 | 管理员列表 + 已提交过需求的用户 |
| POST | `/api/admin/add_admin` | 管理员 | `{userid,name}` 授权管理员 |
| POST | `/api/admin/remove_admin` | 管理员 | `{id}` 移除管理员 |
| GET | `/api/admin/dd_users?name=关键字` | 管理员 | 懒加载搜索钉钉通讯录（缓存 10 分钟） |
| POST | `/api/admin/attachment/retry` | 管理员 | 把「仅本地」的附件重新推送到钉盘 |
| POST | `/api/admin/attachment/delete` | 管理员 | 删除附件记录（清理本地文件） |
| GET | `/api/admin/logs` | 管理员 | 最近 300 行服务日志 |
| POST | `/api/admin/logs/clear` | 管理员 | 清空日志缓冲 |

需要登录的接口通过 `X-Auth-Token` 请求头携带令牌。

## 七、数据表（自动创建）

- `req_items`：需求主表（`username` 存钉钉 userid，用于归属判断）
- `req_admins`：钉钉管理员名单（userid 唯一）
- `req_settings`：KV 设置（`admin_password` 哈希、通讯录缓存）
- `app_sessions`：登录令牌（默认 30 天过期）
- `req_files`：需求附件（文件名/大小/本地路径/钉盘 spaceId+fileId/上传状态）

## 八、安全

发布前已完成代码审计并修复高危项（随机初始密码、登录限流、越权、模板注入、错误信息泄露等），
完整清单与部署方注意事项见仓库根目录的 [`SECURITY.md`](../SECURITY.md)。

## 九、版本记录（CHANGELOG）

- **v1.1.0**
  - **新增附件上传**：填写页支持添加附件（默认 20MB/个、最多 5 个，可配 `max_upload_mb`），提交成功后自动上传——本地留档 + 推送钉钉网盘（钉盘）；权限未开通时自动降级为仅本地保存并写日志提示。
  - **管理员后台查看附件**：管理面板与需求列表新增「📎 附件」列，弹窗查看文件名/大小/上传人/钉盘状态并可下载（本人可看自己需求的附件）。
  - **上传不卡顿**：上传接口先本地落盘并立即响应（实测 ~0.02s），钉盘推送放后台异步执行；前端带 60s 超时保护与进度提示。
  - **附件状态可视化与丢失补救**：显示「已存钉盘 / 同步钉盘中… / 仅服务器本地 / ⚠️ 已丢失」；管理面板可「同步钉盘」「补传附件」「删除记录」。
  - 新需求通知卡片带附件数量；新表 `req_files`；删除需求联动清理附件；新配置项 `uploads_dir` / `drive_space_id` / `max_upload_mb`。
  - 附件接口走新版企业存储（`v1.0/storage`）接口，对应 `Storage.*` 权限点。

## 十、常见问题

- **免登失败 / errcode=15**：旧版 `topapi/user/getuserinfo` 已下线，本程序已优先使用 `/topapi/v2/user/getuserinfo`；若仍失败，检查应用凭证、服务器出口 IP 白名单，以及是否从工作台进入。
- **改了前端但钉钉里没变化**：改完要执行 `node scripts/build_frontend.js`，并把 `handlers.go` 的 `staticVer` 改成新值，再重新编译部署。
- **日志显示已通知但员工没收到**：程序在发送后会回查钉钉投递结果，失败/无效用户会记在日志里。最常见原因是 `userid` 不对，或员工不在该应用的可见范围。
- **部门没有自动带出**：免登时若未取到部门，会以通讯录缓存兜底回填；老会话在下次请求 `/api/me` 时自动补。
- **附件显示「仅服务器本地」/「同步钉盘中」**：说明钉盘权限未开通或上传尚在后台进行。到钉钉开发者后台开通「企业存储」相关权限点后，可在管理面板附件弹窗点「同步钉盘」补推；本地副本已就绪时功能不受影响。

## License

MIT — 见仓库根目录 [LICENSE](../LICENSE)。

# dingtalk-apps

> 钉钉企业内部应用（H5 微应用）合集 —— 开箱即用的单二进制 Go 服务，部署只需「1 个可执行文件 + 1 个 config.json」。

| 应用 | 目录 | 说明 | 状态 |
|---|---|---|---|
| 需求收集 Req Collector | [`req-collector/`](req-collector/) | 员工在钉钉内提需求（可传附件）→ 管理员在后台评审 → 结论以工作通知卡片回推提交人 | ✅ 可用 |

设计原则：

- **单文件部署**：前端（React 18 + Arco Design）编译后 `embed` 进 Go 二进制，目标机器不需要 Node、不需要静态目录。
- **零外部依赖**：只用 Go 标准库 + MySQL 驱动，无 Redis、无消息队列。
- **内网友好**：支持纯内网部署（HTTP + 内网 IP），也支持前置 Nginx 做 HTTPS。
- **业务可配置**：部门、需求类型、紧急程度、判定结论等下拉项全部写在 `config.json` 里，换业务不用改代码。
- **附件可选**：需求可附图片/文档/压缩包，本地留档并异步推送钉钉网盘（钉盘），未开通钉盘权限时自动降级为仅本地保存。

## 快速开始

```bash
git clone https://github.com/logdns/dingtalk-apps.git
cd dingtalk-apps/req-collector
cp config.example.json config.json   # 填 MySQL 与钉钉凭证
go build -o dingtalk-req-collector . # 或 ./scripts/build_release.sh 打全平台包
./dingtalk-req-collector -config config.json
```

完整说明见各应用目录内的 README。

## 安全

发布前已做代码审计，详见 [`SECURITY.md`](SECURITY.md)。概要：

- 管理密码默认**随机生成**（不在日志中明文出现），登录失败 5 次锁定 5 分钟；
- 全部 SQL 参数化，页面模板数据经 JSON/HTML 转义，已加 CSP 等安全响应头；
- `config.json`（含密钥）不入库，`.gitignore` 已排除。

漏洞报告请走 GitHub Issues 或邮件联系维护者，请勿公开披露。

## License

[MIT](LICENSE)

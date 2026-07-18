# CPA Monitor Go 后端

本地 K12/CPA（router-for-me / CLIProxyAPI）订阅监控服务，使用 Go 1.23、`net/http` 和 goquery。

## 运行

从 `server` 目录运行：

```powershell
..\.tools\go\bin\go.exe mod tidy
..\.tools\go\bin\go.exe test ./...
..\.tools\go\bin\go.exe run ./cmd/server
```

默认监听 `127.0.0.1:8080`，项目根目录按运行目录的 `..` 推导，因此默认读写：

- `../k12/**/*.json`：订阅归档扫描源
- `../data/offers.json`：报价缓存
- `../data/alerts.json`：低价提醒
- `../data/settings.json`：服务端设置和 API Key
- `../data/subscription_checks.json`：连通性结果

可通过参数调整：

```powershell
..\.tools\go\bin\go.exe run ./cmd/server -addr 127.0.0.1:9090 -project-root M:\Dev\Projects\CPA_monitor
```

监听地址也可用环境变量 `CPA_MONITOR_ADDR` 设置，命令行 `-addr` 优先。

## 主要 API

- `GET /api/health`
- `GET /api/offers`、`POST /api/offers/refresh`
- `GET/PUT /api/settings`、`POST /api/settings/test-webhook`
- `GET /api/subscriptions?folder=&status=&search=`
- `POST /api/subscriptions/import`（multipart：`file`，单个导入时可选 `acquisitionPrice`）
- `POST /api/subscriptions/{id}/test`
- `POST /api/subscriptions/{id}/sync`
- `GET /api/alerts`
- `GET /api/dashboard`

导入文件始终归档到本地日期 `../k12/MMDD`，同名文件自动添加序号，不覆盖原文件。开启 `syncToCpaAuthDir` 且配置合法的绝对路径 `cpaAuthDir` 后，会额外同步为 `codex_oauth_<sanitized-email>.json`，导入响应中的 `syncedToCpa` 表示是否完成同步。CLIProxyAPI/router-for-me 通常会热加载 auth-dir；连通性测试通过代理的 `/v1/models` 完成。

## 安全边界

- 服务默认只绑定回环地址；CORS 只接受 `http(s)://localhost:<port>` 和 `http(s)://127.0.0.1:<port>`。
- API 永不序列化 `access_token`，设置 API 也永不返回 `apiKey`，只返回 `apiKeyConfigured`。
- 上传 JSON 文件限制为 2 MB；归档文件名会去除目录并清理危险字符，扫描跳过符号链接。
- `baseUrl`、`orderUrl`、webhook 只允许 HTTP/HTTPS，拒绝 URL 内嵌凭据。
- 连通性测试默认只允许 loopback `baseUrl`。只有显式设置 `allowRemoteBaseUrl=true` 后才允许远端目标；重定向同样受该规则约束。
- CPA auth-dir 必须是已存在的绝对目录。实现会解析目录符号链接、限制目标文件位于解析后的目录内，并拒绝目标为符号链接或非普通文件。
- webhook 允许向远端 HTTP/HTTPS 地址发送提醒；这属于管理员显式配置的外连边界。不要把服务暴露到不可信网络，也不要将 `data/settings.json` 分享或提交到版本库。

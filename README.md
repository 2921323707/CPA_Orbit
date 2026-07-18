# CPA Orbit

> v1.0.2 · Local-first operations console for CPA credentials, offer monitoring, quota checks, and SMS verification workflows.

[中文](#中文) · [English](#english) · [更新日志](CHANGELOG.md) · [安全策略](SECURITY.md) · [贡献指南](CONTRIBUTING.md)

## 中文

CPA Orbit 是一套本地优先的运营控制台，将 CPA/Codex 凭据归档、CLIProxyAPI 运行池、K12/GPT Plus 报价、额度检查、提醒和鲁班接码集中到同一工作流中。

### 核心功能

- 订阅归档是唯一事实源，`cpa/auths` 是可重建的运行投影。
- 单个或批量导入 CPA/Codex/Claude/Kimi/DeepSeek 等 Provider JSON；完整 JSON 内容一致时才视为重复。
- 单文件可记录入手价格；订阅列表每页 10 条并自动刷新当前页额度。
- 定时抓取 K12 与 GPT Plus 真实报价，保存最近 14 天平均价格历史并支持双线对照。
- 鲁班接码支持余额、国家、服务单价、获取号码、3 秒验证码轮询和释放号码；API key 不返回浏览器。
- 深色模式、双语全局壳层、艺术加载动画、响应式布局和无障碍状态提示。

### 启动

Windows PowerShell：

```powershell
.\start-dev.ps1
```

服务地址：

- CLIProxyAPI：`http://127.0.0.1:8317/v1`
- Monitor API：`http://127.0.0.1:8080/api`
- Web Console：`http://127.0.0.1:5173/`
- 文档：`http://127.0.0.1:5173/docs`

### 安全边界

默认仅监听回环地址。CPA JSON、OAuth token、本地 API key、鲁班 API key、`data/`、`k12/` 和 `cpa/auths/` 都不应提交到仓库。详细规则见 [SECURITY.md](SECURITY.md)。

## English

CPA Orbit is a local-first operations console that unifies CPA/Codex credential archives, the CLIProxyAPI runtime pool, K12/GPT Plus offer monitoring, quota checks, alerts, and Luban SMS verification.

### Highlights

- Archived subscriptions are the source of truth; `cpa/auths` is a rebuildable runtime projection.
- Single or batch JSON import for CPA/Codex/Claude/Kimi/DeepSeek providers; only identical normalized JSON is considered a duplicate.
- Optional acquisition price for single-file imports, ten-row pagination, and automatic quota refresh for the current page.
- Real K12 and GPT Plus price collection with 14-day history and dual-series comparison.
- Backend-only Luban credentials with balance, catalog pricing, number acquisition, three-second code polling, and release.
- Dark mode, bilingual application shell, restrained orbital loading motion, responsive layouts, and accessible status indicators.

### Development

```powershell
.\start-dev.ps1
```

See [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a change. This repository includes third-party software; review [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) before redistribution.

## License

Original CPA Orbit source code is available under the [MIT License](LICENSE). Bundled or referenced third-party components retain their own licenses.

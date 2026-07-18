# CPA Orbit

> v1.0.2 · A quiet, local-first workspace from K12 price radar to CPA proxy operations.

[中文](#中文) · [English](#english) · [更新日志](CHANGELOG.md) · [安全策略](SECURITY.md) · [贡献指南](CONTRIBUTING.md) · [GitHub](https://github.com/2921323707/CPA_Orbit)

## 中文

CPA Orbit 不想做成一个庞大的后台。它只把几件经常要来回切换的事放在一起：看 K12 价格、归档订阅、检查额度、维护 CPA 代理，再顺手处理 GPT Plus 和接码流程。数据留在本机，目录清楚，出了问题也能沿着记录回溯。

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

### 数据来源与致谢

- 报价与商品信息来自 [PriceAI](https://priceai.cc/)。感谢其公开的商品数据与价格查询入口，让 K12 / GPT Plus 的历史走势有了可靠来源。
- 支付跳转与订单查询来自 [链动小铺 LXDP](https://pay.ldxp.cn/)。感谢其提供的商品支付与订单查询链路。

CPA Orbit 只是聚合、记录和跳转工具，不代表上述平台或商家；价格、库存、支付和售后以来源平台页面为准。

## English

CPA Orbit is a quiet, local-first workspace for the parts of AI subscription operations that usually get scattered across tabs: K12 price discovery, subscription archives, quota checks, CPA proxy runtime, GPT Plus, and SMS verification.

### Highlights

- Archived subscriptions are the source of truth; `cpa/auths` is a rebuildable runtime projection.
- Single or batch JSON import for CPA/Codex/Claude/Kimi/DeepSeek providers; only identical normalized JSON is considered a duplicate.
- Optional acquisition price for single-file imports, ten-row pagination, and automatic quota refresh for the current page.
- Real K12 and GPT Plus price collection with 14-day history and dual-series comparison.
- Backend-only Luban credentials with balance, catalog pricing, number acquisition, three-second code polling, and release.
- Dark mode, bilingual application shell, restrained orbital loading motion, responsive layouts, and accessible status indicators.

### Data sources and thanks

- Offer and price data: [PriceAI](https://priceai.cc/). Thank you for the public product data and price-query surface used by the K12 / GPT Plus history views.
- Payment redirects and order lookup: [LXDP](https://pay.ldxp.cn/). Thank you for providing the checkout and order-query path used by the console.

CPA Orbit aggregates, records, and redirects only. It is not affiliated with either service; live price, stock, payment, and after-sales terms remain subject to the source platform.

### Development

```powershell
.\start-dev.ps1
```

See [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a change. This repository includes third-party software; review [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) before redistribution.

## License

Original CPA Orbit source code is available under the [MIT License](LICENSE). Bundled or referenced third-party components retain their own licenses.

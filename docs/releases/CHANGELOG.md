# Changelog / 更新日志

All notable CPA Orbit changes are documented here. Versions follow Semantic Versioning.

## [Unreleased]

## [1.2.1] - 2026-07-20

### Added

- Added a native macOS Apple Silicon CI build and published ARM64 DMG and ZIP packages with SHA-256 checksums.
- Added architecture and disk-image verification to the macOS packaging workflow.

### Fixed

- Corrected the Wails macOS bundle path to use the generated `CPA Orbit.app` name.

### Contributors

- Thanks to [@tomatokoko](https://github.com/tomatokoko) for proposing native Apple Silicon support in [issue #9](https://github.com/2921323707/CPA_Orbit/issues/9).

## [1.2.0] - 2026-07-20

### Added

- Added K12 and unverified GPT Plus offer tables to one Price workspace, each using compact five-row pagination.
- Added manual deletion of individual historical price samples with immediate dashboard re-visualization.
- Added a Toolbox that combines the subscription JSON converter and the Luban SMS workflow.
- Added GPT Plus low-price alert support alongside K12 alerts.

### Changed

- Switched K12 collection to the PriceAI Team/Business `team_k12` and `liandongShop` filtered source, limited to five offers.
- Switched GPT Plus collection to the PriceAI `account_unverified` source.
- Moved Alerts into independent Settings subpages and limited retained alert history to the latest ten entries, five per page.
- Reworked the Overview offer list to show the three lowest K12 and GPT Plus offers separately.
- Simplified the average-price chart to one selected series and removed the 30-minute range.
- Centered and compacted Price table columns without horizontal scrolling.

### Fixed

- Fixed historical sample deletion failing when browser millisecond timestamps were compared with Go nanosecond timestamps.
- Prevented legacy, unrelated K12 history outliers from becoming permanent by allowing selected records to be removed safely.

## [1.1.0] - 2026-07-19

### Documentation

- Made English the default documentation language and added route-matched Simplified Chinese pages.
- Unified the fixed sidebar hierarchy and strengthened current-page highlighting.
- Rebuilt the introduction with a compact summary and animated local-runtime reactor.
- Added the public appreciation QR code and bilingual sponsorship presentation.
- Published the static documentation under `/cpa_orbit/` without changing the existing server homepage.

### Added

- Added a one-click Wails desktop control plane that starts or reuses the Monitor API and CLIProxyAPI while sharing browser data and settings.
- Added system tray behavior, close-to-tray, native notifications, taskbar flashing, startup-at-login, and Auto/Light/Dark theme settings.
- Added subscription asset insights for health, recorded cost, average acquisition price, and seven-day expiry risk.
- Added endpoint visibility, categorized documentation, polished Mermaid architecture diagrams, sanitized product showcase assets, GitHub community forms, CI, Dependabot, and Playwright E2E coverage.

### Changed

- Fixed the desktop window at 1280×800 and removed resize/aspect polling that caused visual flashing.
- Decoupled Monitor API health from external CLIProxyAPI health so proxy downtime cannot mark the embedded backend offline.
- Reworked Settings secondary navigation as stable in-page buttons without hash-router navigation or scroll-observer flicker.
- Improved route progress, skeleton loading, status refresh behavior, responsive layout, and shared desktop/browser settings.

### Fixed

- Removed a blocking `window.confirm` from single-file imports with an optional empty acquisition price. WebView imports now start immediately and report progress in-page.
- Fixed numeric acquisition prices being treated as strings before upload, which raised before the request was sent and left the desktop action stuck on “导入中”.
- Fixed priced JSON imports stalling in WebView2 by keeping multipart uploads file-only, moving the optional price to a validated request parameter, adding a request timeout, and always restoring the import action state.
- Fixed app and web data divergence by using one application runtime and one mutable data root.

## [1.0.2] - 2026-07-18

### 中文

- 将订阅归档确立为唯一事实源，并让 CPA 运行池成为可重建投影。
- 修复活动池数量与订阅列表不一致、JSON 内容去重和额度检查状态问题。
- 新增单个/批量导入、单文件入手价格、10 条分页和自动额度刷新。
- 新增 K12/GPT Plus 双报价抓取、14 天真实均价历史和双线趋势图。
- 新增鲁班余额、国家/服务价格、获取号码、3 秒验证码轮询和释放号码。
- 新增深色模式、隐藏页面滚动条、艺术加载动画、全局项目 logo 和中英语言基础层。
- 项目命名统一为 CPA Orbit，并补齐开源贡献、安全、支持、许可证和模板规范。
- 感谢 PriceAI 的公开商品与价格数据，以及链动小铺 LXDP 的支付跳转与订单查询支持。

### English

- Established archived subscriptions as the source of truth and the CPA pool as a rebuildable runtime projection.
- Fixed subscription/runtime count drift, full-content JSON identity, and quota-status reporting.
- Added single/batch import, optional acquisition price, ten-row pagination, and automatic quota refresh.
- Added K12/GPT Plus collection, 14-day real average-price history, and dual-series charts.
- Added Luban balance, country/service pricing, number acquisition, three-second code polling, and release.
- Added dark mode, hidden viewport scrollbar, orbital loading motion, a global project mark, and bilingual shell infrastructure.
- Standardized the CPA Orbit name and added open-source contribution, security, support, license, and repository templates.
- Thanks to PriceAI for public offer data and LXDP for checkout and order-query support.

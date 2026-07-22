---
title: 系统架构
description: CPA Orbit 的运行拓扑、数据所有权与信任边界。
---

# 系统架构

CPA Orbit 是本地优先的模块化单体。浏览器与 Wails 桌面端共享一个 Go 控制平面，因此设置、订阅、提醒与运行状态不会在两个后端之间漂移。Orbit 统一管理资产和绑定，Sub2API 负责主号池调度及原始用量，CPA 保留为轻量兜底。

## 分层结构

```text
浏览器 / Wails 桌面端
          ↓
Go Monitor API · 127.0.0.1:8080
          ↓
价格监控 · 订阅管理 · 网关编排 · 用量采集
          ↓
data/ + k12/ + control-plane.db（持久事实源）
          ↓
Sub2API（主池） / cpa/auths/ → CLIProxyAPI（兜底）
```

## 数据所有权

- `k12/` 保存订阅原始归档，是唯一事实源。
- `cpa/auths/` 只用于 CLIProxyAPI 热加载，可以从归档重建。
- `data/` 保存报价、历史、提醒、设置与检查结果。
- `data/control-plane.db` 保存网关目标、主备绑定、操作状态和最长 90 天的 15 分钟用量聚合。
- Sub2API 拥有刷新后的运行凭据、分组调度和原始请求明细；Orbit 不用旧归档覆盖这些状态。
- Web 与桌面端不维护独立账户库。

## 信任边界

服务默认监听回环地址。Token 与后端密钥不进入浏览器响应；远端 URL、Webhook、上传文件和重定向都必须经过验证与大小限制。

详细英文架构资料可切换到本页的 English 版本。架构变更记录见[架构决策](/zh/architecture/adr/)。

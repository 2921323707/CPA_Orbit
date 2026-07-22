---
title: 系统架构
description: CPA Orbit 的运行拓扑、数据所有权与信任边界。
---

# 系统架构

CPA Orbit 是本地优先的模块化单体。浏览器与 Wails 桌面端共享一个 Go 控制平面，因此设置、订阅、提醒与运行状态不会在两个后端之间漂移。CPA 与通用本地 Sub2API companion 都是可配置目标，Auth JSON 必须明确分配给唯一兼容目标。

## 分层结构

```text
浏览器 / Wails 桌面端
          ↓
Go Monitor API · 127.0.0.1:8090
          ↓
价格监控 · 订阅管理 · 网关编排 · 用量采集
          ↓
data/ + subscriptions/{sub2api,cpa}/MMDD/ + control-plane.db（持久事实源）
          ↓
Sub2API companion / cpa/auths/ → CLIProxyAPI companion（显式二选一）
```

## 数据所有权

- `subscriptions/{sub2api,cpa}/MMDD/` 保存按运行池和日期组织的订阅原始归档，是唯一事实源。
- `cpa/auths/` 只用于 CLIProxyAPI 热加载，可以从归档重建。
- `data/` 保存报价、历史、提醒、设置与检查结果。
- `data/control-plane.db` 保存网关目标、逻辑凭据的唯一分配、操作状态和最长 90 天的 15 分钟用量聚合。
- companion 拥有刷新后的运行凭据、分组调度和原始请求明细；Orbit 不用旧归档覆盖这些状态。
- 账号状态/额度轮询独立于报价监控，默认每 5 分钟执行，周期为 `0` 时关闭。
- Web 与桌面端不维护独立账户库。

## 信任边界

服务默认监听回环地址。Token 与后端密钥不进入浏览器响应；远端 URL、Webhook、上传文件和重定向都必须经过验证与大小限制。

详细英文架构资料可切换到本页的 English 版本。架构变更记录见[架构决策](/zh/architecture/adr/)。

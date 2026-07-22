---
title: Sub2API 订阅号池
description: 配置本地 CPA 或 Sub2API companion，执行 Auth JSON 预检并显式分配目标。
---

# Sub2API 订阅号池

CPA Orbit 把“订阅归档、运行网关、Token 使用记录”作为互相关联但权责不同的三层资产。这样既不会让同一份可刷新的 OAuth 凭据长期同时跑在两个网关，也不会用旧归档覆盖 Sub2API 已经刷新的运行状态。

## CPA、Sub2API 与订阅池的关系

| 对象 | 权威方 | 职责 |
|---|---|---|
| 订阅归档 | CPA Orbit 的 `subscriptions/sub2api/MMDD/` | 保存原始凭据、入手价、来源和恢复依据 |
| 网关目标 | CPA Orbit 的 `control-plane.db` | 保存 Sub2API/CPA 地址、兼容能力、部署默认值和只写密钥 |
| 部署绑定 | CPA Orbit 的 `control-plane.db` | 记录某份归档当前在哪个运行池、由谁管理、期望与实际状态 |
| 运行账号 | Sub2API 或 CPA | 负责调度、Token 刷新、分组、并发和实时额度 |
| 请求明细 | Sub2API | 权威请求日志与计费明细 |
| 用量聚合 | CPA Orbit 的 `control-plane.db` | 保留 15 分钟 Token/成本趋势，最长 90 天 |

CPA 与通用本地 Sub2API companion 是彼此独立的可配置目标。每份兼容 Auth JSON 都必须明确分配给唯一目标；Orbit 不会自动兜底，也不会按请求切换流量。让同一个 refresh token 同时在两边刷新可能互相轮换或失效，因此迁移必须执行显式的撤销源绑定/部署目标操作；远端结果 pending 或 uncertain 时保留待核对状态。

## 配置 Sub2API

1. 启动 Sub2API，并在其后台生成管理员 API Key。
2. 打开 **设置 → 网关**（`/settings?section=gateways`），配置本地 Sub2API companion。
3. 填写管理地址和管理密钥。可直接使用 Sub2API 的标准本机地址 `http://127.0.0.1:8080`；Orbit 控制 API 已独立使用 `8090`。
4. 按需填写默认分组 ID、账号并发、调度优先级和成本倍率。
5. 保存后点击 **检查连接**；导入时仍需显式选择该目标。

远程网关必须显式允许并使用 HTTPS。管理密钥是只写字段：只保存在本地，不会通过公开接口回显，也不会进入运维操作日志。

CPA companion 同样在 **设置 → 网关** 中配置；地址、授权目录和同步选项仍由本地设置管理。通用 Sub2API companion 与 CPA 并列使用兼容契约，管理密钥均只写不回显。

## 把 GPT Plus/Codex JSON 转成号池账号

进入 **订阅文件**，选择 Auth JSON 并明确选择唯一兼容目标：

```text
本地安全预检 → Provider/日期归档 → 显式部署到 CPA 或 Sub2API
```

部署失败时归档仍会保留。远端结果 pending 或 uncertain 时，界面会提示需要核对；不会静默重试另一个目标，也不会创建第二个活动分配。历史归档可在详情抽屉中重新分配。

## 归属与安全删除

- `Orbit 托管`：账号由 Orbit 创建，可以由 Orbit 撤销和删除。
- `外部接管`：账号原本就在外部系统；解绑只清除本地关系，不删除远端账号。
- 删除订阅时先撤销所有活动绑定。若托管的远端账号无法安全删除，本地归档不会被删掉。

## 可视化与 Token 记录

账号状态与额度检查独立于报价监控。默认每 5 分钟轮询一次；将周期设为 `0` 可关闭计划轮询，需要时执行明确的手动检查。检查失败或结果不确定会单独标记，不改变凭据分配。

请求原始明细仍由 Sub2API 保管。Orbit 只保存 15 分钟聚合：请求数、成功/失败数、输入/输出/缓存 Token、平均耗时、首 Token 延迟、标准成本和实际成本。90 天以前的聚合会自动清理。

::: warning 服务条款与封号风险
把订阅账号转换为网关号池可能与上游服务条款冲突，也可能造成封号、服务中断或数据损失。请先确认相关协议与使用授权。Sub2API 官方项目本身也明确提示了同类风险。
:::

数据所有权设计见 [ADR-0007](/architecture/adr/0007-gateway-targets-and-managed-bindings)，上游当前行为以 [Sub2API 官方仓库](https://github.com/Wei-Shaw/sub2api)为准。

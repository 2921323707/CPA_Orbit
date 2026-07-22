---
title: 模块介绍
description: CPA Orbit 的核心能力与数据边界。
---

# 模块介绍

CPA Orbit 是本地优先的模块化单体。各模块共享一个 Go 控制平面和一套本地状态，同时保持数据所有权清晰。

## 模块总览

| 模块 | 职责 | 数据归属 |
|---|---|---|
| 总览 | 汇总运行状态 | 只读聚合，不创建第二事实源 |
| 价格情报 | K12 / GPT Plus 快照与趋势 | `data/offers.json`、`data/price_history.json` |
| 订阅资产 | 安全预检、归档、显式分配与账号检查 | `subscriptions/{sub2api,cpa}/MMDD/` 是事实源，`cpa/auths/` 是投影 |
| 网关设置 | 配置本地 CPA/Sub2API 目标并核对分配 | `data/control-plane.db`；运行状态归 companion |
| 提醒 | 阈值历史与 Webhook | `data/alerts.json` |
| 设置 | 地址、周期、阈值与后端密钥 | `data/settings.json` |
| 桌面宿主 | 启动、托盘、通知与系统集成 | 复用同一运行时 |

## 价格情报

K12 与 GPT Plus 共用刷新周期，并保留真实的 14 天均价历史。上游暂时失败时保留最近一次成功快照，同时明确显示失败原因。

## 订阅资产

`subscriptions/{sub2api,cpa}/MMDD` 下的归档是唯一事实源，`cpa/auths` 是提供给 CLIProxyAPI 的可重建投影。健康检查会区分 HTTP 401、HTTP 402、额度耗尽、限流、禁用和未加入活动池等状态。

## CPA 运行与提醒

Monitor API 和 CLIProxyAPI 使用独立健康信号。提醒支持持久历史、浏览器偏好、桌面通知和后端 Webhook；鲁班接码密钥始终保存在后端。

## 网关设置与账号检查

网关配置位于“设置 → 网关”。CPA 与通用本地 Sub2API companion 使用明确的唯一兼容目标分配，Orbit 不会自动兜底。账号状态/额度轮询独立于报价监控，默认每 5 分钟执行；周期设为 `0` 时关闭计划轮询。分配 Auth JSON 前请阅读[网关与订阅指南](/zh/guide/sub2api-pool)。

## 桌面宿主

Wails 桌面端嵌入生产 Vue 构建，与浏览器开发模式复用同一个 Go Runtime，并补充系统托盘、开机启动、原生通知和便携数据目录。

继续阅读[系统架构](/zh/architecture/)、[后端开发](/zh/development/backend)和[桌面端开发](/zh/development/desktop)。

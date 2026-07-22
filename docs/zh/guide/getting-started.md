---
title: 快速开始
description: 启动 CPA Orbit 并导入第一份订阅。
---

# 快速开始

这份指南会启动本地 Web 控制台、Monitor API，以及可选的 CLIProxyAPI 运行时。

::: tip 默认仅限本机
所有服务默认绑定 `127.0.0.1`。没有额外认证和反向代理保护时，不要直接暴露到公网。
:::

## 环境要求

| 依赖 | 最低要求 | 用途 |
|---|---:|---|
| Go | 1.25 | Monitor API 与桌面宿主 |
| Node.js | 20 | Vue 控制台构建 |
| WebView2 | Windows 10/11 | 桌面端渲染 |
| Sub2API | 可选，需单独部署 | 兼容 Auth JSON 的外部网关；CPA Orbit 不内置 |
| CLIProxyAPI | 可选，需单独提供 | CPA companion；CPA Orbit 官方安装包不内置 |

## 启动工作区

```powershell
git clone https://github.com/2921323707/CPA_Orbit.git
cd CPA_Orbit
.\start-dev.ps1
```

| 服务 | 本地地址 |
|---|---|
| Web 控制台 | `http://127.0.0.1:5173/` |
| Monitor API | `http://127.0.0.1:8090/api` |
| CLIProxyAPI | `http://127.0.0.1:8317/v1` |
| Sub2API | 用户自行运行；本机 Docker 通常映射为 `http://127.0.0.1:8080` |

## 配置网关 companion

`start-dev.ps1` 不会安装 Sub2API。请先单独启动 Sub2API，例如使用 Docker，并确认其管理端点可以访问。进入 **Sub2API 后台 → 系统设置 → Admin API Key** 创建密钥，然后打开 **CPA Orbit → 设置 → 网关**（`/settings?section=gateways`），添加 Sub2API 目标，填写端点和密钥，保存后点击 **检查连接**。密钥只写不回显；远程地址必须显式允许并使用 HTTPS，本机地址优先。归档、显式分配和待确认状态见 [网关与订阅指南](/zh/guide/sub2api-pool)。

## 配置与导入

在“设置”中确认本地 `base_url`、`cpa/auths` 目录和 CLIProxyAPI 客户端密钥。保存后的密钥只由后端持有。

```text
本地安全预检 → Provider/日期归档 → 明确选择唯一兼容的 CPA 或 Sub2API 目标
```

导入部署明确失败或结果不确定时保留待处理状态，不会切换到另一个目标；只有结果标记为可重试时，才使用同一操作和同一目标继续，并且不会创建第二份归档。

::: warning 敏感文件
CPA JSON 包含 bearer token。不要上传到 Issue、聊天、日志、截图或公开仓库。
:::

下一步：[浏览模块](/zh/modules/)或阅读[系统架构](/zh/architecture/)。

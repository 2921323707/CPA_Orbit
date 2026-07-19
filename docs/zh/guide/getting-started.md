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
| CLIProxyAPI | 可选 | CPA 代理能力 |

## 启动工作区

```powershell
git clone https://github.com/2921323707/CPA_Orbit.git
cd CPA_Orbit
.\start-dev.ps1
```

| 服务 | 本地地址 |
|---|---|
| Web 控制台 | `http://127.0.0.1:5173/` |
| Monitor API | `http://127.0.0.1:8080/api` |
| CLIProxyAPI | `http://127.0.0.1:8317/v1` |

## 配置与导入

在“设置”中确认本地 `base_url`、`cpa/auths` 目录和 CLIProxyAPI 客户端密钥。保存后的密钥只由后端持有。

```text
选择 JSON → 归档到 k12/MMDD → 投影到 cpa/auths → 健康检查
```

::: warning 敏感文件
CPA JSON 包含 bearer token。不要上传到 Issue、聊天、日志、截图或公开仓库。
:::

下一步：[浏览模块](/zh/modules/)或阅读[系统架构](/zh/architecture/)。

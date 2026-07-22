---
title: 部署上线
description: 构建并发布 CPA Orbit 静态文档站。
---

# 部署上线

文档站是纯静态产物，不需要 Node.js 常驻、数据库或本地 CPA Orbit API。

## 构建

```powershell
cd docs
npm ci
npm run build
```

发布目录为 `docs/.vitepress/dist/`。当前站点使用基础路径 `/cpa_orbit/`，可通过 **[CPA Orbit 在线文档](http://165.154.205.54/cpa_orbit/)** 直接访问。

## 自动发布

`docs/` 是在线文档的唯一内容来源，其中 [`docs/roadmap.md`](../roadmap.md) 构建为 `/cpa_orbit/roadmap`，本文档的中文路线图构建为 `/cpa_orbit/zh/roadmap`。GitHub Actions 的 `Documentation` 工作流会构建每个文档 PR，并在文档改动合并到 `main` 后自动发布。

需要在 GitHub 中建立名为 `documentation` 的 Environment，并配置：

| 名称 | 类型 | 用途 |
| --- | --- | --- |
| `DOCS_SSH_PRIVATE_KEY` | Secret | 受限部署账号的 SSH 私钥。 |
| `DOCS_SSH_KNOWN_HOSTS` | Secret | 文档服务器固定的 SSH host key。 |
| `DOCS_HOST` | Variable | 部署主机，当前为 `165.154.205.54`。 |
| `DOCS_SSH_USER` | Variable | 仅用于文档部署的 SSH 用户。 |
| `DOCS_SSH_PORT` | Variable | SSH 端口；未设置时使用 `22`。 |
| `DOCS_DEPLOY_PATH` | Variable | 必须为 `/cpa_orbit`，工作流会拒绝其他路径。 |

部署账号应当仅能写入 `/cpa_orbit`。工作流通过 `rsync` 同步生成的 `dist/` 文件、清除该限定目录中的旧生成文件，然后通过 HTTP 验证首页、中英文路线图、favicon 以及提交 revision 标记。需要重新发布当前 `main` 文档而不改源码时，可以手动运行该工作流。

## 发布检查

- 首页、指南、路线图与模块页面可以直接刷新。
- 默认显示英文，语言菜单可以切换到对应中文页面。
- `/cpa_orbit/favicon.svg`、`/cpa_orbit/assets/` 与 `/cpa_orbit/revision.txt` 返回 200。
- 线上 `revision.txt` 与 GitHub Actions 构建的提交一致。
- 服务器只公开 `dist/`，不公开源码、`.git/`、`node_modules/` 或运行数据。

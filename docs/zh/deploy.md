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

## 发布检查

- 首页、指南与模块页面可以直接刷新。
- 默认显示英文，语言菜单可以切换到对应中文页面。
- favicon 与静态资源返回 200。
- 服务器只公开 `dist/`，不公开源码、`.git/`、`node_modules/` 或运行数据。

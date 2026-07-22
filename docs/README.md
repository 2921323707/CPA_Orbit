# CPA Orbit 文档源码

这一目录同时保存项目文档与 VitePress 静态站点。Markdown 是唯一内容源；站点构建只负责导航、检索与呈现，不引入服务端运行时。

在线文档：**[http://165.154.205.54/cpa_orbit/](http://165.154.205.54/cpa_orbit/)**

## 本地预览

```powershell
cd docs
npm install
npm run dev
```

## 生产构建

```powershell
cd docs
npm ci
npm run build
```

静态产物位于 `.vitepress/dist/`，可直接托管到 Nginx、Caddy、对象存储或任意静态站点平台。

## 内容地图

- [打开在线文档](http://165.154.205.54/cpa_orbit/)
- [站点首页](index.md)
- [快速开始](guide/getting-started.md)
- [Sub2API 号池指南](zh/guide/sub2api-pool.md)
- [模块介绍](modules/index.md)
- [系统架构](architecture/README.md)
- [后期扩展](roadmap.md)
- [部署上线](deploy.md)
- [参与贡献](contribute.md)
- [赞助项目](sponsor.md)
- [更新日志](releases/CHANGELOG.md)
- [v1.3.0 发布说明](releases/v1.3.0.md)
- [v1.2.0 发布说明](releases/v1.2.0.md)
- [v1.1.0 发布说明](releases/v1.1.0.md)
- [v1.0.2 发布说明](releases/v1.0.2.md)

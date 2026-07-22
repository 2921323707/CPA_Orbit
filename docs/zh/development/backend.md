---
title: 后端开发
description: Go Monitor API 的职责、运行方式与安全边界。
---

# 后端开发

Monitor API 使用 Go、`net/http` 与 goquery，负责价格监控、提醒、Provider/日期订阅归档、独立的账号状态/额度轮询、公开设置、CPA/Sub2API companion 协调和鲁班短信代理。Auth JSON 先做本地预检，再显式选择唯一兼容目标；不会自动兜底。账号轮询默认每 5 分钟一次，周期设为 `0` 时关闭。

## 运行与测试

```powershell
cd server
..\.tools\go\bin\go.exe test ./...
..\.tools\go\bin\go.exe run ./cmd/server
```

默认监听 `127.0.0.1:8090`，与 Sub2API 的标准 `8080` 端口隔离。上传大小、文件名、目录边界、外部 URL、重定向和响应体都应保持有界；API 响应不得包含 Token 或已保存密钥。

切换到 English 可阅读完整端点表与实现细节。

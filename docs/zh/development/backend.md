---
title: 后端开发
description: Go Monitor API 的职责、运行方式与安全边界。
---

# 后端开发

Monitor API 使用 Go、`net/http` 与 goquery，负责价格监控、提醒、订阅归档、额度检查、公开设置、CPA 投影和鲁班短信代理。

## 运行与测试

```powershell
cd server
..\.tools\go\bin\go.exe test ./...
..\.tools\go\bin\go.exe run ./cmd/server
```

默认监听 `127.0.0.1:8080`。上传大小、文件名、目录边界、外部 URL、重定向和响应体都应保持有界；API 响应不得包含 Token 或已保存密钥。

切换到 English 可阅读完整端点表与实现细节。

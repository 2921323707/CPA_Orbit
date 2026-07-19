---
title: 安全策略
description: CPA Orbit 的安全报告与密钥处理规则。
---

# 安全策略

安全修复面向最新版本和当前 `main` 分支。涉及凭据泄露、路径穿越、认证绕过、SSRF、不安全文件写入或敏感日志时，请使用 GitHub 私密漏洞报告，不要创建公开 Issue。

## 密钥处理

- 不提交 `k12/**/*.json`、`cpa/auths/**`、本地配置、日志或 `.env*`。
- 浏览器不得接收 CPA 管理密钥、鲁班密钥或 OAuth Token。
- 外部 URL、重定向、响应大小和文件路径必须验证。
- 公开截图和测试数据必须完全脱敏或使用合成数据。

[提交私密安全报告](https://github.com/2921323707/CPA_Orbit/security/advisories/new)

---
title: 安全策略
description: CPA Orbit 的安全报告与密钥处理规则。
---

# 安全策略

安全修复面向最新已发布版本（当前为 **v1.3.0**）和当前 `main` 分支。涉及凭据泄露、路径穿越、认证绕过、SSRF、不安全文件写入或敏感日志时，请使用 GitHub 私密漏洞报告，不要创建公开 Issue。

## 密钥处理

- 不提交 `subscriptions/**/*.json`、`cpa/auths/**`、本地配置、日志或 `.env*`。Provider/日期归档与本地控制面应纳入受保护备份，不能放入公开构建物。
- 浏览器不得接收 CPA/Sub2API 管理密钥、鲁班密钥或 OAuth Token；界面与 API 均只写不回显。
- Sub2API 是需要独立保护的外部服务，CPA Orbit 不内置。请在 **Sub2API 系统设置 → Admin API Key** 创建管理员密钥，再通过 Orbit 的只写网关表单保存。
- 网关配置位于“设置 → 网关”。远程目标必须显式允许并使用 HTTPS，本机目标优先；CPA Orbit 官方安装包不内置 Sub2API 或 CLIProxyAPI。
- Auth JSON 先做本地安全预检，再明确选择唯一兼容目标；导入 pending/uncertain 只在所选目标进入待核对状态，不会跨目标自动兜底。
- 备份应加密并限制访问；如果主机文件系统没有加密，本地配置和凭据文件可能以明文落盘，必须按敏感文件保护。
- 外部 URL、重定向、响应大小和文件路径必须验证。
- 公开截图和测试数据必须完全脱敏或使用合成数据。

[提交私密安全报告](https://github.com/2921323707/CPA_Orbit/security/advisories/new)

---
title: 桌面端开发
description: Wails 桌面宿主、构建与数据共享方式。
---

# 桌面端开发

Wails 桌面宿主嵌入生产 Vue 构建，并与浏览器控制台共享同一个 Go Runtime、设置和数据目录。

## Windows 构建

```powershell
.\app\build-windows.ps1
```

产物位于 `app/build/bin/CPAOrbit.exe`。仓库内运行时会启动或复用 Monitor API，并按需发现 CLIProxyAPI。桌面端补充系统托盘、关闭到托盘、原生通知、任务栏提醒与开机启动。

## macOS Apple Silicon 构建

在安装 Go 1.25+、Node.js/npm 和 Xcode Command Line Tools 的 Apple Silicon Mac 上运行：

```bash
CPA_ORBIT_MAC_ARCH=arm64 ./app/build-macos.sh
```

脚本会在 `app/build/bin` 生成原生 ARM64 的 `CPA Orbit.app`、ZIP、可拖拽安装的 DMG 和 SHA-256 校验文件，并验证应用架构与磁盘映像。GitHub Actions 会在每个 PR 和 `main` 更新时使用原生 ARM64 runner 构建；推送 `v*` 标签后，DMG、ZIP 和校验文件会自动上传到对应 GitHub Release。

如需签名，请设置 `CPA_ORBIT_CODESIGN_IDENTITY`。Apple 公证需要私密开发者凭据，因此保留给具备凭据的发布环境。切换到 English 可阅读便携目录和环境变量的完整说明。

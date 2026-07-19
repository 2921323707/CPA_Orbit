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

切换到 English 可阅读 macOS、签名、便携目录和环境变量的完整说明。

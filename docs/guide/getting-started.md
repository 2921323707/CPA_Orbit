---
title: Quick start
description: Start CPA Orbit and import the first subscription.
---

# Quick start

This guide starts the local web console, Monitor API, and optional CLIProxyAPI runtime.

::: tip Local by default
All services bind to `127.0.0.1` by default. Do not expose them directly to an untrusted network.
:::

## Requirements

| Dependency | Minimum | Purpose |
|---|---:|---|
| Go | 1.25 | Monitor API and desktop host |
| Node.js | 20 | Vue console build |
| WebView2 | Windows 10/11 | Desktop rendering |
| CLIProxyAPI | Optional | CPA proxy capabilities |

## Start the workspace

```powershell
git clone https://github.com/2921323707/CPA_Orbit.git
cd CPA_Orbit
.\start-dev.ps1
```

| Service | Local address |
|---|---|
| Web console | `http://127.0.0.1:5173/` |
| Monitor API | `http://127.0.0.1:8080/api` |
| CLIProxyAPI | `http://127.0.0.1:8317/v1` |

## Confirm health

The header reports Monitor and CPA health independently. CPA downtime does not mark the embedded Monitor API offline. You can also open:

```text
http://127.0.0.1:8080/api/health
```

## Configure CPA

Open **Settings** and confirm the local base URL, the `cpa/auths` directory, and the CLIProxyAPI client key. Saved keys remain backend-only; the browser receives only a configured flag.

## Import a subscription

```text
Select JSON → archive under k12/MMDD → project to cpa/auths → run health check
```

Single-file imports may record an acquisition price. Identity is based on normalized full JSON content rather than a filename or email address.

::: warning Sensitive material
CPA JSON contains bearer tokens. Never attach it to an issue, chat, log, screenshot, or public repository.
:::

Next: [explore the modules](/modules/) or read the [architecture dossier](/architecture/).

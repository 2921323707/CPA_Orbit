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
| Sub2API | Optional | Configurable local companion for compatible Auth JSON |
| CLIProxyAPI | Optional | Configurable local CPA companion |

## Start the workspace

```powershell
git clone https://github.com/2921323707/CPA_Orbit.git
cd CPA_Orbit
.\start-dev.ps1
```

| Service | Local address |
|---|---|
| Web console | `http://127.0.0.1:5173/` |
| Monitor API | `http://127.0.0.1:8090/api` |
| CLIProxyAPI | `http://127.0.0.1:8317/v1` |

## Confirm health

The header reports Monitor and CPA health independently. CPA downtime does not mark the embedded Monitor API offline. You can also open:

```text
http://127.0.0.1:8090/api/health
```

## Configure CPA

Open **Settings** and confirm the local base URL, the `cpa/auths` directory, and the CLIProxyAPI client key. Saved keys remain backend-only; the browser receives only a configured flag.

## Configure a gateway companion

Open **Settings → Gateways** (`/settings?section=gateways`) and configure a local CPA or generic local Sub2API companion. Keys are write-only. Remote endpoints require explicit approval and HTTPS; loopback is preferred. See [gateway and subscription guidance](/guide/sub2api-pool) for assignment and reconciliation rules.

## Import a subscription

```text
Safe local preflight → provider/date archive → explicitly choose exactly one compatible CPA or Sub2API target
```

A pending or uncertain deployment remains visible for reconciliation; Orbit does not automatically fall back to another target. Single-file imports may record an acquisition price. Identity is based on normalized full JSON content rather than a filename or email address.

::: warning Sensitive material
CPA JSON contains bearer tokens. Never attach it to an issue, chat, log, screenshot, or public repository.
:::

Next: [explore the modules](/modules/) or read the [architecture dossier](/architecture/).

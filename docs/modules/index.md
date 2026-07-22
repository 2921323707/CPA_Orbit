---
title: Modules
description: Core CPA Orbit capabilities and their data boundaries.
---

# Modules

CPA Orbit is a local-first modular monolith. Modules share one Go control plane and one local state model while keeping ownership explicit.

## Overview

| Module | Responsibility | Owned state |
|---|---|---|
| Overview | Operational summary | Read-only aggregation |
| Price intelligence | K12 / GPT Plus snapshots and trends | `data/offers.json`, `data/price_history.json` |
| Subscription assets | Archive, deduplicate, project, and check accounts | `k12/` source, `cpa/auths/` projection |
| Pool operations | Gateway targets, managed bindings, migrations, and Token telemetry | `data/control-plane.db`; Sub2API owns raw request logs |
| Alerts | Threshold history and webhook delivery | `data/alerts.json` |
| Settings | Endpoints, schedules, thresholds, backend keys | `data/settings.json` |
| Desktop host | Startup, tray, notifications, native integration | Reuses the same runtime |

## Price intelligence

K12 and GPT Plus sources share a refresh schedule and retain a truthful 14-day average-price history. A temporary upstream failure preserves the last successful snapshot and exposes the reason instead of replacing it with empty data.

## Subscription assets

The archive under `k12/MMDD` is the source of truth. `cpa/auths` is a rebuildable runtime projection for CLIProxyAPI. Health checks distinguish HTTP 401, HTTP 402, exhausted quota, rate limits, disabled accounts, and archives outside the active pool.

## CPA runtime

Monitor API and CLIProxyAPI have independent health signals. The default endpoints are:

```text
Monitor API  http://127.0.0.1:8080/api
CLIProxyAPI  http://127.0.0.1:8317/v1
```

## Sub2API control plane

Sub2API is the preferred primary subscription pool; CPA remains a lightweight fallback. Orbit stores durable target and binding state, imports Codex sessions through the official Sub2API administrator API, and retains bounded fifteen-minute usage aggregates. Read the [pool guide](/guide/sub2api-pool) before enabling two gateway targets.

## Alerts and SMS

Alerts support persistent history, browser preferences, desktop notifications, and backend Webhooks. The Luban SMS flow handles balance, catalog lookup, number acquisition, polling, and release while keeping its API key in the backend.

## Desktop host

The Wails desktop client embeds the production Vue bundle and reuses the same Go Runtime as browser development. It adds system tray behavior, startup-at-login, native notifications, and portable data directories without creating another account store.

Read the [architecture dossier](/architecture/), [backend guide](/development/backend), or [desktop guide](/development/desktop) for implementation details.

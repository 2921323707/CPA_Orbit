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
| Subscription assets | Safe preflight, archive, explicit assignment, and account checks | `subscriptions/{sub2api,cpa}/MMDD/` source, `cpa/auths/` projection |
| Gateway settings | Configure local CPA/Sub2API targets and reconcile assignments | `data/control-plane.db`; companion owns runtime state |
| Alerts | Threshold history and webhook delivery | `data/alerts.json` |
| Settings | Endpoints, schedules, thresholds, backend keys | `data/settings.json` |
| Desktop host | Startup, tray, notifications, native integration | Reuses the same runtime |

## Price intelligence

K12 and GPT Plus sources share a refresh schedule and retain a truthful 14-day average-price history. A temporary upstream failure preserves the last successful snapshot and exposes the reason instead of replacing it with empty data.

## Subscription assets

The archive under `subscriptions/{sub2api,cpa}/MMDD` is the source of truth. `cpa/auths` is a rebuildable runtime projection for CLIProxyAPI. Health checks distinguish HTTP 401, HTTP 402, exhausted quota, rate limits, disabled accounts, and archives outside the active pool.

## CPA runtime

Monitor API and CLIProxyAPI have independent health signals. The default endpoints are:

```text
Monitor API  http://127.0.0.1:8090/api
CLIProxyAPI  http://127.0.0.1:8317/v1
```

## Gateway settings and account checks

Gateway configuration lives under **Settings → Gateways**. CPA and the generic local Sub2API companion use explicit compatible-target assignment; Orbit never performs automatic fallback. Account status/quota polling is distinct from offer monitoring, runs every five minutes by default, and is disabled when its interval is `0`. Read the [gateway guide](/guide/sub2api-pool) before assigning an Auth JSON.

## Alerts and SMS

Alerts support persistent history, browser preferences, desktop notifications, and backend Webhooks. The Luban SMS flow handles balance, catalog lookup, number acquisition, polling, and release while keeping its API key in the backend.

## Desktop host

The Wails desktop client embeds the production Vue bundle and reuses the same Go Runtime as browser development. It adds system tray behavior, startup-at-login, native notifications, and portable data directories without creating another account store.

Read the [architecture dossier](/architecture/), [backend guide](/development/backend), or [desktop guide](/development/desktop) for implementation details.

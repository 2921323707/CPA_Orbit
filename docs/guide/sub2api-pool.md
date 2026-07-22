---
title: Sub2API subscription pools
description: Configure local CPA or Sub2API companions, preflight Auth JSON, and assign each credential explicitly.
---

# Sub2API subscription pools

CPA Orbit treats the subscription archive, runtime gateways, and usage history as related but distinct assets. This avoids copying one refreshable OAuth credential into two active gateways and prevents Orbit from overwriting state refreshed by Sub2API.

## Operating model

| Object | Authority | Purpose |
|---|---|---|
| Subscription archive | CPA Orbit `subscriptions/sub2api/MMDD/` | Original credential document, acquisition cost, provenance, and recovery source |
| Gateway target | CPA Orbit `control-plane.db` | Sub2API or CPA endpoint, compatibility contract, deployment defaults, and write-only admin secret |
| Deployment binding | CPA Orbit `control-plane.db` | Desired and observed relation between one archive and one runtime account |
| Runtime account | Sub2API or CPA | Scheduling, refreshed credentials, groups, concurrency, and live quota state |
| Usage details | Sub2API | Authoritative request log and billing detail |
| Usage aggregates | CPA Orbit `control-plane.db` | Fifteen-minute Token/cost continuity, retained for up to 90 days |

CPA and Sub2API are independent configurable local companions. Each compatible Auth JSON must be assigned explicitly to exactly one target; Orbit never performs automatic fallback or request-by-request traffic switching. Keeping the same refresh credential active in both gateways could cause competing rotations, so moving a credential requires an explicit detach/deploy action with a visible reconciliation state.

## Configure a Sub2API target

1. Start Sub2API and generate an administrator API key.
2. Open **Settings → Gateways** (`/settings?section=gateways`) and configure a local Sub2API companion.
3. Enter the management base URL and admin key. The standard local Sub2API endpoint `http://127.0.0.1:8080` can be used directly; Orbit uses its own control port at `8090`.
4. Optionally set group IDs, account concurrency, scheduling priority, and cost multiplier.
5. Save the target and run **Check connection**. Import still requires selecting this target explicitly.

Remote targets require an explicit opt-in and HTTPS. The admin key is write-only: it is stored locally and never returned by the public API or shown in operation logs.

Configure the local CPA companion from **Settings → Gateways** as well; its connection, authorization directory, and synchronization settings remain local. The generic Sub2API contract is configured alongside CPA, and each target's admin key is write-only.

## Import and deploy GPT Plus/Codex JSON

In **Subscription files**, select an Auth JSON and choose exactly one compatible configured target. Orbit performs a two-stage flow:

```text
local safe preflight → provider/date archive → explicit deploy to CPA or Sub2API
```

The archive is retained if deployment fails. A pending or uncertain remote result is shown for reconciliation; Orbit does not silently retry against another target or create a second assignment. Existing archives can be assigned later from their detail drawer.

## Ownership and safe deletion

- `managed` accounts were created by Orbit and may be removed by Orbit.
- `adopted` accounts exist outside Orbit; detaching clears only the local relation and never deletes the remote account.
- Deleting a subscription first detaches all active bindings. If a remote managed account cannot be removed safely, the local archive is kept.

## Telemetry

Account status and quota checks are separate from offer monitoring. They poll every five minutes by default; set the interval to `0` to disable scheduled polling, and use an explicit manual check when needed. A failed or uncertain check is marked accordingly without changing the credential assignment.

Raw request logs remain in Sub2API. Orbit stores normalized fifteen-minute aggregates containing request counts, successes/failures, input/output/cache tokens, duration, first-token latency, and cost. Aggregates older than 90 days are deleted automatically.

::: warning Provider terms and account risk
Subscription-to-gateway conversion can conflict with upstream provider terms and can lead to account suspension or service interruption. Review the applicable agreements and use the integration only where authorized. Sub2API itself publishes the same risk warning in its project documentation.
:::

See [ADR-0007](/architecture/adr/0007-gateway-targets-and-managed-bindings) for the ownership decision and the [Sub2API project](https://github.com/Wei-Shaw/sub2api) for current upstream behavior.

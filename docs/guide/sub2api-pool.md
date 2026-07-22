---
title: Sub2API subscription pools
description: Configure Sub2API as the primary gateway, keep CPA as a safe fallback, and understand ownership and telemetry.
---

# Sub2API subscription pools

CPA Orbit treats the subscription archive, runtime gateways, and usage history as related but distinct assets. This avoids copying one refreshable OAuth credential into two active gateways and prevents Orbit from overwriting state refreshed by Sub2API.

## Operating model

| Object | Authority | Purpose |
|---|---|---|
| Subscription archive | CPA Orbit `k12/` | Original credential document, acquisition cost, provenance, and recovery source |
| Gateway target | CPA Orbit `control-plane.db` | Sub2API or CPA endpoint, role, deployment defaults, and write-only admin secret |
| Deployment binding | CPA Orbit `control-plane.db` | Desired and observed relation between one archive and one runtime account |
| Runtime account | Sub2API or CPA | Scheduling, refreshed credentials, groups, concurrency, and live quota state |
| Usage details | Sub2API | Authoritative request log and billing detail |
| Usage aggregates | CPA Orbit `control-plane.db` | Fifteen-minute Token/cost continuity, retained for up to 90 days |

Sub2API is the normal primary target. CPA remains a lightweight local fallback. A fallback deployment is attempted when the primary import fails; it is not request-by-request traffic failover. Keeping the same refresh token active in both gateways could cause competing rotations, so switching gateways uses an explicit detach/deploy migration with rollback.

## Configure a Sub2API target

1. Start Sub2API and generate an administrator API key.
2. Open **Pool operations → Add Sub2API**.
3. Enter the management base URL and admin key. A local URL such as `http://127.0.0.1:8080` is recommended.
4. Optionally set group IDs, account concurrency, scheduling priority, and cost multiplier.
5. Mark the target as primary, save it, and run **Check connection**.

Remote targets require an explicit opt-in and HTTPS. The admin key is write-only: it is stored locally and never returned by the public API or shown in operation logs.

CPA targets reuse the connection, key, authorization directory, and synchronization switch under **Settings → CPA sync**. The control-plane target only records whether CPA is enabled and whether it is primary or fallback.

## Import and deploy GPT Plus/Codex JSON

In **Subscription files**, leave **Deploy to primary pool after import** enabled and select one or more Codex session JSON files. Orbit performs these steps:

```text
validate and deduplicate → archive locally → deploy to Sub2API primary
                                           ↘ CPA fallback if primary import fails
```

The archive is retained even if both deployments fail, and the UI reports that the runtime binding needs attention. Existing archives can be deployed later from their detail drawer. A subscription running on CPA fallback can use **Return to primary pool**; if destination deployment fails, Orbit attempts to restore the source binding.

## Ownership and safe deletion

- `managed` accounts were created by Orbit and may be removed by Orbit.
- `adopted` accounts exist outside Orbit; detaching clears only the local relation and never deletes the remote account.
- Deleting a subscription first detaches all active bindings. If a remote managed account cannot be removed safely, the local archive is kept.

## Telemetry

The operations page shows gateway health, active bindings, recent deployment operations, current Sub2API totals, and a seven-day Token chart. Orbit collects every five minutes and supports a manual refresh. Failed collection marks the snapshot stale but preserves the last valid values.

Raw request logs remain in Sub2API. Orbit stores normalized fifteen-minute aggregates containing request counts, successes/failures, input/output/cache tokens, duration, first-token latency, and cost. Aggregates older than 90 days are deleted automatically.

::: warning Provider terms and account risk
Subscription-to-gateway conversion can conflict with upstream provider terms and can lead to account suspension or service interruption. Review the applicable agreements and use the integration only where authorized. Sub2API itself publishes the same risk warning in its project documentation.
:::

See [ADR-0007](/architecture/adr/0007-gateway-targets-and-managed-bindings) for the ownership decision and the [Sub2API project](https://github.com/Wei-Shaw/sub2api) for current upstream behavior.

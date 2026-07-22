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

CPA and Sub2API are independent configurable companions. In the two-stage import flow, each compatible Auth JSON is assigned explicitly to exactly one target; a failed or uncertain commit remains on that target and never triggers cross-target fallback or request-by-request traffic switching. Keeping the same refresh credential active in both gateways could cause competing rotations, so moving a credential requires an explicit detach/deploy action with a visible reconciliation state.

::: warning External gateway required
CPA Orbit does not include, install, or download Sub2API. Run Sub2API separately—for example with its official Docker deployment—and keep its management endpoint on a trusted interface. Official CPA Orbit packages also do not embed CLIProxyAPI.
:::

## Configure a Sub2API target

1. Install and start Sub2API separately. For a local Docker deployment that publishes port `8080`, confirm `http://127.0.0.1:8080` opens in the browser.
2. Sign in to the Sub2API administrator interface, open **System Settings → Admin API Key**, and create a key. Copy it immediately; the full value is shown only when generated.
3. Open **CPA Orbit → Settings → Gateways** (`/settings?section=gateways`), select **Add gateway**, and choose **Sub2API**.
4. Enter the management URL `http://127.0.0.1:8080` and the Admin API Key. Do not append `/api/v1` or `/api/v1/admin`; Orbit builds the administrator API path itself.
5. Optionally set group IDs, account concurrency, scheduling priority, and cost multiplier. Enable the target and select the primary marker only if it is your preferred target.
6. Save the target and run **Check connection**. A successful check confirms the endpoint and key; importing a credential still requires selecting this target explicitly.

The primary marker is metadata for preferred/default-compatible actions, not automatic failover. CPA is used only when the operator explicitly deploys or migrates a credential to it. Remote targets require an explicit opt-in and HTTPS. The admin key is write-only: it is stored locally and never returned by the public API or shown in operation logs.

Configure the local CPA companion from **Settings → Gateways** as well; its connection, authorization directory, and synchronization settings remain local. The generic Sub2API contract is configured alongside CPA, and each target's admin key is write-only.

## Import and deploy GPT Plus/Codex JSON

In **Subscription files**, select an Auth JSON and choose exactly one compatible configured target. Orbit performs a two-stage flow:

```text
local safe preflight → provider/date archive → explicit deploy to CPA or Sub2API
```

The archive is retained if deployment fails. A definite failure and an uncertain transport result are reported separately with a sanitized operation code, target, HTTP status, and retryability. When retry is safe, Orbit resumes the same durable operation against the same target without creating a second archive; it never silently switches targets or creates a second active assignment. Existing archives can be assigned later from their detail drawer.

## Ownership and safe deletion

- `managed` accounts were created by Orbit and may be removed by Orbit.
- `adopted` accounts exist outside Orbit; detaching clears only the local relation and never deletes the remote account.
- Deleting a subscription first detaches all active bindings. If a remote managed account cannot be removed safely, the local archive is kept.

## Telemetry

Account status and quota checks are separate from offer monitoring. They poll every five minutes by default; set the interval to `0` to disable scheduled polling, and use an explicit manual check when needed. Orbit supports Sub2API's current SSE account-test stream and distinguishes an authoritative unhealthy account from an unavailable inspection attempt: transient inspection failures remain pending/reconcilable instead of being mislabeled as account errors, and they never change the credential assignment.

If a managed Sub2API account is recreated with a new remote ID, Orbit automatically rebinds only when the old ID is definitively absent and exactly one account matches both the `orbit_subscription_id` provenance marker and strong credential identity. Zero, multiple, email-only, or contradictory matches are never guessed. Reconciliation does not re-upload the credential or delete candidate accounts.

Select the filename in the **Subscription file** column to open its detail drawer; status/quota testing, deployment, migration, detach, and CPA synchronization actions live in that drawer. Raw request logs remain in Sub2API. Orbit stores normalized fifteen-minute aggregates containing request counts, successes/failures, input/output/cache tokens, duration, first-token latency, and cost. Aggregates older than 90 days are deleted automatically.

::: warning Provider terms and account risk
Subscription-to-gateway conversion can conflict with upstream provider terms and can lead to account suspension or service interruption. Review the applicable agreements and use the integration only where authorized. Sub2API itself publishes the same risk warning in its project documentation.
:::

See [ADR-0007](/architecture/adr/0007-gateway-targets-and-managed-bindings) for the ownership decision and the [Sub2API project](https://github.com/Wei-Shaw/sub2api) for current upstream behavior.

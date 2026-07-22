# Sub2API Control Plane Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Evolve CPA Orbit into a local-first subscription control plane where Sub2API is the primary gateway and CPA remains a lightweight fallback, with safe GPT Plus/Codex account deployment, unified health, operations, and token-usage views, followed by a verified Windows EXE build.

**Architecture:** Keep the existing Go/Vue/Wails modular monolith. Separate durable subscription assets from gateway targets and deployment bindings, persist relational control-plane and aggregate usage state in SQLite, retain raw credential JSON in the protected archive, and access CPA/Sub2API exclusively through gateway adapters. Sub2API remains authoritative for raw request logs and runtime scheduling; Orbit stores bindings and bounded aggregate snapshots.

**Tech Stack:** Go `net/http`, SQLite, Vue 3, TypeScript, Vite, Wails, existing JSON credential archive, Sub2API Admin API.

---

### Task 1: Record architecture decisions and introduce control-plane storage

**Files:**
- Create: `docs/architecture/adr/0007-gateway-targets-and-managed-bindings.md`
- Create: `server/internal/controlplane/store.go`
- Create: `server/internal/controlplane/store_test.go`
- Modify: `server/go.mod`
- Modify: `server/application/runtime.go`

**Steps:**
1. Write tests for schema creation, gateway-target secret redaction, subscription bindings, sync operations, and aggregate usage upserts.
2. Run the focused test and confirm it fails before the package exists.
3. Add SQLite-backed tables for `gateway_targets`, `deployment_bindings`, `sync_operations`, and `usage_buckets`.
4. Initialize and close the store from the shared runtime.
5. Run the focused tests and the server test suite.

### Task 2: Define the gateway adapter contract and move CPA behind it

**Files:**
- Create: `server/internal/gateways/gateway.go`
- Create: `server/internal/gateways/cpa/client.go`
- Create: `server/internal/gateways/cpa/client_test.go`
- Modify: `server/internal/subscriptions/manager.go`
- Modify: `server/internal/model/model.go`

**Steps:**
1. Write contract tests for health, deploy, test, quota, and detach behavior.
2. Define normalized target, binding, health, quota, usage, and deployment result models.
3. Move CPA management calls and safe auth-directory projection behind the CPA adapter.
4. Preserve existing API behavior through compatibility methods while removing direct CPA assumptions from generic subscription flows.
5. Track files created by Orbit and never delete unowned CPA auth files.

### Task 3: Implement the Sub2API admin client

**Files:**
- Create: `server/internal/gateways/sub2api/client.go`
- Create: `server/internal/gateways/sub2api/client_test.go`
- Create: `server/internal/gateways/sub2api/types.go`

**Steps:**
1. Write HTTP contract tests using `httptest.Server`.
2. Implement admin-key authentication, bounded responses, no redirects, sanitized errors, and configurable timeouts.
3. Implement gateway health/capability probing.
4. Implement Codex session import via `/api/v1/admin/accounts/import/codex-session`.
5. Implement account listing, testing, 5H/7D usage, OpenAI quota, dashboard snapshot, account availability, and paged usage access.
6. Confirm secrets and credential bodies never appear in errors or logs.

### Task 4: Add gateway settings and management APIs

**Files:**
- Modify: `server/internal/config/config.go`
- Modify: `server/internal/config/config_test.go`
- Modify: `server/internal/httpapi/server.go`
- Modify: `server/internal/httpapi/server_test.go`
- Modify: `web/src/types/api.ts`
- Modify: `web/src/services/api.ts`

**Steps:**
1. Add tests for Sub2API URL, admin-key write-only semantics, local/remote opt-in, default group, concurrency, and priority.
2. Add public capability/configuration flags without returning stored keys.
3. Add target list, target test, deployment, detach, migration, overview, token trend, and usage endpoints.
4. Enforce loopback by default and HTTPS for remote targets unless the existing explicit remote override is enabled.
5. Add typed frontend API clients for every new endpoint.

### Task 5: Implement subscription-to-gateway deployment bindings

**Files:**
- Create: `server/internal/deployments/coordinator.go`
- Create: `server/internal/deployments/coordinator_test.go`
- Modify: `server/internal/subscriptions/manager.go`
- Modify: `server/internal/httpapi/server.go`

**Steps:**
1. Write tests for Sub2API-primary deployment, duplicate update, failure recovery, adopted accounts, CPA fallback, and ownership-safe detach.
2. Add durable, idempotent deployment operations and binding state transitions.
3. Make new Codex/GPT subscription imports deploy to configured Sub2API by explicit option, with archive success retained if remote deployment fails.
4. Enforce one primary runtime for refreshable OAuth credentials.
5. Implement CPA-to-Sub2API migration as a resumable sequence rather than simultaneous dual-active sync.

### Task 6: Aggregate Sub2API operations and token usage

**Files:**
- Create: `server/internal/observability/collector.go`
- Create: `server/internal/observability/collector_test.go`
- Modify: `server/application/runtime.go`
- Modify: `server/internal/httpapi/server.go`

**Steps:**
1. Write tests for snapshot normalization, 15-minute bucket upserts, stale data, and bounded retention.
2. Collect Sub2API dashboard/account aggregates without reading its PostgreSQL directly.
3. Persist normalized request, input/output/cache token, cost, latency, account, group, and model buckets.
4. Preserve the last valid snapshot when Sub2API is unavailable.
5. Add configurable 90-day aggregate retention and background collection.

### Task 7: Build the unified gateway, subscription-pool, and operations UI

**Files:**
- Modify: `web/src/views/DashboardView.vue`
- Modify: `web/src/views/SubscriptionsView.vue`
- Modify: `web/src/views/SettingsView.vue`
- Create: `web/src/views/OperationsView.vue`
- Create: `web/src/components/dashboard/GatewayStatusCard.vue`
- Create: `web/src/components/dashboard/TokenUsageChart.vue`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/components/layout/AppShell.vue`
- Modify: `web/src/styles.css`

**Steps:**
1. Add frontend unit/e2e fixtures for configured, offline, stale, and partial-failure states.
2. Add Sub2API-primary and CPA-fallback target cards.
3. Show each subscription's primary gateway, binding state, quota, and deployment actions.
4. Add GPT Plus/Codex import destination and safe migration controls.
5. Add operations KPIs, token/cost trends, account availability, error state, and model/account breakdowns.
6. Verify responsive desktop layout and secret-free rendering.

### Task 8: Migration, documentation, and release verification

**Files:**
- Modify: `docs/architecture/README.md`
- Modify: `docs/architecture/adr/index.md`
- Modify: `docs/guide/getting-started.md`
- Modify: `docs/zh/guide/getting-started.md`
- Modify: `README.md`
- Modify: `tests/run-all.ps1`

**Steps:**
1. Migrate existing CPA settings into a fallback target and existing archives without altering credential bytes.
2. Document Sub2API setup, admin-key security, GPT subscription limitations, and migration recovery.
3. Run Go, frontend, desktop integration, and production build tests.
4. Build the Windows Wails executable with `app/build-windows.ps1`.
5. Verify the generated EXE exists, records its SHA-256, and report its absolute path for user approval.


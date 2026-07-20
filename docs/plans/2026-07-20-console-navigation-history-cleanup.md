# Console Navigation and History Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reorganize Toolbox and Settings navigation, compact the Price tables, add persistent price-history deletion, and simplify Overview before rebuilding the Windows executable.

**Architecture:** Keep existing API compatibility through redirects while moving user-facing navigation to `/toolbox` and Settings query-backed subpages. Add a narrowly scoped DELETE history endpoint that removes one exact timestamp from the selected persisted feed, then let the chart request a dashboard reload. Reuse current offer and alert data models rather than introducing new stores.

**Tech Stack:** Go 1.25, net/http, Vue 3, TypeScript, Vue Router, Vite, Wails v2.

---

### Task 1: Toolbox-only GPT Plus page

**Files:**
- Modify: `web/src/components/layout/AppShell.vue`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/i18n/index.ts`
- Modify: `web/src/views/GptPlusView.vue`

1. Rename the GPT Plus navigation entry to Toolbox and route it to `/toolbox`.
2. Redirect `/gpt-plus` to `/toolbox` for compatibility.
3. Remove all PriceAI offer loading, KPIs, refresh controls, and price table markup from the toolbox page.
4. Retain Luban key, balance, catalog, number-session, polling, and release behavior.
5. Run `vue-tsc -b` and expect success.

### Task 2: Compact Price tables

**Files:**
- Modify: `web/src/views/OffersView.vue`
- Modify: `web/src/styles.css`

1. Add Price-specific table wrapper and table classes.
2. Define fixed column widths with a narrow first-to-second-column gap.
3. Allow merchant/title text to wrap and keep action controls compact.
4. Remove the wide-table minimum width and horizontal overflow for Price only.
5. Build the frontend and verify no horizontal scrollbar at desktop width.

### Task 3: Settings subpages and Alerts integration

**Files:**
- Modify: `web/src/components/layout/AppShell.vue`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/views/SettingsView.vue`
- Reuse: `web/src/views/AlertsView.vue`

1. Remove Alerts from the main navigation.
2. Redirect `/alerts` to `/settings?section=alerts`.
3. Replace scroll-based settings navigation with query-backed subpage navigation.
4. Render exactly one settings section per selected subpage and render `AlertsView` for the alerts subpage.
5. Keep one save action for editable settings subpages and no save action on Alerts.

### Task 4: Persistent history sample deletion

**Files:**
- Modify: `server/internal/httpapi/monitor.go`
- Modify: `server/internal/httpapi/monitor_test.go`
- Modify: `server/internal/httpapi/server.go`
- Modify: `web/src/services/api.ts`
- Modify: `web/src/components/dashboard/PriceDistributionChart.vue`
- Modify: `web/src/views/DashboardView.vue`

1. Add tests for deleting one exact K12 or GPT Plus history timestamp and persisting the retained samples.
2. Implement `Monitor.DeletePriceSample(source, at)` with source validation and exact UTC instant matching.
3. Add `DELETE /api/price-history?source=...&at=...` with clear 400/404 responses.
4. Add a frontend API method and a delete action in the expanded data table.
5. Confirm deletion, show progress/errors, emit completion, reload Dashboard, and redraw the chart.

### Task 5: Overview layout cleanup

**Files:**
- Modify: `web/src/views/DashboardView.vue`

1. Replace merged top-five data with separate K12 lowest-three and GPT Plus lowest-three groups.
2. Remove recent-subscription rendering and replace the right panel with a `待定义` placeholder.
3. Remove unused subscription presentation imports and helpers.
4. Verify the chart data table still expands and refreshes after a deletion.

### Task 6: Verification and Windows package

1. Run `go test ./...` in `server` and `app`.
2. Run Vue type checking and desktop Vite builds for `web/dist` and `app/frontend/dist`.
3. Run `git diff --check`.
4. Stop only the repository `CPAOrbit.exe` process if it holds the output file.
5. Build Wails for `windows/amd64`, copy notices/config, and regenerate `CHECKSUMS-SHA256.txt`.
6. Report EXE path, timestamp, size, and SHA-256.

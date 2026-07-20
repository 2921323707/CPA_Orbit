# Follow-up UI, History, and Alerts Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix historical price deletion and refine Price, Alerts, and Toolbox behavior, then rebuild the Windows executable.

**Architecture:** Keep the existing Go API and Vue views. Make price-history deletion tolerant of JavaScript millisecond timestamp precision, enforce a ten-alert retention cap in the monitor, and implement five-row pagination in the affected Vue tables. Move the existing external JSON converter link from Subscriptions to Toolbox without changing its destination.

**Tech Stack:** Go 1.25, net/http, Vue 3, TypeScript, Vite, Wails v2.

---

### Task 1: Repair historical sample deletion

**Files:**
- Modify: `server/internal/httpapi/monitor.go`
- Modify: `server/internal/httpapi/monitor_test.go`

**Steps:**
1. Add a regression test using a nanosecond timestamp requested at JavaScript millisecond precision.
2. Run the focused Go test and confirm the exact-match implementation fails.
3. Match timestamps by Unix millisecond while deleting only one sample.
4. Run the focused and full server tests.

### Task 2: Refine Price tables

**Files:**
- Modify: `web/src/views/OffersView.vue`
- Modify: `web/src/styles.css`

**Steps:**
1. Set the shared Price page size to five rows.
2. Center every header and corresponding cell, including price badges and action groups.
3. Run Vue type checking and the production build.

### Task 3: Cap and paginate alert history

**Files:**
- Modify: `server/internal/httpapi/monitor.go`
- Modify: `server/internal/httpapi/monitor_test.go`
- Modify: `web/src/views/AlertsView.vue`

**Steps:**
1. Add tests that retain only the ten newest alerts.
2. Apply retention after loading and after creating an alert.
3. Change the Alerts page size to five, yielding at most two pages.
4. Run Go and Vue checks.

### Task 4: Move the JSON converter to Toolbox

**Files:**
- Modify: `web/src/views/SubscriptionsView.vue`
- Modify: `web/src/views/ToolboxView.vue`

**Steps:**
1. Remove only the JSON converter card from Subscriptions, leaving CDK access intact.
2. Add the same converter destination to Toolbox as a separate utility panel.
3. Update page copy and verify imports/type checking.

### Task 5: Package and verify

**Files:**
- Build: `web/dist/`
- Build: `app/frontend/dist/`
- Build: `app/build/bin/CPAOrbit.exe`

**Steps:**
1. Run all Go tests, Vue type checking, and `git diff --check`.
2. Build both desktop frontend outputs.
3. Stop only the old executable from this repository if it is running.
4. Rebuild the Wails executable and regenerate SHA-256 checksums.
5. Report recommendations for the unchanged Overview placeholder.

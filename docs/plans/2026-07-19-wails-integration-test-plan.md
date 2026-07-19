# Wails Integration Verification Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use executing-plans to implement this plan task-by-task.

**Goal:** Validate the Wails desktop feature branch with repeatable unit, integration, browser, and packaging checks; record evidence under `tests/`; remove generated repository garbage; then integrate the verified work into `main`.

**Architecture:** Keep product tests beside the Go and Playwright code they exercise, while placing cross-project orchestration and immutable test evidence in the root `tests/` directory. Verification must use isolated temporary data roots and mocked browser API boundaries so it never reads committed or local credential material. Generated build and Playwright artifacts remain ignored and are cleaned after evidence is captured.

**Tech Stack:** Go `testing`, `go test`, Vue 3/TypeScript/Vite, Playwright, Wails v2.13, PowerShell, Git.

---

### Task 1: Establish the test branch and artifact policy

**Files:**
- Modify: `.gitignore`
- Create: `tests/README.md`
- Create: `tests/run-all.ps1`

**Steps:**

1. Create `codex/wails-integration-tests` from `origin/feature/wails-desktop-app`.
2. Inventory tracked and untracked files before deleting anything.
3. Ignore Wails build output and generated Wails frontend bindings explicitly.
4. Add a root test runner that fails fast and runs server tests, app tests, browser build, desktop build, and Playwright.
5. Run the test runner syntax check and review its exact commands.

### Task 2: Run the existing Go unit and integration suites

**Files:**
- Test: `server/**/*_test.go`
- Test: `app/**/*_test.go`

**Steps:**

1. Run `go test -count=1 ./...` from `server/`.
2. Run `go test -race -count=1 ./...` from `server/` when the Windows toolchain supports the race detector.
3. Run `go test -count=1 ./...` from `app/`.
4. Run `go test -race -count=1 ./...` from `app/` when supported.
5. Record exact pass/fail counts, duration, and any skipped race check.

### Task 3: Close meaningful test gaps

**Files:**
- Modify or create tests only in the package containing the uncovered behavior.

**Steps:**

1. Review coverage output for server and app packages.
2. Prioritize trust boundaries: API routing, desktop same-origin handler, companion ownership, path/config validation, subscription import, and reconciliation.
3. Write a failing regression test for each confirmed gap or defect.
4. Make the smallest production correction necessary.
5. Re-run the focused test and then both complete Go suites.

### Task 4: Validate browser and desktop frontend behavior

**Files:**
- Test: `web/e2e/*.spec.ts`

**Steps:**

1. Run `npm ci` only if the lockfile and installed dependencies are out of sync.
2. Run `npm run build`.
3. Run `npm run build:desktop`.
4. Run `npm run test:e2e` with the repository Playwright configuration.
5. If a regression is found, add a focused Playwright test, verify it fails, implement the fix, and re-run the suite.

### Task 5: Validate Windows packaging and runtime boundaries

**Files:**
- Test: `app/build-windows.ps1`
- Output ignored: `app/build/bin/**`

**Steps:**

1. Stop only the running `app/build/bin/CPAOrbit.exe` instance that locks the disposable build output.
2. Run `app/build-windows.ps1` without the installer option.
3. Verify the EXE exists, the checksum matches, and the required license/config files are packaged.
4. Launch the built EXE against an isolated temporary data directory if it can be done without touching the user's live runtime.
5. Verify `/api/health`, then close only the process started by the test.

### Task 6: Capture evidence and clean generated artifacts

**Files:**
- Create: `tests/reports/2026-07-19-wails-integration-report.md`
- Modify: `.gitignore`

**Steps:**

1. Record environment versions, commit IDs, commands, outcomes, coverage, defects, fixes, and residual risks.
2. Confirm the report contains no tokens, account data, private paths beyond the repository, or management keys.
3. Preview generated-file cleanup with `git clean -nd` against explicit ignored artifact paths.
4. Remove only verified build, report, cache, and test-result artifacts; retain the Markdown evidence.
5. Run `git status`, `git diff --check`, and the complete test runner once more.

### Task 7: Integrate the two branches into main

**Files:**
- Git history only.

**Steps:**

1. Commit the test infrastructure, regression tests, fixes, ignore rules, and report on `codex/wails-integration-tests`.
2. Switch to `main` and fast-forward it from `origin/main`.
3. Merge `feature/wails-desktop-app` into `main` with an explicit merge commit if it is not a fast-forward.
4. Merge `codex/wails-integration-tests` into `main`.
5. Re-run the complete verification from merged `main`.
6. Do not push or create a GitHub pull request unless separately requested.

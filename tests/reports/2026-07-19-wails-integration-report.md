# CPA Orbit Wails integration test report

## Test identification

| Field | Value |
|---|---|
| Date | 2026-07-19 |
| Source branch | `origin/feature/wails-desktop-app` |
| Source commit | `8dae3e7` |
| Test branch | `codex/wails-integration-tests` |
| Platform | Windows amd64 |
| Go | 1.26.5 |
| Node.js | 24.12.0 |
| npm | 11.6.2 |
| Wails | 2.13.0 |
| Playwright project | Chromium |

## Outcome

**Result: PASS with documented environment exclusions.**

- 25 server Go tests passed.
- 10 desktop Go tests passed.
- Both Go modules passed `go vet`.
- Server statement coverage: 30.5% overall.
- Desktop statement coverage: 36.0% overall.
- Browser production and desktop-mode TypeScript/Vite builds passed.
- Three Playwright scenarios passed; a stability run repeated each scenario three times for 9/9 passes.
- The Windows portable Wails package built successfully.
- The packaged EXE checksum matched and all five required distribution files were present.
- The packaged EXE started with an isolated temporary data directory and returned `status=ok`, `name=CPA Orbit` from `/api/health`.

No credentials, production account files, management keys, or mutable repository runtime data were used by the tests.

## Commands and evidence

| Area | Command | Result |
|---|---|---|
| Server unit/integration | `go test -count=1 ./...` in `server/` | PASS |
| Server static analysis | `go vet ./...` in `server/` | PASS |
| Server order independence | `go test -shuffle=on -count=1 ./...` | PASS |
| Desktop unit/integration | `go test -count=1 ./...` in `app/` | PASS |
| Desktop static analysis | `go vet ./...` in `app/` | PASS |
| Desktop order independence | `go test -shuffle=on -count=1 ./...` | PASS |
| Browser production bundle | `npm run build` | PASS, 1,626 modules transformed |
| Desktop frontend bundle | `npm run build:desktop` | PASS, 1,626 modules transformed |
| Browser E2E | `npm run test:e2e` | PASS, 3/3 |
| Browser E2E stability | `npm run test:e2e -- --repeat-each=3` | PASS, 9/9 |
| Windows portable package | `.\app\build-windows.ps1` | PASS, 13,095,424-byte EXE |
| Package integrity | SHA-256 verification | PASS |
| Packaged runtime smoke test | Isolated `CPA_ORBIT_DATA_DIR`, then `GET /api/health` | PASS |
| Standard runner | `.\tests\run-all.ps1 -SkipPackage` | PASS |

## Defect found and corrected

### Clean-checkout desktop tests could not compile

The desktop module embeds `all:frontend/dist`, but a clean checkout contained no file in that directory. `go test ./...` and `go vet ./...` failed before compiling tests with:

```text
pattern all:frontend/dist: no matching files found
```

`app/.gitignore` attempted to allow a placeholder as `!frontend/dist/gitkeep`, while the intended file was `.gitkeep`. The ignore exception was corrected and `app/frontend/dist/.gitkeep` is now tracked. This keeps clean-checkout Go tests and the existing desktop CI job compilable without committing generated frontend assets.

## Environment-only failures and exclusions

1. The first Go coverage invocation used `go` from `PATH`, but this workstation exposes Go through the repository-local `.tools/go/bin/go.exe`. No product test ran in that attempt; all Go commands were repeated with the explicit toolchain path and passed.
2. The first Playwright attempt could not launch because the matching Chromium binary was not installed. `npx playwright install chromium` installed revision 1228; the unchanged suite then passed 3/3 and 9/9 under repetition.
3. `go test -race` was not available because the local Windows Go environment has CGO disabled. Ordinary, shuffled, and repeated tests passed, but race-detector coverage remains a CI/toolchain follow-up.
4. The NSIS installer path was not tested because the request covered the portable build and NSIS availability was not assumed.
5. macOS packaging was not tested on this Windows host.

## Garbage and isolation controls

- Removed stale Playwright results, frontend bundles, generated Wails bindings, package metadata copies, and the previous portable build before verification.
- Stopped only the stale `app/build/bin/CPAOrbit.exe` process that locked disposable output.
- Created packaged-runtime state in a unique system temporary directory, stopped only the process started by the smoke test, and removed that temporary directory afterward.
- `app/build/bin`, `app/frontend/wailsjs`, `web/dist`, `web/test-results`, Playwright reports, and package hash markers remain ignored disposable paths.
- Final cleanup is previewed with path-scoped `git clean -ndX` before deletion.

## Residual risks

- Overall Go coverage is useful but not high; command entry points, long-running monitor loops, native tray/startup behavior, external provider failures, and several HTTP handler branches remain under-covered.
- Playwright currently protects import and Settings navigation regressions, not the complete dashboard, alert, SMS, or subscription lifecycle.
- The smoke test verifies portable startup and the shared health endpoint, but not live CLIProxyAPI credentials or upstream model calls by design.
- Windows tray notifications, startup registration, and WebView2 behavior still need a manual release checklist on a clean user profile.

## Release recommendation

The tested feature branch is suitable for integration into `main` after committing this report and test infrastructure. The documented exclusions are non-blocking for the portable Windows merge but should remain visible in later release work.

## Post-merge verification

The source and verification branches were integrated locally with explicit merge commits:

- `6f40009` — merge `origin/feature/wails-desktop-app` into `main`;
- `7007617` — merge `codex/wails-integration-tests` into `main`.

The complete default `.\tests\run-all.ps1` suite was then executed from merged `main`. Server tests/vet, desktop tests/vet, both frontend builds, Playwright 3/3, and the Windows portable package all passed. Generated package, frontend, Wails binding, and Playwright artifacts were removed afterward with an explicit path-scoped ignored-file cleanup.

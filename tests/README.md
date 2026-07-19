# CPA Orbit test suite

This directory owns cross-project test orchestration and durable verification reports. Package-level tests remain beside the Go or browser code they exercise:

- `server/**/*_test.go` — backend unit and HTTP/runtime integration tests;
- `app/**/*_test.go` — desktop configuration, companion, and shared-runtime integration tests;
- `web/e2e/*.spec.ts` — browser-level regression tests with mocked secret-free API boundaries;
- `tests/reports/*.md` — sanitized release-verification evidence.

## Run the standard suite

From the repository root on Windows:

```powershell
.\tests\run-all.ps1
```

Useful development switches:

```powershell
.\tests\run-all.ps1 -SkipPackage
.\tests\run-all.ps1 -SkipE2E -SkipPackage
```

The default suite runs Go tests and vet for both modules, browser and desktop frontend builds, Playwright E2E, and the portable Windows package build. It does not use local credentials or require CLIProxyAPI to be online. Packaging output, Playwright traces, generated Wails bindings, and frontend bundles are disposable ignored artifacts.

## Test rules

1. Tests must be deterministic and independently repeatable.
2. Network-facing behavior must use local test servers or mocked request boundaries.
3. Temporary state must use an isolated directory and be removed by the test that created it.
4. Never record OAuth JSON, tokens, account email lists, API keys, management keys, or private runtime logs.
5. Every bug fix starts with a failing regression test where practical.
6. Reports must distinguish product failures from missing local toolchain dependencies.
7. A release report records the tested commit, environment, exact commands, outcomes, coverage, skipped checks, and residual risks.

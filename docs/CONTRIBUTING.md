# Contributing to CPA Orbit

Thank you for improving CPA Orbit. Keep changes focused, reproducible, and safe for a repository that handles local credentials.

## Before you start

1. Search existing issues and pull requests.
2. Use a bug report for reproducible defects and a feature request for product proposals.
3. Discuss broad persistence, security-boundary, or architecture changes before implementation.
4. Never attach or commit real subscription JSON, OAuth tokens, API keys, account lists, logs, or private screenshots.

## Development workflow

```powershell
git switch -c fix/short-description

# Backend
Set-Location server
..\.tools\go\bin\go.exe test ./...

# Desktop host
Set-Location ..\app
..\.tools\go\bin\go.exe test ./...
..\.tools\go\bin\go.exe vet ./...

# Frontend
Set-Location ..\web
npm ci
npm run build
npm run build:desktop
npm run test:e2e
```

Use focused branch names such as `fix/import-dialog`, `feat/tray-controls`, or `docs/architecture`. Rebase or merge the current target branch before requesting final review.

## Commit and pull request style

- Prefer Conventional Commit prefixes: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `build:`, or `chore:`.
- Keep unrelated behavior out of the same pull request.
- Explain the problem, user-visible result, security/privacy impact, and verification evidence.
- Add regression coverage for fixed behavior when practical.
- Update the root README, relevant guide, changelog, or ADR when public behavior changes.

## Engineering principles

- Archived subscriptions are the source of truth; CPA auth files are a rebuildable projection.
- Keep secret-bearing values in the Go backend and return only sanitized public models.
- Prefer explicit error states, bounded I/O, loopback defaults, and reversible changes.
- Preserve keyboard access, reduced-motion behavior, and non-color status cues.
- Add or update an ADR for changes to persistence, trust boundaries, external APIs, or source-of-truth rules.

## Pull request readiness

- [ ] Backend and desktop tests pass.
- [ ] Frontend production and desktop builds pass.
- [ ] Relevant E2E flows pass.
- [ ] No secret, runtime data, or private screenshot is included.
- [ ] Documentation and changelog are current.
- [ ] The pull request is small enough to review coherently.

Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md). Security vulnerabilities follow the private process in [SECURITY.md](SECURITY.md).

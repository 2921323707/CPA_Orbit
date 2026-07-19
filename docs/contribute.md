---
title: Contributing
description: Contribute code, documentation, tests, design, or feedback.
---

# Contributing

CPA Orbit welcomes focused, reproducible, and privacy-safe contributions.

## Good starting points

- Report a reproducible defect.
- Describe a feature through a real operating scenario.
- Correct an inaccurate guide or add troubleshooting evidence.
- Read the [architecture dossier](/architecture/) before changing persistence or trust boundaries.

## Development workflow

```powershell
git switch -c fix/short-description

cd server
..\.tools\go\bin\go.exe test ./...

cd ..\web
npm ci
npm run build
npm run test:e2e
```

## Before opening a pull request

- [ ] The change solves one clear problem.
- [ ] Relevant tests and builds pass.
- [ ] Public behavior and documentation agree.
- [ ] No real JSON, token, key, account list, log, or private screenshot is included.
- [ ] Persistence, trust-boundary, or external-API changes include an ADR.
- [ ] The pull request explains the problem, result, risk, and verification.

See the repository [contribution policy](/CONTRIBUTING), [Code of Conduct](/CODE_OF_CONDUCT), and [security reporting process](/SECURITY).

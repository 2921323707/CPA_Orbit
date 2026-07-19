## Summary

<!-- What problem does this PR solve? Keep this focused and reviewable. -->

## User-visible changes

<!-- Describe behavior, interface, API, configuration, or documentation changes. -->

## Technical approach

<!-- Explain important implementation choices and link the relevant issue or ADR. -->

Closes #

## Security and privacy impact

- [ ] No security or privacy impact
- [ ] Security/privacy impact is described below

<!-- Cover credentials, file paths, external requests, persistence, and trust boundaries. -->

## Verification

<!-- Include commands and concise results. Add sanitized screenshots for UI changes. -->

- [ ] Server tests: `cd server && go test ./...`
- [ ] Desktop tests and vet: `cd app && go test ./... && go vet ./...`
- [ ] Web production build: `cd web && npm run build`
- [ ] Desktop web build: `cd web && npm run build:desktop`
- [ ] Relevant E2E tests: `cd web && npm run test:e2e`
- [ ] Manual verification completed where automation is insufficient

## Documentation and release readiness

- [ ] Documentation is updated or not required
- [ ] Changelog is updated or not required
- [ ] ADR is added/updated for architecture, persistence, or trust-boundary changes
- [ ] No credentials, runtime data, logs, account identifiers, or private screenshots are included
- [ ] The change is backwards compatible, or migration notes are included

# Release-readiness and repository presentation design

## Goal

Prepare CPA Orbit for a professional GitHub repository update while preserving its local-first security model. The release surface must explain the product in English, show sanitized visuals, make architectural ownership obvious, and give contributors reproducible issue, pull request, build, and E2E paths.

## Design

The root README is a product entry point rather than an operations manual. It owns the logo, build and technology badges, concise feature framing, sanitized showcase, a high-level Mermaid diagram, recent updates, quick start, and links into the documentation system. Detailed material is categorized under `docs/architecture`, `docs/development`, `docs/releases`, `docs/community`, and `docs/assets`; GitHub-recognized policy files remain directly under `docs/` and are indexed by the community handbook.

The import regression is fixed at the interaction boundary. An optional acquisition price must not invoke a blocking browser-native confirmation in WebView2; the request begins immediately and uses in-page toast/progress feedback. A Playwright regression test mocks only the API boundary and asserts that selecting a single JSON file produces one multipart import request, a success message, and no JavaScript dialog. A second test protects Settings in-page navigation from hash-router regressions.

GitHub-native issue forms and a structured pull request template require reproduction, environment, security impact, test evidence, and secret-safety checks. CI independently verifies the Go server, Windows desktop host, Vue production/desktop builds, and browser E2E suite. Dependency updates are grouped by ecosystem through Dependabot.

## Security and failure handling

Public screenshots may contain synthetic data only. The source subscription screenshot containing real account email addresses is excluded from the repository and replaced by an isolated-runtime capture. CI and documentation must never depend on local credential files, CLIProxyAPI availability, or mutable user data.

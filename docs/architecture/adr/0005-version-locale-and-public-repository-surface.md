# ADR 0005: Version, locale, and public repository surface

## Status

Accepted — 2026-07-18

## Context

CPA Orbit is preparing for a public repository. Version labels, project identity, language selection, and repository links must not drift across pages or expose a broken destination before the repository exists.

## Decision

- The release version is `1.0.2`, recorded in the root `VERSION`, frontend package metadata, backend health response, and release documentation.
- The public product name is **CPA Orbit**. Internal module paths remain unchanged to avoid an unnecessary migration of stable Go imports and local scripts.
- A dependency-free Vue locale catalog owns the global shell, route labels, status messages, common pagination, and loading copy. Locale persists locally and updates the HTML `lang` attribute.
- `PROJECT_GITHUB_URL` is the single repository-link configuration point. An empty value renders an accessible disabled GitHub mark; a configured value automatically renders a safe external link.
- Detailed operational documentation remains available in Chinese while the global bilingual catalog expands incrementally. Release notes are complete in both Chinese and English from v1.0.2 onward.

## Non-functional requirements

- Language switching must not reload the page or change API behavior.
- No external localization runtime is required.
- Missing translation keys fall back to the Chinese catalog key rather than rendering an empty label.
- External repository links use a new tab with `noopener noreferrer`.

## Trade-offs and follow-up

The lightweight catalog avoids another runtime dependency but requires explicit message maintenance. Future feature work should add Chinese and English strings together; existing long-form operational copy can move into the catalog in small, reviewable batches.

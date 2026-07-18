# Contributing to CPA Orbit

感谢参与 CPA Orbit。English contributors are equally welcome; issues and pull requests may be written in Chinese or English.

## Development workflow

1. Create a focused branch and keep unrelated changes out of the same pull request.
2. Never commit CPA JSON, OAuth tokens, API keys, local settings, runtime data, logs, or screenshots containing account information.
3. Run backend tests from `server/` with `go test ./...`.
4. Run the frontend production check from `web/` with `npm run build`.
5. Describe user-visible behavior, security impact, test evidence, and documentation changes in the pull request.

## Style

- Prefer small, reversible changes and explicit error states.
- Keep the UI accessible without relying on color alone.
- Add or update an ADR for changes to source-of-truth rules, secret handling, external APIs, or persistence.
- User-facing Chinese and English copy should be updated together when the locale catalog covers that surface.

By contributing, you agree to follow [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

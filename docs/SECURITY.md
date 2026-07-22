# Security policy

## Supported version

Security fixes target the latest published release, currently **v1.2.1**, and the current `main` branch. The v1.3.0 control-plane work remains unreleased until it is verified, tagged, and published.

## Private reporting

Do not open a public issue for credential disclosure, path traversal, authentication bypass, SSRF, unsafe file writes, or secret-bearing logs. Use GitHub's [private vulnerability report](https://github.com/2921323707/CPA_Orbit/security/advisories/new) and provide:

- the affected version or commit;
- a minimal reproduction using synthetic credentials;
- expected and observed impact;
- any practical mitigation; and
- whether the issue is already public.

Do not include real tokens, subscription archives, account identifiers, or third-party API keys. The maintainer will acknowledge the report, validate severity, coordinate a fix, and publish an advisory when appropriate.

## Secret-handling rules

- Never commit `subscriptions/**/*.json`, `cpa/auths/**`, `data/*.json`, local configuration, logs, or `.env*` files. Provider/date archives and the local control-plane state belong in a protected backup, not a public artifact.
- The browser must never receive stored Sub2API/CPA management keys, Luban API keys, access tokens, refresh tokens, or ID tokens. Keys are write-only in the UI and API.
- Sub2API is an independently secured external service and is not bundled with CPA Orbit. Create its administrator key under **Sub2API System Settings → Admin API Key**, then save it through CPA Orbit's write-only gateway form.
- Remote gateway management targets require explicit opt-in and HTTPS; loopback targets remain preferred. Official CPA Orbit artifacts embed neither Sub2API nor CLIProxyAPI.
- Auth JSON goes through a local preflight and an explicit exactly-one compatible target selection; uncertain import outcomes stay on the selected target for reconciliation rather than triggering cross-target fallback.
- External URLs must be validated, redirects restricted, response sizes bounded, and errors sanitized.
- Imported filenames and archive paths must remain inside the intended project directories.
- Backups should be encrypted and access-controlled. Local settings and credential files may remain plaintext on disk unless the host filesystem provides encryption; treat them accordingly.
- Public screenshots and test fixtures must contain synthetic or fully redacted data.

## Scope

Third-party service and CLIProxyAPI vulnerabilities belong to their respective maintainers unless CPA Orbit introduces the issue through its integration, storage, proxying, or UI behavior.

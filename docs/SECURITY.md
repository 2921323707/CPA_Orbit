# Security policy

## Supported version

Security fixes target the latest release, currently **v1.2.0**, and the current `main` branch.

## Private reporting

Do not open a public issue for credential disclosure, path traversal, authentication bypass, SSRF, unsafe file writes, or secret-bearing logs. Use GitHub's [private vulnerability report](https://github.com/2921323707/CPA_Orbit/security/advisories/new) and provide:

- the affected version or commit;
- a minimal reproduction using synthetic credentials;
- expected and observed impact;
- any practical mitigation; and
- whether the issue is already public.

Do not include real tokens, subscription archives, account identifiers, or third-party API keys. The maintainer will acknowledge the report, validate severity, coordinate a fix, and publish an advisory when appropriate.

## Secret-handling rules

- Never commit `k12/**/*.json`, `cpa/auths/**`, `data/*.json`, local configuration, logs, or `.env*` files.
- The browser must never receive stored CPA management keys, Luban API keys, access tokens, refresh tokens, or ID tokens.
- External URLs must be validated, redirects restricted, response sizes bounded, and errors sanitized.
- Imported filenames and archive paths must remain inside the intended project directories.
- Public screenshots and test fixtures must contain synthetic or fully redacted data.

## Scope

Third-party service and CLIProxyAPI vulnerabilities belong to their respective maintainers unless CPA Orbit introduces the issue through its integration, storage, proxying, or UI behavior.

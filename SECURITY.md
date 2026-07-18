# Security Policy

## Supported version

Security fixes target the latest release, currently **v1.0.2**.

## Reporting

Do not open a public issue for vulnerabilities involving credential disclosure, path traversal, authentication bypass, SSRF, or secret-bearing logs. Until a repository security contact is configured, keep the report private and provide only a minimal reproduction without real tokens.

## Secret-handling rules

- Never commit `k12/**/*.json`, `cpa/auths/**`, `data/*.json`, local configuration, logs, or `.env*` files.
- The browser must never receive stored CPA management keys, Luban API keys, access tokens, refresh tokens, or ID tokens.
- External URLs must be validated, redirects restricted, response sizes bounded, and errors sanitized.
- Imported filenames and archive paths must remain inside the intended project directories.

## Scope

Third-party services and CLIProxyAPI vulnerabilities should be reported to their respective maintainers unless CPA Orbit introduces the issue through its integration.

# CPA Orbit Go backend

The local Monitor API is built with Go, `net/http`, and goquery. It owns price monitoring, alerts, provider/date subscription archives, independent account status/quota polling, public settings, gateway companion coordination, and the Luban SMS proxy.

## Run and test

From `server/`:

```powershell
..\.tools\go\bin\go.exe mod tidy
..\.tools\go\bin\go.exe test ./...
..\.tools\go\bin\go.exe run ./cmd/server
```

The default listener is `127.0.0.1:8090`. With the repository root as its project root, the runtime owns:

- `subscriptions/{sub2api,cpa}/MMDD/*.json` — archived subscription source of truth
- `data/offers.json` — K12 offer snapshot
- `data/gpt_plus_offers.json` — GPT Plus offer snapshot
- `data/price_history.json` — real collected price history
- `data/alerts.json` — low-price alerts
- `data/settings.json` — backend settings and local API keys
- `data/subscription_checks.json` — connectivity and quota results

Override the listener and project root when isolating a test runtime:

```powershell
..\.tools\go\bin\go.exe run ./cmd/server -addr 127.0.0.1:9090 -project-root C:\temp\cpa-orbit-test
```

`CPA_MONITOR_ADDR` can also set the listener; the command-line `-addr` value takes precedence.

## Primary endpoints

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/api/health` | Monitor API identity and health |
| `GET` | `/api/cpa/status` | Independent CLIProxyAPI status |
| `GET` / `POST` | `/api/offers`, `/api/offers/refresh` | K12 offers and refresh |
| `GET` / `POST` | `/api/gpt-plus`, `/api/gpt-plus/refresh` | GPT Plus offers and refresh |
| `GET` / `PUT` | `/api/settings` | Sanitized public settings |
| `POST` | `/api/settings/test-webhook` | Validate and test a webhook |
| `GET` | `/api/subscriptions` | Filtered, paginated archives |
| `POST` | `/api/subscriptions/import` | Multipart JSON import with optional `acquisitionPrice` |
| `POST` | `/api/subscriptions/{id}/test` | Connectivity and quota check |
| `POST` | `/api/subscriptions/{id}/sync` | Reconcile one CPA projection |
| `DELETE` | `/api/subscriptions/{id}` | Delete an archive and matching projection |
| `GET` | `/api/alerts` | Alert history |
| `GET` | `/api/dashboard` | Aggregated workspace summary |

Imports use a bounded local Auth JSON preflight before archive or remote writes. They are stored under `subscriptions/{sub2api,cpa}/MMDD` according to explicit provider/target intent, and an operator must choose exactly one compatible configured CPA or Sub2API companion. Existing filenames are never overwritten. There is no automatic fallback; pending or uncertain remote outcomes remain reconcilable. When CPA sync is explicitly selected and its auth directory is valid, the manager writes a sanitized provider projection that CLIProxyAPI can hot-load.

## Security boundaries

- The service binds to loopback by default. CORS accepts only loopback browser origins.
- API responses never serialize access, refresh, or ID tokens. Saved management and Luban keys are represented only by configured flags.
- JSON uploads are limited to 2 MiB. Filenames are cleaned, archive targets stay inside the owned directory, and symlinks are skipped or rejected.
- `baseUrl`, order URLs, and webhooks accept only HTTP(S) URLs without embedded credentials.
- Account status/quota polling is independent of offer monitoring, defaults to every five minutes, and is disabled when its interval is `0`.
- Gateway companions default to loopback targets. Remote base URLs require explicit opt-in and HTTPS; redirects are revalidated.
- CPA auth directories must already exist and be absolute. Resolved symlinks, target containment, and regular-file requirements are enforced.
- Webhooks and upstream API calls are explicit external boundaries. Never expose the Monitor API to an untrusted network without an additional authentication layer.

See the [architecture dossier](../architecture/README.md) for ownership and trust-boundary diagrams.

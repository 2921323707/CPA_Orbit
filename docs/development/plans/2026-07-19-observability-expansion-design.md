# Observability Expansion Design

## Context

CPA Orbit already owns subscription archives, acquisition prices, remaining days, connectivity checks, alerts, and a local desktop runtime. The [`seakee/CPA-Manager-Plus`](https://github.com/seakee/CPA-Manager-Plus) project demonstrates three adjacent product directions: persistent request observability, cost analytics, and controlled account-health automation.

CPA Orbit should adopt those ideas incrementally instead of duplicating a full CPA management panel. Its distinctive role remains a compact local-first desktop companion with shared browser data and straightforward operations.

## Considered approaches

1. **Subscription asset insights — selected first.** Aggregate existing local data into health, recorded cost, average acquisition price, and expiry-risk indicators. This provides immediate value without capturing prompts, responses, tokens, or new secrets.
2. **Persistent request history.** Store redacted CPA usage events and offer filters for model, account, latency, status, and time range. High value, but it needs retention limits, schema migrations, redaction guarantees, and bounded storage.
3. **Automated account actions.** Queue reauthorization, cooldown, restore, and review actions based on quota or provider failures. This must be owner-scoped, reversible, and opt-in so CPA Orbit never disables credentials it did not manage.

## First implementation slice

The subscriptions endpoint returns aggregate insights for the current filter set:

- healthy and abnormal account counts;
- subscriptions with recorded acquisition prices;
- total and average recorded cost;
- accounts expiring within seven days.

The frontend renders these metrics as a compact responsive strip above filters. The calculation stays server-side so pagination cannot distort totals. Tokens and raw subscription JSON never enter the response.

## Next phases

- Add a redacted diagnostics page with request counts, latency percentiles, failure categories, and retention controls.
- Add JSONL export/import for sanitized request history only after redaction tests exist.
- Add an opt-in account action queue with audit records, reversible cooldowns, and explicit automation ownership.
- Add backup/restore only with an encrypted format and clear separation between redacted diagnostics and credential-bearing archives.

## Verification

- Unit-test insight aggregation independently of pagination.
- E2E-test priced and unpriced imports, including the numeric Vue model boundary.
- Build both browser and desktop bundles.
- Verify the packaged Windows runtime and shared API before release.

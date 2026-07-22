# ADR-0007: Gateway targets and managed deployment bindings

## Status

Accepted

## Context

CPA Orbit originally treated archived subscription JSON as the durable source of truth and the local CLIProxyAPI auth directory as a rebuildable projection. Sub2API is a stateful gateway platform: imported accounts accumulate refreshed credentials, scheduling state, groups, quotas, billing data, and usage history. Treating its complete account database as a disposable projection would destroy state owned by Sub2API and could overwrite credentials refreshed after the original archive was created.

The same OAuth refresh credential must also not be active in CPA and Sub2API by default. Concurrent refreshers can rotate or invalidate each other's tokens.

## Decision

- CPA Orbit remains a local-first modular monolith and becomes the control plane for multiple gateway targets.
- A subscription archive owns acquisition metadata, account identity, provenance, and the initially imported credential document.
- A gateway target owns its runtime scheduling and refreshed credential state.
- A deployment binding records the desired and observed relationship between one subscription and one gateway target.
- Only objects explicitly marked `managed` may be changed or deleted automatically by CPA Orbit. Adopted or external objects are observed without destructive reconciliation.
- Refreshable OAuth credentials have at most one active runtime binding by default. Its role is `primary` or `fallback`; CPA is the lightweight fallback and Sub2API is the default primary target for new compatible imports. Switching targets uses migration rather than active-active copying.
- CPA and Sub2API are integrated through adapters. Neither management protocol is made to impersonate the other.
- Relational control-plane state and bounded aggregate usage snapshots are stored in local SQLite. Raw credential JSON remains in the protected archive and is never copied into public API responses or usage tables.

## Consequences

### Positive

- CPA and Sub2API can coexist without control-plane protocol coupling.
- Sub2API groups, usage history, billing, and refreshed tokens are not destroyed by archive reconciliation.
- Sync, migration, retry, and ownership state become auditable and recoverable.
- More gateway implementations can be added without expanding subscription parsing into provider-specific branches.

### Negative

- The local application gains a schema migration lifecycle and another bounded state file.
- Remote deployment becomes eventually consistent and requires explicit failure states.
- Moving a live OAuth credential between gateways requires a resumable migration instead of copying a JSON file twice.

### Neutral

- Sub2API remains authoritative for raw request logs. Orbit retains only normalized aggregates needed for its unified dashboard and historical continuity.

## Alternatives Considered

- **Make the subscription archive authoritative for every runtime field.** Rejected because refreshed credentials and Sub2API scheduling/billing state cannot be safely reconstructed from the original file.
- **Keep independent CPA and Sub2API pools.** Rejected because asset cost, expiry, health, and deletion would drift across tools.
- **Replace CPA entirely.** Rejected because CPA remains useful as a lightweight local fallback and migration source.

## References

- [ADR-0001](0001-subscriptions-as-runtime-source-of-truth.md)
- [Architecture dossier](../README.md)
- [Sub2API](https://github.com/Wei-Shaw/sub2api)

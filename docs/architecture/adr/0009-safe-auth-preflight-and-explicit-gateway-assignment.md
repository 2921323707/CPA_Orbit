# ADR 0009: Safe Auth JSON preflight and explicit gateway assignment

## Status

Accepted

## Context

The stabilization pass removes the dedicated pool-operations route and the external JSON conversion surface. Gateway setup now belongs to Settings, while the remaining subscription import path must make target ownership explicit before any credential is deployed. A refreshable Auth JSON can be valid for more than one runtime, but assigning it to multiple pools would create competing refreshers and ambiguous health or quota state.

Account status and quota checks also have a different cadence and failure model from offer monitoring. They must not be described as a side effect of loading a page or refreshing offers.

## Decision

- Gateway configuration is exposed from Settings at `?section=gateways`; there is no separate operations page or route.
- Auth JSON import uses two stages: a local, bounded preflight first validates the JSON shape, supported provider, credential identity, and archive destination; only an explicit deployment action may perform the remote write.
- A deployment must name exactly one compatible target: either a configured CPA companion or a configured Sub2API companion. Orbit never automatically falls back from one target to the other.
- A successful assignment gives the logical credential one active pool assignment. It is not copied into both pools. A pending or uncertain remote result remains visible for reconciliation and is not silently retried as another target.
- Provider/date archives remain durable provenance and recovery assets under `subscriptions/{sub2api,cpa}/MMDD/`; ADR 0008 continues to define that layout.
- Account status and quota polling is independent of offer monitoring. Its default interval is five minutes; setting the interval to `0` disables scheduled polling. Manual checks remain explicit.
- CPA and a generic local Sub2API companion are configurable local contracts. Their management keys are write-only and their remote endpoints must use HTTPS when they are not loopback.

## Consequences

- Operators can review preflight results before a credential leaves the local archive boundary.
- A failed, pending, or uncertain deployment cannot accidentally create an automatic dual-pool assignment or hide an unresolved binding.
- Offer schedules and account-health schedules can be tuned independently, including disabling account polling without disabling offer collection.
- Backups must include the provider/date subscription archives and local control-plane/settings state, while excluding generated runtime projections and secrets from public artifacts. Local settings and credential files remain plaintext unless the host filesystem provides encryption.

## Alternatives considered

- **Automatic CPA fallback after a Sub2API failure.** Rejected because it obscures target ownership and can activate the same refreshable credential twice.
- **Keep the pool-operations route.** Rejected because gateway configuration is a Settings concern and the separate page duplicated navigation and state.
- **Use an external JSON converter before import.** Rejected because conversion adds an unnecessary credential-bearing boundary; Luban remains available as the SMS toolbox.
- **Refresh account quota when offers or subscription pages load.** Rejected because unrelated page activity makes polling unpredictable and can increase upstream load.

## References

- [ADR 0007: Gateway targets and managed deployment bindings](0007-gateway-targets-and-managed-bindings.md)
- [ADR 0008: Provider-aware dated subscription archive](0008-provider-dated-subscription-archive.md)
- [Architecture dossier](../README.md)

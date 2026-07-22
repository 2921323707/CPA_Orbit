# ADR 0006: Shared desktop runtime and independent health boundaries

## Status

Accepted — 2026-07-19

## Context

The first desktop host could render the Vue application while relying on a separately started backend. This made double-click startup ambiguous and allowed browser and desktop clients to point at different mutable roots. Monitor health and CLIProxyAPI health were also checked inside one timeout boundary, so an unavailable proxy could make the embedded backend appear offline.

## Decision

- The Wails host constructs the same reusable Go application runtime used by the standalone server.
- Repository builds use the repository root for `data/` and `subscriptions/`; standalone builds use a user configuration directory unless explicitly overridden.
- The desktop host binds the Monitor API to `127.0.0.1:8090` or validates and reuses an existing compatible CPA Orbit listener.
- CLIProxyAPI discovery and startup are companion lifecycle concerns. The application stops only the process it started.
- Monitor API and CLIProxyAPI health checks use independent requests, timeouts, and UI states.
- Desktop-only behavior—tray, close-to-tray, startup-at-login, notifications, and taskbar flashing—reads shared settings but remains implemented by the native host.

## Consequences

### Positive

- Browser and desktop clients share subscriptions, settings, keys, alerts, and price history.
- Double-click startup produces a complete local control plane without an extra terminal.
- Proxy degradation no longer produces a false embedded-backend outage.
- The backend remains independently usable for browser development and automation.

### Negative

- Port 8090 becomes a local singleton and must be version-validated before reuse; Sub2API keeps its standard port 8080.
- Companion discovery differs between repository and standalone layouts.
- Native settings updates need a callback so startup and tray behavior can react without restarting.

## Alternatives considered

- Run a separate backend process for every desktop launch: rejected because it creates duplicate ownership and port conflicts.
- Store desktop data under a separate application directory even in repository builds: rejected because it breaks the shared-data requirement.
- Collapse all health into one aggregate flag: rejected because it hides which local dependency actually failed.

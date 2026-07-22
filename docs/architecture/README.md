# CPA Orbit architecture dossier

## 1. System intent

CPA Orbit is a local-first modular monolith. It deliberately uses one Go control plane for the browser and desktop clients so operational state cannot drift between two backends. The desktop host embeds the production Vue bundle, exposes the same API handler through Wails, and also publishes the Monitor API on loopback for the browser console. Archived subscription JSON remains the durable asset source; local SQLite records gateway targets, ownership, deployments, operations, and bounded usage aggregates. Sub2API owns its refreshed runtime account state and raw request history, while the CLIProxyAPI auth directory remains a rebuildable CPA projection.

### Non-functional requirements

| Quality | Target |
|---|---|
| Privacy | Secrets remain on the local machine and never enter public API responses |
| Availability | Desktop startup reuses a healthy local API and independently reports CPA degradation |
| Reliability | Archive writes are bounded, sanitized, deduplicated, and never overwrite existing files |
| Performance | UI navigation stays responsive while network checks and refresh jobs run asynchronously |
| Maintainability | One shared application runtime, explicit storage ownership, and recorded architecture decisions |
| Portability | Browser development plus lightweight Windows/macOS Wails packaging |
| Interoperability | Sub2API and CPA remain independent adapters instead of impersonating each other's management protocol |

## 2. Layered topology

```mermaid
flowchart TB
  subgraph PRESENTATION["01 · PRESENTATION"]
    direction LR
    BROWSER["Browser Workspace<br/><small>Vue 3 · Vite · History Router</small>"]
    DESKTOP["Native Workspace<br/><small>Wails · WebView · Hash Router</small>"]
  end

  subgraph CONTROL["02 · LOCAL CONTROL PLANE"]
    direction LR
    EDGE["API Edge<br/><small>CORS · validation · bounded upload</small>"]
    MONITOR["Monitor Engine<br/><small>prices · alerts · schedules</small>"]
		SUBS["Subscription Engine<br/><small>archive · dedupe · quota</small>"]
		ROUTER["Gateway Coordinator<br/><small>deploy · detach · migrate · rollback</small>"]
		TELEMETRY["Telemetry Collector<br/><small>snapshot · 15 min buckets</small>"]
    LUBAN["SMS Gateway<br/><small>secret-isolated upstream proxy</small>"]
  end

  subgraph PERSISTENCE["03 · OWNED LOCAL STATE"]
    direction LR
    SETTINGS[("Settings & Keys")]
    HISTORY[("Offers · History · Alerts")]
		ARCHIVE[("Subscription Archive")]
		CONTROLDB[("SQLite Control Plane")]
  end

	 subgraph RUNTIME["04 · RUNTIME GATEWAYS"]
    direction LR
    AUTH[("CPA Auth Pool")]
    CLI["CLIProxyAPI"]
		SUB2API["Sub2API<br/><small>configured companion · raw usage</small>"]
  end

  BROWSER -->|"127.0.0.1:8090/api"| EDGE
  DESKTOP -->|"embedded /api"| EDGE
	EDGE --> MONITOR & SUBS & ROUTER & LUBAN
  MONITOR <--> HISTORY
  MONITOR <--> SETTINGS
  SUBS <--> SETTINGS
  SUBS <--> ARCHIVE
	ROUTER <--> CONTROLDB
	ROUTER -->|"official admin API"| SUB2API
	ROUTER -->|"managed projection"| AUTH
	TELEMETRY --> SUB2API
	TELEMETRY --> CONTROLDB
  AUTH --> CLI
  EDGE -. "independent health probe" .-> CLI

  classDef presentation fill:#0e2028,stroke:#3fd7c5,color:#f4fffd,stroke-width:2px;
  classDef control fill:#e8fbf7,stroke:#0f8f82,color:#123b35,stroke-width:2px;
  classDef persistence fill:#fff8e7,stroke:#d69d2f,color:#49350f,stroke-width:2px;
	classDef projection fill:#f0efff,stroke:#7068d8,color:#29235b,stroke-width:2px;
  class BROWSER,DESKTOP presentation;
	class EDGE,MONITOR,SUBS,ROUTER,TELEMETRY,LUBAN control;
	class SETTINGS,HISTORY,ARCHIVE,CONTROLDB persistence;
	class AUTH,CLI,SUB2API projection;
```

## 3. Subscription import sequence

```mermaid
sequenceDiagram
  autonumber
  actor Operator
  participant UI as Vue Workspace
  participant API as Monitor API
  participant Guard as Import Guard
  participant Archive as Provider/Date Archive
	participant Coordinator as Gateway Coordinator
	participant Target as Explicit CPA/Sub2API Target

  Operator->>UI: Select one or more JSON files
  UI->>UI: Validate extension and show queue
	UI->>API: POST /subscriptions/import/preflight
  API->>Guard: Bound size, sanitize name, parse JSON
  Guard->>Archive: Compare canonical full-document identity
  Guard-->>UI: Signed preflight + compatible targets
  Operator->>UI: Select exactly one target
	UI->>API: POST /subscriptions/import/commit?targetId=...
  API->>Guard: Verify signed digest and target compatibility
  Guard->>Archive: Atomic provider/date archive write
  API->>Coordinator: Deploy archived credential
  Coordinator->>Target: Import through target admin contract
  alt First successful commit
    Target-->>Coordinator: Managed or adopted account ID
    API-->>UI: 201 + archive, binding, and outcome
  else Same successful operation submitted again
    API-->>UI: 200 + idempotent existing result
  else Retryable failure or uncertain transport outcome
    Coordinator-->>API: Persist sanitized outcome and binding
    API-->>UI: Typed error + archived=true + retryable flag
    opt Operator retries safely
      UI->>API: Re-submit same commit token, file, and target
      API->>Coordinator: Resume same durable operation
    end
  else Invalid, conflict, or definite failure
    API-->>UI: Typed sanitized 4xx/5xx result
  end
```

The optional acquisition price never gates the import. It is metadata and part of the durable operation's idempotency contract, not a confirmation boundary. The confirmation boundary is the second-stage explicit selection of exactly one compatible target after safe local Auth JSON preflight. A retryable commit resumes the same operation and target without creating another archive or switching gateways.

## 4. Trust boundaries and secret flow

```mermaid
flowchart LR
  USER["Operator"] --> UI["WebView / Browser UI"]
  UI -->|"public settings only"| API["Local Go API"]
  API -->|"read/write"| SECRET[("Local secret store")]
  API -->|"server-side credentials"| UPSTREAM["Approved upstream services"]
  API -->|"sanitized models"| UI

	BLOCKED["Never returned:<br/>OAuth tokens · Sub2API admin key · CPA key · Luban key"]
  SECRET -.-> BLOCKED

  classDef safe fill:#e8fbf7,stroke:#139486,color:#103a35,stroke-width:2px;
  classDef secret fill:#fff1f1,stroke:#d9534f,color:#541d1b,stroke-width:2px;
  classDef external fill:#eef1ff,stroke:#6c72d9,color:#292c62,stroke-width:2px;
  class USER,UI,API safe;
  class SECRET,BLOCKED secret;
  class UPSTREAM external;
```

## 5. Failure modes

| Failure | User-visible behavior | Recovery |
|---|---|---|
| Monitor API cannot bind port 8090 | Startup validates the existing listener; a non-CPA service is rejected | Free the port or start the configured CPA Orbit backend |
| Selected CPA/Sub2API target is unavailable during import | Archive remains safe; a sanitized failed or uncertain outcome identifies the original operation and target | Retry only when marked safe to resume the same operation; otherwise reconcile/detach before explicitly choosing another target |
| CLIProxyAPI is unavailable | Embedded backend remains online; CPA status is shown independently as offline | Start the companion runtime or correct its configured path |
| Upstream price/SMS service fails | Last valid snapshot remains available with a sanitized error state | Retry without inventing price or verification data |
| Import is invalid or duplicated | No archive overwrite; typed error appears in the UI | Correct the document or select a distinct credential file |
| CPA projection is removed | Subscription archive remains authoritative | Reconcile the projection from archived subscriptions |
| Account status/quota polling fails | Last valid status remains visible and is marked stale or uncertain | Retry manually or wait for the independent five-minute poller; interval `0` disables it |

## 6. Decisions and evolution

Significant persistence, trust-boundary, source-of-truth, or external API changes require an ADR under [`architecture/adr/`](adr/). The current system intentionally remains a modular monolith: splitting local services would increase deployment and synchronization risk without improving the single-operator workload.

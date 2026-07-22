<div align="center">
  <img src="app/build/appicon.png" width="112" alt="CPA Orbit logo" />
  <h1>CPA Orbit</h1>
  <p><strong>A local-first control plane for AI subscriptions, provider/date archives, account health, price intelligence, and configurable CPA or Sub2API companions.</strong></p>

  <p>
    <a href="http://165.154.205.54/cpa_orbit/">
      <img src="https://img.shields.io/badge/Documentation-Open_online_docs-0B6159?style=for-the-badge&amp;logo=readthedocs&amp;logoColor=white" alt="Open CPA Orbit online documentation" />
    </a>
    <a href="https://github.com/2921323707/CPA_Orbit/releases/tag/v1.3.0">
      <img src="https://img.shields.io/badge/Windows-Download_v1.3.0-2563EB?style=for-the-badge&amp;logo=windows11&amp;logoColor=white" alt="Download CPA Orbit v1.3.0 for Windows" />
    </a>
  </p>
  <p><sub>English · 简体中文 · Searchable guides · Architecture · Deployment · Release notes</sub></p>

  <p>
    <a href="https://github.com/2921323707/CPA_Orbit/actions"><img src="https://img.shields.io/badge/CI-configured-2088FF?style=flat-square&amp;logo=githubactions&amp;logoColor=white" alt="CI configured" /></a>
		<a href="https://github.com/2921323707/CPA_Orbit/releases"><img src="https://img.shields.io/badge/version-v1.3.0-2563EB?style=flat-square" alt="Version 1.3.0" /></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-22C55E?style=flat-square" alt="MIT License" /></a>
    <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat-square&amp;logo=go&amp;logoColor=white" alt="Go 1.25 or newer" /></a>
    <a href="https://vuejs.org/"><img src="https://img.shields.io/badge/Vue-3.5-42B883?style=flat-square&amp;logo=vuedotjs&amp;logoColor=white" alt="Vue 3.5" /></a>
    <a href="https://wails.io/"><img src="https://img.shields.io/badge/Wails-2.13-DF0000?style=flat-square" alt="Wails 2.13" /></a>
  </p>

  <p>
    <a href="#overview">Overview</a> ·
    <a href="#showcase">Showcase</a> ·
    <a href="#architecture">Architecture</a> ·
    <a href="#quick-start">Quick start</a> ·
    <a href="docs/CONTRIBUTING.md">Contributing</a>
  </p>
</div>

## Overview

CPA Orbit brings the operational paths that normally live across scripts, browser tabs, and local folders into one coherent workspace. The desktop application and browser console share the same Go runtime, settings, credentials, subscription archive, alerts, and price history. Everything stays local by default and the network services bind to loopback interfaces.

- **Subscription assets** — two-stage safe Auth JSON preflight, canonical-content deduplication, provider/date archive management, explicit one-target assignment, and account status/quota checks.
- **Gateway companions** — Explicitly assign each compatible Auth JSON to exactly one configured local CPA or Sub2API companion; write-only admin keys and safe managed deletion preserve ownership.
- **Token telemetry** — Sub2API snapshots plus normalized 15-minute request, Token, latency, and cost aggregates retained locally for up to 90 days.
- **Price intelligence** — K12 and GPT Plus offer collection, current inventory, threshold alerts, and truthful 14-day average-price history.
- **CPA runtime control** — automatic CLIProxyAPI discovery and startup, live health state, auth-pool projection, and shared endpoint visibility.
- **Desktop integration** — compact Wails host, system tray, close-to-tray behavior, native notifications, taskbar alerts, and startup-at-login.
- **SMS workflow** — backend-protected Luban key, country/service discovery, balance, number acquisition, three-second verification polling, and release.
- **Focused interface** — responsive web layout, fixed 1280×800 desktop composition, Auto/Light/Dark themes, accessible states, and restrained loading motion.

## Showcase

<p align="center">
  <a href="docs/assets/showcase/showcase-grid.png">
    <img src="docs/assets/showcase/showcase-grid.png" width="100%" alt="Labeled CPA Orbit showcase with uncropped Overview, GPT Plus and SMS, Subscriptions, and Documentation screenshots" />
  </a>
</p>

> Showcase data is synthetic. Credential-bearing JSON, tokens, account lists, and private runtime screenshots are never committed.

## Architecture

```mermaid
flowchart LR
  subgraph UX["Experience Layer"]
    WEB["Browser Console<br/>Vue 3 + Vite"]
    APP["Desktop Console<br/>Wails + WebView2"]
  end

  subgraph CORE["Local Control Plane · 127.0.0.1:8090"]
    API["Monitor API<br/>Go · net/http"]
    MON["Price & Alert Monitor"]
		SUB["Subscription Assets"]
		ROUTE["Gateway Coordinator"]
		OBS["Usage Collector"]
    SMS["Luban SMS Proxy"]
  end

  subgraph STATE["Local Source of Truth"]
    DATA[("data/<br/>settings · history · alerts")]
		ARCH[("subscriptions/<br/>sub2api|cpa / MMDD")]
		DB[("control-plane.db<br/>targets · bindings · usage")]
  end

	 subgraph RUNTIME["Runtime Gateways"]
    SUB2["Sub2API<br/>configured companion"]
    CPA["CLIProxyAPI<br/>127.0.0.1:8317"]
    AUTH[("cpa/auths/")]
  end

  WEB -->|"HTTP /api"| API
  APP -->|"same-origin /api"| API
	API --> MON & SUB & ROUTE & SMS
  MON <--> DATA
  SUB <--> ARCH
	ROUTE <--> DB
	ROUTE -->|"admin API"| SUB2
	ROUTE -->|"managed projection"| AUTH
	OBS --> SUB2 & DB
  AUTH --> CPA
  API -->|"health & quota"| CPA

  classDef client fill:#10262d,stroke:#36c2b4,color:#f4fffd,stroke-width:2px;
  classDef service fill:#e8f8f5,stroke:#138a7e,color:#12352f,stroke-width:2px;
  classDef store fill:#fff8e8,stroke:#d59b2d,color:#4c3510,stroke-width:2px;
  classDef runtime fill:#eef2ff,stroke:#5c6ac4,color:#222a57,stroke-width:2px;
  class WEB,APP client;
	class API,MON,SUB,ROUTE,OBS,SMS service;
	class DATA,ARCH,DB store;
	class SUB2,CPA,AUTH runtime;
```

The provider/date archive under `subscriptions/{sub2api,cpa}/MMDD/` is the durable subscription asset source. The current two-stage import flow performs a safe local Auth JSON preflight before the operator explicitly selects exactly one compatible CPA or Sub2API target; that flow never switches to another target after a failed or uncertain deployment. SQLite records ownership and desired/observed bindings without becoming another credential store. Older targetless deployment actions remain compatibility paths and should not be confused with the explicit import contract. See the [architecture dossier](docs/architecture/README.md), [gateway guide](docs/guide/sub2api-pool.md), and [ADR 0009](docs/architecture/adr/0009-safe-auth-preflight-and-explicit-gateway-assignment.md).

## Recent updates

- Moved CPA/Sub2API gateway configuration into **Settings → Gateways** (`?section=gateways`) and removed the dedicated operations route.
- Added two-stage safe Auth JSON preflight and explicit exactly-one compatible-target assignment; the import commit never changes targets after failure, and pending/uncertain results remain visible for same-target reconciliation.
- Retained provider/date archives and one logical credential assignment per active pool, with account status/quota polling independent from offer monitoring (five minutes by default; `0` disables it).
- Unified K12 and unverified GPT Plus offers in a compact Price workspace with five-row pagination and direct checkout links.
- Added deletable price-history samples with immediate chart re-rendering, single-source trend views, and corrected K12 collection filters.
- Kept the Toolbox focused on Luban SMS operations; the external subscription JSON converter has been removed.
- Integrated Alerts into independent Settings subpages and capped history at ten records with five-row pagination.
- Unified desktop and browser data, settings, secrets, subscription state, and backend health reporting.
- Added one-click desktop startup for the Monitor API and CLIProxyAPI, plus tray, notifications, taskbar flashing, and startup-at-login controls.
- Added Auto/Light/Dark appearance modes and a stable fixed-size desktop window without resize polling.
- Decoupled Monitor and CLIProxyAPI health checks to prevent false offline status.
- Rebuilt Settings navigation as stable in-page controls and removed route/hash interference.
- Fixed optional-price imports across the Vue numeric model and WebView2 upload boundary; imports now start immediately with bounded requests and guaranteed action recovery.
- Added subscription asset insights for account health, recorded cost, average acquisition price, and seven-day expiry risk.
- Improved route loading, skeleton states, endpoint visibility, shared status feedback, and responsive layouts.
- Added Playwright regression coverage, GitHub CI, structured issue/PR templates, an English-only README, and a categorized documentation system.

See the complete [changelog](docs/releases/CHANGELOG.md).

## Quick start

### Prerequisites

- Go 1.25 or newer
- Node.js 20 or newer with npm
- Windows 10/11 with WebView2 for the desktop executable
- A separately installed CLIProxyAPI runtime only when CPA proxy features are required
- A separately deployed Sub2API service and its administrator API key when Sub2API is configured; CPA Orbit does not install or bundle Sub2API

### Development workspace

```powershell
git clone https://github.com/2921323707/CPA_Orbit.git
cd CPA_Orbit
.\start-dev.ps1
```

| Service | Local endpoint |
|---|---|
| Web console | `http://127.0.0.1:5173/` |
| Monitor API | `http://127.0.0.1:8090/api` |
| CLIProxyAPI | `http://127.0.0.1:8317/v1` |
| Sub2API | External service configured by the operator; Docker on `http://127.0.0.1:8080` is the common local setup |
| In-app guide | `http://127.0.0.1:5173/docs` |

### Windows desktop build

```powershell
.\app\build-windows.ps1
```

The portable executable is written to `app/build/bin/CPAOrbit.exe`. A repository build shares the root `data/` and `subscriptions/` directories with the browser console and can discover the repository-local CLIProxyAPI runtime. Official packages embed neither CLIProxyAPI nor Sub2API; external companions must be installed and started separately.

### macOS Apple Silicon build

```bash
CPA_ORBIT_MAC_ARCH=arm64 ./app/build-macos.sh
```

On an Apple Silicon Mac, this writes a native `CPA Orbit.app`, ZIP, drag-to-install DMG, and SHA-256 checksums to `app/build/bin`. Pull requests and `main` updates build the ARM64 package on a native GitHub Actions runner; `v*` tags publish the package to GitHub Releases.

## Verification

```powershell
# Backend
.\.tools\go\bin\go.exe -C server test ./...

# Desktop host
.\.tools\go\bin\go.exe -C app test ./...

# Frontend production build and browser regression suite
cd web
npm ci
npm run build
npx playwright install chromium
npm run test:e2e
```

## Documentation

Browse the complete, searchable documentation at **[165.154.205.54/cpa_orbit](http://165.154.205.54/cpa_orbit/)**. The repository links below remain available for offline reading and source review.

| Area | Guide |
|---|---|
| Online documentation | **[Open documentation site](http://165.154.205.54/cpa_orbit/)** |
| Architecture and ADRs | [docs/architecture](docs/architecture/README.md) |
| Desktop development and distribution | [docs/development/desktop.md](docs/development/desktop.md) |
| Backend API and security boundaries | [docs/development/backend.md](docs/development/backend.md) |
| Releases and changelog | [docs/releases](docs/releases/CHANGELOG.md) |
| Contribution and community policies | [docs/community](docs/community/README.md) |
| Complete documentation index | [docs/README.md](docs/README.md) |

## Security

CPA Orbit is local-first, not credential-free. Never commit or share CPA/Sub2API JSON, OAuth tokens, administrator keys, `data/`, provider/date `subscriptions/`, `cpa/auths/`, logs, or screenshots containing account information. Keys are write-only; remote targets require explicit approval and HTTPS, while loopback is preferred. Back up both `data/` and `subscriptions/` using encryption and access controls. Local credentials may otherwise remain plaintext on disk unless the host filesystem encrypts them. Review the [security policy](docs/SECURITY.md) before exposing an endpoint or redistributing a build.

## Data sources and acknowledgements

Offer and price data comes from [PriceAI](https://priceai.cc/). Checkout redirects and order lookup use [LXDP](https://pay.ldxp.cn/). CPA Orbit aggregates, records, and redirects only; source platforms remain authoritative for live prices, inventory, payment, and after-sales terms.

## License

Original CPA Orbit source code is available under the [MIT License](LICENSE). Bundled or referenced third-party components retain their own licenses; see the [third-party notices](docs/THIRD_PARTY_NOTICES.md).

# CPA Orbit Desktop

This directory contains the lightweight Wails desktop host for the existing Go backend and Vue frontend. It uses the operating system WebView, embeds the production frontend, and routes `/api` directly to the Go handler without opening port 8080.

## Windows one-click build

Double-click `build-windows.cmd`, or run:

```powershell
.\app\build-windows.ps1
```

The portable executable is written to `app/build/bin/CPAOrbit.exe`. To build a per-user NSIS installer after installing NSIS:

```powershell
.\app\build-windows.ps1 -Installer
```

Requirements: Go 1.25+, Node.js/npm, and WebView2 on the target computer. The script automatically uses the repository-local Go toolchain when it exists. Wails is pinned to `v2.13.0`; no global Wails installation is required.

## macOS one-command build

Run on a Mac with Go 1.25+, Node.js/npm, and Xcode Command Line Tools:

```bash
./app/build-macos.sh
```

The script builds the current machine architecture and writes `CPAOrbit.app` plus a distributable ZIP to `app/build/bin`. Set `CPA_ORBIT_MAC_ARCH=universal` to build a universal application. For signing, set `CPA_ORBIT_CODESIGN_IDENTITY` to the signing identity before running the script. Apple notarization is intentionally left to the release environment because it requires private Apple credentials.

## Configuration and data migration

By default, mutable files are stored under the current user's application configuration directory:

- Windows: `%AppData%\CPA Orbit`
- macOS: `~/Library/Application Support/CPA Orbit`

The directory contains the existing portable layout (`data/` and `k12/`). To migrate to another computer or operating system, close CPA Orbit, copy this directory, and point the new installation at it.

For portable or custom storage, copy `cpa-orbit.config.example.json` next to the executable/app launcher as `cpa-orbit.config.json` and edit `dataDir`. A relative `dataDir` is resolved relative to the configuration file, so the EXE, configuration, and data directory can be moved together.

Environment overrides are also supported:

- `CPA_ORBIT_CONFIG`: absolute or relative path to a required desktop JSON configuration file.
- `CPA_ORBIT_DATA_DIR`: data-directory override; takes precedence over the JSON file.

The desktop configuration is deliberately separate from `data/settings.json`. The latter remains managed by the Settings screen and stores monitor/CLIProxyAPI settings. Secret-bearing repository-local files are never copied into a build.

## CLIProxyAPI

The desktop package remains lightweight and does not bundle the ignored, Windows-only local CLIProxyAPI executable. Configure an existing local or remote CLIProxyAPI instance in Settings. The default URL remains `http://127.0.0.1:8317/v1`, and remote addresses require the existing explicit opt-in.

## Development

From `app/`:

```powershell
go run github.com/wailsapp/wails/v2/cmd/wails@v2.13.0 dev
```

The Wails project uses `../web` as its frontend source. Desktop builds use hash routing and same-origin `/api`; ordinary browser development continues to use history routing and `http://127.0.0.1:8080/api`.

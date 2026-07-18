# Mature CLIProxyAPI migration

- Source runtime: `M:\Utilities\CPA`
- Migrated on: 2026-07-18
- CLIProxyAPI: `7.2.71`
- Commit: `5b7f2361`
- Build time: `2026-07-12T16:35:23Z`
- Runtime platform: Windows x64

## Migrated

- Mature `cli-proxy-api.exe` and its official runtime documentation.
- Private `config.yaml`, with only `auth-dir` rewritten to this project's `cpa/auths` directory.
- 75 top-level production Codex credential JSON files.
- 3 historical credential backups, kept separately under `cpa/backups`.
- Original `cpa.ps1` and `cpa.cmd`, retained under `cpa/legacy-scripts` for reference.
- Legacy Chat UI `pricing.json` and `usage.json`, retained under `cpa/legacy-data`; the old UI itself was not copied because the Vue console replaces it.

## Deliberately not migrated

- Old runtime logs. They contained authorization-related material and are not needed to operate the merged service.
- The 3,605-file source checkout. It remains at the source location for audit/upgrade reference and is not a runtime dependency.
- Three unmatched `k12` archive files were not automatically synced because they lacked refresh tokens and were already marked expired.

## Data notes

- Six of the nine existing `k12` archive credentials matched the mature production pool by email and account ID.
- The mature pool contains one duplicated account ID represented by two files. Both were preserved; no credential was deleted automatically.
- The old directory is intentionally retained as rollback material until the merged service has been operated successfully.

Sensitive runtime files are ignored by Git and protected with restricted Windows ACLs.

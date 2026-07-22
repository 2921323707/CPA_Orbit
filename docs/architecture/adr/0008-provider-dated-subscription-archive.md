# ADR 0008: Provider-aware dated subscription archive

## Status

Accepted

## Context

The original archive lived directly under `k12/MMDD`. That name described one acquisition channel, not the runtime destination, and made it difficult to distinguish Sub2API-first imports from CPA-only archives. A flat archive root also encouraged operational files and runtime projections to be mixed together.

## Decision

Use the repository-level `subscriptions/` directory with this layout:

```text
subscriptions/
├── sub2api/MMDD/*.json
└── cpa/MMDD/*.json
```

Imports explicitly assigned to a Sub2API companion are archived under `sub2api/MMDD`; imports explicitly assigned to the CPA path use `cpa/MMDD`. The archive remains the durable source document. SQLite deployment bindings remain the authoritative record of the actual single active runtime target, so a file's folder is provenance/intent rather than a duplicate runtime copy.

On startup, the application migrates legacy `k12/MMDD` entries into `subscriptions/sub2api/MMDD`. Migration refuses symlinks and destination collisions and never overwrites an existing file.

## Alternatives considered

- Keep `k12/` and add metadata: rejected because the root name remains misleading and filesystem operations cannot be organized by runtime path.
- Duplicate each JSON into both `sub2api/` and `cpa/`: rejected because two mutable copies would drift and could be mistaken for two independent credentials.
- Put archives under `data/`: rejected because `data/` is runtime state (SQLite, checks, offers, and alerts), while subscription JSON is a durable user asset that should remain portable and auditable.

## Consequences

- New imports and UI folder filters expose `sub2api/MMDD` and `cpa/MMDD` directly.
- Existing `k12/` layouts are migrated once without changing credential bytes.
- Backup and transfer instructions must include `subscriptions/` alongside `data/`.

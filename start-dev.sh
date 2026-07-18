#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
trap 'kill 0' EXIT
(cd "$ROOT/server" && go run ./cmd/server) &
(cd "$ROOT/web" && npm run dev) &
wait

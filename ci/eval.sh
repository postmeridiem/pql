#!/usr/bin/env bash
# CI ranking-quality eval entry. Scheduled job; not blocking. Regressions
# surface as visible drift in the metrics record (whatever telemetry is wired).
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> ranking-quality eval"
go test -tags=eval ./internal/connect/rank/... -v

# TODO: post metrics to wherever telemetry lives (file in artefacts? remote?).
# Wired once the eval harness produces a report file.

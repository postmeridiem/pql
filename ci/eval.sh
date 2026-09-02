#!/usr/bin/env bash
# Ranking-quality eval. A MANUAL, LOCAL tool — `make eval` is its only caller
# and no workflow runs or schedules it (T-70).
#
# The header used to call this a scheduled job, which was never true and read
# as a promise that ranking regressions would be caught automatically. They are
# not. Run this yourself when changing a signal, a weight, or the candidate
# generator; treat the numbers as a diff against the previous run rather than
# as a pass/fail gate.
#
# Scheduling it is a decision to take once there is somewhere for the metrics
# to go and the golden set is green — neither is true today. See T-120.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> ranking-quality eval"
go test -tags=eval ./internal/connect/rank/... -v

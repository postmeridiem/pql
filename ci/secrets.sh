#!/usr/bin/env bash
# Secret and PII scan over the commits about to be pushed.
set -euo pipefail

cd "$(dirname "$0")/.."

# Unlike ci/lint.sh, a missing tool is a skip and not a failure. The pre-push
# hook here is opt-in, this repo is public, and a contributor who opts in
# should not be blocked by a tool they never agreed to install. The skip is
# printed rather than silent — an unannounced skip reads as a clean scan,
# which is the failure this whole check exists to prevent.
if ! command -v gitleaks >/dev/null; then
  echo "ci/secrets.sh: skip — gitleaks not on PATH (brew install gitleaks)" >&2
  exit 0
fi

# Scan the outgoing range, NOT full history.
#
# History carries findings that were resolved by removing the file from
# tracking rather than by rewriting the past: .pql/hooks/* held an absolute
# home-directory path back when they were tracked, and the T-25 changelog
# entry quoted one. Those are immutable and deliberate. A full-history scan
# reports 28 of them and fails every push forever, and a gate that can never
# pass gets bypassed within a week — strictly worse than no gate.
#
# What matters is what is about to become public, which is exactly this range.
if upstream=$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null); then
  range="$upstream..HEAD"
elif git rev-parse --verify --quiet origin/main >/dev/null; then
  range="origin/main..HEAD"
else
  range=""
fi

echo "==> gitleaks ${range:-(working tree)}"

if [ -z "$range" ]; then
  # No upstream to diff against — a fresh clone or a detached checkout. Scan
  # the working tree instead of falling back to full history, which would
  # fail for the reason above.
  gitleaks dir . --redact --no-banner --exit-code 1
  exit $?
fi

if [ -z "$(git log --oneline "$range" 2>/dev/null)" ]; then
  echo "    nothing to push"
  exit 0
fi

# .gitleaks.toml is picked up automatically when present. It is untracked by
# design — see .git/info/exclude — because the rules in it name the things
# they guard, and this repo is public.
gitleaks git . --log-opts="$range" --redact --no-banner --exit-code 1

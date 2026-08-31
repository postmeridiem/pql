#!/usr/bin/env bash
# Secret and PII scan over the commits about to be pushed.
set -euo pipefail

cd "$(dirname "$0")/.."

# Unlike ci/lint.sh, a missing tool is a skip and not a failure. The pre-push
# hook here is opt-in, this repo is public, and a contributor who opts in
# should not be blocked by a tool they never agreed to install. The skip is
# printed rather than silent — an unannounced skip reads as a clean scan,
# which is the failure this whole check exists to prevent.
#
# The same instinct governs the optional local ruleset below: absent, it is
# announced rather than assumed. What is NOT optional is .gitleaks.toml, which
# is committed — if that has gone missing the scan would silently fall back to
# the bare defaults, so it is checked and refused.
if ! command -v gitleaks >/dev/null; then
  echo "ci/secrets.sh: skip — gitleaks not on PATH (brew install gitleaks)" >&2
  exit 0
fi

# --- Which ruleset ---------------------------------------------------------
#
# Two layers, split by whether a rule can survive being published.
#
#   .gitleaks.toml         COMMITTED. Every rule is a SHAPE — home paths,
#                          sibling-checkout paths, RFC1918, IBAN, cards, SSN,
#                          BSN, email — so publishing it reveals nothing it
#                          guards. This is the floor, and everyone gets it:
#                          contributors, CI, a fresh clone on a new machine.
#
#   .gitleaks.local.toml   OPTIONAL, gitignored, per-machine. Rules that must
#                          NAME what they guard — this environment's hostnames
#                          and the private repos beside it. A rule has to
#                          contain the string it matches, so committing these
#                          would publish exactly what they exclude.
#
# The local layer is REQUIRED BY NOBODY. Absent it, the generic rules run and
# the scan says so. That is a deliberate reversal of an earlier draft that
# failed closed here, and the reasoning is worth keeping straight, because the
# same word — "missing config" — described two very different situations:
#
#   Before: a missing config meant NO rules at all, because every rule lived
#           in one untracked file. The gate reported clean while checking
#           nothing, for months. Silence about that is a lie, so refusing was
#           the right response.
#
#   Now:    a missing local file means the generic rules ran and the hostname
#           rules did not. The floor is committed, so the degradation is
#           partial, bounded and announceable. Refusing would block every
#           contributor over a file that is not theirs to have and that
#           protects a machine that is not theirs either.
#
# So: this system is protected by its own local file; anyone else's system is
# their business, and they still get the generic rules for free.
config=".gitleaks.toml"
scope="generic rules"

if [ -f .gitleaks.local.toml ]; then
  config=".gitleaks.local.toml"   # extends .gitleaks.toml
  scope="generic + local environment rules"
fi

if [ ! -f .gitleaks.toml ]; then
  echo "ci/secrets.sh: FAIL — .gitleaks.toml is missing from the repo root" >&2
  echo "  It is committed, and the local layer extends it, so neither works" >&2
  echo "  without it. Restore it rather than scanning with the bare defaults," >&2
  echo "  which are a secrets scanner and not a PII scanner." >&2
  exit 1
fi

# Prove the ruleset still matches what it claims BEFORE trusting a clean result
# from it. A scanner config fails silently by construction — a broken rule
# returns no findings, which is indistinguishable from a clean repo and reads as
# a pass. Running the selftest here rather than as a separate gate means the
# proof and the use cannot drift apart, and it costs single-digit milliseconds.
# It is inside the gitleaks-present branch, so a contributor without the tool
# still skips cleanly rather than being blocked by a test they cannot run.
./ci/secrets-selftest.sh

# --- Which commits ---------------------------------------------------------
#
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
elif git rev-parse --verify --quiet origin/main >/dev/null 2>&1; then
  range="origin/main..HEAD"
else
  range=""
fi

# Say which ruleset ran, every time and not only when something is wrong. The
# scope is the difference between "clean" meaning something and meaning nothing,
# so it belongs next to the result rather than in a doc nobody reads at push
# time.
echo "==> gitleaks ${range:-(working tree)} [${scope}]"

if [ -z "$range" ]; then
  # No upstream to diff against — a fresh clone or a detached checkout. Scan
  # the working tree instead of falling back to full history, which would
  # fail for the reason above.
  gitleaks dir . --config "$config" --redact --no-banner --exit-code 1
  exit $?
fi

if [ -z "$(git log --oneline "$range" 2>/dev/null)" ]; then
  echo "    nothing to push"
  exit 0
fi

gitleaks git . --log-opts="$range" --config "$config" --redact --no-banner --exit-code 1

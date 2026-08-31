#!/usr/bin/env bash
# Proves .gitleaks.toml still matches what it claims to match.
#
# A scanner config is the one kind of code whose failure mode is silence. A
# broken rule does not error, it returns clean — indistinguishable from a clean
# repo, and strictly more dangerous, because the gate reports success for a
# question it never asked. Three separate ways to write a rule that matches
# nothing were hit while writing this config, two of them silent; see the notes
# in .gitleaks.toml. This test is what keeps them from coming back.
#
# It asserts counts per rule, in BOTH directions. A rule that stops matching
# and a rule that starts over-matching are both regressions, and the expected
# totals below are the contract.
set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v gitleaks >/dev/null; then
  echo "ci/secrets-selftest.sh: FAIL — gitleaks not on PATH (brew install gitleaks)" >&2
  echo "  Unlike ci/secrets.sh this is not skippable: the point of this script" >&2
  echo "  is to prove the ruleset works, and it cannot prove that without the" >&2
  echo "  scanner. A skip here would be the same silent pass it exists to stop." >&2
  exit 1
fi

config="$PWD/.gitleaks.toml"
fixture="$PWD/ci/testdata/gitleaks-fixture.txt"

for f in "$config" "$fixture"; do
  [ -f "$f" ] || { echo "ci/secrets-selftest.sh: FAIL — missing $f" >&2; exit 1; }
done

# Scan a COPY, at a path the config does not exempt. The real fixture path is
# allowlisted in .gitleaks.toml — it is wall-to-wall planted samples, so the
# push gate would flag it on every commit — and scanning it in place would mean
# this test always passed with zero findings, which is precisely the shape of
# bug it exists to catch.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cp "$fixture" "$tmp/subject.txt"

report="$tmp/report.json"
# --exit-code 0 because findings ARE the expected result here; this script
# decides pass or fail from the counts, not from gitleaks' status.
#
# The output is captured and echoed on failure rather than discarded. Discarding
# it cost an hour: a config error (`invalid regex secret group 1, max regex
# secret group 0`) became a bare exit 1 under `set -e`, with the trap cleaning
# up and nothing printed at all. A test that cannot say why it failed is barely
# better than one that does not fail.
if ! gitleaks_output=$(gitleaks dir "$tmp" --config "$config" --no-banner \
      --exit-code 0 --report-format json --report-path "$report" 2>&1); then
  echo "ci/secrets-selftest.sh: FAIL — gitleaks could not run against the config" >&2
  echo "$gitleaks_output" >&2
  exit 1
fi

if [ ! -s "$report" ]; then
  echo "ci/secrets-selftest.sh: FAIL — gitleaks wrote no report" >&2
  echo "$gitleaks_output" >&2
  exit 1
fi

# rule=expected-hits. Update deliberately when the fixture changes.
expected="\
pql-absolute-home-path=2
pql-sibling-checkout-path=2
pql-private-ip=3
pql-iban=1
pql-credit-card=3
pql-us-ssn=1
pql-nl-bsn=3
pql-personal-email=1"

fail=0
total_expected=0

while IFS='=' read -r rule want; do
  total_expected=$((total_expected + want))
  got=$(grep -c "\"RuleID\": *\"${rule}\"" "$report" || true)
  if [ "$got" -ne "$want" ]; then
    if [ "$got" -eq 0 ]; then
      echo "FAIL ${rule}: matched nothing (expected ${want}) — the rule is dead, not the fixture" >&2
    else
      echo "FAIL ${rule}: ${got} findings, expected ${want}" >&2
    fi
    fail=1
  fi
done <<< "$expected"

# Catches over-matching that the per-rule counts cannot: a negative sample
# firing a rule it was written to prove is ignored shows up only here.
total_got=$(grep -c '"RuleID":' "$report" || true)
if [ "$total_got" -ne "$total_expected" ]; then
  echo "FAIL total: ${total_got} findings, expected ${total_expected}" >&2
  echo "  A negative sample fired, or a rule matched more than intended." >&2
  echo "  Report: $report" >&2
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  echo "" >&2
  echo "ci/secrets-selftest.sh: .gitleaks.toml no longer matches what it claims." >&2
  echo "  Re-read the three silent-breakage notes at the top of .gitleaks.toml" >&2
  echo "  before assuming the fixture is wrong." >&2
  exit 1
fi

echo "==> gitleaks selftest: ${total_got} findings across 8 rules, as expected"

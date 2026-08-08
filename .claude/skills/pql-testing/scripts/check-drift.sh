#!/usr/bin/env bash
# Compare the three copies of the pql skill that can disagree.
#
# Neither `pql skill status` nor `pql doctor` can detect this: both compare the
# installed file against the binary's own embedded copy, so an edit to the repo
# file that has not been rebuilt reads as "current" everywhere.
#
#   repo      internal/skill/SKILL.md — what has been written
#   shipped   embedded in the pql on PATH — what a consumer would get
#   installed ~/.claude/skills/pql/SKILL.md — what this machine is using
#
# Exit 0 when all three agree, 1 otherwise.
set -euo pipefail

cd "$(dirname "$0")/../../../.."

hash_of() { sha256sum "$1" | cut -c1-16; }

repo=$(hash_of internal/skill/SKILL.md)

# `skill show --raw` writes the file's bytes and nothing else, so the hash is a
# straight sha256sum. This used to extract the body from the JSON bundle with
# python3 — an undeclared dependency, added because there was no way to get the
# skill as text. There is now.
shipped=$(pql skill show --raw | sha256sum | cut -c1-16)

installed_path="$HOME/.claude/skills/pql/SKILL.md"
if [ -f "$installed_path" ]; then
  installed=$(hash_of "$installed_path")
else
  installed="(not installed)"
fi

printf '  repo       %s  internal/skill/SKILL.md\n' "$repo"
printf '  shipped    %s  embedded in %s\n' "$shipped" "$(command -v pql)"
printf '  installed  %s  %s\n' "$installed" "$installed_path"
echo

if [ "$repo" != "$shipped" ]; then
  echo "DRIFT: the repo file is not what the binary ships."
  echo "The skill is embedded at build time — rebuild before auditing, or you"
  echo "will test a stale artefact:"
  echo "    make build && cp ./bin/pql ~/.local/bin/pql && pql skill install"
  exit 1
fi

if [ "$shipped" != "$installed" ]; then
  echo "DRIFT: the installed copy is not what the binary ships."
  echo "    pql skill install"
  exit 1
fi

echo "All three agree. Safe to audit."

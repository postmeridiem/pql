#!/usr/bin/env bash
# Compare the commit the pql on PATH was built from against the working tree.
#
# `pql --version` cannot answer this. Version strings are deliberately clean —
# just the number, no SHA, no dirty marker (D-12) — so two binaries built from
# different commits print the same string whenever the release version has not
# been bumped between them. An audit that checks version equality and concludes
# "the binary is current" is checking the one field guaranteed not to move.
#
# The commit is stamped by the Makefile's ldflags at build time and reported by
# `pql version --build-info`, so comparing that against HEAD answers the real
# question: is the binary I am about to test built from the code in front of me?
#
# Exit 0 when they agree, 1 otherwise.
set -euo pipefail

cd "$(dirname "$0")/../../../.."

head_commit=$(git rev-parse --short HEAD)
binary_path=$(command -v pql || echo "")

if [ -z "$binary_path" ]; then
  echo "No pql on PATH. Build and install one before auditing:"
  echo "    make install"
  exit 1
fi

binary_commit=$(pql version --build-info | sed -n 's/.*"commit"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
dirty=""
if ! git diff --quiet || ! git diff --cached --quiet; then
  dirty=" (working tree has uncommitted changes)"
fi

printf '  HEAD      %s%s\n' "$head_commit" "$dirty"
printf '  binary    %s  %s\n' "${binary_commit:-unknown}" "$binary_path"
echo

if [ -z "$binary_commit" ] || [ "$binary_commit" = "unknown" ]; then
  echo "The binary carries no commit stamp — it was not built by the Makefile."
  echo "    make install"
  exit 1
fi

if [ "$binary_commit" != "$head_commit" ]; then
  echo "DRIFT: the binary on PATH was built from a different commit than HEAD."
  echo "Version strings will still agree, so this is invisible to \`pql --version\`."
  echo "    make install"
  exit 1
fi

if [ -n "$dirty" ]; then
  echo "The binary matches HEAD, but the working tree has uncommitted changes."
  echo "Anything you have edited and not rebuilt is not in the binary under test."
  echo "    make install"
  exit 1
fi

echo "Binary matches HEAD. Safe to audit."

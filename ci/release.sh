#!/usr/bin/env bash
# CI release entry. Invoked by .github/workflows/release.yaml's release job,
# which runs on a push to main that carries a dated CHANGELOG section — the
# workflow mints and pushes the tag itself, so there is no v*-tag trigger.
#
# Requires GITHUB_TOKEN in env so goreleaser can publish to GitHub Releases,
# and goreleaser on PATH at the version the lint job ran `goreleaser check`
# with. Both are supplied by the workflow.
set -euo pipefail

cd "$(dirname "$0")/.."

# Same reasoning as ci/lint.sh's preamble: without this the script dies on a
# bare "command not found" partway through a publish, which reads like a
# release failure rather than a missing binary. Worth more here than there,
# because this is the one script that runs with a tag already pushed.
for tool in go goreleaser; do
  command -v "$tool" >/dev/null && continue
  echo "ci/release.sh: $tool not on PATH" >&2
  echo "  In CI the release job installs goreleaser at the pinned version." >&2
  echo "  Locally, prefer 'make snapshot' — it dry-runs the same config" >&2
  echo "  without publishing. Do not run this script by hand to cut a" >&2
  echo "  release; pushing a dated CHANGELOG section to main is the trigger." >&2
  exit 1
done

echo "==> goreleaser release"
goreleaser release --clean

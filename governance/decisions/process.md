# Process Decisions

How work moves through this repo — checks, gates, releases. The commands
themselves are `make help`; what lives here is why they are shaped the way they
are.

---

### D-33: CI scripts are the definition; callers invoke, never restate
- **Date:** 2026-09-02
- **Decision:** The substance of every check lives in a script under `ci/`. Workflows and Makefile targets **invoke** that script and never re-list what it does. A caller that restates a script's steps is a defect, not a convenience — including a Makefile target that looks like a faithful copy on the day it is written.

  Three corollaries, each of which was a separate incident before it was a rule:
  - **A tool version is pinned once per caller-file and referenced**, not repeated per step. Two invocations of the same tool in one workflow must resolve to the same build, or a check does not cover what it appears to gate.
  - **Each script is runnable standalone** and states its own prerequisites, because it is now the definition rather than a fragment of one. Concretely: a missing-tool preamble that fails with something actionable, as `ci/lint.sh` and `ci/release.sh` both carry.
  - **Not every script is workflow-run, and each one says which it is.** `ci/secrets.sh` and `ci/eval.sh` are deliberately local-only. The rule is about restating, not about universal CI membership — conflating the two is exactly what drifted in T-70.

  Enforced by `ci/workflows_test.go` rather than by review: a `ci/*.sh` with no caller in any workflow or Makefile target fails `make test`, as does a workflow invoking a script that does not exist.

- **Rationale:** The property being protected is that **a green check locally means the same thing as a green check in CI**. That is what makes a pre-push gate worth running and what lets the CI provider be swapped without renegotiating what a release does. It does not survive a second copy: when a caller restates a script's steps, the two definitions do not fail on divergence — they simply stop agreeing, silently, and the discrepancy surfaces at the worst available moment.

  It has failed twice here, in both directions, which is why it is a record rather than an assumption:

  - **`make lint`, at the 2.0.0 release.** The target ran `golangci-lint` alone while `ci/lint.sh` had grown two further stages. A workflow verified locally with `make lint` then failed in CI on a tool that had never been installed. The target was a faithful copy of the script on the day it was written.
  - **`ci/release.sh`, T-70.** The workflow used `goreleaser-action` while two documents described the script as the release path, so the script was dead code for months and the documented swap-the-provider property was false for the one workflow that publishes binaries. Underneath it, the action ran `version: latest` while the `goreleaser check` gating it was pinned to v2.16.0 — the check validated a different build than the one that published.

  The second incident is the more instructive: the drift was not in the script or the workflow but in the *relationship* between them, which no single file was responsible for stating. That is why the enforcement is a test over the pair rather than a lint rule over either.

  This is the same instinct as [D-32](testing.md#d-32-choose-the-default-that-makes-an-omission-safe) applied to build plumbing. A restated step list is an optimistic default: it is correct when written and degrades silently, and no signal moves when it stops being true.

- **Cost:** One layer of indirection. `make lint` no longer tells you what it runs, and you read `ci/lint.sh` to find out — mitigated by naming the script in the target's `##` help text, which is what `make help` prints. Standalone-runnability means each script carries its own tool-presence preamble, so that shape is duplicated across `ci/*.sh`; accepted deliberately, because the alternative is a shared library every script must source, which reintroduces a thing that can be forgotten.

  The enforcement test is homegrown and narrower than `actionlint`. It knowingly does not check expression syntax, deprecated action versions or runner labels. That was the trade: `actionlint` would be a fourth pinned binary for every contributor and CI job, and it cannot check either property that actually broke, because both require knowing what this repo's `ci/` directory means.

- **Raised by:** T-70, generalising the `make lint` divergence that broke the 2.0.0 release. Both incidents were found by a human auditing files side by side; neither was caught by anything the repo runs.

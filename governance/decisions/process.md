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

### D-34: Unreleased section; the version is chosen at release
- **Date:** 2026-09-02
- **Decision:** Unreleased work accumulates in `CHANGELOG.md` under `## [Unreleased]`, per [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). No version number is invented for work in progress.

  Releasing is **one commit**: rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD`, set `project.yaml`'s `version:` to the same `X.Y.Z`, update its `status:` summary, and open a fresh empty `## [Unreleased]` above. Pushing that to `main` is the release signal; `release.yaml` mints the tag and publishes. Tags are never created by hand.

  It follows that **`project.yaml`'s `version:` is the last released version**, not the next one, so a build between releases stamps the version that last shipped.

- **Rationale:** Naming the working section after a version requires knowing, before the work exists, which semver bump the work will eventually justify. That is a prediction, and predictions are wrong: a `2.2.1` section was minted in advance and then shipped as part of `2.3.0`, so its entries had to be hand-carried across at release time — recorded in the changelog itself, a few sections below the preamble that caused it. `[Unreleased]` moves the choice to the moment the section can be read and semver applied to what is actually in it, which is the only moment the choice can be made correctly.

  The mechanism cost nothing. `release.yaml` looks for a dated section matching the declared version, and at release time the section *is* named for the version, so its logic is unchanged. What changed is which guard stops an accidental publish between releases: it used to be "the section is undated", and is now "the tag already exists". Both fail closed, and an absent section releases nothing — the same instinct as [D-32](testing.md#d-32-choose-the-default-that-makes-an-omission-safe), applied to the one workflow that publishes.

  The preamble this replaces claimed to follow Keep a Changelog while doing the opposite, which is worth noting as its own failure: a convention that cites a standard it does not implement is harder to correct than one that never cited anything, because the citation reads as prior consideration.

- **Cost:** A dev build stamps the last released version, so `pql --version` cannot distinguish a build from `main` from the release it names. This is not a new gap — `make binary-drift` exists precisely because `--version` has never been able to answer "was this built from HEAD", and its help says so. The alternative was a `-dev` suffixed next-version, which reintroduces the guessed number in visibly-provisional form and would need the release regex widened to ignore it; the guess is the thing being removed, so a tidier guess is not an improvement.

  The operational steps live in three places — `CHANGELOG.md`'s preamble, the `git-commit` skill, and `release.yaml`'s header comment — because three different readers need them at three different moments. Each states what to do and points here for why, which is the split [D-33](#d-33-ci-scripts-are-the-definition-callers-invoke-never-restate) asks for; the rationale is not restated in any of them.

- **Raised by:** Noticed immediately after the 2.3.0 release, when the post-release commit had to invent `2.3.1` with nothing yet in it — the prescience the old convention required, made visible by having to perform it.

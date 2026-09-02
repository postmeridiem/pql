# Testing Decisions

Quality strategy for pql — how tests and checks are written, as distinct from
which ones exist. The catalogue of tiers, tags and fixtures is
`docs/structure/project-structure.md`, "Test infrastructure"; this file holds
the conventions that govern what those tests actually claim.

---

### D-32: Choose the default that makes an omission safe
- **Date:** 2026-09-02
- **Decision:** Default to failure and require a positive success condition, rather than defaulting to success and enumerating the failures you thought of. An unanticipated failure — or a forgotten line — then lands on the safe side. Three surfaces, one rule:
  - **Assertions.** Assert the exact expected result, not the absence of a named wrong one. `!strings.Contains(got, "error")` is satisfied by every wrong output that merely avoids that word.
  - **Checks and error handling.** A check that could not determine anything reports failure, not success. A failed read, an empty probe, or a dropped error leaving the zero value flowing on must not be indistinguishable from a real pass.
  - **Initialisation.** A validity flag starts `false` and is set only on success. The version that starts `true` has to be cleared by every early return, and is broken by the one that forgets.

  **The edge, which is part of the decision and not an escape from it:** a strict assertion is wrong where the expected value could only be produced by reimplementing the thing under test, because the test then passes whenever both copies share a bug — it asserts agreement, not correctness. There, assert a property a wrong implementation cannot satisfy (output that is a subsequence of its input; a count that must sit between two others) **and say in the test why the weaker claim is the stronger one.** An unexplained weak assertion is indistinguishable from a lazy one, which is how the exception becomes the excuse.

- **Rationale:** The two failure modes are not symmetric. An assertion that is too strict fails loudly on the next legitimate change and gets fixed within the hour. An assertion that is too weak passes forever, and — this is the part that matters — *stops covering new failure modes silently as the code around it grows*. No coverage number moves when a test stops covering something, so nothing surfaces the decay. Choosing the default that makes an omission safe is the only version of the rule that survives an author who is tired, in a hurry, or absent.

  The same asymmetry outside tests. A bare `_` on an error yields a zero value that flows on as data, so `0` or `""` reads downstream as a measurement rather than as a failure to measure. A field initialised optimistically puts the burden on every future early return. In both cases the safe version costs one line at the point of construction and cannot be undone by an omission elsewhere.

  This repo has already paid for the rule twice, which is why it is being written down rather than assumed. The first gitleaks design required an untracked config file that existed in no clone, so `make secrets` reported clean for months while running the bare defaults — success by default, absence of a finding read as absence of a problem. The fix was `make secrets-selftest`, which asserts counts in **both** directions against planted positives and negative controls, because "a rule that stopped matching" is precisely the regression that does not announce itself. And `make lint` was golangci-lint alone while `ci/lint.sh` had grown two more stages, so a release was verified against a gate that ran a third of what its name claimed.

  The same instinct at the CLI boundary is already recorded: [D-29](architecture.md#d-29-an-argument-that-names-a-thing-is-validated-a-value-that-filters-a-set-is-not) makes a mistyped *name* exit `66` rather than return an empty list that reads as "nothing is related to this file". An empty result that is indistinguishable from a real one is the same defect whether it comes from an assertion, a dropped error or an exit code.

- **Cost:** Strict assertions are more work to write and break on benign changes — a renamed field, a reordered key, a reworded message. That churn is the price, paid deliberately: a test that never breaks under refactoring is usually a test that would not break under a regression either. Where the churn is genuinely not worth it, the edge above is the sanctioned route, and it costs a sentence of justification in the test.

  **This record does not license a mechanical sweep.** T-114 counts roughly 128 negative-assertion sites, and expects a substantial fraction of them to be correct as written — where the absence *is* the property under test ("the error message must not leak an absolute path", "the receipt must not contain the old label"). Those stay, and separating them from the ones that merely dodge a named mistake is reading work that a grep cannot do. A survey that returns a single large count invites a single large diff, which is the outcome to avoid.

  Correspondingly, this record **sanctions** the `q, _ := cmd.Flags().GetBool("quiet")` spelling (18 sites) rather than requiring it to be swept: cobra errors there only when the flag is undefined, which is a programming mistake a test catches immediately, not a runtime condition. The general form: *a bare `_` discarding an error that can fire at runtime is a bug; one discarding an error that can only fire on programmer error is a convention.* Count and report them as two numbers, never one.

- **Raised by:** T-114, which surveyed the codebase for optimistic defaults and produced this principle as its generalisation. Filed as a record of its own via T-115: the survey for an existing home found that everything the repo said about testing was *which tests exist* and *what to run*, that nothing addressed how strong a claim an assertion should make, and that the two candidate documents had drifted from each other and from the Makefile — so the drift was fixed first and the principle placed afterwards, in the `testing` domain [D-21](architecture.md#d-21-dqr-layout--governance-parent-with-per-type-subdirectories) had already reserved for it.

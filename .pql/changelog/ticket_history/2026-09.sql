INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G5HXSMTZDG68R0WRV0V4S25M', 'status', 'backlog', 'done', NULL, '2026-09-01 15:43:44', '2026-09-01 15:43:44.496', '2026-09-01 15:43:44.496', NULL, '78397468f3e9f09516ad2d500369e34e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G5F9X8E0KRWPQG7WVNH5TXVC', 'status', 'backlog', 'in_progress', NULL, '2026-09-01 15:44:59', '2026-09-01 15:44:59.256', '2026-09-01 15:44:59.256', NULL, 'd718b391433b0c2e569271872aa261dd', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G5FBJJ93GWTXRHNWSQ432ZWM', 'status', 'backlog', 'in_progress', NULL, '2026-09-01 15:45:27', '2026-09-01 15:45:27.838', '2026-09-01 15:45:27.838', NULL, 'ea573182af2b595be703232e6feeed0a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G5F9X8E0KRWPQG7WVNH5TXVC', 'status', 'in_progress', 'backlog', NULL, '2026-09-01 15:45:32', '2026-09-01 15:45:32.561', '2026-09-01 15:45:32.561', NULL, '1f2e01a50c70532b7ead23e7ec792a5f', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G5F9X8E0KRWPQG7WVNH5TXVC', 'decision_ref', NULL, 'D-32', NULL, '2026-09-02 10:49:03', '2026-09-02 10:49:03.019', '2026-09-02 10:49:03.019', NULL, 'd68137d0121b3a6f205895ce9d09348c', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G5F9X8E0KRWPQG7WVNH5TXVC', 'description', 'A pattern worth combing this codebase for rather than fixing in one place. Code
tends to default to success and then enumerate the failures the author
anticipated. The safer inversion is to default to failure and require a positive
success condition, so that an unanticipated failure - or a forgotten line -
lands on the safe side.

THREE FORMS TO LOOK FOR

- Tests asserting what must NOT appear rather than what must. A check that the
  bad value is absent (!strings.Contains(got, "error")) passes for every wrong
  output that merely avoids it, and quietly stops covering new failure modes as
  the code grows - no coverage number moves when it stops covering something.
  The strict form asserts the exact expected result, so a mutation has to
  reproduce it rather than dodge one named mistake.

- Checks and error handling that report success when nothing could actually be
  determined. A failed read, an empty probe, or a parse whose error is dropped
  and leaves the zero value flowing on (a bare 0 or "" from an ignored err) each
  render as a pass that is indistinguishable from a real one.

- Fields, flags and defaults initialised to the optimistic value, so every early
  return has to remember to clear them (an ok bool, or a Valid: true set at
  construction). The version that starts false and is set only on success cannot
  be broken by an omission.

SCOPE

Survey and report before changing anything. The goal is to learn how common the
pattern is, not to land a large diff. Findings get named; scope for fixing comes
back from the user.

WHERE TO START - TWO COUNTS TAKEN WHILE FILING THIS

Both are grep counts, not a survey. They size the problem; they do not classify
it, and classifying is most of the work.

FORM 1, roughly 128 sites. Negative assertions in tests (!strings.Contains or
!bytes.Contains), concentrated in six files:

  internal/query/dsl/eval/compile_test.go        30
  internal/cli/integration_test.go               30
  internal/cli/init_test.go                      16
  internal/cli/init_replace_test.go               8
  internal/planning/migrate/migrate_test.go       5
  internal/cli/render/render_test.go              5

That total is the headline number but it is the least trustworthy of the two.
Some of these are legitimately negative claims - "the error message must not
leak an absolute path", "the receipt must not contain the old label" - where
the absence IS the property under test and a positive assertion would be the
wrong shape. Those should stay. The survey''s real job is separating them from
the ones that only avoid a named mistake, and the two read identically at a
grep. Expect the defensible fraction to be substantial.

FORM 2, 18 sites. Flag reads that discard the error:

  q, _ := cmd.Flags().GetBool("quiet")
  limit, _ := cmd.Flags().GetInt("limit")

Each yields the zero value on failure, indistinguishable from the flag being
genuinely unset. Here it is probably harmless - cobra errors only when the flag
is undefined, a programming mistake a test would catch - but it is exactly the
shape this ticket describes, and "probably harmless" is a judgement the next
reader has to re-derive at every one of the 18. Worth settling once: either a
helper that panics on an undefined flag, or a sentence somewhere sanctioning the
spelling.

That distinction generalises, and the survey should carry it throughout: a bare
_ discarding a real runtime error is a bug; one discarding an error that can
only fire on programmer error is a convention. Report them as two numbers, not
one. A survey that returns a single large count invites a large mechanical diff,
which is the outcome this ticket''s scope section is trying to avoid.

FORM 3 was not counted. Optimistically-initialised fields have no grep signature
- an `ok: true` at construction looks like any other struct literal - so it
needs reading rather than matching. Do not report a zero here and call it clean.

RELATED

T-115 records the documentation survey done alongside this ticket, including the
drift it turned up. The convention this ticket implies - choose the default that
makes an omission safe - was deliberately NOT written down yet, because the two
candidate homes for it have already drifted from each other; see T-115.', 'A pattern worth combing this codebase for rather than fixing in one place. Code
tends to default to success and then enumerate the failures the author
anticipated. The safer inversion is to default to failure and require a positive
success condition, so that an unanticipated failure - or a forgotten line -
lands on the safe side.

THREE FORMS TO LOOK FOR

- Tests asserting what must NOT appear rather than what must. A check that the
  bad value is absent (!strings.Contains(got, "error")) passes for every wrong
  output that merely avoids it, and quietly stops covering new failure modes as
  the code grows - no coverage number moves when it stops covering something.
  The strict form asserts the exact expected result, so a mutation has to
  reproduce it rather than dodge one named mistake.

- Checks and error handling that report success when nothing could actually be
  determined. A failed read, an empty probe, or a parse whose error is dropped
  and leaves the zero value flowing on (a bare 0 or "" from an ignored err) each
  render as a pass that is indistinguishable from a real one.

- Fields, flags and defaults initialised to the optimistic value, so every early
  return has to remember to clear them (an ok bool, or a Valid: true set at
  construction). The version that starts false and is set only on success cannot
  be broken by an omission.

SCOPE

Survey and report before changing anything. The goal is to learn how common the
pattern is, not to land a large diff. Findings get named; scope for fixing comes
back from the user.

WHERE TO START - TWO COUNTS TAKEN WHILE FILING THIS

Both are grep counts, not a survey. They size the problem; they do not classify
it, and classifying is most of the work.

FORM 1, roughly 128 sites. Negative assertions in tests (!strings.Contains or
!bytes.Contains), concentrated in six files:

  internal/query/dsl/eval/compile_test.go        30
  internal/cli/integration_test.go               30
  internal/cli/init_test.go                      16
  internal/cli/init_replace_test.go               8
  internal/planning/migrate/migrate_test.go       5
  internal/cli/render/render_test.go              5

That total is the headline number but it is the least trustworthy of the two.
Some of these are legitimately negative claims - "the error message must not
leak an absolute path", "the receipt must not contain the old label" - where
the absence IS the property under test and a positive assertion would be the
wrong shape. Those should stay. The survey''s real job is separating them from
the ones that only avoid a named mistake, and the two read identically at a
grep. Expect the defensible fraction to be substantial.

FORM 2, 18 sites. Flag reads that discard the error:

  q, _ := cmd.Flags().GetBool("quiet")
  limit, _ := cmd.Flags().GetInt("limit")

Each yields the zero value on failure, indistinguishable from the flag being
genuinely unset. Here it is probably harmless - cobra errors only when the flag
is undefined, a programming mistake a test would catch - but it is exactly the
shape this ticket describes, and "probably harmless" is a judgement the next
reader has to re-derive at every one of the 18. Worth settling once: either a
helper that panics on an undefined flag, or a sentence somewhere sanctioning the
spelling.

That distinction generalises, and the survey should carry it throughout: a bare
_ discarding a real runtime error is a bug; one discarding an error that can
only fire on programmer error is a convention. Report them as two numbers, not
one. A survey that returns a single large count invites a large mechanical diff,
which is the outcome this ticket''s scope section is trying to avoid.

FORM 3 was not counted. Optimistically-initialised fields have no grep signature
- an `ok: true` at construction looks like any other struct literal - so it
needs reading rather than matching. Do not report a zero here and call it clean.

RELATED

T-115 records the documentation survey done alongside this ticket, including the
drift it turned up. The convention this ticket implies - choose the default that
makes an omission safe - was deliberately NOT written down yet, because the two
candidate homes for it have already drifted from each other; see T-115.

UPDATE 2026-09-02 (via T-115)

The convention this ticket implies is now written down: D-32, "choose the
default that makes an omission safe", in governance/decisions/testing.md. This
ticket''s decision_ref points at it. The RELATED note above is superseded - the
two candidate homes were reconciled first, and there is now one place for the
principle to live.

D-32 also settles the two judgements this ticket asked to have settled once:
the 18 flag-read sites are sanctioned as written, and the ~128 negative
assertions are explicitly NOT a mechanical sweep. The survey scope here is
unchanged - classify before changing anything, report the two forms as two
numbers.', NULL, '2026-09-02 10:52:21', '2026-09-02 10:52:21.330', '2026-09-02 10:52:21.330', NULL, 'bb149312b3ac6280d42554afda624809', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G5FBJJ93GWTXRHNWSQ432ZWM', 'description', 'Filed as the survey output from an attempt to record one new convention: "choose
the default that makes an omission safe" (the principle behind T-114). The
instruction was to find the existing home for verification discipline and extend
it rather than start a second one. The survey says there is no such home, and
that the nearest candidates have already drifted - so nothing was written, and
where the principle belongs is the maintainer''s call rather than a side effect
of filing T-114.

FINDING 1 - THERE IS NO EXISTING HOME FOR ASSERTION DISCIPLINE

Everything the repo says about testing is about WHICH tests exist and WHAT to
run. Nothing says how to write an assertion, how strong it should be, or when a
weaker claim is the correct one.

  docs/structure/project-structure.md, "Test infrastructure" (l.155-163)
      the three tiers, their build tags, fixture locations.
  docs/structure/project-structure.md, "Verification" (l.210-223)
      a nine-step end-to-end milestone checklist - commands to run, not
      properties to assert.
  CLAUDE.md, "Test infrastructure" (l.125-133)
      the same three tiers, compressed.
  .claude/skills/pql-testing/
      audits the embedded SKILL.md by consuming it. Scope is the skill''s
      accuracy, not the Go tests. Not a home for this.
  governance/decisions/architecture.md
      31 confirmed decisions, none about testing philosophy. The one adjacent
      mention is inside D-23''s cost section, noting that a regression test
      guards write-through - an application, not a convention.

So the principle would be new material wherever it lands. The candidates are a
new D record in governance/decisions/architecture.md, or a subsection of
project-structure.md''s Verification. A D record looks right - it is a choice
with alternatives and a rationale, which is what that tree is for, and D-22
already sets the precedent of recording an output-contract-shaped rule there -
but see finding 3 before adding to project-structure.md either way.

FINDING 2 - THE SAME MATERIAL IS STATED TWICE, AND ONE COPY IS STALE

CLAUDE.md and project-structure.md both carry a Makefile target table and a
test-tier list. They agree with each other and disagree with the Makefile:

  CLAUDE.md:93                      | `make lint` | `golangci-lint run` |
  project-structure.md:179          | `make lint` | `golangci-lint run` |
  Makefile:144                      lint: ./ci/lint.sh
                                    (golangci-lint + goreleaser check +
                                     govulncheck)

This is not a cosmetic gap. `make lint` is a gate, and both docs describe it as
one third of what it is - a contributor who reads either and runs golangci-lint
directly believes they have passed a check they have not run. The Makefile knows
this failure mode by name; the comment directly above the target records that it
already happened once:

    # That is not hypothetical - `make lint` was golangci-lint alone while
    # ci/lint.sh had grown two more stages, so a workflow verified with
    # `make lint` failed on a tool it never installed.

The fix was applied to the Makefile and not to either doc, which is how the
drift got in.

FINDING 3 - project-structure.md CONTRADICTS ITSELF

Line 179 says make lint is golangci-lint run. Line 191, twelve lines later,
correctly describes ci/lint.sh as golangci-lint + goreleaser check +
govulncheck. Both are in the "Build & release pipeline" section. A reader
resolving the conflict has to go to the Makefile, at which point the document
is not serving its purpose.

FINDING 4 - TWO SMALLER STALE REFERENCES IN THE SAME FILE

  project-structure.md:208   names `decisions/` as the DQR location. D-21 moved
                             it to governance/{decisions,questions,rejected}/.
                             The path in the doc no longer exists.

  project-structure.md:193   describes ci/release.sh as the release path.
                             Already known dead - T-70 tracks that
                             release.yaml calls goreleaser directly.

WHY NOTHING WAS WRITTEN

Adding a verification convention to a pair of documents that already say the
same thing differently, one of them stale and one of them self-contradicting,
makes the problem the convention is about. The instruction that prompted this
put it plainly: a principle written in two places diverges quietly, and the
divergence stays invisible until the two copies contradict each other. That has
already happened here, to the material next door.

WHAT WOULD RESOLVE IT

Maintainer''s call, in this order:

1. Decide whether CLAUDE.md''s build/test tables are the canonical copy or a
   summary that should point at project-structure.md instead of restating it.
   Every duplicated table is a future instance of finding 2.
2. Reconcile the make lint description in whichever copies survive, and fix
   findings 3 and 4 while in the file.
3. Then place the T-114 principle, once there is one place for it to go.

Steps 1 and 2 are prerequisites for 3, not tidying to be done afterwards - the
whole point of the survey was to avoid adding a second home, and today there is
no first home to extend.

TEXT TO PLACE, ONCE THERE IS SOMEWHERE TO PUT IT

Recorded here so the drafting is not lost, deliberately NOT committed to any
doc:

  Choose the default that makes an omission safe. Default to failure and require
  a positive success condition, rather than defaulting to success and
  enumerating the failures you thought of. An unanticipated failure, or a
  forgotten line, then lands on the safe side.

  The edge, which has to be stated alongside it or the exception becomes the
  excuse: a strict assertion is wrong where the expected value could only be
  produced by reimplementing the thing under test, because the test then passes
  whenever both copies share a bug. There, assert a property a wrong
  implementation cannot satisfy - output that is a subsequence of its input, a
  count that must sit between two others - and say in the test why the weaker
  claim is the stronger one.

RELATED

T-114 - the code sweep this survey was filed alongside.
T-70  - ci/release.sh is dead code (finding 4).', 'Filed as the survey output from an attempt to record one new convention: "choose
the default that makes an omission safe" (the principle behind T-114). The
instruction was to find the existing home for verification discipline and extend
it rather than start a second one. The survey says there is no such home, and
that the nearest candidates have already drifted - so nothing was written, and
where the principle belongs is the maintainer''s call rather than a side effect
of filing T-114.

FINDING 1 - THERE IS NO EXISTING HOME FOR ASSERTION DISCIPLINE

Everything the repo says about testing is about WHICH tests exist and WHAT to
run. Nothing says how to write an assertion, how strong it should be, or when a
weaker claim is the correct one.

  docs/structure/project-structure.md, "Test infrastructure" (l.155-163)
      the three tiers, their build tags, fixture locations.
  docs/structure/project-structure.md, "Verification" (l.210-223)
      a nine-step end-to-end milestone checklist - commands to run, not
      properties to assert.
  CLAUDE.md, "Test infrastructure" (l.125-133)
      the same three tiers, compressed.
  .claude/skills/pql-testing/
      audits the embedded SKILL.md by consuming it. Scope is the skill''s
      accuracy, not the Go tests. Not a home for this.
  governance/decisions/architecture.md
      31 confirmed decisions, none about testing philosophy. The one adjacent
      mention is inside D-23''s cost section, noting that a regression test
      guards write-through - an application, not a convention.

So the principle would be new material wherever it lands. The candidates are a
new D record in governance/decisions/architecture.md, or a subsection of
project-structure.md''s Verification. A D record looks right - it is a choice
with alternatives and a rationale, which is what that tree is for, and D-22
already sets the precedent of recording an output-contract-shaped rule there -
but see finding 3 before adding to project-structure.md either way.

FINDING 2 - THE SAME MATERIAL IS STATED TWICE, AND ONE COPY IS STALE

CLAUDE.md and project-structure.md both carry a Makefile target table and a
test-tier list. They agree with each other and disagree with the Makefile:

  CLAUDE.md:93                      | `make lint` | `golangci-lint run` |
  project-structure.md:179          | `make lint` | `golangci-lint run` |
  Makefile:144                      lint: ./ci/lint.sh
                                    (golangci-lint + goreleaser check +
                                     govulncheck)

This is not a cosmetic gap. `make lint` is a gate, and both docs describe it as
one third of what it is - a contributor who reads either and runs golangci-lint
directly believes they have passed a check they have not run. The Makefile knows
this failure mode by name; the comment directly above the target records that it
already happened once:

    # That is not hypothetical - `make lint` was golangci-lint alone while
    # ci/lint.sh had grown two more stages, so a workflow verified with
    # `make lint` failed on a tool it never installed.

The fix was applied to the Makefile and not to either doc, which is how the
drift got in.

FINDING 3 - project-structure.md CONTRADICTS ITSELF

Line 179 says make lint is golangci-lint run. Line 191, twelve lines later,
correctly describes ci/lint.sh as golangci-lint + goreleaser check +
govulncheck. Both are in the "Build & release pipeline" section. A reader
resolving the conflict has to go to the Makefile, at which point the document
is not serving its purpose.

FINDING 4 - TWO SMALLER STALE REFERENCES IN THE SAME FILE

  project-structure.md:208   names `decisions/` as the DQR location. D-21 moved
                             it to governance/{decisions,questions,rejected}/.
                             The path in the doc no longer exists.

  project-structure.md:193   describes ci/release.sh as the release path.
                             Already known dead - T-70 tracks that
                             release.yaml calls goreleaser directly.

WHY NOTHING WAS WRITTEN

Adding a verification convention to a pair of documents that already say the
same thing differently, one of them stale and one of them self-contradicting,
makes the problem the convention is about. The instruction that prompted this
put it plainly: a principle written in two places diverges quietly, and the
divergence stays invisible until the two copies contradict each other. That has
already happened here, to the material next door.

WHAT WOULD RESOLVE IT

Maintainer''s call, in this order:

1. Decide whether CLAUDE.md''s build/test tables are the canonical copy or a
   summary that should point at project-structure.md instead of restating it.
   Every duplicated table is a future instance of finding 2.
2. Reconcile the make lint description in whichever copies survive, and fix
   findings 3 and 4 while in the file.
3. Then place the T-114 principle, once there is one place for it to go.

Steps 1 and 2 are prerequisites for 3, not tidying to be done afterwards - the
whole point of the survey was to avoid adding a second home, and today there is
no first home to extend.

TEXT TO PLACE, ONCE THERE IS SOMEWHERE TO PUT IT

Recorded here so the drafting is not lost, deliberately NOT committed to any
doc:

  Choose the default that makes an omission safe. Default to failure and require
  a positive success condition, rather than defaulting to success and
  enumerating the failures you thought of. An unanticipated failure, or a
  forgotten line, then lands on the safe side.

  The edge, which has to be stated alongside it or the exception becomes the
  excuse: a strict assertion is wrong where the expected value could only be
  produced by reimplementing the thing under test, because the test then passes
  whenever both copies share a bug. There, assert a property a wrong
  implementation cannot satisfy - output that is a subsequence of its input, a
  count that must sit between two others - and say in the test why the weaker
  claim is the stronger one.

RELATED

T-114 - the code sweep this survey was filed alongside.
T-70  - ci/release.sh is dead code (finding 4).

RESOLVED 2026-09-02

Maintainer''s calls, both taken:

1. NEITHER doc restates the Makefile. `make help` generates the target list
   from the Makefile''s own `##` comments and is the only copy that cannot
   drift. CLAUDE.md''s table and project-structure.md''s table are both deleted.
   CLAUDE.md keeps four bullets of judgement `make help` has no room for -
   which targets are gates, what `make lint` actually contains, the order and
   reasoning of `make pre-push`, and that `make install` also refreshes the
   embedded skill. project-structure.md points at CLAUDE.md for that and says
   why it does not carry a second copy.

2. The T-114 principle is D-32 in governance/decisions/testing.md - a new
   `testing` domain, the one D-21 recommended and nothing had used. T-114''s
   decision_ref now points at it. D-32 carries the edge case as part of the
   decision rather than as a footnote, and settles T-114''s two open judgements
   (the 18 flag reads are sanctioned; the ~128 negative assertions are not a
   sweep).

Findings 3 and 4 fixed in the same pass, plus four more found while in the
files:

- make lint described as golangci-lint run, in both docs (finding 2)
- project-structure.md contradicting itself twelve lines later (finding 3)
- flat `decisions/` path, superseded by D-21 (finding 4)
- ci/release.sh described as the release path (finding 4) - now marked dead
  with T-70 named, rather than deciding T-70''s question here
- ci/eval.sh described as a scheduled job; nothing schedules it (T-70)
- `make eval` described as a bare `go test -tags=eval`; it delegates to
  ci/eval.sh
- `make pre-push` described as "lint + vuln + test + test-race"; it is
  secrets + lint + test + test-race, and vuln is inside lint
- pql.db growth step described as a forward migration, contradicting D-19
- ci/secrets.sh and ci/secrets-selftest.sh missing from the layout entirely
- testdata/council-snapshot/ described as "planned"; it has existed for months

Deliberately out of scope: docs/structure/initial-plan.md and
docs/structure/planning.md carry the same stale `decisions/` path and the same
old `make lint` description. Both are framed as historical documents -
initial-plan.md is explicitly retained for its grammar/schema specifics with
its framing superseded - so correcting them is a separate call about whether
they are maintained or archived.', NULL, '2026-09-02 10:52:34', '2026-09-02 10:52:34.547', '2026-09-02 10:52:34.547', NULL, 'b6cedcead27b0655e3c3f10100aa564d', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G5FBJJ93GWTXRHNWSQ432ZWM', 'decision_ref', NULL, 'D-32', NULL, '2026-09-02 10:54:09', '2026-09-02 10:54:09.441', '2026-09-02 10:54:09.441', NULL, '9fe6c45169bd22f02dc98ba0fdf96e8a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G5FBJJ93GWTXRHNWSQ432ZWM', 'status', 'in_progress', 'review', NULL, '2026-09-02 11:01:22', '2026-09-02 11:01:22.594', '2026-09-02 11:01:22.594', NULL, '15358b0b717f0a3995eba041819d1437', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G5FBJJ93GWTXRHNWSQ432ZWM', 'status', 'review', 'done', NULL, '2026-09-02 11:01:47', '2026-09-02 11:01:47.739', '2026-09-02 11:01:47.739', NULL, '8d9aa80aa009b225a934fbb62c0b5162', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY1N5XNKDTCQ354QCFAPQAA4', 'status', 'backlog', 'in_progress', NULL, '2026-09-02 11:28:11', '2026-09-02 11:28:11.598', '2026-09-02 11:28:11.598', NULL, 'c4e11581f1b17c3f77726f0db8fdbe02', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G63Y5D09S38Q3YWRKPBGHN80', 'description', NULL, 'Found 2026-09-02 while working T-70, which asked whether ci/eval.sh should be
scheduled. It cannot be scheduled as it stands, because it is red.

`make eval` fails today, on a clean checkout, with no local state involved:

    --- FAIL: TestEval_Council/context_members/vaasa/persona.md
        NDCG@5=0.000  MRR=0.000  P@5=0.000
        NDCG@5 = 0 - no expected results in top-5

The other two cases (related, search) pass.

CAUSE - A TYPO IN THE GOLDEN, NOT A RANKING REGRESSION

internal/connect/rank/testdata/golden/council.json, the context case, expects:

    "expected_top_k": [
      "members/koskela/persona",        <- no .md
      "members/vaasa/journal.md"
    ]

`pql context members/vaasa/persona.md` against testdata/council-snapshot
returns:

    members/koskela/persona.md    0.5000

So the first expectation misses on the extension alone. This is the exact trap
the pql skill documents for `outlinks`: its `target` is the raw link text, and
a wikilink is written without `.md`, so a golden authored from outlink output
carries the unresolved spelling. The eval compares strings, so it scores zero
rather than reporting a near-miss.

A SECOND, SEPARATE PROBLEM UNDERNEATH IT

Fixing the extension is not the whole answer. `context` returns exactly ONE
result for that target, so `members/vaasa/journal.md` is absent regardless of
spelling. After the typo fix the case would pass - the assertion is only
"NDCG@5 != 0" - while still returning half of what the golden says it should.

That is worth deciding rather than papering over. Either the golden''s second
expectation is wrong (journal.md is same-directory, which is `related`''s
property, and context weights link overlap and path proximity differently), or
context is under-returning on a link-sparse fixture. The note on the case says
"koskela is linked from vaasa; journal is same directory", which suggests the
author wanted both and got one.

WHY THIS IS NOT FIXED IN T-70

Correcting the golden is a ranking-quality judgement, not a typo sweep: it
decides what `context` is supposed to return, which is the thing the eval
exists to hold pql to. Changing a golden to match current output is how an
eval stops being an eval - the strict form is to decide the expected set first
and let the number fall out. See D-32.

RELATED

T-70 - decided ci/eval.sh is a manual local tool, partly because of this.', NULL, '2026-09-02 11:54:52', '2026-09-02 11:54:52.194', '2026-09-02 11:54:52.194', NULL, '5720d51ae303c8a2de3a4f85a2f88a12', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY1N5XNKDTCQ354QCFAPQAA4', 'description', 'Found 2026-08-08 while auditing the Makefile for the divergence that broke the
2.0.0 release (`make lint` had drifted from `ci/lint.sh`).

`CLAUDE.md` states: "CI substance lives in `ci/{lint,test,release,eval}.sh`.
GitHub Actions workflows in `.github/workflows/` are thin wrappers around these —
keeps local and CI behaviour identical and lets the provider be swapped without
rewriting the scripts."

That is true for lint and test, and false for the other two:

- **`ci/release.sh` is invoked by nothing.** `release.yaml` uses
  `goreleaser/goreleaser-action@v6` with `args: release --clean` directly. The
  script runs `goreleaser release --clean`, so today they agree by coincidence —
  but nothing keeps them in step, and the stated swap-the-provider property does
  not hold for the one workflow that publishes binaries.
- **`ci/eval.sh` is invoked by nothing either.** Its header calls it a scheduled
  job, but no workflow schedules it. `make eval` now delegates to it, so it is at
  least exercised locally.

Options for release, in preference order:

1. Point `release.yaml` at `./ci/release.sh`, installing goreleaser the way the
   lint job does. Restores the documented property. Costs the action''s built-in
   caching and version pinning, which is worth checking before assuming it is
   free — the action pins a goreleaser version, the script uses whatever is on
   PATH.
2. Delete `ci/release.sh` and amend `CLAUDE.md` to say the release path
   deliberately uses the action. Honest, smaller, and gives up the swap property
   for that one workflow.

Either is fine; drifting docs are not. Pick one and make the doc match.

For eval: decide whether it is a scheduled job (add the schedule) or a manual
local tool (say so in the script header and in CLAUDE.md).

Deliberately not done during the 2.0.0 release — editing the release workflow
while a release is in flight re-triggers it.', 'Found 2026-08-08 while auditing the Makefile for the divergence that broke the
2.0.0 release (`make lint` had drifted from `ci/lint.sh`).

`CLAUDE.md` states: "CI substance lives in `ci/{lint,test,release,eval}.sh`.
GitHub Actions workflows in `.github/workflows/` are thin wrappers around these —
keeps local and CI behaviour identical and lets the provider be swapped without
rewriting the scripts."

That is true for lint and test, and false for the other two:

- **`ci/release.sh` is invoked by nothing.** `release.yaml` uses
  `goreleaser/goreleaser-action@v6` with `args: release --clean` directly. The
  script runs `goreleaser release --clean`, so today they agree by coincidence —
  but nothing keeps them in step, and the stated swap-the-provider property does
  not hold for the one workflow that publishes binaries.
- **`ci/eval.sh` is invoked by nothing either.** Its header calls it a scheduled
  job, but no workflow schedules it. `make eval` now delegates to it, so it is at
  least exercised locally.

Options for release, in preference order:

1. Point `release.yaml` at `./ci/release.sh`, installing goreleaser the way the
   lint job does. Restores the documented property. Costs the action''s built-in
   caching and version pinning, which is worth checking before assuming it is
   free — the action pins a goreleaser version, the script uses whatever is on
   PATH.
2. Delete `ci/release.sh` and amend `CLAUDE.md` to say the release path
   deliberately uses the action. Honest, smaller, and gives up the swap property
   for that one workflow.

Either is fine; drifting docs are not. Pick one and make the doc match.

For eval: decide whether it is a scheduled job (add the schedule) or a manual
local tool (say so in the script header and in CLAUDE.md).

Deliberately not done during the 2.0.0 release — editing the release workflow
while a release is in flight re-triggers it.

RESOLVED 2026-09-02

OPTION 1, and the cost this ticket flagged for it turned out to be backwards.

The ticket warned that pointing release.yaml at ./ci/release.sh "costs the
action''s built-in caching and version pinning ... the action pins a goreleaser
version, the script uses whatever is on PATH". The action was configured with
`version: latest`. Meanwhile v2.16.0 was pinned in three places - ci.yaml''s
lint job, ci.yaml''s snapshot job, and release.yaml''s own lint job.

So the publish step was the ONLY unpinned goreleaser invocation in the repo,
and it sat immediately downstream of a `goreleaser check` that validated
.goreleaser.yaml against a different build. The gate did not cover the thing it
gated. That is a live defect this ticket found by accident while framing it as
a cost, and option 1 fixes it rather than paying for it.

Third argument, which settles it: .goreleaser.yaml''s SBOM and signing blocks
are commented out with the note "wired in CI when ci/release.sh grows install
steps for syft and cosign". The plan for signing was already written against
the script. Option 2 would have deleted the file that plan depends on.

WHAT CHANGED

- release.yaml: workflow-level `env:` holds GORELEASER_VERSION and
  GOLANGCI_LINT_VERSION, so the two jobs cannot diverge. The publish step
  installs the pinned goreleaser and runs ./ci/release.sh.
- ci/release.sh: header corrected - it is invoked on a push to main carrying a
  dated CHANGELOG section, not on a tag push; the workflow mints the tag
  itself. Gained the same missing-tool preamble ci/lint.sh has, which matters
  more here because this script runs with a tag already pushed.
- ci/eval.sh: declared a manual local tool in its own header. Not scheduled,
  no metrics sink.
- CLAUDE.md and project-structure.md: both now say ci/{lint,test,release}.sh
  are workflow-run and secrets/eval are local-only, with the caller named per
  script - the distinction that drifted in the first place.

EVAL: MANUAL LOCAL TOOL, NOT A SCHEDULED JOB

Decided on the facts rather than the intent. `make eval` FAILS today on a clean
checkout - the context case scores NDCG@5=0, MRR=0, P@5=0. Scheduling a red job
with no metrics sink produces either a permanently-failing badge or noise
nobody reads. Filed as T-120: the golden expects "members/koskela/persona"
without the .md extension, so it misses on spelling, and there is a second
question underneath about context returning one result where the golden wants
two.

TWO DOC CLAIMS CORRECTED THAT THIS TICKET DID NOT NAME

Both were downstream of the same drift:

- The docs said releases publish "signed binaries + SHA256SUMS + SBOM". SBOM
  and cosign signing are commented out in .goreleaser.yaml. Verified by running
  `make snapshot`: 5 platforms and checksums.txt, no signing or SBOM stage.
- The Verification checklist said to tag a pre-release and push the tag. There
  is no v*-tag trigger; the release signal is a dated CHANGELOG section on main.

NOT DONE, DELIBERATELY

"CI scripts are the definition; workflows and Makefile targets shell out to
them and never restate their steps" is now a property this repo has broken
twice - `make lint` (2.0.0) and this ticket - and it is still recorded only as
prose in two documents. It has the shape of a D record. Not filed here because
this ticket''s scope was to pick an option and make the doc match.', NULL, '2026-09-02 12:15:02', '2026-09-02 12:15:02.331', '2026-09-02 12:15:02.331', NULL, '208774b09eb7ca09dc8e2b827b1db2bc', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY1N5XNKDTCQ354QCFAPQAA4', 'status', 'in_progress', 'review', NULL, '2026-09-02 12:15:08', '2026-09-02 12:15:08.206', '2026-09-02 12:15:08.206', NULL, '45e4995802fec8e8b56cfc291d071031', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY1N5XNKDTCQ354QCFAPQAA4', 'decision_ref', NULL, 'D-33', NULL, '2026-09-02 12:38:19', '2026-09-02 12:38:19.948', '2026-09-02 12:38:19.948', NULL, '09e6bbd4e453c9ad870f3bf766f7262e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G64AGK5DW8R57VTSHEBM6BCC', 'status', 'backlog', 'review', NULL, '2026-09-02 13:28:12', '2026-09-02 13:28:12.072', '2026-09-02 13:28:12.072', NULL, '2a243b3ccccec5bf368dad774343421e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G64AJBN8ESS2889RXA8HVYW8', 'status', 'backlog', 'review', NULL, '2026-09-02 13:28:12', '2026-09-02 13:28:12.080', '2026-09-02 13:28:12.080', NULL, 'b4c5f8590ee8f5caa28dbcd2d17542a8', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY1N5XNKDTCQ354QCFAPQAA4', 'status', 'review', 'done', NULL, '2026-09-02 15:39:43', '2026-09-02 15:39:43.084', '2026-09-02 15:39:43.084', NULL, '7ed9f2b68b4ed851379bb931e4f02ffe', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G64AGK5DW8R57VTSHEBM6BCC', 'status', 'review', 'done', NULL, '2026-09-02 15:39:55', '2026-09-02 15:39:55.216', '2026-09-02 15:39:55.216', NULL, 'ee7f231a3216bffa2334cb5bf3e0037d', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G64AJBN8ESS2889RXA8HVYW8', 'status', 'review', 'done', NULL, '2026-09-02 15:40:02', '2026-09-02 15:40:02.025', '2026-09-02 15:40:02.025', NULL, '1c2319fb2c28648f7fc6e24ebd22afd6', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G65T19H1A3JHSQ3T0KREGWXG', 'description', NULL, 'Found 2026-09-02 while fixing T-120. `make eval` is green again, but green
means less here than it looks, and the reason is the fixture rather than the
harness.

MEASURED, not inferred. From `search council --fields path,score,signals`
against testdata/council-snapshot:

- recency        raw=0 on every file in every case
- link_overlap   raw=0
- tag_overlap    raw=0
- path_proximity raw=0
- centrality     raw=1 on exactly one file, 0 on the rest

So one signal separates one file, and the other four separate nothing. A
weight change, a normalisation change, or an outright bug in four of the five
signals would not move a single number the eval reports.

TWO CAUSES, BOTH PROPERTIES OF THE SNAPSHOT

1. Uniform mtimes. git sets every file''s mtime to checkout time, so the whole
   fixture has one timestamp and recency - weighted 0.25 on search, the second
   heaviest signal there - normalises to zero across the board. This affects
   any consumer of the snapshot, not just the eval.

2. One link in the entire vault. members/vaasa/persona.md links to
   members/koskela/persona. That is the only edge, so centrality is 1 for the
   target and 0 for all 25 other files, link_overlap can never be non-zero
   (no file shares an edge with any other), and context''s candidate set is at
   most one file for any target.

WHY IT MATTERS MORE THAN A THIN FIXTURE USUALLY WOULD

The eval exists to make ranking regressions as visible as test failures - that
is its stated job in project-structure.md, and "ranking is the product" is the
philosophy doc''s line. An eval that cannot move under four of five signals is
not doing that job, and it reports NDCG=1.000 while not doing it, which is the
most misleading result available.

T-120 strengthened the harness assertion so a golden''s expectations must
actually hold. That was the right fix for what T-120 was about and does not
touch this: a stricter assertion over a fixture with no signal is still an
assertion over no signal.

DESIRED, not a design. Enough structure in the fixture that each weighted
signal can be non-zero and can differ between files - some link density, some
shared tags, and mtimes that vary. Options worth weighing rather than
picking blind:

- Refresh the snapshot from a richer source vault. `make refresh-fixtures`
  already exists; the question is whether the source has the structure.
- Author a synthetic fixture for eval specifically. internal/fixture/ is named
  in project-structure.md for exactly this and was never built.
- Set mtimes deliberately as a fixture step, since git will never preserve
  them. Cheap and fixes recency on its own.

The third is worth doing regardless of the other two - it is a few lines and
recency is the second-heaviest weight on search.

RELATED

T-120 - fixed the two wrong golden expectations and the weak assertion.
T-65  - build the FR-2 golden eval set; overlaps on what a good fixture is.', NULL, '2026-09-02 16:16:36', '2026-09-02 16:16:36.555', '2026-09-02 16:16:36.555', NULL, '4f5eb798db22ebebe5a25a3982a87c5c', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G63Y5D09S38Q3YWRKPBGHN80', 'status', 'backlog', 'review', NULL, '2026-09-02 16:16:49', '2026-09-02 16:16:49.388', '2026-09-02 16:16:49.388', NULL, '05a26ac32c94f37e20bf088d1ad12689', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G63Y5D09S38Q3YWRKPBGHN80', 'status', 'review', 'done', NULL, '2026-09-02 16:21:17', '2026-09-02 16:21:17.126', '2026-09-02 16:21:17.126', NULL, 'cb01e28e49c7ccf6aedd4e39d15b8f1e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G66AQKC2Z6RAJZ0GQA725GWW', 'description', '`pql self-update` cannot succeed on any platform for any release. It constructs the release asset''s name without the version, and goreleaser publishes it with the version.

  internal/cli/selfupdate.go:142   fmt.Sprintf("pql_%s_%s.%s", osName, archName, ext)   -> pql_Linux_x86_64.tar.gz
  .goreleaser.yaml:32-37           {{ .ProjectName }}_{{ .Version }}_{{ title .Os }}_...  -> pql_2.3.0_Linux_x86_64.tar.gz

OBSERVED on 2.2.0 updating to v2.3.0: exit 69 with {"code":"cli.exit","msg":"no asset \"pql_Linux_x86_64.tar.gz\" in release v2.3.0"}. The release does carry pql_2.3.0_Linux_x86_64.tar.gz, alongside Darwin arm64/x86_64, Linux arm64 and a Windows zip. v2.2.0''s assets follow the same versioned naming, so this is not a regression introduced by the 2.3.0 release — no released binary has been able to update itself.

THE FAILURE IS LOUD, WHICH IS THE GOOD HALF. It exits Unavail rather than reporting success, and the diagnostic quotes the exact name it looked for, which is what made this diagnosable in one step rather than by reading code. A self-updater that silently did nothing would be far worse. Only the resolution is wrong.

CHECK THE WHOLE PATH, NOT ONLY THE LOOKUP. assetName is threaded into three places — the asset match, verifyChecksum(archiveData, assetName, checksumURL), and extractBinary(archiveData, assetName). checksums.txt lists the versioned names, so a fix that only corrects the download lookup moves the failure into checksum verification instead of removing it. Whatever produces the name has to produce the same string all three uses expect.

DESIRED, not a design: self-update resolves an asset that exists, and stays correct if the archive naming changes again. The release''s own version is already in hand as rel.TagName at the point of the lookup, so building the name from it is one option; matching by platform suffix rather than exact equality is another; reading checksums.txt as the manifest of what the release actually shipped is a third, and has the property that the name is no longer inferred at all. Preferring an option that derives the name from the release rather than from a template repeated in two places would keep this from recurring — the defect is precisely that two files independently describe one string.

WORTH A TEST THAT WOULD HAVE CAUGHT IT: nothing asserts that the name self-update constructs matches what .goreleaser.yaml produces. The two live in different languages in different files, which is why they drifted silently. A test that renders the goreleaser template, or that checks the constructed name against a real release''s asset list, closes it.', '`pql self-update` cannot succeed on any platform for any release. It constructs the release asset''s name without the version, and goreleaser publishes it with the version.

  internal/cli/selfupdate.go:142   fmt.Sprintf("pql_%s_%s.%s", osName, archName, ext)   -> pql_Linux_x86_64.tar.gz
  .goreleaser.yaml:32-37           {{ .ProjectName }}_{{ .Version }}_{{ title .Os }}_...  -> pql_2.3.0_Linux_x86_64.tar.gz

OBSERVED on 2.2.0 updating to v2.3.0: exit 69 with {"code":"cli.exit","msg":"no asset \"pql_Linux_x86_64.tar.gz\" in release v2.3.0"}. The release does carry pql_2.3.0_Linux_x86_64.tar.gz, alongside Darwin arm64/x86_64, Linux arm64 and a Windows zip. v2.2.0''s assets follow the same versioned naming, so this is not a regression introduced by the 2.3.0 release — no released binary has been able to update itself.

THE FAILURE IS LOUD, WHICH IS THE GOOD HALF. It exits Unavail rather than reporting success, and the diagnostic quotes the exact name it looked for, which is what made this diagnosable in one step rather than by reading code. A self-updater that silently did nothing would be far worse. Only the resolution is wrong.

CHECK THE WHOLE PATH, NOT ONLY THE LOOKUP. assetName is threaded into three places — the asset match, verifyChecksum(archiveData, assetName, checksumURL), and extractBinary(archiveData, assetName). checksums.txt lists the versioned names, so a fix that only corrects the download lookup moves the failure into checksum verification instead of removing it. Whatever produces the name has to produce the same string all three uses expect.

DESIRED, not a design: self-update resolves an asset that exists, and stays correct if the archive naming changes again. The release''s own version is already in hand as rel.TagName at the point of the lookup, so building the name from it is one option; matching by platform suffix rather than exact equality is another; reading checksums.txt as the manifest of what the release actually shipped is a third, and has the property that the name is no longer inferred at all. Preferring an option that derives the name from the release rather than from a template repeated in two places would keep this from recurring — the defect is precisely that two files independently describe one string.

WORTH A TEST THAT WOULD HAVE CAUGHT IT: nothing asserts that the name self-update constructs matches what .goreleaser.yaml produces. The two live in different languages in different files, which is why they drifted silently. A test that renders the goreleaser template, or that checks the constructed name against a real release''s asset list, closes it.

THE FIX IS SMALLER THAN THE OPTIONS ABOVE SUGGEST: the data is already in the function and is discarded.

runSelfUpdate has both halves before it needs them:

  rel, err := fetchLatestRelease()                             // rel.Assets is the published file list
  latestVersion := strings.TrimPrefix(rel.TagName, "v")        // the version, five lines above the defect
  ...
  assetName := archiveNameForPlatform()                        // ignores both and guesses

It then loops rel.Assets comparing each real name against the constructed one. So the release''s own manifest is already loaded, already parsed, and already being iterated — the bug is not a missing lookup, it is a guess being preferred over data in hand. No extra request, no template rendering, and nothing to keep in step with .goreleaser.yaml.

TWO SHAPES, BOTH USING WHAT IS ALREADY THERE:

  1. Select from rel.Assets by platform suffix. The loop already walks every published name; matching on the OS/arch/extension tail rather than on full equality means the version segment never has to be known, and a future change to the prefix cannot break it. This removes name construction entirely.

  2. Interpolate latestVersion into the constructed name. One variable, already computed. Smaller diff, but it keeps two descriptions of one string and only resynchronises them — the next naming change breaks it again.

The first is preferable for the reason this ticket exists: the defect is that two files independently describe one string, and only the first shape stops describing it twice. Whichever is chosen, the same string must reach verifyChecksum and extractBinary, since checksums.txt lists the versioned names.

Worth noting the diagnostic already proves the list was available — ''no asset "pql_Linux_x86_64.tar.gz" in release v2.3.0'' is printed by code that has just finished iterating the assets it could have chosen from.', NULL, '2026-09-02 17:31:57', '2026-09-02 17:31:57.765', '2026-09-02 17:31:57.765', NULL, 'd42cf4b71a8abd6cdd19e7345236ff2d', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G66AQKC2Z6RAJZ0GQA725GWW', 'description', '`pql self-update` cannot succeed on any platform for any release. It constructs the release asset''s name without the version, and goreleaser publishes it with the version.

  internal/cli/selfupdate.go:142   fmt.Sprintf("pql_%s_%s.%s", osName, archName, ext)   -> pql_Linux_x86_64.tar.gz
  .goreleaser.yaml:32-37           {{ .ProjectName }}_{{ .Version }}_{{ title .Os }}_...  -> pql_2.3.0_Linux_x86_64.tar.gz

OBSERVED on 2.2.0 updating to v2.3.0: exit 69 with {"code":"cli.exit","msg":"no asset \"pql_Linux_x86_64.tar.gz\" in release v2.3.0"}. The release does carry pql_2.3.0_Linux_x86_64.tar.gz, alongside Darwin arm64/x86_64, Linux arm64 and a Windows zip. v2.2.0''s assets follow the same versioned naming, so this is not a regression introduced by the 2.3.0 release — no released binary has been able to update itself.

THE FAILURE IS LOUD, WHICH IS THE GOOD HALF. It exits Unavail rather than reporting success, and the diagnostic quotes the exact name it looked for, which is what made this diagnosable in one step rather than by reading code. A self-updater that silently did nothing would be far worse. Only the resolution is wrong.

CHECK THE WHOLE PATH, NOT ONLY THE LOOKUP. assetName is threaded into three places — the asset match, verifyChecksum(archiveData, assetName, checksumURL), and extractBinary(archiveData, assetName). checksums.txt lists the versioned names, so a fix that only corrects the download lookup moves the failure into checksum verification instead of removing it. Whatever produces the name has to produce the same string all three uses expect.

DESIRED, not a design: self-update resolves an asset that exists, and stays correct if the archive naming changes again. The release''s own version is already in hand as rel.TagName at the point of the lookup, so building the name from it is one option; matching by platform suffix rather than exact equality is another; reading checksums.txt as the manifest of what the release actually shipped is a third, and has the property that the name is no longer inferred at all. Preferring an option that derives the name from the release rather than from a template repeated in two places would keep this from recurring — the defect is precisely that two files independently describe one string.

WORTH A TEST THAT WOULD HAVE CAUGHT IT: nothing asserts that the name self-update constructs matches what .goreleaser.yaml produces. The two live in different languages in different files, which is why they drifted silently. A test that renders the goreleaser template, or that checks the constructed name against a real release''s asset list, closes it.

THE FIX IS SMALLER THAN THE OPTIONS ABOVE SUGGEST: the data is already in the function and is discarded.

runSelfUpdate has both halves before it needs them:

  rel, err := fetchLatestRelease()                             // rel.Assets is the published file list
  latestVersion := strings.TrimPrefix(rel.TagName, "v")        // the version, five lines above the defect
  ...
  assetName := archiveNameForPlatform()                        // ignores both and guesses

It then loops rel.Assets comparing each real name against the constructed one. So the release''s own manifest is already loaded, already parsed, and already being iterated — the bug is not a missing lookup, it is a guess being preferred over data in hand. No extra request, no template rendering, and nothing to keep in step with .goreleaser.yaml.

TWO SHAPES, BOTH USING WHAT IS ALREADY THERE:

  1. Select from rel.Assets by platform suffix. The loop already walks every published name; matching on the OS/arch/extension tail rather than on full equality means the version segment never has to be known, and a future change to the prefix cannot break it. This removes name construction entirely.

  2. Interpolate latestVersion into the constructed name. One variable, already computed. Smaller diff, but it keeps two descriptions of one string and only resynchronises them — the next naming change breaks it again.

The first is preferable for the reason this ticket exists: the defect is that two files independently describe one string, and only the first shape stops describing it twice. Whichever is chosen, the same string must reach verifyChecksum and extractBinary, since checksums.txt lists the versioned names.

Worth noting the diagnostic already proves the list was available — ''no asset "pql_Linux_x86_64.tar.gz" in release v2.3.0'' is printed by code that has just finished iterating the assets it could have chosen from.', '`pql self-update` cannot succeed on any platform for any release. It constructs the release asset''s name without the version, and goreleaser publishes it with the version.

  internal/cli/selfupdate.go:142   fmt.Sprintf("pql_%s_%s.%s", osName, archName, ext)   -> pql_Linux_x86_64.tar.gz
  .goreleaser.yaml:32-37           {{ .ProjectName }}_{{ .Version }}_{{ title .Os }}_...  -> pql_2.3.0_Linux_x86_64.tar.gz

OBSERVED on 2.2.0 updating to v2.3.0: exit 69 with {"code":"cli.exit","msg":"no asset \"pql_Linux_x86_64.tar.gz\" in release v2.3.0"}. The release does carry pql_2.3.0_Linux_x86_64.tar.gz, alongside Darwin arm64/x86_64, Linux arm64 and a Windows zip. v2.2.0''s assets follow the same versioned naming, so this is not a regression introduced by the 2.3.0 release — no released binary has been able to update itself.

THE FAILURE IS LOUD, WHICH IS THE GOOD HALF. It exits Unavail rather than reporting success, and the diagnostic quotes the exact name it looked for, which is what made this diagnosable in one step rather than by reading code. A self-updater that silently did nothing would be far worse. Only the resolution is wrong.

CHECK THE WHOLE PATH, NOT ONLY THE LOOKUP. assetName is threaded into three places — the asset match, verifyChecksum(archiveData, assetName, checksumURL), and extractBinary(archiveData, assetName). checksums.txt lists the versioned names, so a fix that only corrects the download lookup moves the failure into checksum verification instead of removing it. Whatever produces the name has to produce the same string all three uses expect.

DESIRED, not a design: self-update resolves an asset that exists, and stays correct if the archive naming changes again. The release''s own version is already in hand as rel.TagName at the point of the lookup, so building the name from it is one option; matching by platform suffix rather than exact equality is another; reading checksums.txt as the manifest of what the release actually shipped is a third, and has the property that the name is no longer inferred at all. Preferring an option that derives the name from the release rather than from a template repeated in two places would keep this from recurring — the defect is precisely that two files independently describe one string.

WORTH A TEST THAT WOULD HAVE CAUGHT IT: nothing asserts that the name self-update constructs matches what .goreleaser.yaml produces. The two live in different languages in different files, which is why they drifted silently. A test that renders the goreleaser template, or that checks the constructed name against a real release''s asset list, closes it.

THE FIX IS SMALLER THAN THE OPTIONS ABOVE SUGGEST: the data is already in the function and is discarded.

runSelfUpdate has both halves before it needs them:

  rel, err := fetchLatestRelease()                             // rel.Assets is the published file list
  latestVersion := strings.TrimPrefix(rel.TagName, "v")        // the version, five lines above the defect
  ...
  assetName := archiveNameForPlatform()                        // ignores both and guesses

It then loops rel.Assets comparing each real name against the constructed one. So the release''s own manifest is already loaded, already parsed, and already being iterated — the bug is not a missing lookup, it is a guess being preferred over data in hand. No extra request, no template rendering, and nothing to keep in step with .goreleaser.yaml.

TWO SHAPES, BOTH USING WHAT IS ALREADY THERE:

  1. Select from rel.Assets by platform suffix. The loop already walks every published name; matching on the OS/arch/extension tail rather than on full equality means the version segment never has to be known, and a future change to the prefix cannot break it. This removes name construction entirely.

  2. Interpolate latestVersion into the constructed name. One variable, already computed. Smaller diff, but it keeps two descriptions of one string and only resynchronises them — the next naming change breaks it again.

The first is preferable for the reason this ticket exists: the defect is that two files independently describe one string, and only the first shape stops describing it twice. Whichever is chosen, the same string must reach verifyChecksum and extractBinary, since checksums.txt lists the versioned names.

Worth noting the diagnostic already proves the list was available — ''no asset "pql_Linux_x86_64.tar.gz" in release v2.3.0'' is printed by code that has just finished iterating the assets it could have chosen from.

RETRACTING THE THIRD OPTION offered above, so this ticket stops recommending a worse path than the one it later argues for.

''Reading checksums.txt as the manifest'' was written before noticing that rel.Assets is already fetched and already iterated in the same function. It costs a second request to obtain a list the code is holding, and it makes the checksum file load-bearing for asset discovery as well as verification — coupling two concerns that are currently independent. There are two shapes, not three, and the suffix match is the one that stops describing the filename twice.', NULL, '2026-09-02 18:03:32', '2026-09-02 18:03:32.103', '2026-09-02 18:03:32.103', NULL, '102fe46ce8fde38838cd8ff3d00439f3', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89QDBQTD7E1H8XZ8B4BDEQ8', 'description', NULL, '`internal/connect/signal/recency.go:23` scores from `time.Since(time.Unix(mtime, 0))`.
The reference point is the moment the query runs, so an unchanged vault produces
different scores on every invocation.

This is not merely cosmetic drift — it reorders results, via D-11''s
max-normalization.

Mechanism. With `raw_i = 1 - age_i/2160` and `normalized_i = raw_i / max(raw)`,
let all files age by the same δ. Both numerator and denominator shrink by δ, and
for any candidate below the maximum that ratio *falls*:

    (raw_i - δ) / (raw_max - δ)  <  raw_i / raw_max     for raw_i < raw_max

So the normalized recency spread widens monotonically over time while every
other signal stays fixed. In a weighted sum against centrality and the rest,
two candidates whose totals sat close together will eventually swap. Nothing
about the vault changed.

The 90-day clamp compounds it: files aging past `decayHours` pin to 0 and
collapse into ties, which then hit the unstable sort in the sibling ticket.

D-11 accepts that "the same file can score differently in different queries…
rankings are always relative". That reasoning covers batch-relative scoring and
does not extend to this: here the same file scores differently in *the same*
query at a different time, and the relative order changes with it.

Exposure is high because recency carries weight 0.25 on `search` and is often
the only non-zero signal. Observed on pql''s own vault:

    pql search architecture --full
    → top hit scores exactly 0.25, every signal zero except recency

That result is ordered purely by a float derived from the current time.

Possible directions, cheapest first:

1. Make the reference time injectable — a clock on `signal.Context`, defaulting
   to `time.Now()`. Tests pin it; production is unchanged. Fixes reproducibility
   without touching semantics.
2. Derive the reference from the corpus (e.g. max mtime in the candidate batch)
   rather than wall-clock, making recency purely a function of the vault.
3. Let a vault zero the recency weight in its profile. Purely additive, useful
   for authored corpora where mtime is an install artifact rather than a date.

(1) and (3) are compatible and neither changes behaviour for existing callers.
(2) is a semantic change and wants its own decision.

Note this touches only the ranking signal. It is independent of the other two
mtime consumers — `index/indexer.go` change detection and
`planning/changelog/importer.go` cross-repo sync — which read mtime from
`os.FileInfo` and the files table directly and never go through this path.', NULL, '2026-09-09 06:34:06', '2026-09-09 06:34:06.575', '2026-09-09 06:34:06.575', NULL, '485cd77ed830c0206ac0ee63a644ac6a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89QXQ7QBFKMY53RAXEGYP1C', 'description', NULL, '`internal/connect/rank.go:52` sorts with `sort.Slice` and a comparator that
compares `Score` alone:

    sort.Slice(enriched, func(i, j int) bool {
        return enriched[i].Score > enriched[j].Score
    })

`sort.Slice` is not stable, and the comparator defines only a partial order, so
tied candidates come out in whatever arrangement pdqsort leaves them in. That is
a function of the input order, which the sibling ticket shows is itself not
pinned.

Ties are common rather than exotic. Any link-sparse vault gives centrality,
link_overlap and tag_overlap of 0 across the whole batch, so the score collapses
onto one or two signals and clusters. Files aged past the 90-day recency clamp
tie at exactly 0.

Fix is two lines — a total order plus a stable sort:

    sort.SliceStable(enriched, func(i, j int) bool {
        if enriched[i].Score != enriched[j].Score {
            return enriched[i].Score > enriched[j].Score
        }
        return enriched[i].Path < enriched[j].Path
    })

The `Path` tie-break is the load-bearing half: it makes the ordering total, so
the result no longer depends on candidate order at all. That is worth having
regardless of what the sibling ticket does about ORDER BY, and it is the cheapest
way to make ranked output reproducible.

`SliceStable` on its own would only preserve an input order that is not
guaranteed, so prefer the explicit tie-break over relying on stability.', NULL, '2026-09-09 06:34:34', '2026-09-09 06:34:34.968', '2026-09-09 06:34:34.968', NULL, 'dc8a8dae43d9507abd5a2a1f144f6fb6', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89R15HFKK0Y3EFGJXDAQEBG', 'description', NULL, 'None of the three `gatherCandidates` functions pins row order:

- `internal/intent/search/search.go:43`
- `internal/intent/related/related.go:41`
- `internal/intent/context/context.go:57`

Each is a `SELECT DISTINCT path FROM ( ... UNION ... )` with no ORDER BY. SQLite
does not guarantee row order without one.

In practice the order is stable today: `UNION` (as opposed to `UNION ALL`)
requires deduplication, which SQLite currently implements with a temporary
B-tree, and that happens to emit rows sorted. So this is latent, not an active
bug — which is precisely why it is worth pinning explicitly rather than leaving
to chance. The behaviour rests on a query-planner implementation detail, and a
SQLite upgrade, an added index, or different ANALYZE stats can change it with no
change to this code. That failure would be silent: results shift, nothing errors.

Fix is `ORDER BY path` on the outer SELECT in all three.

Worth doing even after the tie-break in the sibling ticket lands. The tie-break
makes the final output order independent of candidate order, so the two are not
redundant defences of the same thing:

- ORDER BY pins what the ranking layer *receives*, which matters for debugging,
  for reproducible `--full` signal output, and for anything that reads
  candidates before scoring.
- The tie-break pins what callers *see*.

Cost is negligible: these batches are small and the temp B-tree is already
sorting.', NULL, '2026-09-09 06:34:51', '2026-09-09 06:34:51.932', '2026-09-09 06:34:51.932', NULL, '750165e7b0c6e66e873c97103d1bf572', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89R35V3EJQDYRRW7K98VWZG', 'description', NULL, '`search`, `related` and `context` can return different results for an unchanged
vault, across three independent causes. None is a scoring-quality question — the
issue is that the same inputs do not reliably produce the same output.

Found 2026-09-06 while evaluating pql as the corpus substrate for another
project that needs byte-reproducible query results for headless testing. That
project has since gone its own way, so this is filed purely on pql''s own merits:
several consuming repos call the ranked verbs today, so unreproducible output is
a live problem rather than a hypothetical one.

Children:

- Recency signal derives from wall-clock (the one that actively changes results)
- Rank sorts with an unstable sort and no tie-break
- gatherCandidates has no ORDER BY in any of the three verbs

The last two are latent rather than active: SQLite''s UNION dedup currently emits
rows in sorted order via a temp B-tree, so candidate order happens to be stable
today. That is a query-planner implementation detail, not a contract — a SQLite
upgrade, a new index, or ANALYZE stats can change it with no code change here.

Suggested order of work: the two ordering fixes first (cheap, self-contained,
and together they make ranking total-ordered regardless of what SQLite does),
then the recency clock, which needs a design call rather than a patch.', NULL, '2026-09-09 06:34:55', '2026-09-09 06:34:55.717', '2026-09-09 06:34:55.717', NULL, '6a5ed125c6d563c13ec0f8956ae5a1f0', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89QDBQTD7E1H8XZ8B4BDEQ8', 'parent_id', NULL, 'T-129', NULL, '2026-09-09 06:34:58', '2026-09-09 06:34:58.166', '2026-09-09 06:34:58.166', NULL, '750b6f0d1256d064eaaa05a26d1785fe', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89R15HFKK0Y3EFGJXDAQEBG', 'parent_id', NULL, 'T-129', NULL, '2026-09-09 06:34:58', '2026-09-09 06:34:58.174', '2026-09-09 06:34:58.174', NULL, '21d9be37ee3d22792d1a3b68bdb1a68b', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89QXQ7QBFKMY53RAXEGYP1C', 'parent_id', NULL, 'T-129', NULL, '2026-09-09 06:34:58', '2026-09-09 06:34:58.174', '2026-09-09 06:34:58.174', NULL, '93a7008d7f59eaf9f715d5ea5d0c2ff0', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89S0F3600NPD1978YNJP9CR', 'description', NULL, 'Found 2026-09-09 while removing sibling-repo names and a hostname from a ticket
description before pushing. `make secrets` caught it, which is the system
working — CLAUDE.md is explicit that ticket prose is published prose, and the
gate ran before anything left the machine. What is missing is the route from
"caught it" back to a clean changelog.

WHY THE OBVIOUS FIXES DO NOT WORK

Fixing forward makes it worse. `ticket refine write` appends a ticket_history
row whose `old_value` is the previous description, so correcting a leaked
description writes the leaked text into the changelog a second time. The
outgoing diff then contains two copies rather than none.

Wiping and re-exporting does not work either. `plan export` emits "every
replicated planning row that has been modified since the last export", so it is
watermark-driven. Restoring the changelog files to an earlier state and
re-running it re-emits only rows touched since the watermark — in this case two
rows out of the fifteen or so that were needed. The rest stayed in pql.db,
absent from the changelog, with no supported way to get them back out.

WHAT IT ACTUALLY TOOK

    git reset --soft origin/main
    git checkout origin/main -- .pql/changelog
    rm .pql/pql.db
    pql plan rebuild --verify
    # then re-create four tickets by hand, re-entering every description,
    # and re-attach the parent links

That works and `--verify` reported 0 rows lost, but it is a hand-rolled
procedure recovered from reading the exporter''s help text under time pressure,
and the re-entry step is transcription with no check on it. Anyone hitting this
without the descriptions still in front of them loses the prose.

THE ASYMMETRY

`plan rebuild` reconstructs pql.db from the changelog and can be forced at any
time. There is no inverse. The pair is documented as replication, but only one
direction can be regenerated on demand — the other is append-only and
watermarked, so pql.db''s current state cannot be re-expressed as changelog
content once the watermark has passed it.

THE DESIGN QUESTION UNDERNEATH, which is why this is not just a missing flag

Is the changelog an append-only *log*, or a materialised *replica*?

- As a log, redaction is illegitimate by construction and the honest answer is
  that scrubbing prose requires rewriting git history, with the D-16 hashes and
  LWW guards recomputed. A `--force-full` export would be a footgun that
  silently rewrites replicated history other clones have already replayed.
- As a replica, regenerating it from pql.db is the natural operation and its
  absence is the defect.

D-15 and D-16 lean toward log (monthly append files, inline LWW guards, content
hashes, ON CONFLICT DO NOTHING on replay), but `plan rebuild` treats pql.db as
fully derivable from it, which is replica-shaped. The two readings have not had
to disagree until now.

Worth noting the blast radius differs by case. Redacting a ticket that has never
been pushed — this case — touches nothing another clone has seen, and a full
regeneration is safe. Redacting one that has been replayed elsewhere is a
distributed-state problem and probably out of scope for any flag.

DESIRED, not a design

Someone who has just been told by `make secrets` that a ticket description
leaks should have a supported route to fix it that does not involve re-entering
prose. Shapes worth weighing:

- A full/forced export that rewrites the month''s files from pql.db, refusing (or
  loudly warning) when the affected rows appear in commits already pushed.
- A redact verb scoped to descriptions, which rewrites the row and its history
  entries in place rather than appending.
- Nothing in the tool, and instead a documented procedure in CLAUDE.md next to
  the "ticket prose is published prose" table — cheapest, and honest if the log
  reading wins.

The third is a legitimate outcome. What is not legitimate is the current state,
where the gate reliably catches the problem and the recovery is undocumented.

RELATED

D-15, D-16 - changelog replication and the guards that make replay safe.
T-123     - the other place the single-writer assumption shows at the seam.', NULL, '2026-09-09 06:39:18', '2026-09-09 06:39:18.723', '2026-09-09 06:39:18.723', NULL, 'f1ef7d75cdc7ca3d4decb1f264db022b', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY7JHX6RH2BQK3R9VVP1ZX68', 'parent_id', NULL, 'T-133', NULL, '2026-09-09 09:24:32', '2026-09-09 09:24:32.580', '2026-09-09 09:24:32.580', NULL, 'd5ba67be89d81c65dc531067a603f452', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FZ4FHC4YQRSRC071QNEWM64G', 'parent_id', NULL, 'T-133', NULL, '2026-09-09 09:24:32', '2026-09-09 09:24:32.588', '2026-09-09 09:24:32.588', NULL, 'ab0619c93591a2217dce2802b46dde0a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G64CE0WTT40RJ9TE0VT95FYW', 'parent_id', NULL, 'T-133', NULL, '2026-09-09 09:24:32', '2026-09-09 09:24:32.589', '2026-09-09 09:24:32.589', NULL, '8ec506982e9f728588e9126ea917b48d', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89S0F3600NPD1978YNJP9CR', 'parent_id', NULL, 'T-133', NULL, '2026-09-09 09:24:32', '2026-09-09 09:24:32.590', '2026-09-09 09:24:32.590', NULL, '8e1c0f3b8396bc3eb8fb5d3095b9fdcb', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FZ4CTAFNM4369HAGBYS2F728', 'description', '`pql ticket statuslist` already returns `is_terminal` per status. D-24 made the status vocabulary configurable and had the engine reason about *classes* rather than literal names, precisely so consumers would stop mirroring pql''s enum. So "is this ticket still open" is expressible in pql''s own model, and pql already computes it internally: the terminal set clears blockers, is excluded from `--unblocked` and from refine, and is what `plan whatsnext` keys off.

`ticket list` cannot ask it. `--status` takes one literal name and there is no flag for the distinction, so every caller reimplements the concept. The obvious implementation is to exclude the names `done` and `cancelled` — which are the *default* vocabulary, not the guaranteed one. A vault that configures its own statuses, which is the entire point of D-24, gets a silently wrong answer from that filter: closed work counted as open, no error, nothing spelled wrong anywhere. The failure is invisible in exactly the vaults the configurability was added for.

Suggested shape:

    pql ticket list --open      # status is_terminal = false
    pql ticket list --closed    # the complement

Composing with the existing filters the way `--leaf` and `--unblocked` already do.

The complement is worth naming separately, because it is the more thoroughly missing half. There is no way to ask "what reached a terminal status", so "what got closed" — the question a review or a release note is assembled from — has no expression at all. Note honestly that `--closed` alone does not finish that job: there is no date axis on `ticket list` either, so "closed in this period" still needs something `--closed` does not provide. Worth recording as the adjacent gap rather than smuggling into this one.

A generalisation to weigh before implementing: D-24 models four classes, not two, so a `--class terminal|review|active|initial` filter would cover this plus "what is in flight" with one flag rather than a pair, and would stay correct if a vault''s vocabulary grows. Against it: `--open` is the word callers actually reach for, and the terminal split is the one the engine treats as load-bearing everywhere else. Either is defensible; picking the class filter and documenting `--open` as its common case would be the more conservative choice.

This is not the absence filter D-31 declined, and the distinction is worth stating so the ticket is not read as relitigating it. Every filter D-31 refused is a join predicate in disguise — "has no linked ticket", "has no label" — a claim about the absence of rows in another table. `--open` is a predicate on a column this row already carries, resolved through a vocabulary pql itself defines and publishes via `statuslist`. It is nearer to `--status` than to `--unimplemented`: same axis, coarser grain, and it opens no door to `--untagged` or `--childless` because those are still about other tables. If that reading is wrong then this should be closed against D-31 rather than implemented — but the line D-31 draws looks stable under it.

One observation about what pql does today, because it argues for the flag rather than against the contract. A caller reaching for `--open` gets exit 64 and `{"level":"error","code":"cli.error","msg":"unknown flag: --open"}`. That is correct in every respect: an unknown flag is a caller mistake, the diagnostic names it, stdout stays empty. The failure observed was one layer up — a wrapper invoking pql with stderr suppressed, reading the empty stdout as "no open tickets", and reporting a clean board. pql said what was wrong and the caller discarded the channel it said it on. The interesting part is *why* the caller guessed that flag: because the concept exists in the model and is simply not reachable from the read surface. People reach for `--open` because pql already knows what open means.

Backward compatibility: an additive optional flag. Absent, nothing changes — no default projection change, unlike D-27.', '`pql ticket statuslist` already returns `is_terminal` per status. D-24 made the status vocabulary configurable and had the engine reason about *classes* rather than literal names, precisely so consumers would stop mirroring pql''s enum. So "is this ticket still open" is expressible in pql''s own model, and pql already computes it internally: the terminal set clears blockers, is excluded from `--unblocked` and from refine, and is what `plan whatsnext` keys off.

`ticket list` cannot ask it. `--status` takes one literal name and there is no flag for the distinction, so every caller reimplements the concept. The obvious implementation is to exclude the names `done` and `cancelled` — which are the *default* vocabulary, not the guaranteed one. A vault that configures its own statuses, which is the entire point of D-24, gets a silently wrong answer from that filter: closed work counted as open, no error, nothing spelled wrong anywhere. The failure is invisible in exactly the vaults the configurability was added for.

Suggested shape:

    pql ticket list --open      # status is_terminal = false
    pql ticket list --closed    # the complement

Composing with the existing filters the way `--leaf` and `--unblocked` already do.

The complement is worth naming separately, because it is the more thoroughly missing half. There is no way to ask "what reached a terminal status", so "what got closed" — the question a review or a release note is assembled from — has no expression at all. Note honestly that `--closed` alone does not finish that job: there is no date axis on `ticket list` either, so "closed in this period" still needs something `--closed` does not provide. Worth recording as the adjacent gap rather than smuggling into this one.

A generalisation to weigh before implementing: D-24 models four classes, not two, so a `--class terminal|review|active|initial` filter would cover this plus "what is in flight" with one flag rather than a pair, and would stay correct if a vault''s vocabulary grows. Against it: `--open` is the word callers actually reach for, and the terminal split is the one the engine treats as load-bearing everywhere else. Either is defensible; picking the class filter and documenting `--open` as its common case would be the more conservative choice.

This is not the absence filter D-31 declined, and the distinction is worth stating so the ticket is not read as relitigating it. Every filter D-31 refused is a join predicate in disguise — "has no linked ticket", "has no label" — a claim about the absence of rows in another table. `--open` is a predicate on a column this row already carries, resolved through a vocabulary pql itself defines and publishes via `statuslist`. It is nearer to `--status` than to `--unimplemented`: same axis, coarser grain, and it opens no door to `--untagged` or `--childless` because those are still about other tables. If that reading is wrong then this should be closed against D-31 rather than implemented — but the line D-31 draws looks stable under it.

One observation about what pql does today, because it argues for the flag rather than against the contract. A caller reaching for `--open` gets exit 64 and `{"level":"error","code":"cli.error","msg":"unknown flag: --open"}`. That is correct in every respect: an unknown flag is a caller mistake, the diagnostic names it, stdout stays empty. The failure observed was one layer up — a wrapper invoking pql with stderr suppressed, reading the empty stdout as "no open tickets", and reporting a clean board. pql said what was wrong and the caller discarded the channel it said it on. The interesting part is *why* the caller guessed that flag: because the concept exists in the model and is simply not reachable from the read surface. People reach for `--open` because pql already knows what open means.

Backward compatibility: an additive optional flag. Absent, nothing changes — no default projection change, unlike D-27.

AUDIT NOTE (2026-09-09). Half of this shipped in v2.1: ''ticket board --open'' exists (internal/cli/ticket.go:1304-1305) and drops terminal columns. ''ticket list'' still has no --open — its filters are --status, --team, --assigned, --decision, --label, --under, --leaf, --unblocked (ticket.go:419-426). Remaining scope narrows to the list verb: an --open flag (or equivalent terminal-class filter) so a caller can ask for actionable tickets without naming every non-terminal status. The status-class vocabulary from D-27 already models the distinction; this is surface, not model.', NULL, '2026-09-09 09:24:43', '2026-09-09 09:24:43.016', '2026-09-09 09:24:43.016', NULL, '26f263b1cc1df7086bf5b650260efdf4', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FYCKFVEFFH2DPVB97FM1NF6W', 'description', 'There is no way to ask ''has this already been raised?''. ''ticket list'' filters by status, team, assignee, label, decision, parent, leaf and unblocked - every axis except the words. So the check costs reading every row by eye, or a pipe to grep, and in an agent harness a pipe defeats prefix allowlisting and triggers a permission prompt. A check that expensive gets skipped, and then duplicates get filed. That happened in a vault I maintain on 2026-08-09: a new ticket restated an existing one that had already recorded, triaged and suppressed the same finding, at 41 tickets. This vault is at 99.

The gap looks coherent rather than accidental, which is why it is worth naming. ''pql search'' exists but searches the vault - paths, tags, frontmatter, headings - and tickets have no markdown source at all, they live only in SQLite and travel via the changelog. So the one search surface structurally cannot reach them, and the planning surface never grew its own.

I am aware of T-92 and the no-fake-filters principle, and I do not think this falls foul of it. The rejected spellings there are negative filters and join predicates in disguise, which do not compose and which an empty result would misrepresent as an answer. A substring match over title and description is neither: it is the same primitive ''pql search'' already applies to the vault, it composes with the existing filters rather than replacing them, and an empty result means what it says.

Suggested shape, matching what search already does elsewhere:

  pql ticket list --matching "gitea password"

One literal lowercase substring against title and description, combinable with --status and the rest. The caveat that applies to ''pql search'' applies here too and should be documented the same way: it is a substring filter, not a search engine, so a multi-word query is one literal string and an empty result is not evidence of absence.

Worth considering alongside: the same absence applies to decisions. ''decisions list'' filters by type, domain and status, so ''did we already decide this?'' has the identical problem, and it is the question most likely to be asked before writing a new record.', 'There is no way to ask ''has this already been raised?''. ''ticket list'' filters by status, team, assignee, label, decision, parent, leaf and unblocked - every axis except the words. So the check costs reading every row by eye, or a pipe to grep, and in an agent harness a pipe defeats prefix allowlisting and triggers a permission prompt. A check that expensive gets skipped, and then duplicates get filed. That happened in a vault I maintain on 2026-08-09: a new ticket restated an existing one that had already recorded, triaged and suppressed the same finding, at 41 tickets. This vault is at 99.

The gap looks coherent rather than accidental, which is why it is worth naming. ''pql search'' exists but searches the vault - paths, tags, frontmatter, headings - and tickets have no markdown source at all, they live only in SQLite and travel via the changelog. So the one search surface structurally cannot reach them, and the planning surface never grew its own.

I am aware of T-92 and the no-fake-filters principle, and I do not think this falls foul of it. The rejected spellings there are negative filters and join predicates in disguise, which do not compose and which an empty result would misrepresent as an answer. A substring match over title and description is neither: it is the same primitive ''pql search'' already applies to the vault, it composes with the existing filters rather than replacing them, and an empty result means what it says.

Suggested shape, matching what search already does elsewhere:

  pql ticket list --matching "gitea password"

One literal lowercase substring against title and description, combinable with --status and the rest. The caveat that applies to ''pql search'' applies here too and should be documented the same way: it is a substring filter, not a search engine, so a multi-word query is one literal string and an empty result is not evidence of absence.

Worth considering alongside: the same absence applies to decisions. ''decisions list'' filters by type, domain and status, so ''did we already decide this?'' has the identical problem, and it is the question most likely to be asked before writing a new record.

AUDIT NOTE (2026-09-09). Cross-link: open question Q-2 (FTS for ticket/decision search, governance/decisions/architecture.md) asks the same thing from the design side — whichever implementation lands should answer Q-2 and close both. Also note --grep shipped in v2.3 (T-113) and covers part of this: ''pql ticket list --grep <regex>'' filters any projected field case-insensitively. What it does not reach is description text unless projected, ranked results, or fuzzy match — the duplicate-detection use case this ticket names still stands, but the body should be read net of --grep.', NULL, '2026-09-09 09:24:48', '2026-09-09 09:24:48.464', '2026-09-09 09:24:48.464', NULL, 'd0f9d08726b260fc57cae12041892390', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G65T19H1A3JHSQ3T0KREGWXG', 'description', 'Found 2026-09-02 while fixing T-120. `make eval` is green again, but green
means less here than it looks, and the reason is the fixture rather than the
harness.

MEASURED, not inferred. From `search council --fields path,score,signals`
against testdata/council-snapshot:

- recency        raw=0 on every file in every case
- link_overlap   raw=0
- tag_overlap    raw=0
- path_proximity raw=0
- centrality     raw=1 on exactly one file, 0 on the rest

So one signal separates one file, and the other four separate nothing. A
weight change, a normalisation change, or an outright bug in four of the five
signals would not move a single number the eval reports.

TWO CAUSES, BOTH PROPERTIES OF THE SNAPSHOT

1. Uniform mtimes. git sets every file''s mtime to checkout time, so the whole
   fixture has one timestamp and recency - weighted 0.25 on search, the second
   heaviest signal there - normalises to zero across the board. This affects
   any consumer of the snapshot, not just the eval.

2. One link in the entire vault. members/vaasa/persona.md links to
   members/koskela/persona. That is the only edge, so centrality is 1 for the
   target and 0 for all 25 other files, link_overlap can never be non-zero
   (no file shares an edge with any other), and context''s candidate set is at
   most one file for any target.

WHY IT MATTERS MORE THAN A THIN FIXTURE USUALLY WOULD

The eval exists to make ranking regressions as visible as test failures - that
is its stated job in project-structure.md, and "ranking is the product" is the
philosophy doc''s line. An eval that cannot move under four of five signals is
not doing that job, and it reports NDCG=1.000 while not doing it, which is the
most misleading result available.

T-120 strengthened the harness assertion so a golden''s expectations must
actually hold. That was the right fix for what T-120 was about and does not
touch this: a stricter assertion over a fixture with no signal is still an
assertion over no signal.

DESIRED, not a design. Enough structure in the fixture that each weighted
signal can be non-zero and can differ between files - some link density, some
shared tags, and mtimes that vary. Options worth weighing rather than
picking blind:

- Refresh the snapshot from a richer source vault. `make refresh-fixtures`
  already exists; the question is whether the source has the structure.
- Author a synthetic fixture for eval specifically. internal/fixture/ is named
  in project-structure.md for exactly this and was never built.
- Set mtimes deliberately as a fixture step, since git will never preserve
  them. Cheap and fixes recency on its own.

The third is worth doing regardless of the other two - it is a few lines and
recency is the second-heaviest weight on search.

RELATED

T-120 - fixed the two wrong golden expectations and the weak assertion.
T-65  - build the FR-2 golden eval set; overlaps on what a good fixture is.', 'Found 2026-09-02 while fixing T-120. `make eval` is green again, but green
means less here than it looks, and the reason is the fixture rather than the
harness.

MEASURED, not inferred. From `search council --fields path,score,signals`
against testdata/council-snapshot:

- recency        raw=0 on every file in every case
- link_overlap   raw=0
- tag_overlap    raw=0
- path_proximity raw=0
- centrality     raw=1 on exactly one file, 0 on the rest

So one signal separates one file, and the other four separate nothing. A
weight change, a normalisation change, or an outright bug in four of the five
signals would not move a single number the eval reports.

TWO CAUSES, BOTH PROPERTIES OF THE SNAPSHOT

1. Uniform mtimes. git sets every file''s mtime to checkout time, so the whole
   fixture has one timestamp and recency - weighted 0.25 on search, the second
   heaviest signal there - normalises to zero across the board. This affects
   any consumer of the snapshot, not just the eval.

2. One link in the entire vault. members/vaasa/persona.md links to
   members/koskela/persona. That is the only edge, so centrality is 1 for the
   target and 0 for all 25 other files, link_overlap can never be non-zero
   (no file shares an edge with any other), and context''s candidate set is at
   most one file for any target.

WHY IT MATTERS MORE THAN A THIN FIXTURE USUALLY WOULD

The eval exists to make ranking regressions as visible as test failures - that
is its stated job in project-structure.md, and "ranking is the product" is the
philosophy doc''s line. An eval that cannot move under four of five signals is
not doing that job, and it reports NDCG=1.000 while not doing it, which is the
most misleading result available.

T-120 strengthened the harness assertion so a golden''s expectations must
actually hold. That was the right fix for what T-120 was about and does not
touch this: a stricter assertion over a fixture with no signal is still an
assertion over no signal.

DESIRED, not a design. Enough structure in the fixture that each weighted
signal can be non-zero and can differ between files - some link density, some
shared tags, and mtimes that vary. Options worth weighing rather than
picking blind:

- Refresh the snapshot from a richer source vault. `make refresh-fixtures`
  already exists; the question is whether the source has the structure.
- Author a synthetic fixture for eval specifically. internal/fixture/ is named
  in project-structure.md for exactly this and was never built.
- Set mtimes deliberately as a fixture step, since git will never preserve
  them. Cheap and fixes recency on its own.

The third is worth doing regardless of the other two - it is a few lines and
recency is the second-heaviest weight on search.

RELATED

T-120 - fixed the two wrong golden expectations and the weak assertion.
T-65  - build the FR-2 golden eval set; overlaps on what a good fixture is.

AUDIT NOTE (2026-09-09). Concrete evidence from a structure audit: testdata/council-snapshot is checked out by git, so every file shares the clone''s mtime — the recency signal is a constant across the corpus — and the fixture''s link graph is近 empty, so link_overlap, tag_overlap and path_proximity contribute (near-)constant scores too. Four of five signals flat means the eval can only detect regressions in the textual signal. Cross-links: T-129 (ranked-verb reproducibility epic — a fixture that exercises all signals is also what makes those fixes verifiable) and T-120 (the golden set is currently red, so eval output is read as a diff, not pass/fail).', NULL, '2026-09-09 09:24:57', '2026-09-09 09:24:57.620', '2026-09-09 09:24:57.620', NULL, 'f55b91a441b59349a4e85a63d391e6e9', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G65T19H1A3JHSQ3T0KREGWXG', 'description', 'Found 2026-09-02 while fixing T-120. `make eval` is green again, but green
means less here than it looks, and the reason is the fixture rather than the
harness.

MEASURED, not inferred. From `search council --fields path,score,signals`
against testdata/council-snapshot:

- recency        raw=0 on every file in every case
- link_overlap   raw=0
- tag_overlap    raw=0
- path_proximity raw=0
- centrality     raw=1 on exactly one file, 0 on the rest

So one signal separates one file, and the other four separate nothing. A
weight change, a normalisation change, or an outright bug in four of the five
signals would not move a single number the eval reports.

TWO CAUSES, BOTH PROPERTIES OF THE SNAPSHOT

1. Uniform mtimes. git sets every file''s mtime to checkout time, so the whole
   fixture has one timestamp and recency - weighted 0.25 on search, the second
   heaviest signal there - normalises to zero across the board. This affects
   any consumer of the snapshot, not just the eval.

2. One link in the entire vault. members/vaasa/persona.md links to
   members/koskela/persona. That is the only edge, so centrality is 1 for the
   target and 0 for all 25 other files, link_overlap can never be non-zero
   (no file shares an edge with any other), and context''s candidate set is at
   most one file for any target.

WHY IT MATTERS MORE THAN A THIN FIXTURE USUALLY WOULD

The eval exists to make ranking regressions as visible as test failures - that
is its stated job in project-structure.md, and "ranking is the product" is the
philosophy doc''s line. An eval that cannot move under four of five signals is
not doing that job, and it reports NDCG=1.000 while not doing it, which is the
most misleading result available.

T-120 strengthened the harness assertion so a golden''s expectations must
actually hold. That was the right fix for what T-120 was about and does not
touch this: a stricter assertion over a fixture with no signal is still an
assertion over no signal.

DESIRED, not a design. Enough structure in the fixture that each weighted
signal can be non-zero and can differ between files - some link density, some
shared tags, and mtimes that vary. Options worth weighing rather than
picking blind:

- Refresh the snapshot from a richer source vault. `make refresh-fixtures`
  already exists; the question is whether the source has the structure.
- Author a synthetic fixture for eval specifically. internal/fixture/ is named
  in project-structure.md for exactly this and was never built.
- Set mtimes deliberately as a fixture step, since git will never preserve
  them. Cheap and fixes recency on its own.

The third is worth doing regardless of the other two - it is a few lines and
recency is the second-heaviest weight on search.

RELATED

T-120 - fixed the two wrong golden expectations and the weak assertion.
T-65  - build the FR-2 golden eval set; overlaps on what a good fixture is.

AUDIT NOTE (2026-09-09). Concrete evidence from a structure audit: testdata/council-snapshot is checked out by git, so every file shares the clone''s mtime — the recency signal is a constant across the corpus — and the fixture''s link graph is近 empty, so link_overlap, tag_overlap and path_proximity contribute (near-)constant scores too. Four of five signals flat means the eval can only detect regressions in the textual signal. Cross-links: T-129 (ranked-verb reproducibility epic — a fixture that exercises all signals is also what makes those fixes verifiable) and T-120 (the golden set is currently red, so eval output is read as a diff, not pass/fail).', 'Found 2026-09-02 while fixing T-120. `make eval` is green again, but green
means less here than it looks, and the reason is the fixture rather than the
harness.

MEASURED, not inferred. From `search council --fields path,score,signals`
against testdata/council-snapshot:

- recency        raw=0 on every file in every case
- link_overlap   raw=0
- tag_overlap    raw=0
- path_proximity raw=0
- centrality     raw=1 on exactly one file, 0 on the rest

So one signal separates one file, and the other four separate nothing. A
weight change, a normalisation change, or an outright bug in four of the five
signals would not move a single number the eval reports.

TWO CAUSES, BOTH PROPERTIES OF THE SNAPSHOT

1. Uniform mtimes. git sets every file''s mtime to checkout time, so the whole
   fixture has one timestamp and recency - weighted 0.25 on search, the second
   heaviest signal there - normalises to zero across the board. This affects
   any consumer of the snapshot, not just the eval.

2. One link in the entire vault. members/vaasa/persona.md links to
   members/koskela/persona. That is the only edge, so centrality is 1 for the
   target and 0 for all 25 other files, link_overlap can never be non-zero
   (no file shares an edge with any other), and context''s candidate set is at
   most one file for any target.

WHY IT MATTERS MORE THAN A THIN FIXTURE USUALLY WOULD

The eval exists to make ranking regressions as visible as test failures - that
is its stated job in project-structure.md, and "ranking is the product" is the
philosophy doc''s line. An eval that cannot move under four of five signals is
not doing that job, and it reports NDCG=1.000 while not doing it, which is the
most misleading result available.

T-120 strengthened the harness assertion so a golden''s expectations must
actually hold. That was the right fix for what T-120 was about and does not
touch this: a stricter assertion over a fixture with no signal is still an
assertion over no signal.

DESIRED, not a design. Enough structure in the fixture that each weighted
signal can be non-zero and can differ between files - some link density, some
shared tags, and mtimes that vary. Options worth weighing rather than
picking blind:

- Refresh the snapshot from a richer source vault. `make refresh-fixtures`
  already exists; the question is whether the source has the structure.
- Author a synthetic fixture for eval specifically. internal/fixture/ is named
  in project-structure.md for exactly this and was never built.
- Set mtimes deliberately as a fixture step, since git will never preserve
  them. Cheap and fixes recency on its own.

The third is worth doing regardless of the other two - it is a few lines and
recency is the second-heaviest weight on search.

RELATED

T-120 - fixed the two wrong golden expectations and the weak assertion.
T-65  - build the FR-2 golden eval set; overlaps on what a good fixture is.

AUDIT NOTE (2026-09-09). Concrete evidence from a structure audit: testdata/council-snapshot is checked out by git, so every file shares the clone''s mtime — the recency signal is a constant across the corpus — and the fixture''s link graph is near-empty, so link_overlap, tag_overlap and path_proximity contribute (near-)constant scores too. Four of five signals flat means the eval can only detect regressions in the textual signal. Cross-links: T-129 (ranked-verb reproducibility epic — a fixture that exercises all signals is also what makes those fixes verifiable) and T-120 (the golden set is currently red, so eval output is read as a diff, not pass/fail).', NULL, '2026-09-09 09:25:12', '2026-09-09 09:25:12.891', '2026-09-09 09:25:12.891', NULL, '0305fcd972091bb9c3e7bc8dccc4710a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G8AYTGD7JX35YTJSCAP1PEBG', 'assigned_to', NULL, 'claude', NULL, '2026-09-09 09:34:45', '2026-09-09 09:34:45.411', '2026-09-09 09:34:45.411', NULL, '55920b017955e1338d44dfbab90ead2b', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G8AYTGD7JX35YTJSCAP1PEBG', 'status', 'backlog', 'in_progress', NULL, '2026-09-09 09:34:51', '2026-09-09 09:34:51.735', '2026-09-09 09:34:51.735', NULL, '67011964190321c64db9de8b0828723f', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G8AYTGD7JX35YTJSCAP1PEBG', 'status', 'in_progress', 'done', NULL, '2026-09-09 09:36:52', '2026-09-09 09:36:52.082', '2026-09-09 09:36:52.082', NULL, '9b9078b2f1a1b8399b6355364dbaa018', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G66AQKC2Z6RAJZ0GQA725GWW', 'status', 'backlog', 'in_progress', NULL, '2026-09-09 09:41:13', '2026-09-09 09:41:13.041', '2026-09-09 09:41:13.041', NULL, '2e3fe4a0b5dae6a62c3bcc72a6f6cfdf', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G66AQKC2Z6RAJZ0GQA725GWW', 'description', '`pql self-update` cannot succeed on any platform for any release. It constructs the release asset''s name without the version, and goreleaser publishes it with the version.

  internal/cli/selfupdate.go:142   fmt.Sprintf("pql_%s_%s.%s", osName, archName, ext)   -> pql_Linux_x86_64.tar.gz
  .goreleaser.yaml:32-37           {{ .ProjectName }}_{{ .Version }}_{{ title .Os }}_...  -> pql_2.3.0_Linux_x86_64.tar.gz

OBSERVED on 2.2.0 updating to v2.3.0: exit 69 with {"code":"cli.exit","msg":"no asset \"pql_Linux_x86_64.tar.gz\" in release v2.3.0"}. The release does carry pql_2.3.0_Linux_x86_64.tar.gz, alongside Darwin arm64/x86_64, Linux arm64 and a Windows zip. v2.2.0''s assets follow the same versioned naming, so this is not a regression introduced by the 2.3.0 release — no released binary has been able to update itself.

THE FAILURE IS LOUD, WHICH IS THE GOOD HALF. It exits Unavail rather than reporting success, and the diagnostic quotes the exact name it looked for, which is what made this diagnosable in one step rather than by reading code. A self-updater that silently did nothing would be far worse. Only the resolution is wrong.

CHECK THE WHOLE PATH, NOT ONLY THE LOOKUP. assetName is threaded into three places — the asset match, verifyChecksum(archiveData, assetName, checksumURL), and extractBinary(archiveData, assetName). checksums.txt lists the versioned names, so a fix that only corrects the download lookup moves the failure into checksum verification instead of removing it. Whatever produces the name has to produce the same string all three uses expect.

DESIRED, not a design: self-update resolves an asset that exists, and stays correct if the archive naming changes again. The release''s own version is already in hand as rel.TagName at the point of the lookup, so building the name from it is one option; matching by platform suffix rather than exact equality is another; reading checksums.txt as the manifest of what the release actually shipped is a third, and has the property that the name is no longer inferred at all. Preferring an option that derives the name from the release rather than from a template repeated in two places would keep this from recurring — the defect is precisely that two files independently describe one string.

WORTH A TEST THAT WOULD HAVE CAUGHT IT: nothing asserts that the name self-update constructs matches what .goreleaser.yaml produces. The two live in different languages in different files, which is why they drifted silently. A test that renders the goreleaser template, or that checks the constructed name against a real release''s asset list, closes it.

THE FIX IS SMALLER THAN THE OPTIONS ABOVE SUGGEST: the data is already in the function and is discarded.

runSelfUpdate has both halves before it needs them:

  rel, err := fetchLatestRelease()                             // rel.Assets is the published file list
  latestVersion := strings.TrimPrefix(rel.TagName, "v")        // the version, five lines above the defect
  ...
  assetName := archiveNameForPlatform()                        // ignores both and guesses

It then loops rel.Assets comparing each real name against the constructed one. So the release''s own manifest is already loaded, already parsed, and already being iterated — the bug is not a missing lookup, it is a guess being preferred over data in hand. No extra request, no template rendering, and nothing to keep in step with .goreleaser.yaml.

TWO SHAPES, BOTH USING WHAT IS ALREADY THERE:

  1. Select from rel.Assets by platform suffix. The loop already walks every published name; matching on the OS/arch/extension tail rather than on full equality means the version segment never has to be known, and a future change to the prefix cannot break it. This removes name construction entirely.

  2. Interpolate latestVersion into the constructed name. One variable, already computed. Smaller diff, but it keeps two descriptions of one string and only resynchronises them — the next naming change breaks it again.

The first is preferable for the reason this ticket exists: the defect is that two files independently describe one string, and only the first shape stops describing it twice. Whichever is chosen, the same string must reach verifyChecksum and extractBinary, since checksums.txt lists the versioned names.

Worth noting the diagnostic already proves the list was available — ''no asset "pql_Linux_x86_64.tar.gz" in release v2.3.0'' is printed by code that has just finished iterating the assets it could have chosen from.

RETRACTING THE THIRD OPTION offered above, so this ticket stops recommending a worse path than the one it later argues for.

''Reading checksums.txt as the manifest'' was written before noticing that rel.Assets is already fetched and already iterated in the same function. It costs a second request to obtain a list the code is holding, and it makes the checksum file load-bearing for asset discovery as well as verification — coupling two concerns that are currently independent. There are two shapes, not three, and the suffix match is the one that stops describing the filename twice.', '`pql self-update` cannot succeed on any platform for any release. It constructs the release asset''s name without the version, and goreleaser publishes it with the version.

  internal/cli/selfupdate.go:142   fmt.Sprintf("pql_%s_%s.%s", osName, archName, ext)   -> pql_Linux_x86_64.tar.gz
  .goreleaser.yaml:32-37           {{ .ProjectName }}_{{ .Version }}_{{ title .Os }}_...  -> pql_2.3.0_Linux_x86_64.tar.gz

OBSERVED on 2.2.0 updating to v2.3.0: exit 69 with {"code":"cli.exit","msg":"no asset \"pql_Linux_x86_64.tar.gz\" in release v2.3.0"}. The release does carry pql_2.3.0_Linux_x86_64.tar.gz, alongside Darwin arm64/x86_64, Linux arm64 and a Windows zip. v2.2.0''s assets follow the same versioned naming, so this is not a regression introduced by the 2.3.0 release — no released binary has been able to update itself.

THE FAILURE IS LOUD, WHICH IS THE GOOD HALF. It exits Unavail rather than reporting success, and the diagnostic quotes the exact name it looked for, which is what made this diagnosable in one step rather than by reading code. A self-updater that silently did nothing would be far worse. Only the resolution is wrong.

CHECK THE WHOLE PATH, NOT ONLY THE LOOKUP. assetName is threaded into three places — the asset match, verifyChecksum(archiveData, assetName, checksumURL), and extractBinary(archiveData, assetName). checksums.txt lists the versioned names, so a fix that only corrects the download lookup moves the failure into checksum verification instead of removing it. Whatever produces the name has to produce the same string all three uses expect.

DESIRED, not a design: self-update resolves an asset that exists, and stays correct if the archive naming changes again. The release''s own version is already in hand as rel.TagName at the point of the lookup, so building the name from it is one option; matching by platform suffix rather than exact equality is another; reading checksums.txt as the manifest of what the release actually shipped is a third, and has the property that the name is no longer inferred at all. Preferring an option that derives the name from the release rather than from a template repeated in two places would keep this from recurring — the defect is precisely that two files independently describe one string.

WORTH A TEST THAT WOULD HAVE CAUGHT IT: nothing asserts that the name self-update constructs matches what .goreleaser.yaml produces. The two live in different languages in different files, which is why they drifted silently. A test that renders the goreleaser template, or that checks the constructed name against a real release''s asset list, closes it.

THE FIX IS SMALLER THAN THE OPTIONS ABOVE SUGGEST: the data is already in the function and is discarded.

runSelfUpdate has both halves before it needs them:

  rel, err := fetchLatestRelease()                             // rel.Assets is the published file list
  latestVersion := strings.TrimPrefix(rel.TagName, "v")        // the version, five lines above the defect
  ...
  assetName := archiveNameForPlatform()                        // ignores both and guesses

It then loops rel.Assets comparing each real name against the constructed one. So the release''s own manifest is already loaded, already parsed, and already being iterated — the bug is not a missing lookup, it is a guess being preferred over data in hand. No extra request, no template rendering, and nothing to keep in step with .goreleaser.yaml.

TWO SHAPES, BOTH USING WHAT IS ALREADY THERE:

  1. Select from rel.Assets by platform suffix. The loop already walks every published name; matching on the OS/arch/extension tail rather than on full equality means the version segment never has to be known, and a future change to the prefix cannot break it. This removes name construction entirely.

  2. Interpolate latestVersion into the constructed name. One variable, already computed. Smaller diff, but it keeps two descriptions of one string and only resynchronises them — the next naming change breaks it again.

The first is preferable for the reason this ticket exists: the defect is that two files independently describe one string, and only the first shape stops describing it twice. Whichever is chosen, the same string must reach verifyChecksum and extractBinary, since checksums.txt lists the versioned names.

Worth noting the diagnostic already proves the list was available — ''no asset "pql_Linux_x86_64.tar.gz" in release v2.3.0'' is printed by code that has just finished iterating the assets it could have chosen from.

RETRACTING THE THIRD OPTION offered above, so this ticket stops recommending a worse path than the one it later argues for.

''Reading checksums.txt as the manifest'' was written before noticing that rel.Assets is already fetched and already iterated in the same function. It costs a second request to obtain a list the code is holding, and it makes the checksum file load-bearing for asset discovery as well as verification — coupling two concerns that are currently independent. There are two shapes, not three, and the suffix match is the one that stops describing the filename twice.

RESOLVED (2026-09-09). Resolution now derives the asset name from the release''s own asset list: platformSuffix() builds only the platform tail (_Linux_x86_64.tar.gz, 386→i386 and windows→zip mirrored from the template) and the lookup picks the unique rel.Assets entry carrying it, refusing loudly on zero or multiple matches. The matched name is what flows to verifyChecksum and extractBinary, so all three uses agree by construction. The test the ticket asked for exists: selfupdate_test.go parses .goreleaser.yaml, renders the real name_template for every platform in the build matrix, and asserts the suffix matcher picks exactly one asset per platform — and that the match is version-agnostic, which is the axis that drifted. Verified end-to-end against the live release: a dev build ran self-update --force, matched pql_2.3.0_Linux_x86_64.tar.gz, passed checksum verification, extracted and atomically replaced itself — the first successful self-update by any pql binary.', NULL, '2026-09-09 09:43:27', '2026-09-09 09:43:27.100', '2026-09-09 09:43:27.100', NULL, 'b2256c6f767181ab51649d87520c00ac', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G66AQKC2Z6RAJZ0GQA725GWW', 'status', 'in_progress', 'done', NULL, '2026-09-09 09:43:27', '2026-09-09 09:43:27.123', '2026-09-09 09:43:27.123', NULL, '8a71e0996c1b2511acbd995f23239880', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89QXQ7QBFKMY53RAXEGYP1C', 'status', 'backlog', 'in_progress', NULL, '2026-09-09 09:44:40', '2026-09-09 09:44:40.926', '2026-09-09 09:44:40.926', NULL, '6d08644016017cae0bdf4d1b09ce51b0', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89R15HFKK0Y3EFGJXDAQEBG', 'status', 'backlog', 'in_progress', NULL, '2026-09-09 09:44:40', '2026-09-09 09:44:40.934', '2026-09-09 09:44:40.934', NULL, 'f1177c30107c4639c8c29e157ba6d3d7', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89QXQ7QBFKMY53RAXEGYP1C', 'status', 'in_progress', 'done', NULL, '2026-09-09 09:46:10', '2026-09-09 09:46:10.086', '2026-09-09 09:46:10.086', NULL, '6048088a04457c3a26afcf0b20473a73', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89R15HFKK0Y3EFGJXDAQEBG', 'status', 'in_progress', 'done', NULL, '2026-09-09 09:46:16', '2026-09-09 09:46:16.710', '2026-09-09 09:46:16.710', NULL, '7ecf0aa1d78c36100a7fe5ee209afc0a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89QDBQTD7E1H8XZ8B4BDEQ8', 'status', 'backlog', 'in_progress', NULL, '2026-09-09 09:47:06', '2026-09-09 09:47:06.529', '2026-09-09 09:47:06.529', NULL, 'c70da5cd38f1360f15751f4bafba6d62', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89QDBQTD7E1H8XZ8B4BDEQ8', 'description', '`internal/connect/signal/recency.go:23` scores from `time.Since(time.Unix(mtime, 0))`.
The reference point is the moment the query runs, so an unchanged vault produces
different scores on every invocation.

This is not merely cosmetic drift — it reorders results, via D-11''s
max-normalization.

Mechanism. With `raw_i = 1 - age_i/2160` and `normalized_i = raw_i / max(raw)`,
let all files age by the same δ. Both numerator and denominator shrink by δ, and
for any candidate below the maximum that ratio *falls*:

    (raw_i - δ) / (raw_max - δ)  <  raw_i / raw_max     for raw_i < raw_max

So the normalized recency spread widens monotonically over time while every
other signal stays fixed. In a weighted sum against centrality and the rest,
two candidates whose totals sat close together will eventually swap. Nothing
about the vault changed.

The 90-day clamp compounds it: files aging past `decayHours` pin to 0 and
collapse into ties, which then hit the unstable sort in the sibling ticket.

D-11 accepts that "the same file can score differently in different queries…
rankings are always relative". That reasoning covers batch-relative scoring and
does not extend to this: here the same file scores differently in *the same*
query at a different time, and the relative order changes with it.

Exposure is high because recency carries weight 0.25 on `search` and is often
the only non-zero signal. Observed on pql''s own vault:

    pql search architecture --full
    → top hit scores exactly 0.25, every signal zero except recency

That result is ordered purely by a float derived from the current time.

Possible directions, cheapest first:

1. Make the reference time injectable — a clock on `signal.Context`, defaulting
   to `time.Now()`. Tests pin it; production is unchanged. Fixes reproducibility
   without touching semantics.
2. Derive the reference from the corpus (e.g. max mtime in the candidate batch)
   rather than wall-clock, making recency purely a function of the vault.
3. Let a vault zero the recency weight in its profile. Purely additive, useful
   for authored corpora where mtime is an install artifact rather than a date.

(1) and (3) are compatible and neither changes behaviour for existing callers.
(2) is a semantic change and wants its own decision.

Note this touches only the ranking signal. It is independent of the other two
mtime consumers — `index/indexer.go` change detection and
`planning/changelog/importer.go` cross-repo sync — which read mtime from
`os.FileInfo` and the files table directly and never go through this path.', '`internal/connect/signal/recency.go:23` scores from `time.Since(time.Unix(mtime, 0))`.
The reference point is the moment the query runs, so an unchanged vault produces
different scores on every invocation.

This is not merely cosmetic drift — it reorders results, via D-11''s
max-normalization.

Mechanism. With `raw_i = 1 - age_i/2160` and `normalized_i = raw_i / max(raw)`,
let all files age by the same δ. Both numerator and denominator shrink by δ, and
for any candidate below the maximum that ratio *falls*:

    (raw_i - δ) / (raw_max - δ)  <  raw_i / raw_max     for raw_i < raw_max

So the normalized recency spread widens monotonically over time while every
other signal stays fixed. In a weighted sum against centrality and the rest,
two candidates whose totals sat close together will eventually swap. Nothing
about the vault changed.

The 90-day clamp compounds it: files aging past `decayHours` pin to 0 and
collapse into ties, which then hit the unstable sort in the sibling ticket.

D-11 accepts that "the same file can score differently in different queries…
rankings are always relative". That reasoning covers batch-relative scoring and
does not extend to this: here the same file scores differently in *the same*
query at a different time, and the relative order changes with it.

Exposure is high because recency carries weight 0.25 on `search` and is often
the only non-zero signal. Observed on pql''s own vault:

    pql search architecture --full
    → top hit scores exactly 0.25, every signal zero except recency

That result is ordered purely by a float derived from the current time.

Possible directions, cheapest first:

1. Make the reference time injectable — a clock on `signal.Context`, defaulting
   to `time.Now()`. Tests pin it; production is unchanged. Fixes reproducibility
   without touching semantics.
2. Derive the reference from the corpus (e.g. max mtime in the candidate batch)
   rather than wall-clock, making recency purely a function of the vault.
3. Let a vault zero the recency weight in its profile. Purely additive, useful
   for authored corpora where mtime is an install artifact rather than a date.

(1) and (3) are compatible and neither changes behaviour for existing callers.
(2) is a semantic change and wants its own decision.

Note this touches only the ranking signal. It is independent of the other two
mtime consumers — `index/indexer.go` change detection and
`planning/changelog/importer.go` cross-repo sync — which read mtime from
`os.FileInfo` and the files table directly and never go through this path.

RESOLVED (2026-09-09) via direction (1) from the body: signal.Context gains a Now field, captured once per enrichment pass in connect.Bundle, and recency scores age against it — so a batch is self-consistent (one instant for every candidate, where each Score call used to read the clock itself) and tests pin it exactly (recency_test.go asserts midlife.md at 45 days scores precisely 0.5). Zero-value Now falls back to the wall clock, so a Context built without it stays correct. Direction (2) — corpus-derived reference, which would remove wall-clock from ranking entirely and is a semantic change — is now Q-14 in governance/questions/architecture.md, carrying this ticket''s normalization-spread mechanism as context. Direction (3) — vault-zeroable recency weight — is noted inside Q-14 as additive under either answer.', NULL, '2026-09-09 09:49:11', '2026-09-09 09:49:11.341', '2026-09-09 09:49:11.341', NULL, '1b03b34024ec725a896df24229983348', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89QDBQTD7E1H8XZ8B4BDEQ8', 'status', 'in_progress', 'done', NULL, '2026-09-09 09:49:11', '2026-09-09 09:49:11.361', '2026-09-09 09:49:11.361', NULL, '1f84bd4710bc039a52b6a28910c1de3c', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89R35V3EJQDYRRW7K98VWZG', 'description', '`search`, `related` and `context` can return different results for an unchanged
vault, across three independent causes. None is a scoring-quality question — the
issue is that the same inputs do not reliably produce the same output.

Found 2026-09-06 while evaluating pql as the corpus substrate for another
project that needs byte-reproducible query results for headless testing. That
project has since gone its own way, so this is filed purely on pql''s own merits:
several consuming repos call the ranked verbs today, so unreproducible output is
a live problem rather than a hypothetical one.

Children:

- Recency signal derives from wall-clock (the one that actively changes results)
- Rank sorts with an unstable sort and no tie-break
- gatherCandidates has no ORDER BY in any of the three verbs

The last two are latent rather than active: SQLite''s UNION dedup currently emits
rows in sorted order via a temp B-tree, so candidate order happens to be stable
today. That is a query-planner implementation detail, not a contract — a SQLite
upgrade, a new index, or ANALYZE stats can change it with no code change here.

Suggested order of work: the two ordering fixes first (cheap, self-contained,
and together they make ranking total-ordered regardless of what SQLite does),
then the recency clock, which needs a design call rather than a patch.', '`search`, `related` and `context` can return different results for an unchanged
vault, across three independent causes. None is a scoring-quality question — the
issue is that the same inputs do not reliably produce the same output.

Found 2026-09-06 while evaluating pql as the corpus substrate for another
project that needs byte-reproducible query results for headless testing. That
project has since gone its own way, so this is filed purely on pql''s own merits:
several consuming repos call the ranked verbs today, so unreproducible output is
a live problem rather than a hypothetical one.

Children:

- Recency signal derives from wall-clock (the one that actively changes results)
- Rank sorts with an unstable sort and no tie-break
- gatherCandidates has no ORDER BY in any of the three verbs

The last two are latent rather than active: SQLite''s UNION dedup currently emits
rows in sorted order via a temp B-tree, so candidate order happens to be stable
today. That is a query-planner implementation detail, not a contract — a SQLite
upgrade, a new index, or ANALYZE stats can change it with no code change here.

Suggested order of work: the two ordering fixes first (cheap, self-contained,
and together they make ranking total-ordered regardless of what SQLite does),
then the recency clock, which needs a design call rather than a patch.

CLOSED (2026-09-09). All three children landed: T-127 (total order — score then path tie-break under a stable sort), T-128 (ORDER BY path in all three gatherCandidates), T-126 (one reference instant per enrichment pass, injectable for tests). Verified: back-to-back identical ''pql related'' runs produce byte-identical output. What remains deliberately out of scope: wall-clock drift of recency between runs separated in time — that is the signal''s semantics per D-11, and moving to a corpus-derived reference is now Q-14.', NULL, '2026-09-09 09:49:16', '2026-09-09 09:49:16.295', '2026-09-09 09:49:16.295', NULL, 'c2819a0221decc65e936e5c317d4de14', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89R35V3EJQDYRRW7K98VWZG', 'status', 'backlog', 'done', NULL, '2026-09-09 09:49:16', '2026-09-09 09:49:16.316', '2026-09-09 09:49:16.316', NULL, '4c8e7aed11d97ecf8ffa7b5878c39592', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G0MRDXFNP425XTEVVMQB21X8', 'status', 'backlog', 'in_progress', NULL, '2026-09-09 10:15:08', '2026-09-09 10:15:08.787', '2026-09-09 10:15:08.787', NULL, '280794f128852dcea0a67f01fcfdf62a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G0MRDXFNP425XTEVVMQB21X8', 'description', '`make fmt` writes. `make fmt-check` reads. Neither is reachable from any gate, so formatting drifts silently and lands all at once.

Verified across every layer that could plausibly hold it: `pre-push` runs secrets, lint, test and test-race; `lint` runs golangci-lint, goreleaser check and govulncheck; the golangci config sets `default: none` with an explicit enable list that contains no formatter and has no `formatters:` section, which is where the v2 schema puts one; and no CI step runs gofmt either. `fmt-check` is an orphan target.

WHAT THIS COSTS, measured rather than supposed. A single manual `make fmt` rewrote 19 files in one go — months of accumulated drift arriving as one diff nobody can review. `git diff --stat` reported 124 insertions and 77 deletions, which reads as a formatting sweep and invites being committed as one. `git diff -w` reported 6 insertions and 5 deletions. Eleven real lines were hiding inside the whitespace, and finding them took knowing to ask for `-w`.

THE SHARP EDGE. Three of those eleven were semantic, and they are the reason this is worth more than tidiness.

gofmt reformats doc comments and applies the old godoc typographic convention: two apostrophes become a closing curly quote and two backquotes become an opening one. Confirmed on Go 1.25.12, and confirmed to apply to doc comments only — the identical text in a comment inside a function body is untouched.

The affected comments document escaping syntax. That is exactly why they contain those digraphs literally, and exactly why the conversion breaks them. One now tells the reader that a curly quote is un-escaped to an apostrophe, which is not what the code does. Another names a fence character that is not the one the extractor matches.

It compiles. Every test passes. The lint gate is green. Nothing in the toolchain inspects prose inside a comment, so the rewritten comment and a correct one are indistinguishable to every check that exists. T-72 already named this failure mode for a different command: a confident wrong answer is worse than an error, and it is reached by doing the obvious thing.

TWO DECISIONS, and the second is the one that makes this more than a one-line Makefile change.

**Should a format check join the gate?** If yes, it needs a normalizing commit first, or its first run fails on all of the accumulated drift at once and the gate gets bypassed on the day it is added.

**What happens to doc comments that must contain those digraphs?** This matters because a lexer and a markdown extractor have a legitimate need to write them. gofmt does not convert inside an indented block within a doc comment, so that form survives — but it changes how the comment reads, and whether it is worth it differs per comment.

Adding the check without settling the second leaves the repo unable to state its own escaping rules in a doc comment, and a normalizing commit made in ignorance of it would bake the false statements in permanently rather than fixing them. Order matters here: decide the comment form, correct the three, then gate.', '`make fmt` writes. `make fmt-check` reads. Neither is reachable from any gate, so formatting drifts silently and lands all at once.

Verified across every layer that could plausibly hold it: `pre-push` runs secrets, lint, test and test-race; `lint` runs golangci-lint, goreleaser check and govulncheck; the golangci config sets `default: none` with an explicit enable list that contains no formatter and has no `formatters:` section, which is where the v2 schema puts one; and no CI step runs gofmt either. `fmt-check` is an orphan target.

WHAT THIS COSTS, measured rather than supposed. A single manual `make fmt` rewrote 19 files in one go — months of accumulated drift arriving as one diff nobody can review. `git diff --stat` reported 124 insertions and 77 deletions, which reads as a formatting sweep and invites being committed as one. `git diff -w` reported 6 insertions and 5 deletions. Eleven real lines were hiding inside the whitespace, and finding them took knowing to ask for `-w`.

THE SHARP EDGE. Three of those eleven were semantic, and they are the reason this is worth more than tidiness.

gofmt reformats doc comments and applies the old godoc typographic convention: two apostrophes become a closing curly quote and two backquotes become an opening one. Confirmed on Go 1.25.12, and confirmed to apply to doc comments only — the identical text in a comment inside a function body is untouched.

The affected comments document escaping syntax. That is exactly why they contain those digraphs literally, and exactly why the conversion breaks them. One now tells the reader that a curly quote is un-escaped to an apostrophe, which is not what the code does. Another names a fence character that is not the one the extractor matches.

It compiles. Every test passes. The lint gate is green. Nothing in the toolchain inspects prose inside a comment, so the rewritten comment and a correct one are indistinguishable to every check that exists. T-72 already named this failure mode for a different command: a confident wrong answer is worse than an error, and it is reached by doing the obvious thing.

TWO DECISIONS, and the second is the one that makes this more than a one-line Makefile change.

**Should a format check join the gate?** If yes, it needs a normalizing commit first, or its first run fails on all of the accumulated drift at once and the gate gets bypassed on the day it is added.

**What happens to doc comments that must contain those digraphs?** This matters because a lexer and a markdown extractor have a legitimate need to write them. gofmt does not convert inside an indented block within a doc comment, so that form survives — but it changes how the comment reads, and whether it is worth it differs per comment.

Adding the check without settling the second leaves the repo unable to state its own escaping rules in a doc comment, and a normalizing commit made in ignorance of it would bake the false statements in permanently rather than fixing them. Order matters here: decide the comment form, correct the three, then gate.

RESOLVED (2026-09-09). The two prerequisites the body ordered turned out to be already done: commit b9ff24b applied the normalizing sweep and corrected the three doc comments gofmt had broken (lex.go''s escaping doc now carries the digraph in an indented block, the gofmt-stable form; the fence comment reads correctly). What remained was the wiring, done where the body looked for it and found nothing: .golangci.yaml now has a formatters: section enabling gofmt, which golangci-lint v2 runs in check mode during run — so make lint, CI and pre-push all enforce it with no new stage, per D-33 (the script is the definition; the config travels with it). gofmt only, deliberately: the repo does not enforce import grouping, so goimports stays a write-time convenience in make fmt. Verified the gate bites: two lines of real struct-tag drift in decisions.go failed golangci-lint run with ''File is not properly formatted (gofmt)'' before make fmt cleared it. make fmt-check remains as the read-only listing tool.', NULL, '2026-09-09 10:16:54', '2026-09-09 10:16:54.395', '2026-09-09 10:16:54.395', NULL, 'dc5cb13eebd3e7cc48d1e4397c0ddeff', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G0MRDXFNP425XTEVVMQB21X8', 'status', 'in_progress', 'done', NULL, '2026-09-09 10:16:54', '2026-09-09 10:16:54.438', '2026-09-09 10:16:54.438', NULL, '8384e421dad7345ffb55aa58713c3be0', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY7JHX6RH2BQK3R9VVP1ZX68', 'status', 'backlog', 'in_progress', NULL, '2026-09-09 10:19:19', '2026-09-09 10:19:19.559', '2026-09-09 10:19:19.559', NULL, '587aa29af44645bb4924d8155b067485', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY7JHX6RH2BQK3R9VVP1ZX68', 'description', 'Observed 2026-08-09 in pql''s own repo. Running ''pql --vault <repo> ticket new'' where .pql/pql.db does not exist but .pql/changelog/ holds a full history does NOT import the changelog first. It creates an empty db and mints the new ticket as T-1, colliding with the existing T-1. The vault already ran to T-94. Nothing is corrupted - identity is the ULID and the changelog is append-only - but the label collision is silent at creation time. It surfaced only on the next plan rebuild, which reported it correctly and clearly. Two contributing gaps. First, the docs say plan import runs automatically on a fresh clone; that holds only where pql init has planted the hooks. pql''s own repo has never been inited (doctor reports config.loaded false, no .pql/config.yaml), so nothing auto-imports there. Second, and the real fix: any planning mutation should detect changelog-present-but-db-empty and either import first or refuse with a diagnostic. Silently starting the label sequence at 1 is the one behaviour that produces a collision. Recovery, for the record: plan rebuild --verify restores everything and reports the collision, then relabel by record_id. Note relabel T-1 fails with a circular message telling you to run relabel; the ambiguous label cannot address either record, so the ULID is required. Worth a clearer diagnostic there too.

CORRECTION to the paragraph above about this repo never having been inited.

That claim was overstated and was used to explain the wrong thing. What is true: this repo has no .pql/config.yaml and no planted hooks, and that is still why the clone-time import never fires, which is the substance of this ticket. What is not true is the inference drawn from it at the time - that the vault held no decisions. It holds 44, in governance/decisions/architecture.md, D-1 through D-31 plus questions and rejected records. They were simply never synced into the database; one decisions sync populated all of them and reported 69 cross-references and nothing broken.

The mistake underneath it is worth recording because it is the same shape as this ticket''s own subject. pql doctor reports db.exists for index.db, the query cache. Planning state is a different database, pql.db, and it did exist. Reading one field as evidence about the other produced a confident and wrong conclusion about the repo''s state - exactly the way an empty label sequence produced a confident and wrong T-1 above.

Nothing about the proposed fix changes. The guard still belongs in the mutation path rather than in the hooks, for the reason already given: the hooks are what is missing whenever this bites.', 'Observed 2026-08-09 in pql''s own repo. Running ''pql --vault <repo> ticket new'' where .pql/pql.db does not exist but .pql/changelog/ holds a full history does NOT import the changelog first. It creates an empty db and mints the new ticket as T-1, colliding with the existing T-1. The vault already ran to T-94. Nothing is corrupted - identity is the ULID and the changelog is append-only - but the label collision is silent at creation time. It surfaced only on the next plan rebuild, which reported it correctly and clearly. Two contributing gaps. First, the docs say plan import runs automatically on a fresh clone; that holds only where pql init has planted the hooks. pql''s own repo has never been inited (doctor reports config.loaded false, no .pql/config.yaml), so nothing auto-imports there. Second, and the real fix: any planning mutation should detect changelog-present-but-db-empty and either import first or refuse with a diagnostic. Silently starting the label sequence at 1 is the one behaviour that produces a collision. Recovery, for the record: plan rebuild --verify restores everything and reports the collision, then relabel by record_id. Note relabel T-1 fails with a circular message telling you to run relabel; the ambiguous label cannot address either record, so the ULID is required. Worth a clearer diagnostic there too.

CORRECTION to the paragraph above about this repo never having been inited.

That claim was overstated and was used to explain the wrong thing. What is true: this repo has no .pql/config.yaml and no planted hooks, and that is still why the clone-time import never fires, which is the substance of this ticket. What is not true is the inference drawn from it at the time - that the vault held no decisions. It holds 44, in governance/decisions/architecture.md, D-1 through D-31 plus questions and rejected records. They were simply never synced into the database; one decisions sync populated all of them and reported 69 cross-references and nothing broken.

The mistake underneath it is worth recording because it is the same shape as this ticket''s own subject. pql doctor reports db.exists for index.db, the query cache. Planning state is a different database, pql.db, and it did exist. Reading one field as evidence about the other produced a confident and wrong conclusion about the repo''s state - exactly the way an empty label sequence produced a confident and wrong T-1 above.

Nothing about the proposed fix changes. The guard still belongs in the mutation path rather than in the hooks, for the reason already given: the hooks are what is missing whenever this bites.

RESOLVED (2026-09-09). The guard the body asked for exists, in the mutation path as prescribed: changelog.GuardReplicaCurrent refuses when tickets is empty while .pql/changelog/tickets/ holds non-empty history — refuse rather than auto-import, per fail-unless-proven, and because auto-importing would paper over the missing hooks that cause the state. It lives in the changelog package so a future planning MCP inherits it; the CLI wraps it as openPlanningDBForMutation, used by all 12 ticket mutation verbs (new, append, status, relabel, assign, setparent, decision, block, unblock, team, label, refine write). Reads and the remedies (plan import/rebuild) are unguarded. Refusal is exit 65 with a hint naming pql plan import. Verified end-to-end on a scratch vault: create T-1, rm pql.db, ticket new refuses with the diagnostic; plan import then ticket new mints T-2 — the exact collision this ticket observed is unreachable. Unit tests cover fresh-vault pass, populated pass, the refusal, import-clears-it, and empty-changelog-files pass. The secondary observation (relabel T-1''s circular message when the ambiguous label cannot address either record) was not addressed here — it only occurs post-collision, which the guard now prevents from arising.', NULL, '2026-09-09 10:23:59', '2026-09-09 10:23:59.645', '2026-09-09 10:23:59.645', NULL, '4d0ae6bbef66bf57828a7a78fdd6c02c', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY7JHX6RH2BQK3R9VVP1ZX68', 'status', 'in_progress', 'done', NULL, '2026-09-09 10:23:59', '2026-09-09 10:23:59.663', '2026-09-09 10:23:59.663', NULL, 'be81c665f42005c0dbcd55aa20bd8bf4', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G64CE0WTT40RJ9TE0VT95FYW', 'status', 'backlog', 'in_progress', NULL, '2026-09-09 10:25:00', '2026-09-09 10:25:00.311', '2026-09-09 10:25:00.311', NULL, 'cbfa52836ff91abc5afc36f3e510d5b2', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G64CE0WTT40RJ9TE0VT95FYW', 'description', 'Label collisions are detected at replay and repaired by `ticket relabel --fix-prose`. Nothing checks before the label is handed out, so the window between allocation and replay is where the damage happens.

THE SEQUENCE. A clone whose .pql/changelog is behind its remote still has a local pql.db that answers ''the next free label is T-N''. `ticket new` takes it and succeeds. The label is then used immediately — in the ticket''s own prose, in a commit message, and in a consuming project that cites the upstream ticket by label. Only later, when the changelog is pulled and replayed, does the collision surface. By then the wrong label has been published in places replay cannot see.

WHY --fix-prose DOES NOT CLOSE IT. That flag rewrites whole-word mentions in the DQR tree, which is the right scope for the vault it runs in. A label also travels outward: into commit bodies, and into other repositories that cite this one''s tickets. Those are not rewritable — a pushed commit message cannot be corrected without rewriting history, and the citing repo may not even be checked out. Observed: a consuming project committed ''filed upstream as T-<n>'' where <n> had been allocated from a stale replica; the ticket now carries a different label, upstream''s real T-<n> is an unrelated ticket, and the citing commit is already published.

THE DESIGN IS NOT THE PROBLEM. record_id is stable (D-26), the friendly label is explicitly not authoritative, collisions are anticipated, and the repair verb is thorough. The gap is that the only signal arrives after the label has been used, and the caller had no way to know the replica was stale — `pql doctor` reports config, db, skills, vault and version, and nothing about whether the replica is behind what is tracked.

DESIRED, not a design: someone about to file learns that this vault''s replica is behind before a label is allocated, not after it has been cited. A warning or refusal from `ticket new` when the changelog on disk holds labels the db does not, a staleness line in `doctor`, or an allocation that consults the changelog rather than only the replica would each do it. Refusing outright may be too strong for a vault with no remote at all, so the check probably has to be conditional on there being something to be behind.

This is the single-writer assumption showing at the seam rather than a bug in any one verb: with one clone, allocating from local state is always right and replay is a formality. With two, it is optimistic locking without the check, and the failure is silent.', 'Label collisions are detected at replay and repaired by `ticket relabel --fix-prose`. Nothing checks before the label is handed out, so the window between allocation and replay is where the damage happens.

THE SEQUENCE. A clone whose .pql/changelog is behind its remote still has a local pql.db that answers ''the next free label is T-N''. `ticket new` takes it and succeeds. The label is then used immediately — in the ticket''s own prose, in a commit message, and in a consuming project that cites the upstream ticket by label. Only later, when the changelog is pulled and replayed, does the collision surface. By then the wrong label has been published in places replay cannot see.

WHY --fix-prose DOES NOT CLOSE IT. That flag rewrites whole-word mentions in the DQR tree, which is the right scope for the vault it runs in. A label also travels outward: into commit bodies, and into other repositories that cite this one''s tickets. Those are not rewritable — a pushed commit message cannot be corrected without rewriting history, and the citing repo may not even be checked out. Observed: a consuming project committed ''filed upstream as T-<n>'' where <n> had been allocated from a stale replica; the ticket now carries a different label, upstream''s real T-<n> is an unrelated ticket, and the citing commit is already published.

THE DESIGN IS NOT THE PROBLEM. record_id is stable (D-26), the friendly label is explicitly not authoritative, collisions are anticipated, and the repair verb is thorough. The gap is that the only signal arrives after the label has been used, and the caller had no way to know the replica was stale — `pql doctor` reports config, db, skills, vault and version, and nothing about whether the replica is behind what is tracked.

DESIRED, not a design: someone about to file learns that this vault''s replica is behind before a label is allocated, not after it has been cited. A warning or refusal from `ticket new` when the changelog on disk holds labels the db does not, a staleness line in `doctor`, or an allocation that consults the changelog rather than only the replica would each do it. Refusing outright may be too strong for a vault with no remote at all, so the check probably has to be conditional on there being something to be behind.

This is the single-writer assumption showing at the seam rather than a bug in any one verb: with one clone, allocating from local state is always right and replay is a formality. With two, it is optimistic locking without the check, and the failure is silent.

RESOLVED (2026-09-09). Allocation now cannot outrun the changelog: GuardReplicaCurrent (the T-96 guard, extended) compares the replica''s max minted label against the changelog''s — read the way replay reads it, staged through SQLite per D-28, and counting every label ever minted including superseded mappings, because never-re-mint-a-used-label is exactly allocation''s rule. A replica behind the changelog refuses any ticket mutation with exit 65, the diagnostic naming both positions (replica reaches T-x, changelog reaches T-y) and the hint naming plan import. A replica AHEAD passes — that is write-through with an export pending, not staleness. The check is conditional on there being something to be behind, as the body asked: no ticket history in the changelog means no comparison and no refusal, so a vault with no remote is untouched. Verified end-to-end: snapshot replica at T-2, mint T-3 (changelog advances), restore stale replica, ticket new refuses naming T-2/T-3; plan import; next create mints T-4. The doctor staleness line and allocation-consults-changelog alternatives from the body were not needed — the mutation-path refusal covers the window described.', NULL, '2026-09-09 10:27:45', '2026-09-09 10:27:45.985', '2026-09-09 10:27:45.985', NULL, 'ebc90bb1b7eea54c3d3442fb2b8bc76d', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G64CE0WTT40RJ9TE0VT95FYW', 'status', 'in_progress', 'done', NULL, '2026-09-09 10:27:46', '2026-09-09 10:27:46.006', '2026-09-09 10:27:46.006', NULL, '3d79253713431c80f4492f063e150e53', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FZ4FHC4YQRSRC071QNEWM64G', 'decision_ref', NULL, 'D-35', NULL, '2026-09-09 10:33:59', '2026-09-09 10:33:59.273', '2026-09-09 10:33:59.273', NULL, '698e748e6529d6cd10073bcf3ab01fb7', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89S0F3600NPD1978YNJP9CR', 'decision_ref', NULL, 'D-35', NULL, '2026-09-09 10:33:59', '2026-09-09 10:33:59.275', '2026-09-09 10:33:59.275', NULL, '7fc9dd4ed07dc0fdebd474958c5ec224', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89S0F3600NPD1978YNJP9CR', 'description', 'Found 2026-09-09 while removing sibling-repo names and a hostname from a ticket
description before pushing. `make secrets` caught it, which is the system
working — CLAUDE.md is explicit that ticket prose is published prose, and the
gate ran before anything left the machine. What is missing is the route from
"caught it" back to a clean changelog.

WHY THE OBVIOUS FIXES DO NOT WORK

Fixing forward makes it worse. `ticket refine write` appends a ticket_history
row whose `old_value` is the previous description, so correcting a leaked
description writes the leaked text into the changelog a second time. The
outgoing diff then contains two copies rather than none.

Wiping and re-exporting does not work either. `plan export` emits "every
replicated planning row that has been modified since the last export", so it is
watermark-driven. Restoring the changelog files to an earlier state and
re-running it re-emits only rows touched since the watermark — in this case two
rows out of the fifteen or so that were needed. The rest stayed in pql.db,
absent from the changelog, with no supported way to get them back out.

WHAT IT ACTUALLY TOOK

    git reset --soft origin/main
    git checkout origin/main -- .pql/changelog
    rm .pql/pql.db
    pql plan rebuild --verify
    # then re-create four tickets by hand, re-entering every description,
    # and re-attach the parent links

That works and `--verify` reported 0 rows lost, but it is a hand-rolled
procedure recovered from reading the exporter''s help text under time pressure,
and the re-entry step is transcription with no check on it. Anyone hitting this
without the descriptions still in front of them loses the prose.

THE ASYMMETRY

`plan rebuild` reconstructs pql.db from the changelog and can be forced at any
time. There is no inverse. The pair is documented as replication, but only one
direction can be regenerated on demand — the other is append-only and
watermarked, so pql.db''s current state cannot be re-expressed as changelog
content once the watermark has passed it.

THE DESIGN QUESTION UNDERNEATH, which is why this is not just a missing flag

Is the changelog an append-only *log*, or a materialised *replica*?

- As a log, redaction is illegitimate by construction and the honest answer is
  that scrubbing prose requires rewriting git history, with the D-16 hashes and
  LWW guards recomputed. A `--force-full` export would be a footgun that
  silently rewrites replicated history other clones have already replayed.
- As a replica, regenerating it from pql.db is the natural operation and its
  absence is the defect.

D-15 and D-16 lean toward log (monthly append files, inline LWW guards, content
hashes, ON CONFLICT DO NOTHING on replay), but `plan rebuild` treats pql.db as
fully derivable from it, which is replica-shaped. The two readings have not had
to disagree until now.

Worth noting the blast radius differs by case. Redacting a ticket that has never
been pushed — this case — touches nothing another clone has seen, and a full
regeneration is safe. Redacting one that has been replayed elsewhere is a
distributed-state problem and probably out of scope for any flag.

DESIRED, not a design

Someone who has just been told by `make secrets` that a ticket description
leaks should have a supported route to fix it that does not involve re-entering
prose. Shapes worth weighing:

- A full/forced export that rewrites the month''s files from pql.db, refusing (or
  loudly warning) when the affected rows appear in commits already pushed.
- A redact verb scoped to descriptions, which rewrites the row and its history
  entries in place rather than appending.
- Nothing in the tool, and instead a documented procedure in CLAUDE.md next to
  the "ticket prose is published prose" table — cheapest, and honest if the log
  reading wins.

The third is a legitimate outcome. What is not legitimate is the current state,
where the gate reliably catches the problem and the recovery is undocumented.

RELATED

D-15, D-16 - changelog replication and the guards that make replay safe.
T-123     - the other place the single-writer assumption shows at the seam.', 'Found 2026-09-09 while removing sibling-repo names and a hostname from a ticket
description before pushing. `make secrets` caught it, which is the system
working — CLAUDE.md is explicit that ticket prose is published prose, and the
gate ran before anything left the machine. What is missing is the route from
"caught it" back to a clean changelog.

WHY THE OBVIOUS FIXES DO NOT WORK

Fixing forward makes it worse. `ticket refine write` appends a ticket_history
row whose `old_value` is the previous description, so correcting a leaked
description writes the leaked text into the changelog a second time. The
outgoing diff then contains two copies rather than none.

Wiping and re-exporting does not work either. `plan export` emits "every
replicated planning row that has been modified since the last export", so it is
watermark-driven. Restoring the changelog files to an earlier state and
re-running it re-emits only rows touched since the watermark — in this case two
rows out of the fifteen or so that were needed. The rest stayed in pql.db,
absent from the changelog, with no supported way to get them back out.

WHAT IT ACTUALLY TOOK

    git reset --soft origin/main
    git checkout origin/main -- .pql/changelog
    rm .pql/pql.db
    pql plan rebuild --verify
    # then re-create four tickets by hand, re-entering every description,
    # and re-attach the parent links

That works and `--verify` reported 0 rows lost, but it is a hand-rolled
procedure recovered from reading the exporter''s help text under time pressure,
and the re-entry step is transcription with no check on it. Anyone hitting this
without the descriptions still in front of them loses the prose.

THE ASYMMETRY

`plan rebuild` reconstructs pql.db from the changelog and can be forced at any
time. There is no inverse. The pair is documented as replication, but only one
direction can be regenerated on demand — the other is append-only and
watermarked, so pql.db''s current state cannot be re-expressed as changelog
content once the watermark has passed it.

THE DESIGN QUESTION UNDERNEATH, which is why this is not just a missing flag

Is the changelog an append-only *log*, or a materialised *replica*?

- As a log, redaction is illegitimate by construction and the honest answer is
  that scrubbing prose requires rewriting git history, with the D-16 hashes and
  LWW guards recomputed. A `--force-full` export would be a footgun that
  silently rewrites replicated history other clones have already replayed.
- As a replica, regenerating it from pql.db is the natural operation and its
  absence is the defect.

D-15 and D-16 lean toward log (monthly append files, inline LWW guards, content
hashes, ON CONFLICT DO NOTHING on replay), but `plan rebuild` treats pql.db as
fully derivable from it, which is replica-shaped. The two readings have not had
to disagree until now.

Worth noting the blast radius differs by case. Redacting a ticket that has never
been pushed — this case — touches nothing another clone has seen, and a full
regeneration is safe. Redacting one that has been replayed elsewhere is a
distributed-state problem and probably out of scope for any flag.

DESIRED, not a design

Someone who has just been told by `make secrets` that a ticket description
leaks should have a supported route to fix it that does not involve re-entering
prose. Shapes worth weighing:

- A full/forced export that rewrites the month''s files from pql.db, refusing (or
  loudly warning) when the affected rows appear in commits already pushed.
- A redact verb scoped to descriptions, which rewrites the row and its history
  entries in place rather than appending.
- Nothing in the tool, and instead a documented procedure in CLAUDE.md next to
  the "ticket prose is published prose" table — cheapest, and honest if the log
  reading wins.

The third is a legitimate outcome. What is not legitimate is the current state,
where the gate reliably catches the problem and the recovery is undocumented.

RELATED

D-15, D-16 - changelog replication and the guards that make replay safe.
T-123     - the other place the single-writer assumption shows at the seam.

DECIDED (2026-09-09): D-35 settles the log-or-replica question this ticket raised — log, with the push boundary as the rewrite rule. Of the three shapes weighed here, the outcome is closest to the first, narrowed: not a force-full export, but a redact-class verb that rewrites the affected rows in pql.db, ticket_history (old_value copies included) and the changelog lines together, refusing when any affected line is already in a pushed commit. Pushed-history redaction stays a documented git-rewrite procedure, per the record. This ticket becomes the implementation carrier for that verb.', NULL, '2026-09-09 10:34:06', '2026-09-09 10:34:06.701', '2026-09-09 10:34:06.701', NULL, 'fb150ff1f8ea0861041f8f0a3a741067', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FZ4FHC4YQRSRC071QNEWM64G', 'description', 'A value removed from a committed changelog file is re-published by the next ticket mutation, because pql.db still holds the original row and the write-through export (D-23) re-derives the file from it.

OBSERVED 2026-08-11. A ticket_history row had been scrubbed by hand — one line edited — before the commit that introduced it. Creating an unrelated ticket months later triggered the export, which re-appended that same row: byte-identical to the committed one apart from the scrubbed line, which came back in full. Same content hash on both. It was caught in review and removed from the working tree before staging, so nothing was published, but only because someone happened to diff the export before committing it.

TWO MECHANISMS COMPOUND, and either alone would be survivable.

1. The export boundary appears to be inclusive. The re-appended row''s updated_at equalled last_export_marker exactly, so a row already exported was exported again. A row that has not changed since the last export has nothing new to say, and re-emitting it is what turned a stale row into a live one.

2. The changelog is derived, and a scrub edits only the derivation. pql.db is the source the export reads. Editing the artefact leaves the source untouched, so the edit survives exactly until the next write — which is the least intuitive moment for it to be undone, because nothing about creating an unrelated ticket suggests it will rewrite history.

WHY THIS MATTERS MORE THAN IT LOOKS. This repo''s own CLAUDE.md states that everything committed here is published and indexed, that ticket prose is published prose, and that the changelog is committed by design so tickets travel with a clone. It also documents that history already carries findings resolved by untracking a file rather than rewriting the past. So scrubbing-before-commit is an established practice here, and this makes that practice unreliable in a way its user cannot see.

The pre-push gate scans the outgoing range, so it can catch a re-published value — but only for patterns its ruleset knows. A consuming repo''s name, a project path, or anything else specific to an operator''s environment is not a secret by any default ruleset, and those are precisely what the scrub-before-commit habit exists to remove.

WHAT ACTUALLY WORKED, and is worth documenting either way: `rm .pql/pql.db && pql plan rebuild` rebuilt the database from the scrubbed changelog and dropped the row. The repo''s documented recovery path is also its scrub-completion path, which is not obvious from either description. Verified: the row was present before the rebuild and absent after, with no ticket lost.

DIRECTIONS, not a prescription:
  - Make the export boundary exclusive, so an unchanged row is not re-emitted. Necessary, not sufficient: it fixes recurrence, not the divergence.
  - Treat a changelog edit as a database edit, or refuse it — the artefact and its source must not be independently editable if one regenerates the other.
  - At minimum, document that a scrub is incomplete until the database is rebuilt from the scrubbed file, and say so where the scrub-before-commit practice is described rather than only under recovery.

Distinct from T-96, which is about a mutation against a populated changelog with an empty database. This is the reverse: a populated database re-deriving over an edited changelog.

CORRECTION (T-106/T-107 round, 2026-08-11). The first prescribed direction here —
"make the export boundary exclusive" — is wrong and must not be implemented as
written.

The boundary is inclusive (`updated_at >= marker`) by design, not by oversight.
`exporter.go:26-33` documents why: write-through (D-23) calls Export after every
mutation, advancing the marker to "now" at second granularity. A mutation landing
in the SAME second as the marker would be silently skipped under a strict `>` —
which is exactly the data-loss class write-through exists to close. Making the
boundary exclusive trades this ticket''s re-emission-of-a-stale-row problem for
silent non-persistence of a live one. The second problem is worse: this ticket''s
symptom is noise a diff can catch (as this one was); the exclusive-boundary
failure mode is data that was never written and gives no signal that it is
missing.

The actual mechanism, confirmed by tracing `fileSink.appendLine` /
`fileSink.loadSeen` (`exporter.go:108-157`): a re-scanned row that is
byte-identical to what is already in the file is deduped and never written twice
— dedup is by content, not by marker exclusion. This ticket''s failure case is
narrower than "the boundary re-emits": it is specifically the scrub case, where
the row on disk and the row in pql.db have *diverged* (the file was hand-edited,
the database was not), so the re-scanned line is no longer byte-identical to what
loadSeen has on record and gets written as new content. An exclusive boundary
would not touch this at all, because the row that re-appears was never re-scanned
by the exclusive/inclusive distinction in the failing case that matters —
it would only stop the *harmless* re-scan-of-identical-content case T-107
separately (and also wrongly) worried about.

Second and third directions stand. "Treat a changelog edit as a database edit,
or refuse it" is the real fix and is still undecided — out of scope for the
T-106/T-107 round, which fixed the two rebuild-side defects (T-106: decisions
lost on recovery; T-107: investigated and found already fixed by T-26, see that
ticket''s closing note) without touching the export-boundary question this
ticket raises. "Document that a scrub is incomplete until rebuilt" is now true
by construction where it matters: `pql plan rebuild` (this ticket''s own
documented recovery) now also restores decisions (T-106''s fix), so the rebuild
path is a more complete scrub-completion step than it was when this ticket was
filed, but the underlying editable-artefact-vs-source divergence this ticket
names is unresolved and open.', 'A value removed from a committed changelog file is re-published by the next ticket mutation, because pql.db still holds the original row and the write-through export (D-23) re-derives the file from it.

OBSERVED 2026-08-11. A ticket_history row had been scrubbed by hand — one line edited — before the commit that introduced it. Creating an unrelated ticket months later triggered the export, which re-appended that same row: byte-identical to the committed one apart from the scrubbed line, which came back in full. Same content hash on both. It was caught in review and removed from the working tree before staging, so nothing was published, but only because someone happened to diff the export before committing it.

TWO MECHANISMS COMPOUND, and either alone would be survivable.

1. The export boundary appears to be inclusive. The re-appended row''s updated_at equalled last_export_marker exactly, so a row already exported was exported again. A row that has not changed since the last export has nothing new to say, and re-emitting it is what turned a stale row into a live one.

2. The changelog is derived, and a scrub edits only the derivation. pql.db is the source the export reads. Editing the artefact leaves the source untouched, so the edit survives exactly until the next write — which is the least intuitive moment for it to be undone, because nothing about creating an unrelated ticket suggests it will rewrite history.

WHY THIS MATTERS MORE THAN IT LOOKS. This repo''s own CLAUDE.md states that everything committed here is published and indexed, that ticket prose is published prose, and that the changelog is committed by design so tickets travel with a clone. It also documents that history already carries findings resolved by untracking a file rather than rewriting the past. So scrubbing-before-commit is an established practice here, and this makes that practice unreliable in a way its user cannot see.

The pre-push gate scans the outgoing range, so it can catch a re-published value — but only for patterns its ruleset knows. A consuming repo''s name, a project path, or anything else specific to an operator''s environment is not a secret by any default ruleset, and those are precisely what the scrub-before-commit habit exists to remove.

WHAT ACTUALLY WORKED, and is worth documenting either way: `rm .pql/pql.db && pql plan rebuild` rebuilt the database from the scrubbed changelog and dropped the row. The repo''s documented recovery path is also its scrub-completion path, which is not obvious from either description. Verified: the row was present before the rebuild and absent after, with no ticket lost.

DIRECTIONS, not a prescription:
  - Make the export boundary exclusive, so an unchanged row is not re-emitted. Necessary, not sufficient: it fixes recurrence, not the divergence.
  - Treat a changelog edit as a database edit, or refuse it — the artefact and its source must not be independently editable if one regenerates the other.
  - At minimum, document that a scrub is incomplete until the database is rebuilt from the scrubbed file, and say so where the scrub-before-commit practice is described rather than only under recovery.

Distinct from T-96, which is about a mutation against a populated changelog with an empty database. This is the reverse: a populated database re-deriving over an edited changelog.

CORRECTION (T-106/T-107 round, 2026-08-11). The first prescribed direction here —
"make the export boundary exclusive" — is wrong and must not be implemented as
written.

The boundary is inclusive (`updated_at >= marker`) by design, not by oversight.
`exporter.go:26-33` documents why: write-through (D-23) calls Export after every
mutation, advancing the marker to "now" at second granularity. A mutation landing
in the SAME second as the marker would be silently skipped under a strict `>` —
which is exactly the data-loss class write-through exists to close. Making the
boundary exclusive trades this ticket''s re-emission-of-a-stale-row problem for
silent non-persistence of a live one. The second problem is worse: this ticket''s
symptom is noise a diff can catch (as this one was); the exclusive-boundary
failure mode is data that was never written and gives no signal that it is
missing.

The actual mechanism, confirmed by tracing `fileSink.appendLine` /
`fileSink.loadSeen` (`exporter.go:108-157`): a re-scanned row that is
byte-identical to what is already in the file is deduped and never written twice
— dedup is by content, not by marker exclusion. This ticket''s failure case is
narrower than "the boundary re-emits": it is specifically the scrub case, where
the row on disk and the row in pql.db have *diverged* (the file was hand-edited,
the database was not), so the re-scanned line is no longer byte-identical to what
loadSeen has on record and gets written as new content. An exclusive boundary
would not touch this at all, because the row that re-appears was never re-scanned
by the exclusive/inclusive distinction in the failing case that matters —
it would only stop the *harmless* re-scan-of-identical-content case T-107
separately (and also wrongly) worried about.

Second and third directions stand. "Treat a changelog edit as a database edit,
or refuse it" is the real fix and is still undecided — out of scope for the
T-106/T-107 round, which fixed the two rebuild-side defects (T-106: decisions
lost on recovery; T-107: investigated and found already fixed by T-26, see that
ticket''s closing note) without touching the export-boundary question this
ticket raises. "Document that a scrub is incomplete until rebuilt" is now true
by construction where it matters: `pql plan rebuild` (this ticket''s own
documented recovery) now also restores decisions (T-106''s fix), so the rebuild
path is a more complete scrub-completion step than it was when this ticket was
filed, but the underlying editable-artefact-vs-source divergence this ticket
names is unresolved and open.

DECIDED (2026-09-09): D-35 resolves the editable-artefact-vs-source divergence this ticket named as the real fix. Direction two (''treat a changelog edit as a database edit, or refuse it'') is adopted in both halves: the redact verb (implementation under T-130) moves both sides together so divergence is unrepresentable, and a detected hand-edit divergence is a defect state whose report names the verb. The scrub-completion documentation direction is superseded by the same record: unpushed scrubs get the verb, pushed scrubs get the documented git-rewrite procedure.', NULL, '2026-09-09 10:34:06', '2026-09-09 10:34:06.722', '2026-09-09 10:34:06.722', NULL, 'f86f29b5c8f07ae1daebbf9a156affe3', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89S0F3600NPD1978YNJP9CR', 'status', 'backlog', 'in_progress', NULL, '2026-09-09 10:38:13', '2026-09-09 10:38:13.720', '2026-09-09 10:38:13.720', NULL, 'ed559545b4bafc3b9470ad3ab02d174f', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89S0F3600NPD1978YNJP9CR', 'description', 'Found 2026-09-09 while removing sibling-repo names and a hostname from a ticket
description before pushing. `make secrets` caught it, which is the system
working — CLAUDE.md is explicit that ticket prose is published prose, and the
gate ran before anything left the machine. What is missing is the route from
"caught it" back to a clean changelog.

WHY THE OBVIOUS FIXES DO NOT WORK

Fixing forward makes it worse. `ticket refine write` appends a ticket_history
row whose `old_value` is the previous description, so correcting a leaked
description writes the leaked text into the changelog a second time. The
outgoing diff then contains two copies rather than none.

Wiping and re-exporting does not work either. `plan export` emits "every
replicated planning row that has been modified since the last export", so it is
watermark-driven. Restoring the changelog files to an earlier state and
re-running it re-emits only rows touched since the watermark — in this case two
rows out of the fifteen or so that were needed. The rest stayed in pql.db,
absent from the changelog, with no supported way to get them back out.

WHAT IT ACTUALLY TOOK

    git reset --soft origin/main
    git checkout origin/main -- .pql/changelog
    rm .pql/pql.db
    pql plan rebuild --verify
    # then re-create four tickets by hand, re-entering every description,
    # and re-attach the parent links

That works and `--verify` reported 0 rows lost, but it is a hand-rolled
procedure recovered from reading the exporter''s help text under time pressure,
and the re-entry step is transcription with no check on it. Anyone hitting this
without the descriptions still in front of them loses the prose.

THE ASYMMETRY

`plan rebuild` reconstructs pql.db from the changelog and can be forced at any
time. There is no inverse. The pair is documented as replication, but only one
direction can be regenerated on demand — the other is append-only and
watermarked, so pql.db''s current state cannot be re-expressed as changelog
content once the watermark has passed it.

THE DESIGN QUESTION UNDERNEATH, which is why this is not just a missing flag

Is the changelog an append-only *log*, or a materialised *replica*?

- As a log, redaction is illegitimate by construction and the honest answer is
  that scrubbing prose requires rewriting git history, with the D-16 hashes and
  LWW guards recomputed. A `--force-full` export would be a footgun that
  silently rewrites replicated history other clones have already replayed.
- As a replica, regenerating it from pql.db is the natural operation and its
  absence is the defect.

D-15 and D-16 lean toward log (monthly append files, inline LWW guards, content
hashes, ON CONFLICT DO NOTHING on replay), but `plan rebuild` treats pql.db as
fully derivable from it, which is replica-shaped. The two readings have not had
to disagree until now.

Worth noting the blast radius differs by case. Redacting a ticket that has never
been pushed — this case — touches nothing another clone has seen, and a full
regeneration is safe. Redacting one that has been replayed elsewhere is a
distributed-state problem and probably out of scope for any flag.

DESIRED, not a design

Someone who has just been told by `make secrets` that a ticket description
leaks should have a supported route to fix it that does not involve re-entering
prose. Shapes worth weighing:

- A full/forced export that rewrites the month''s files from pql.db, refusing (or
  loudly warning) when the affected rows appear in commits already pushed.
- A redact verb scoped to descriptions, which rewrites the row and its history
  entries in place rather than appending.
- Nothing in the tool, and instead a documented procedure in CLAUDE.md next to
  the "ticket prose is published prose" table — cheapest, and honest if the log
  reading wins.

The third is a legitimate outcome. What is not legitimate is the current state,
where the gate reliably catches the problem and the recovery is undocumented.

RELATED

D-15, D-16 - changelog replication and the guards that make replay safe.
T-123     - the other place the single-writer assumption shows at the seam.

DECIDED (2026-09-09): D-35 settles the log-or-replica question this ticket raised — log, with the push boundary as the rewrite rule. Of the three shapes weighed here, the outcome is closest to the first, narrowed: not a force-full export, but a redact-class verb that rewrites the affected rows in pql.db, ticket_history (old_value copies included) and the changelog lines together, refusing when any affected line is already in a pushed commit. Pushed-history redaction stays a documented git-rewrite procedure, per the record. This ticket becomes the implementation carrier for that verb.', 'Found 2026-09-09 while removing sibling-repo names and a hostname from a ticket
description before pushing. `make secrets` caught it, which is the system
working — CLAUDE.md is explicit that ticket prose is published prose, and the
gate ran before anything left the machine. What is missing is the route from
"caught it" back to a clean changelog.

WHY THE OBVIOUS FIXES DO NOT WORK

Fixing forward makes it worse. `ticket refine write` appends a ticket_history
row whose `old_value` is the previous description, so correcting a leaked
description writes the leaked text into the changelog a second time. The
outgoing diff then contains two copies rather than none.

Wiping and re-exporting does not work either. `plan export` emits "every
replicated planning row that has been modified since the last export", so it is
watermark-driven. Restoring the changelog files to an earlier state and
re-running it re-emits only rows touched since the watermark — in this case two
rows out of the fifteen or so that were needed. The rest stayed in pql.db,
absent from the changelog, with no supported way to get them back out.

WHAT IT ACTUALLY TOOK

    git reset --soft origin/main
    git checkout origin/main -- .pql/changelog
    rm .pql/pql.db
    pql plan rebuild --verify
    # then re-create four tickets by hand, re-entering every description,
    # and re-attach the parent links

That works and `--verify` reported 0 rows lost, but it is a hand-rolled
procedure recovered from reading the exporter''s help text under time pressure,
and the re-entry step is transcription with no check on it. Anyone hitting this
without the descriptions still in front of them loses the prose.

THE ASYMMETRY

`plan rebuild` reconstructs pql.db from the changelog and can be forced at any
time. There is no inverse. The pair is documented as replication, but only one
direction can be regenerated on demand — the other is append-only and
watermarked, so pql.db''s current state cannot be re-expressed as changelog
content once the watermark has passed it.

THE DESIGN QUESTION UNDERNEATH, which is why this is not just a missing flag

Is the changelog an append-only *log*, or a materialised *replica*?

- As a log, redaction is illegitimate by construction and the honest answer is
  that scrubbing prose requires rewriting git history, with the D-16 hashes and
  LWW guards recomputed. A `--force-full` export would be a footgun that
  silently rewrites replicated history other clones have already replayed.
- As a replica, regenerating it from pql.db is the natural operation and its
  absence is the defect.

D-15 and D-16 lean toward log (monthly append files, inline LWW guards, content
hashes, ON CONFLICT DO NOTHING on replay), but `plan rebuild` treats pql.db as
fully derivable from it, which is replica-shaped. The two readings have not had
to disagree until now.

Worth noting the blast radius differs by case. Redacting a ticket that has never
been pushed — this case — touches nothing another clone has seen, and a full
regeneration is safe. Redacting one that has been replayed elsewhere is a
distributed-state problem and probably out of scope for any flag.

DESIRED, not a design

Someone who has just been told by `make secrets` that a ticket description
leaks should have a supported route to fix it that does not involve re-entering
prose. Shapes worth weighing:

- A full/forced export that rewrites the month''s files from pql.db, refusing (or
  loudly warning) when the affected rows appear in commits already pushed.
- A redact verb scoped to descriptions, which rewrites the row and its history
  entries in place rather than appending.
- Nothing in the tool, and instead a documented procedure in CLAUDE.md next to
  the "ticket prose is published prose" table — cheapest, and honest if the log
  reading wins.

The third is a legitimate outcome. What is not legitimate is the current state,
where the gate reliably catches the problem and the recovery is undocumented.

RELATED

D-15, D-16 - changelog replication and the guards that make replay safe.
T-123     - the other place the single-writer assumption shows at the seam.

DECIDED (2026-09-09): D-35 settles the log-or-replica question this ticket raised — log, with the push boundary as the rewrite rule. Of the three shapes weighed here, the outcome is closest to the first, narrowed: not a force-full export, but a redact-class verb that rewrites the affected rows in pql.db, ticket_history (old_value copies included) and the changelog lines together, refusing when any affected line is already in a pushed commit. Pushed-history redaction stays a documented git-rewrite procedure, per the record. This ticket becomes the implementation carrier for that verb.

RESOLVED (2026-09-09). pql ticket redact <id> <value> <replacement> is the supported route this ticket asked for. Shape: closest to the second option weighed here (a redact verb scoped to prose, rewriting rows in place rather than appending), governed by D-35 rather than a force-full export — the changelog stays a log, and the push boundary is the rewrite rule. It reaches the tickets row, every ticket_history row including old_value copies (the fix-forward duplication this ticket documented), and the changelog lines carrying them, through the D-28 staged-SQLite path with hashes recomputed by the same canonical projection the db side uses — so untouched lines return byte-identical and the rewritten file replays with verifying hashes (tested: replay into a fresh replica, rehash, compare). The asymmetry stands by design: plan export still cannot regenerate the changelog, because regeneration is exactly the replica reading D-35 rejected; what redaction needed was never regeneration, only a rewrite that moves both sides together. Pushed history refuses with the documented git-rewrite procedure as the named remedy. Verified live: leak planted via ticket append, caught in both tickets/ and ticket_history/ files, redacted in one command, plan rebuild --verify reports 0 rows lost.', NULL, '2026-09-09 10:43:30', '2026-09-09 10:43:30.310', '2026-09-09 10:43:30.310', NULL, 'c12c359cbac75e5822ea3d1815e68d67', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G89S0F3600NPD1978YNJP9CR', 'status', 'in_progress', 'done', NULL, '2026-09-09 10:43:30', '2026-09-09 10:43:30.331', '2026-09-09 10:43:30.331', NULL, '430f9c1a16b129b880e2a2c2626bb32e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FZ4FHC4YQRSRC071QNEWM64G', 'description', 'A value removed from a committed changelog file is re-published by the next ticket mutation, because pql.db still holds the original row and the write-through export (D-23) re-derives the file from it.

OBSERVED 2026-08-11. A ticket_history row had been scrubbed by hand — one line edited — before the commit that introduced it. Creating an unrelated ticket months later triggered the export, which re-appended that same row: byte-identical to the committed one apart from the scrubbed line, which came back in full. Same content hash on both. It was caught in review and removed from the working tree before staging, so nothing was published, but only because someone happened to diff the export before committing it.

TWO MECHANISMS COMPOUND, and either alone would be survivable.

1. The export boundary appears to be inclusive. The re-appended row''s updated_at equalled last_export_marker exactly, so a row already exported was exported again. A row that has not changed since the last export has nothing new to say, and re-emitting it is what turned a stale row into a live one.

2. The changelog is derived, and a scrub edits only the derivation. pql.db is the source the export reads. Editing the artefact leaves the source untouched, so the edit survives exactly until the next write — which is the least intuitive moment for it to be undone, because nothing about creating an unrelated ticket suggests it will rewrite history.

WHY THIS MATTERS MORE THAN IT LOOKS. This repo''s own CLAUDE.md states that everything committed here is published and indexed, that ticket prose is published prose, and that the changelog is committed by design so tickets travel with a clone. It also documents that history already carries findings resolved by untracking a file rather than rewriting the past. So scrubbing-before-commit is an established practice here, and this makes that practice unreliable in a way its user cannot see.

The pre-push gate scans the outgoing range, so it can catch a re-published value — but only for patterns its ruleset knows. A consuming repo''s name, a project path, or anything else specific to an operator''s environment is not a secret by any default ruleset, and those are precisely what the scrub-before-commit habit exists to remove.

WHAT ACTUALLY WORKED, and is worth documenting either way: `rm .pql/pql.db && pql plan rebuild` rebuilt the database from the scrubbed changelog and dropped the row. The repo''s documented recovery path is also its scrub-completion path, which is not obvious from either description. Verified: the row was present before the rebuild and absent after, with no ticket lost.

DIRECTIONS, not a prescription:
  - Make the export boundary exclusive, so an unchanged row is not re-emitted. Necessary, not sufficient: it fixes recurrence, not the divergence.
  - Treat a changelog edit as a database edit, or refuse it — the artefact and its source must not be independently editable if one regenerates the other.
  - At minimum, document that a scrub is incomplete until the database is rebuilt from the scrubbed file, and say so where the scrub-before-commit practice is described rather than only under recovery.

Distinct from T-96, which is about a mutation against a populated changelog with an empty database. This is the reverse: a populated database re-deriving over an edited changelog.

CORRECTION (T-106/T-107 round, 2026-08-11). The first prescribed direction here —
"make the export boundary exclusive" — is wrong and must not be implemented as
written.

The boundary is inclusive (`updated_at >= marker`) by design, not by oversight.
`exporter.go:26-33` documents why: write-through (D-23) calls Export after every
mutation, advancing the marker to "now" at second granularity. A mutation landing
in the SAME second as the marker would be silently skipped under a strict `>` —
which is exactly the data-loss class write-through exists to close. Making the
boundary exclusive trades this ticket''s re-emission-of-a-stale-row problem for
silent non-persistence of a live one. The second problem is worse: this ticket''s
symptom is noise a diff can catch (as this one was); the exclusive-boundary
failure mode is data that was never written and gives no signal that it is
missing.

The actual mechanism, confirmed by tracing `fileSink.appendLine` /
`fileSink.loadSeen` (`exporter.go:108-157`): a re-scanned row that is
byte-identical to what is already in the file is deduped and never written twice
— dedup is by content, not by marker exclusion. This ticket''s failure case is
narrower than "the boundary re-emits": it is specifically the scrub case, where
the row on disk and the row in pql.db have *diverged* (the file was hand-edited,
the database was not), so the re-scanned line is no longer byte-identical to what
loadSeen has on record and gets written as new content. An exclusive boundary
would not touch this at all, because the row that re-appears was never re-scanned
by the exclusive/inclusive distinction in the failing case that matters —
it would only stop the *harmless* re-scan-of-identical-content case T-107
separately (and also wrongly) worried about.

Second and third directions stand. "Treat a changelog edit as a database edit,
or refuse it" is the real fix and is still undecided — out of scope for the
T-106/T-107 round, which fixed the two rebuild-side defects (T-106: decisions
lost on recovery; T-107: investigated and found already fixed by T-26, see that
ticket''s closing note) without touching the export-boundary question this
ticket raises. "Document that a scrub is incomplete until rebuilt" is now true
by construction where it matters: `pql plan rebuild` (this ticket''s own
documented recovery) now also restores decisions (T-106''s fix), so the rebuild
path is a more complete scrub-completion step than it was when this ticket was
filed, but the underlying editable-artefact-vs-source divergence this ticket
names is unresolved and open.

DECIDED (2026-09-09): D-35 resolves the editable-artefact-vs-source divergence this ticket named as the real fix. Direction two (''treat a changelog edit as a database edit, or refuse it'') is adopted in both halves: the redact verb (implementation under T-130) moves both sides together so divergence is unrepresentable, and a detected hand-edit divergence is a defect state whose report names the verb. The scrub-completion documentation direction is superseded by the same record: unpushed scrubs get the verb, pushed scrubs get the documented git-rewrite procedure.', 'A value removed from a committed changelog file is re-published by the next ticket mutation, because pql.db still holds the original row and the write-through export (D-23) re-derives the file from it.

OBSERVED 2026-08-11. A ticket_history row had been scrubbed by hand — one line edited — before the commit that introduced it. Creating an unrelated ticket months later triggered the export, which re-appended that same row: byte-identical to the committed one apart from the scrubbed line, which came back in full. Same content hash on both. It was caught in review and removed from the working tree before staging, so nothing was published, but only because someone happened to diff the export before committing it.

TWO MECHANISMS COMPOUND, and either alone would be survivable.

1. The export boundary appears to be inclusive. The re-appended row''s updated_at equalled last_export_marker exactly, so a row already exported was exported again. A row that has not changed since the last export has nothing new to say, and re-emitting it is what turned a stale row into a live one.

2. The changelog is derived, and a scrub edits only the derivation. pql.db is the source the export reads. Editing the artefact leaves the source untouched, so the edit survives exactly until the next write — which is the least intuitive moment for it to be undone, because nothing about creating an unrelated ticket suggests it will rewrite history.

WHY THIS MATTERS MORE THAN IT LOOKS. This repo''s own CLAUDE.md states that everything committed here is published and indexed, that ticket prose is published prose, and that the changelog is committed by design so tickets travel with a clone. It also documents that history already carries findings resolved by untracking a file rather than rewriting the past. So scrubbing-before-commit is an established practice here, and this makes that practice unreliable in a way its user cannot see.

The pre-push gate scans the outgoing range, so it can catch a re-published value — but only for patterns its ruleset knows. A consuming repo''s name, a project path, or anything else specific to an operator''s environment is not a secret by any default ruleset, and those are precisely what the scrub-before-commit habit exists to remove.

WHAT ACTUALLY WORKED, and is worth documenting either way: `rm .pql/pql.db && pql plan rebuild` rebuilt the database from the scrubbed changelog and dropped the row. The repo''s documented recovery path is also its scrub-completion path, which is not obvious from either description. Verified: the row was present before the rebuild and absent after, with no ticket lost.

DIRECTIONS, not a prescription:
  - Make the export boundary exclusive, so an unchanged row is not re-emitted. Necessary, not sufficient: it fixes recurrence, not the divergence.
  - Treat a changelog edit as a database edit, or refuse it — the artefact and its source must not be independently editable if one regenerates the other.
  - At minimum, document that a scrub is incomplete until the database is rebuilt from the scrubbed file, and say so where the scrub-before-commit practice is described rather than only under recovery.

Distinct from T-96, which is about a mutation against a populated changelog with an empty database. This is the reverse: a populated database re-deriving over an edited changelog.

CORRECTION (T-106/T-107 round, 2026-08-11). The first prescribed direction here —
"make the export boundary exclusive" — is wrong and must not be implemented as
written.

The boundary is inclusive (`updated_at >= marker`) by design, not by oversight.
`exporter.go:26-33` documents why: write-through (D-23) calls Export after every
mutation, advancing the marker to "now" at second granularity. A mutation landing
in the SAME second as the marker would be silently skipped under a strict `>` —
which is exactly the data-loss class write-through exists to close. Making the
boundary exclusive trades this ticket''s re-emission-of-a-stale-row problem for
silent non-persistence of a live one. The second problem is worse: this ticket''s
symptom is noise a diff can catch (as this one was); the exclusive-boundary
failure mode is data that was never written and gives no signal that it is
missing.

The actual mechanism, confirmed by tracing `fileSink.appendLine` /
`fileSink.loadSeen` (`exporter.go:108-157`): a re-scanned row that is
byte-identical to what is already in the file is deduped and never written twice
— dedup is by content, not by marker exclusion. This ticket''s failure case is
narrower than "the boundary re-emits": it is specifically the scrub case, where
the row on disk and the row in pql.db have *diverged* (the file was hand-edited,
the database was not), so the re-scanned line is no longer byte-identical to what
loadSeen has on record and gets written as new content. An exclusive boundary
would not touch this at all, because the row that re-appears was never re-scanned
by the exclusive/inclusive distinction in the failing case that matters —
it would only stop the *harmless* re-scan-of-identical-content case T-107
separately (and also wrongly) worried about.

Second and third directions stand. "Treat a changelog edit as a database edit,
or refuse it" is the real fix and is still undecided — out of scope for the
T-106/T-107 round, which fixed the two rebuild-side defects (T-106: decisions
lost on recovery; T-107: investigated and found already fixed by T-26, see that
ticket''s closing note) without touching the export-boundary question this
ticket raises. "Document that a scrub is incomplete until rebuilt" is now true
by construction where it matters: `pql plan rebuild` (this ticket''s own
documented recovery) now also restores decisions (T-106''s fix), so the rebuild
path is a more complete scrub-completion step than it was when this ticket was
filed, but the underlying editable-artefact-vs-source divergence this ticket
names is unresolved and open.

DECIDED (2026-09-09): D-35 resolves the editable-artefact-vs-source divergence this ticket named as the real fix. Direction two (''treat a changelog edit as a database edit, or refuse it'') is adopted in both halves: the redact verb (implementation under T-130) moves both sides together so divergence is unrepresentable, and a detected hand-edit divergence is a defect state whose report names the verb. The scrub-completion documentation direction is superseded by the same record: unpushed scrubs get the verb, pushed scrubs get the documented git-rewrite procedure.

RESOLVED (2026-09-09), via D-35 and the redact verb (T-130). The re-publication mechanism this ticket documented — a hand-scrubbed file silently re-derived from an unscrubbed pql.db — is closed from both ends. Forward: pql ticket redact rewrites db and changelog together, so the scrub-then-diverge state is never created; the verb is the supported form of ''treat a changelog edit as a database edit''. Backward: a hand-edit remains illegitimate per D-35, and its failure mode is unchanged but now documented as such — the record names hand-editing a defect state and the verb the remedy. The export boundary was left inclusive per this ticket''s own correction (an exclusive boundary trades visible noise for silent data loss). The third direction (document that a scrub is incomplete until rebuilt) is superseded: with the verb, a scrub is complete when the command returns.', NULL, '2026-09-09 10:43:37', '2026-09-09 10:43:37.404', '2026-09-09 10:43:37.404', NULL, '1e7fff9842fd86fe2db91f8d17328d14', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FZ4FHC4YQRSRC071QNEWM64G', 'status', 'backlog', 'done', NULL, '2026-09-09 10:43:37', '2026-09-09 10:43:37.427', '2026-09-09 10:43:37.427', NULL, '5e6fc388b27f2b15c4a188f92c420c8e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G8AYX5XJWXT91BZHE2A8JYAG', 'description', 'Four shipped defects share one root: pql mutates planning state as if the local replica (.pql/pql.db) were authoritative, when the committed changelog is the log of record (D-15, D-28). T-96: a mutation against a populated changelog with an empty db silently relabels from T-1. T-105: scrubbing a changelog file does not survive the next mutation, because export is watermark-driven and re-emits the unscrubbed row. T-123: ticket new allocates a label from a replica it never checks is current. T-130: plan export cannot regenerate the changelog, so redaction means rebuild-and-refile by hand. Each child is fixable alone, but the durable fix is one discipline: before allocating or emitting, a mutation consults the changelog side, not just the replica. Adjacent but out of scope: T-111 (the pre-commit hook exports whatever vault the environment names) — that is an init/hook defect, not a replica-trust one.', 'Four shipped defects share one root: pql mutates planning state as if the local replica (.pql/pql.db) were authoritative, when the committed changelog is the log of record (D-15, D-28). T-96: a mutation against a populated changelog with an empty db silently relabels from T-1. T-105: scrubbing a changelog file does not survive the next mutation, because export is watermark-driven and re-emits the unscrubbed row. T-123: ticket new allocates a label from a replica it never checks is current. T-130: plan export cannot regenerate the changelog, so redaction means rebuild-and-refile by hand. Each child is fixable alone, but the durable fix is one discipline: before allocating or emitting, a mutation consults the changelog side, not just the replica. Adjacent but out of scope: T-111 (the pre-commit hook exports whatever vault the environment names) — that is an init/hook defect, not a replica-trust one.

CLOSED (2026-09-09). All four children done in one arc, plus the decision that unblocked the last two. T-96/T-123: GuardReplicaCurrent refuses mutations against a replica that is empty or behind the changelog''s ever-minted labels — allocation can no longer outrun the log. D-35 settled the log-or-replica ambiguity: the changelog is a log, and the push boundary is the rewrite rule. T-105/T-130: pql ticket redact rewrites db and changelog together for unpushed history, refusing on published values — the discipline this epic named (''before allocating or emitting, consult the changelog side'') is now enforced on both the allocation seam and the redaction seam. Adjacent and still open: T-111 (hook exports whatever vault the environment names), out of scope here as an init/hook defect.', NULL, '2026-09-09 10:43:50', '2026-09-09 10:43:50.493', '2026-09-09 10:43:50.493', NULL, 'dfc642994115e51d3dfba28ff8f96c4e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G8AYX5XJWXT91BZHE2A8JYAG', 'status', 'backlog', 'done', NULL, '2026-09-09 10:43:50', '2026-09-09 10:43:50.515', '2026-09-09 10:43:50.515', NULL, 'c5e367f7ba937a531106e29237cec2cf', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C83X7F5WDSXK11XWKM47C', 'status', 'ready', 'in_progress', NULL, '2026-09-09 10:45:17', '2026-09-09 10:45:17.540', '2026-09-09 10:45:17.540', NULL, '1ca5ca0cf1d6c017ac8567188e7fb84d', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C83X7F5WDSXK11XWKM47C', 'description', 'Implements D-30.

The mutation verbs return whatever each one happened to return. ticket assign gives a whole record including a 2 KB description; ticket label gives {ticket_ids, action, label}. Neither is documented, and --fields is rejected, so the verbs with the fattest output are the ones with no way to trim.

D-30 keeps projection off the mutation surface — a mutation''s return value is a receipt, and a caller who trimmed it to id has confirmed nothing — and standardises the shapes instead. One record changed returns that record whole. Several changed return a summary object naming the ids and what was applied, which is what ticket label already does.

Work: audit every mutation verb (status, assign, team, label, setparent, decision, block, unblock, append, refine write) for which shape it currently returns, converge the batch cases on the summary, and document both shapes in the skill. The documentation half is worth doing first and separately — it is the part a caller is blocked on today, and it does not change behaviour.

The shape change lands in a minor.

Split 2026-08-08. The documentation half shipped in 2.2.0 (commit a1e5172): the skill now states that mutation verbs reject --fields deliberately — a receipt trimmed to id confirms nothing — that changing one record returns that record whole including its description, that changing several returns a summary object, and that not every batch verb has converged on the summary yet.

That was the half blocking callers, since the shapes were entirely undocumented. What remains is the convergence itself: audit status, assign, team, label, setparent, decision, block, unblock, append and refine write for which shape each returns today, and move the batch cases onto the summary.

Held back from 2.2.0 deliberately. It changes ten verbs'' output shapes with real regression surface, immediately after a release, and none of it is blocking anyone now that the current behaviour is written down. Stays ready rather than backlog — it is well-specified and can be picked up whenever, it just should not have been rushed alongside two behaviour changes.', 'Implements D-30.

The mutation verbs return whatever each one happened to return. ticket assign gives a whole record including a 2 KB description; ticket label gives {ticket_ids, action, label}. Neither is documented, and --fields is rejected, so the verbs with the fattest output are the ones with no way to trim.

D-30 keeps projection off the mutation surface — a mutation''s return value is a receipt, and a caller who trimmed it to id has confirmed nothing — and standardises the shapes instead. One record changed returns that record whole. Several changed return a summary object naming the ids and what was applied, which is what ticket label already does.

Work: audit every mutation verb (status, assign, team, label, setparent, decision, block, unblock, append, refine write) for which shape it currently returns, converge the batch cases on the summary, and document both shapes in the skill. The documentation half is worth doing first and separately — it is the part a caller is blocked on today, and it does not change behaviour.

The shape change lands in a minor.

Split 2026-08-08. The documentation half shipped in 2.2.0 (commit a1e5172): the skill now states that mutation verbs reject --fields deliberately — a receipt trimmed to id confirms nothing — that changing one record returns that record whole including its description, that changing several returns a summary object, and that not every batch verb has converged on the summary yet.

That was the half blocking callers, since the shapes were entirely undocumented. What remains is the convergence itself: audit status, assign, team, label, setparent, decision, block, unblock, append and refine write for which shape each returns today, and move the batch cases onto the summary.

Held back from 2.2.0 deliberately. It changes ten verbs'' output shapes with real regression surface, immediately after a release, and none of it is blocking anyone now that the current behaviour is written down. Stays ready rather than backlog — it is well-specified and can be picked up whenever, it just should not have been rushed alongside two behaviour changes.

RESOLVED (2026-09-09). All six batch-capable verbs converge on one renderer (renderMutationReceipt): one record changed returns that record whole, several return the summary shape ticket label established — {ticket_ids, action, <value-key>: <applied>}, with the applied value keyed semantically (status, assigned_to, team, parent_id, decision_ref, label). ticket label itself gained the missing half: a single-id call now returns the whole record where it previously returned the summary regardless of count. status --force''s cascade receipt lists every ticket actually closed, not just the ids named. block/unblock keep their pair receipt, append and refine write keep their documented single-record shapes, new keeps its {id}. The skill''s ''not every batch verb has converged'' caveat is replaced by the rule stated plainly, and the integration test that pinned the old N-records cascade shape now asserts the summary and confirms state via show — which is the documented receipt workflow. Shape change ships in the next minor per this ticket''s own hold-back note.', NULL, '2026-09-09 10:48:59', '2026-09-09 10:48:59.799', '2026-09-09 10:48:59.799', NULL, 'b52c11ac580bb5a70315ee37727e2d54', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C83X7F5WDSXK11XWKM47C', 'status', 'in_progress', 'done', NULL, '2026-09-09 10:48:59', '2026-09-09 10:48:59.831', '2026-09-09 10:48:59.831', NULL, 'd844fe7a0d6741980e83535d510eaab5', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06GCTEGNA7CSH8H9VHM3KKC024', 'description', NULL, 'From a pql-driven session in a downstream repo, filing detailed tickets (multi-paragraph specs) took two calls every time: `pql ticket new <type> <title> --description "..."` cannot take a file, so long descriptions either go through shell quoting (fragile, and command substitution trips agent permission allowlists) or through a second `pql ticket append <id> --file <path>` call.

`ticket append` already has the right input modes (`--file`, `--stdin`, mutually exclusive with the positional text). Mirror them on `ticket new`:

- `pql ticket new <type> <title> --description-file <path>`
- `pql ticket new <type> <title> --description-stdin`

mutually exclusive with `--description`. Same encoding handling as `append --file`.

Acceptance: a ticket created with `--description-file` has exactly the file''s contents as its description (no trailing-newline drift); passing two description sources is a usage error; the changelog entry records the description like any other create.', NULL, '2026-09-23 07:45:37', '2026-09-23 07:45:37.299', '2026-09-23 07:45:37.299', NULL, 'f06fe3f8d76c5a3c5c255b49e4da54fc', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06GCTEGRSZ9911SRAF9RBDHWJ8', 'description', NULL, 'Batch ids are comma-separated (`pql ticket status T-1,T-2 done`, `pql ticket show T-1,T-2`), but the natural first guess is space-separated. `pql ticket status T-592 T-593 review` fails with cobra''s generic `accepts 2 arg(s), received 3`, which says nothing about the comma syntax. Agents driving pql hit this repeatedly and fall back to one call per id (or a shell loop, which trips permission allowlists).

Options, pick one:
1. Accept space-separated ids where the grammar is unambiguous: for `status`, every positional but the last is an id; for `show`, every positional is an id. Commas keep working.
2. Keep the grammar, but when extra positionals all look like ticket ids, fail with a hint: `use commas to batch: pql ticket status T-592,T-593 review`.

(1) is friendlier; (2) is the minimal fix. Apply the same treatment to the other comma-batching verbs (assign, label, team, setparent, …) so the rule is uniform.

Acceptance: `pql ticket status T-1 T-2 done` either works or names the comma syntax in its error; same for `ticket show T-1 T-2`; covered by CLI tests.', NULL, '2026-09-23 07:45:38', '2026-09-23 07:45:38.153', '2026-09-23 07:45:38.153', NULL, 'e30e54100b17e877b011fa595b036cf1', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06GCTEGW5RJJJ44ZGKJ8F1RFH4', 'description', NULL, '`pql ticket new task "..." --decision D-110` where D-110 is written in the markdown but not yet synced into pql.db fails with the raw SQLite error:

    {"level":"error","code":"cli.exit","msg":"repo: create ticket: constraint failed: FOREIGN KEY constraint failed (787)"}

It doesn''t say which reference is missing (the decision? the parent?) or what to do. The same raw error is likely for an unknown `--parent`.

Fix: check the references before the insert (or map the FK failure) and name them:

- `decision D-110 not found in the planning DB — if it exists in the markdown, run "pql decisions sync"`
- `parent T-9999 not found`

Exit with the usual user-error code, not `cli.exit`. Worth auditing the other verbs that take a decision/ticket reference (`ticket decision`, `setparent`, `block`) for the same raw error.

Acceptance: each unknown-reference case names the missing id and suggests the fix; no raw `constraint failed` text reaches the user; covered by CLI tests.', NULL, '2026-09-23 07:45:38', '2026-09-23 07:45:38.643', '2026-09-23 07:45:38.643', NULL, 'dc8eee4a3dcfa8634b35817131b2a73c', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06GCTEGZ3106HDDASVEHMHCF9C', 'description', NULL, 'Follow-up to T-58. T-58 made a linked git worktree its own vault root, which is right for the markdown side: DQR edits in the worktree are what `decisions validate` should see. But the planning DB (`.pql/pql.db`) is gitignored, user-authored state, so a fresh worktree has none. Inside a worktree every planning read is empty ("ticket T-1 not found") and every write lands in a throwaway DB that no other checkout sees.

Field report (a downstream repo running several agents in parallel worktrees, 2026-09-23):
- `pql ticket show` / `status` from inside the worktrees saw no tickets.
- The replication pre-commit shim that `pql init` plants sources `<toplevel>/.pql/hooks/pre-commit`, which is also untracked, so it is missing in the worktree and **every `git commit` in the worktree fails**. The only way through is `--no-verify`, which the repo forbids. The workaround was for agents to never commit (patch files applied from the main checkout).
- Had the hook run, its `plan export` from an empty worktree DB would have staged a changelog export that deletes every ticket.

Proposal: split resolution in a linked worktree (`git rev-parse --git-dir` ≠ `--git-common-dir`):
1. **Markdown / DQR / index:** the worktree root, as T-58 does now.
2. **Planning state (`pql.db`):** the main checkout''s `.pql/pql.db` (the worktree containing the common dir). Tickets are shared state, so every checkout should read and write one DB, and the changelog export stays single-writer.
3. **Hook shims:** resolve the hook script via the common dir, not `--show-toplevel`, so they exist in every worktree.
4. **Changelog export in a worktree:** don''t export (or export nothing to stage). The main checkout owns `.pql/changelog/`; worktree branches committing their own exports would conflict on every merge. The pre-commit shim in a worktree should succeed as a no-op.
5. `pql doctor` should report both roots when they differ (markdown root vs planning root).

Open question: a user who wants per-worktree ticket sandboxes (experiments) — an explicit `--db` / config opt-out would cover it.

Acceptance: from a linked worktree, `pql ticket list` returns the main checkout''s tickets; a ticket created in the worktree is visible from main immediately; `git commit` in a worktree succeeds with the hooks installed and stages no `.pql/changelog` changes; `decisions validate` still reads the worktree''s markdown; `pql doctor` shows both roots; integration tests cover a worktree inside and outside the main checkout.', NULL, '2026-09-23 07:45:39', '2026-09-23 07:45:39.282', '2026-09-23 07:45:39.282', NULL, '58fc9cfe6108e61bbdebf4749e5e4eb7', 2) ON CONFLICT(hash) DO NOTHING;

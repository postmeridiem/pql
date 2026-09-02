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

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

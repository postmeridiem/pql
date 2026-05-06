# clean-house rule catalog

The rules clean-house runs and the reasoning behind each. New rules
land here as their own entry; deletions stay below in the changelog so
the trail of which rules used to fire (and why they were retired) is
recoverable.

## Rule changelog

- **v1.0** — Initial catalog. Eight starter rules: four mechanical,
  four judgment. Detection sections name the pql verb or file
  primitive each rule depends on.

## Reading a rule

Each entry below has:

- **ID** — Stable identifier (e.g. `RULE-ANCHOR-DRIFT`). Used in
  AskUserQuestion prompts and skip-ledger entries so the user learns
  what's being checked.
- **Category** — `mechanical` or `judgment`. Drives whether findings
  batch into one prompt or each get their own.
- **Detection** — Concrete steps to find violations. Names the
  pql command or file primitive used.
- **Fix** — Mechanical: deterministic action. Judgment: the prompt
  option list and what each option does.
- **Why** — One paragraph on the failure mode this rule guards
  against. Future-you reading the rule benefits from the reasoning,
  not just the check.

---

## RULE-ANCHOR-DRIFT

**Category:** mechanical

**Detection:**

For each decision record, fetch its body and headings via
`pql decisions read <id>`. The response includes a `headings` array
with `{level, text, slug}` for every heading in the body. Within the
body, find inline markdown links of the shape `[text](#anchor-slug)`
or `[D-N](#anchor-slug)`. For each link target, check whether the
slug exists in the headings array of the referenced record (the
record matching `D-N` if the link is cross-record, or the same record
for intra-record anchors). Flag each link whose target slug is absent.

**Fix:**

If exactly one heading with a closely-matching slug exists (slug
edit-distance ≤ 2 or substring match), rewrite the link to point at
that heading. If no plausible match exists, downgrade the finding to
judgment ("no auto-fix; the heading was removed, not renamed") and
prompt the user with options: drop the link / point elsewhere / mark
as intentionally dangling.

**Why:**

When a heading is renamed for clarity, every cross-reference pointing
at the old slug silently breaks. The rendered docs still look fine —
the link just goes nowhere. Without periodic sweeps these decay
indefinitely; the cost of the sweep is small compared to the cost of
a reader following a dead link and losing trust in the index.

---

## RULE-MISSING-Q-BACKLINK

**Category:** mechanical

**Detection:**

For each D record body, find lines of the shape `Resolves: Q-N`
(plain text, typically in the metadata block at the top of the
record). For each Q-N referenced, fetch that Q record's body via
`pql decisions read Q-N` and check whether it contains a line of the
shape `Resolved → D-N` (or equivalent backlink phrasing — match the
project's own convention; default pattern is `Resolved → D-N`).

Also check the reverse direction: any Q record claiming
`Resolved → D-N` whose D-N body lacks `Resolves: Q-N`.

**Fix:**

Insert the missing backlink in-place using `Edit`:

- Missing on the D side: add `Resolves: Q-N` to the D's metadata
  block (typically right after `Domain:` / `Status:` lines).
- Missing on the Q side: append `Resolved → D-N` to the Q's status
  line (or in the conventional position for that project).

**Why:**

Bidirectional Q↔D links are the navigation backbone of the DQR
system. When one side drifts, search-by-decision finds nothing for
that question and search-by-question doesn't surface its resolution.
The asymmetric state usually arises from a hand-edit on one record
that forgot to update the other; the fix is purely mechanical because
the correct content is already determined by the existing pointer
in the other direction.

---

## RULE-RECORD-SORT

**Category:** mechanical

**Detection:**

For each `decisions/*.md` file, read it directly (no pql) and find
all top-level `## D-N` / `## Q-N` / `## R-N` headings in order. Strip
the prefix, parse the numeric suffix, and check whether the sequence
is monotonically ascending within each ID family (D, Q, R kept
separate — files commonly mix families).

Only flag a file if it is **otherwise tidy** — currently no out-of-
order amendments interleaved with new records. The heuristic: if the
sort would touch fewer than three records out of position, apply it;
if more, downgrade to judgment ("this file looks deliberately
arranged — confirm before reordering"). Three is a soft threshold; if
the file's last commit message contains `WIP` or `do-not-sort`, skip.

**Fix:**

Reorder the records within the file so each ID family is ascending.
Preserve everything else (headings between record blocks, any prose
prelude/postlude). Use `Edit` with full block replacement, not in-
place line shuffling — easier to verify the diff.

**Why:**

Records added at the bottom of a file are easy to write but hard to
find. Sorted IDs let a reader find D-37 by jumping to the
two-thirds mark of the file rather than scanning. The cost of the
sort is one reordering pass; the benefit is paid back on every
subsequent read.

---

## RULE-EOF-NORMALIZATION

**Category:** mechanical

**Detection:**

For each `decisions/*.md` file (and any other markdown the skill
touched during this run), read directly. Flag if:

- File does not end with exactly one `\n`.
- Any line contains trailing whitespace before its `\n`.

**Fix:**

Trim trailing whitespace from each line. Ensure exactly one trailing
newline at end of file. Use `Edit` only if a violation was found —
this rule must not produce a no-op diff.

**Why:**

Editor and git config drift causes whitespace creep that's invisible
in rendering but pollutes diffs (every record edit ends up touching
unrelated lines). A periodic normalization keeps future diffs clean
without forcing per-editor enforcement on every contributor.

---

## RULE-SUNSET-WITHOUT-TICKET

**Category:** judgment

**Detection:**

For each D record body (via `pql decisions read <id>`), grep for
sunset-shaped phrases. Default regex set:

```
(?i)\b(delete|remove|sunset|kill[ -]?switch|tear[ -]?down) when\b
(?i)\bmust (track|monitor|watch|follow)\b
(?i)\brevisit (when|after|once)\b
(?i)\b(deprecate|retire) (when|after|once)\b
```

For each match, run `pql ticket list --decision <id>`. If no tickets
exist, the D has work-shaped intent without a tracked T — flag.

**Fix (prompt options):**

- **File T now** — Run `pql ticket new task "<phrase>" --decision <id>`,
  then prompt the user for a description (or call `pql ticket refine
  write <T> --description ...` after creation).
- **Draft a T description first** — Open an `AskUserQuestion` for the
  description, then file as above.
- **Mark as already covered** — Add a note to the D body
  ("Tracked under T-N") via `Edit`. Skill does not assert which T;
  user provides the ID.
- **Skip** — Adds to the ledger.
- **Stop asking** — Skip remaining sunset findings, summarize.

**Why:**

Sunset-shaped intent ("we'll handle X when Y happens") is the most
common source of accumulated debt in a DQR system: the trigger
condition arrives, nobody remembers the D, the work doesn't happen.
Linking each sunset to a T is the simplest defense — a T is a
backlog item that surfaces in `plan whatsnext` / `plan board`. The
fix is judgment because the right action depends on whether the D's
condition is still relevant, whether it's already covered, and what
the right scope is for the resulting T.

---

## RULE-FILE-OVER-THRESHOLD

**Category:** judgment

**Detection:**

For each `decisions/*.md` file, count lines (via `wc -l` or
equivalent). Default threshold: **350 lines**. Configurable per
project — read `decisions/.clean-house.yaml` if present, key
`file_threshold`. Fall back to 350.

**Fix (prompt options):**

- **Split now (which axis?)** — Prompt for the split axis: by ID
  family (D/Q/R), by domain, by date range, custom. Then perform the
  split: create new file(s), move records, update any anchor links
  pointing into the moved records (run RULE-ANCHOR-DRIFT in apply
  mode against the affected files after the split).
- **Accept and raise the threshold** — Update
  `decisions/.clean-house.yaml`'s `file_threshold` to the next
  reasonable round number above the current line count.
- **Defer** — Skip until next run.

**Why:**

A single decisions file growing past ~350 lines is the point at which
linear scanning starts to lose to grep, and where the cost of
splitting (renaming anchors) is still small. Beyond ~600 lines the
split cost compounds. The threshold is judgment because some projects
deliberately keep one file per domain and accept the size; others
split aggressively. The skill surfaces the question; it doesn't
decide.

---

## RULE-DEAD-FILE-REFERENCE

**Category:** judgment

**Detection:**

For each D record body (via `pql decisions read <id>`), grep for
path-shaped tokens. Default regex:

```
\b([\w./-]+\.(md|go|py|sql|yaml|yml|toml))\b
```

For each match, check whether the path exists in the repo (via
`os.Stat` from a `Bash` call: `[ -e <path> ]`). Flag any reference
to a non-existent path.

**Fix (prompt options):**

- **Update reference** — Prompt for the new path; rewrite the
  reference via `Edit`. If the user types a path, validate it exists
  before applying.
- **Mark superseded** — Add a note to the D ("This decision
  references files no longer present; the underlying constraint
  was retired by D-N") and prompt for the superseding D ID.
- **Defer** — Skip until next run.

**Why:**

Decisions reference code paths to ground their reasoning in the
codebase that motivated them. When the code moves or gets deleted,
the D's reasoning becomes harder to verify. Dead refs are a signal
that either the decision should be updated or the decision itself is
no longer load-bearing — both are judgment calls the user needs to
make.

---

## RULE-STALE-OPEN-Q

**Category:** judgment

**Detection:**

Run `pql decisions list --type question --status open`. For each
returned record, check the `date` field. Default staleness threshold:
**60 days** (configurable via `decisions/.clean-house.yaml` key
`stale_open_q_days`). Flag any Q with `date` older than the
threshold.

**Fix (prompt options):**

- **Still relevant** — Touch the Q's `date` field to today, optionally
  add a one-line "still open as of YYYY-MM-DD: <reason>" to the body.
- **Nudge to load-bearing index** — If the project keeps a
  load-bearing Qs list (a curated index of unresolved questions
  affecting current work), prompt for inclusion and add the Q there.
- **Mark withdrawn** — Change the Q's status to `withdrawn` (the DQR
  system's terminal state for "no longer worth answering"); the
  user supplies a one-line reason.
- **Defer** — Skip until next run.

**Why:**

Open questions are useful when they're current; stale ones become
ambient noise that drowns the signal of new questions. Most
projects don't enforce explicit Q-lifecycle, so without a periodic
prompt the open list grows forever. The right action depends on
whether the question is still open in fact — not just in metadata —
which only the user can confirm.

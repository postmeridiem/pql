INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06G5HXSMTZDG68R0WRV0V4S25M', 'task', NULL, 'Rename decisions resolve to close, widen its targets, and cover decision supersession', '`pql decisions resolve <Q-N> --into <D-N>` (T-99) hardcodes one legal target
type and carries a name that fits only that case. Both should widen.

WHAT `close` MEANS

One verb, one flag. The relation follows from the pair of record types, so the
caller never has to name it and pql owns the vocabulary:

  pql decisions close Q-2  --into D-23    question answered          -> resolved
  pql decisions close Q-4  --into R-1     answered "no"              -> resolved
  pql decisions close D-13 --into D-15    replaced by a newer record -> superseded

D -> Q is rejected: a decision does not go into a question.

`resolve` is accurate for Q -> D and inaccurate for everything else, which is
the whole argument for the rename. `close` is honest across every terminal
disposition a record can reach.

RENAME NOW, IT IS ONLY FREE NOW

`resolve` shipped in 2.3.0, which is unreleased. Renaming before that release
costs nothing. After it, the choice is a compatibility break for no functional
gain, or carrying two verbs that perform the same file surgery under names only
one of them fits. This is the cheap half of the ticket and can land alone.

THE SUBSTANTIVE HALF IS DECISION -> DECISION

This is the case with real evidence behind it. Decisions supersede each other
routinely in this tree and every instance is hand-maintained today:

  D-15 supersedes D-13
  D-14 supersedes D-10

The relation is genuinely different from a resolution, not a rewording of it:

  - It writes a PAIR of fields across TWO records - `**Supersedes:**` on the
    new record and `**Superseded by:**` on the old one - where Q -> D writes
    one line on the question and optionally one on the decision.
  - Both sides must agree. A half-written pair is worse than none, because
    `decisions list --status active` would still show a superseded record as
    live while the other file says otherwise.
  - The resulting status is `superseded`, not `resolved`, and it is inferred
    from the `**Superseded by:**` line rather than from `**Status:**`.

So the file surgery is not a widened branch of the existing one. Encapsulating
that difference is the point of the verb.

QUESTION -> REJECTION

Legitimate and cheap: "should we do X?" closed by an R record recording that X
was considered and rejected. Currently refused, for the same
one-hardcoded-target-type reason.

Worth knowing before scheduling it: there are zero instances. `governance/`
has no `rejected/` records at all, so this branch would ship untested against
real data and unexercised in practice. Do it because it completes the type
matrix, not because anything is waiting on it.

CLOSING WITHOUT A TARGET

A question can stop mattering without being answered - the subsystem it asked
about was removed, the constraint that raised it lifted, the framing turned out
to be wrong. There is nothing to point `--into` at, and inventing a D record to
absorb it would be manufacturing a decision nobody made.

  pql decisions close Q-7 --obsolete

Open point, and it is a real one: the status vocabulary has no value for this.
`inferStatus` returns `resolved` only when the `**Status:**` line begins with
"resolved", so a line reading "Obsolete" falls through to `open` and the record
never leaves the open count - the exact failure T-99 set out to fix, arriving
by a different door. Either the written wording starts with "Resolved" and says
obsolete afterwards, or the parser learns a fifth status. The first is cheaper
and needs no schema change; the second is more honest, since "resolved" claims
an answer that does not exist. Decide before implementing.

THE REJECTION MESSAGE IS PART OF THE FEATURE

An unsupported pair must not simply be refused. The three routes out are not
guessable from a bare "invalid target", and a caller who reaches for Q -> Q has
a real disposition in mind and needs to be told which of the legal ones it is.
So the error names the whole set:

  error: Q-13 is a question; a question cannot close into another question
    a question is closed by what ANSWERS it, or not at all:
      --into D-N      a decision that answers it
      --into R-N      a rejection that answers it "no"
      --obsolete      it stopped mattering; nothing answered it
    if Q-13 merely develops or narrows Q-9, that is a cross-reference and
    already recorded — leave Q-9 open

That last line matters most, because it is the case that produced this ticket''s
original mistake, and the error is the right place to stop the next person
making it. Same principle as T-112: the program knows the accepted set at the
moment it rejects the input, so it should print it.

CORRECTION TO THIS TICKET''S ORIGINAL PREMISE

Filed originally on the claim that Q-9 could not be closed because it is
absorbed by Q-13, and that this was a supersession the verb could not express.
That was a misreading, corrected by the maintainer, and the original framing is
recorded here rather than quietly dropped because the mistake is instructive.

Q-9 says its converged direction "is developed in" Q-13, and Q-13 calls itself
"a concrete instantiation of Q-9". Q-13 narrows one branch of Q-9. It does not
replace it and does not render it obsolete, which is what supersession means.
Q-9 asks how multiple contributors mint ticket ids without collisions, and that
is still undecided - when Q-13 lands as a decision, that D resolves both. The
pointer between them is a see-also, and the tree already records it as a ref.

Two things follow. There is no Q -> Q case to support, so it is deliberately
absent from the matrix above. And the survey that produced this ticket reported
"one closable question" - that was wrong. All twelve open questions are open,
and the count remains accurate for the reason T-99 wanted to establish.

SCOPE NOTE

Whatever ships keeps T-99''s two rules, load-bearing there and here:

  - Write the markdown. The DQR tree is the source of truth for decisions
    (D-8), so a status written only to pql.db is reverted by the next sync.
  - Never clobber prose that already exists in a target field. Add when
    absent, report when present. The pair-writing in the D -> D case makes
    this sharper, not softer: two files, two chances to overwrite something a
    human wrote.

Re-sync pql.db and regenerate the DQR README before returning, as T-99 does -
the README splits questions into open and resolved buckets and would otherwise
contradict the database.

RELATED

T-99  - the verb this renames and extends.
D-29  - an argument that names a thing is validated. Both ids here are names,
        so an unknown one, or a type the pair matrix does not allow, exits
        non-zero naming the problem.
', 'done', 'medium', NULL, NULL, 'D-29', '2026-08-31 17:56:31.063', '2026-09-01 15:43:44.496', NULL, '89ac5deb6024e5cac490bec085221775', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;

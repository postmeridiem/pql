package changelog

import (
	"fmt"
	"strings"
)

// tableSpec describes how one replicated table serialises to a changelog line.
//
// The shape used to live as five inlined format strings, one per exporter. That
// was survivable while only the exporter wrote lines, but a format upgrade has
// to *re-emit* existing rows through the very same renderer — otherwise the
// upgrade's output drifts from what a normal export would have produced, and
// two clones stop agreeing byte-for-byte. One spec, one renderer, one place the
// emitted guard is defined.
type tableSpec struct {
	// Name is the SQL table.
	Name string
	// Columns is the emitted column list, in order. Values passed to render
	// must line up with it.
	Columns []string
	// Key is the conflict target: the columns after ON CONFLICT.
	Key []string
	// AppendOnly marks a table whose rows never mutate — the conflict action is
	// DO NOTHING and no LWW guard applies. ticket_history is the only one:
	// audit rows are immutable, and UNIQUE(hash) makes replay idempotent.
	AppendOnly bool
}

// changelogTables is the closed set of replicated tables. Order matches the
// exporters; the upgrader iterates the same set so neither can silently cover
// fewer tables than the other.
var changelogTables = []tableSpec{
	{
		Name: "tickets",
		Columns: []string{
			"record_id", "type", "parent_record_id", "title", "description",
			"status", "priority", "assigned_to", "team", "decision_ref",
			"created_at", "updated_at", "deleted_at", "hash", "canonical_version",
		},
		Key: []string{"record_id"},
	},
	{
		Name: "ticket_idmap",
		Columns: []string{
			"record_id", "ticket_id",
			"created_at", "updated_at", "deleted_at", "hash", "canonical_version",
		},
		Key: []string{"record_id"},
	},
	{
		Name: "ticket_deps",
		Columns: []string{
			"blocker_record_id", "blocked_record_id",
			"created_at", "updated_at", "deleted_at", "hash", "canonical_version",
		},
		Key: []string{"blocker_record_id", "blocked_record_id"},
	},
	{
		Name: "ticket_labels",
		Columns: []string{
			"ticket_record_id", "label",
			"created_at", "updated_at", "deleted_at", "hash", "canonical_version",
		},
		Key: []string{"ticket_record_id", "label"},
	},
	{
		Name: "ticket_history",
		Columns: []string{
			"ticket_record_id", "field", "old_value", "new_value", "changed_by",
			"changed_at", "created_at", "updated_at", "deleted_at",
			"hash", "canonical_version",
		},
		Key:        []string{"hash"},
		AppendOnly: true,
	},
}

// renderRow is the exporters' entry point: look up the table's spec and render
// one row. An unknown table is a programming error rather than a data problem,
// so it surfaces as an error rather than a silently skipped row.
func renderRow(table string, values []string) (string, error) {
	spec, ok := specFor(table)
	if !ok {
		return "", fmt.Errorf("changelog: no statement spec for table %q", table)
	}
	return spec.render(values)
}

// specFor returns the spec for a table name.
func specFor(name string) (tableSpec, bool) {
	for _, s := range changelogTables {
		if s.Name == name {
			return s, true
		}
	}
	return tableSpec{}, false
}

// updateColumns are the columns a conflicting row overwrites: everything except
// the conflict key (which by definition matches) and created_at (the row's
// birth, which a later write must not move). Derived rather than listed so a
// new column is carried automatically instead of being silently dropped by a
// stale hand-maintained list.
func (s tableSpec) updateColumns() []string {
	key := make(map[string]bool, len(s.Key))
	for _, k := range s.Key {
		key[k] = true
	}
	out := make([]string, 0, len(s.Columns))
	for _, c := range s.Columns {
		if key[c] || c == "created_at" {
			continue
		}
		out = append(out, c)
	}
	return out
}

// render produces the exact changelog line for one row. values are pre-quoted
// SQL literals (see sqlStr / sqlNullStr / sqlNullInt), one per column, in
// Columns order.
//
// The trailing `WHERE excluded.updated_at >= <table>.updated_at` is the inline
// last-writer-wins guard (D-16, amended by T-59): a strictly newer row wins,
// and an exact tie resolves in favour of whichever statement is applied later —
// the row appended later in the changelog, since replay walks tables, files and
// lines in sorted order.
func (s tableSpec) render(values []string) (string, error) {
	if len(values) != len(s.Columns) {
		return "", fmt.Errorf("changelog: %s: %d values for %d columns",
			s.Name, len(values), len(s.Columns))
	}

	var b strings.Builder
	b.WriteString("INSERT INTO " + s.Name + " (")
	b.WriteString(strings.Join(s.Columns, ", "))
	b.WriteString(") VALUES (")
	b.WriteString(strings.Join(values, ", "))
	b.WriteString(") ON CONFLICT(")
	b.WriteString(strings.Join(s.Key, ", "))
	b.WriteString(")")

	if s.AppendOnly {
		b.WriteString(" DO NOTHING;")
		return b.String(), nil
	}

	b.WriteString(" DO UPDATE SET ")
	sets := s.updateColumns()
	for i, c := range sets {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(c + "=excluded." + c)
	}
	b.WriteString(" WHERE excluded.updated_at >= " + s.Name + ".updated_at;")
	return b.String(), nil
}

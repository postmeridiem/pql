package changelog

import (
	"strings"
	"testing"
)

// The renderer replaced five inlined format strings. These goldens are those
// strings, copied verbatim before the swap: the changelog is an on-disk format
// read by every clone, and fileSink dedupes on exact line equality, so a
// whitespace difference is a real compatibility break, not a cosmetic one.
func TestRender_MatchesTheEmittedFormatExactly(t *testing.T) {
	tests := []struct {
		table  string
		values []string
		want   string
	}{
		{
			table: "tickets",
			values: []string{
				"'REC1'", "'task'", "NULL", "'title'", "NULL",
				"'backlog'", "'medium'", "NULL", "NULL", "NULL",
				"'2026-01-01 00:00:00'", "'2026-01-01 00:00:00'", "NULL", "'abc'", "1",
			},
			want: `INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('REC1', 'task', NULL, 'title', NULL, 'backlog', 'medium', NULL, NULL, NULL, '2026-01-01 00:00:00', '2026-01-01 00:00:00', NULL, 'abc', 1) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;`,
		},
		{
			table: "ticket_idmap",
			values: []string{
				"'REC1'", "'T-1'",
				"'2026-01-01 00:00:00'", "'2026-01-01 00:00:00'", "NULL", "'abc'", "1",
			},
			want: `INSERT INTO ticket_idmap (record_id, ticket_id, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('REC1', 'T-1', '2026-01-01 00:00:00', '2026-01-01 00:00:00', NULL, 'abc', 1) ON CONFLICT(record_id) DO UPDATE SET ticket_id=excluded.ticket_id, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= ticket_idmap.updated_at;`,
		},
		{
			table: "ticket_deps",
			values: []string{
				"'REC1'", "'REC2'",
				"'2026-01-01 00:00:00'", "'2026-01-01 00:00:00'", "NULL", "'abc'", "1",
			},
			want: `INSERT INTO ticket_deps (blocker_record_id, blocked_record_id, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('REC1', 'REC2', '2026-01-01 00:00:00', '2026-01-01 00:00:00', NULL, 'abc', 1) ON CONFLICT(blocker_record_id, blocked_record_id) DO UPDATE SET updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= ticket_deps.updated_at;`,
		},
		{
			table: "ticket_labels",
			values: []string{
				"'REC1'", "'urgent'",
				"'2026-01-01 00:00:00'", "'2026-01-01 00:00:00'", "NULL", "'abc'", "1",
			},
			want: `INSERT INTO ticket_labels (ticket_record_id, label, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('REC1', 'urgent', '2026-01-01 00:00:00', '2026-01-01 00:00:00', NULL, 'abc', 1) ON CONFLICT(ticket_record_id, label) DO UPDATE SET updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= ticket_labels.updated_at;`,
		},
		{
			table: "ticket_history",
			values: []string{
				"'REC1'", "'status'", "'backlog'", "'done'", "NULL",
				"'2026-01-01 00:00:00'", "'2026-01-01 00:00:00'", "'2026-01-01 00:00:00'", "NULL",
				"'abc'", "1",
			},
			want: `INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('REC1', 'status', 'backlog', 'done', NULL, '2026-01-01 00:00:00', '2026-01-01 00:00:00', '2026-01-01 00:00:00', NULL, 'abc', 1) ON CONFLICT(hash) DO NOTHING;`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.table, func(t *testing.T) {
			spec, ok := specFor(tc.table)
			if !ok {
				t.Fatalf("no spec for %s", tc.table)
			}
			got, err := spec.render(tc.values)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if got != tc.want {
				t.Errorf("rendered line differs from the on-disk format.\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

func TestRender_RejectsValueCountMismatch(t *testing.T) {
	spec, _ := specFor("tickets")
	if _, err := spec.render([]string{"'only-one'"}); err == nil {
		t.Error("expected an error when values do not line up with columns")
	}
}

// created_at must never appear in the update set: a later write updates a row,
// it does not move its birth.
func TestUpdateColumns_ExcludeKeyAndCreatedAt(t *testing.T) {
	for _, spec := range changelogTables {
		if spec.AppendOnly {
			continue
		}
		for _, c := range spec.updateColumns() {
			if c == "created_at" {
				t.Errorf("%s: created_at is in the update set", spec.Name)
			}
			for _, k := range spec.Key {
				if c == k {
					t.Errorf("%s: conflict key %s is in the update set", spec.Name, k)
				}
			}
		}
	}
}

// Every replicated table needs a spec, and the rebuild path's table list is the
// other half of the same closed set — a table in one and not the other would
// replicate without upgrading, or upgrade without replicating.
func TestSpecs_CoverEveryReplicatedTable(t *testing.T) {
	for _, name := range replicatedTables {
		if _, ok := specFor(name); !ok {
			t.Errorf("replicated table %s has no changelog spec", name)
		}
	}
	for _, spec := range changelogTables {
		found := false
		for _, name := range replicatedTables {
			if name == spec.Name {
				found = true
			}
		}
		if !found {
			t.Errorf("spec %s is not in replicatedTables", spec.Name)
		}
	}
}

// The guard is the whole point of format 2; a spec that lost it would silently
// re-introduce the hash tiebreaker on re-emit.
func TestRender_CarriesThePositionGuard(t *testing.T) {
	for _, spec := range changelogTables {
		values := make([]string, len(spec.Columns))
		for i := range values {
			values[i] = "NULL"
		}
		got, err := spec.render(values)
		if err != nil {
			t.Fatalf("%s: render: %v", spec.Name, err)
		}
		if spec.AppendOnly {
			if !strings.HasSuffix(got, "DO NOTHING;") {
				t.Errorf("%s: append-only table should end in DO NOTHING", spec.Name)
			}
			continue
		}
		if !strings.Contains(got, "WHERE excluded.updated_at >= "+spec.Name+".updated_at;") {
			t.Errorf("%s: missing the position guard: %s", spec.Name, got)
		}
		if strings.Contains(got, "excluded.hash >") {
			t.Errorf("%s: still carries the retired hash tiebreaker", spec.Name)
		}
	}
}

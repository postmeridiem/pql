package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TicketCollision reports a single ticket id claimed by more than one
// lineage in the changelog. Ticket ids are allocated as max(existing)+1
// against the *local* changelog (T-54), so two clones/branches/sessions
// that file between syncs can pick the same T-NNN. That's accepted — but
// `tickets` replays with ON CONFLICT(id) DO UPDATE, which would silently
// merge the two unrelated tickets into one row. created_at is immutable
// across a real ticket's update rows, so two distinct created_at values
// for one id mean two distinct lineages collided.
type TicketCollision struct {
	ID       string          `json:"id"`
	Lineages []TicketLineage `json:"lineages"`
}

// TicketLineage identifies one of the colliding tickets by its immutable
// created_at, its most recent title, and its row hash.
type TicketLineage struct {
	CreatedAt string `json:"created_at"`
	Title     string `json:"title"`
	Hash      string `json:"hash"`
}

// ticketInsertPrefix is the exact column-list-and-VALUES preamble the
// exporter writes for every ticket row (see exportTickets). Matching it
// verbatim both filters non-ticket statements and pins where the VALUES
// tuple begins. If the exporter's column order changes, the round-trip
// test (TestDetectCollisions_*) fails loudly so this stays in sync.
const ticketInsertPrefix = `INSERT INTO tickets (id, type, parent_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES (`

// Field positions within the VALUES tuple, matching ticketInsertPrefix.
const (
	fieldID        = 0
	fieldTitle     = 3
	fieldCreatedAt = 10
	fieldHash      = 13
	fieldCount     = 15
)

// detectTicketCollisions scans every tickets/*.sql changelog file and
// returns the ids claimed by more than one lineage. It is read-only and
// independent of the replay cutoff, so both `pql plan import` and
// `pql plan rebuild` see the full picture. A missing tickets dir (fresh
// vault) is not an error — it returns no collisions.
func detectTicketCollisions(root string) ([]TicketCollision, error) {
	dir := filepath.Join(root, "tickets")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("changelog: read %s: %w", dir, err)
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	// id -> created_at -> lineage. order preserves first-seen created_at
	// per id so the reported lineages are stable.
	type idState struct {
		order     []string
		byCreated map[string]TicketLineage
	}
	states := map[string]*idState{}

	for _, name := range files {
		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path) //nolint:gosec // G304: path is from a directory we just listed
		if err != nil {
			return nil, fmt.Errorf("changelog: read %s: %w", path, err)
		}
		for _, stmt := range splitStatements(string(content)) {
			lin, id, ok := parseTicketLineage(stmt)
			if !ok {
				continue
			}
			st := states[id]
			if st == nil {
				st = &idState{byCreated: map[string]TicketLineage{}}
				states[id] = st
			}
			if _, seen := st.byCreated[lin.CreatedAt]; !seen {
				st.order = append(st.order, lin.CreatedAt)
			}
			st.byCreated[lin.CreatedAt] = lin // last write wins → most recent title/hash
		}
	}

	var out []TicketCollision
	for id, st := range states {
		if len(st.order) < 2 {
			continue
		}
		lineages := make([]TicketLineage, 0, len(st.order))
		for _, c := range st.order {
			lineages = append(lineages, st.byCreated[c])
		}
		out = append(out, TicketCollision{ID: id, Lineages: lineages})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// parseTicketLineage extracts (created_at, title, hash) and the id from a
// single ticket INSERT statement. ok is false for any statement that is
// not a ticket insert (schema files, other tables) or that doesn't parse.
func parseTicketLineage(stmt string) (lin TicketLineage, id string, ok bool) {
	stmt = strings.TrimSpace(stmt)
	if !strings.HasPrefix(stmt, ticketInsertPrefix) {
		return TicketLineage{}, "", false
	}
	fields, ok := splitValuesTuple(stmt[len(ticketInsertPrefix):])
	if !ok || len(fields) < fieldCount {
		return TicketLineage{}, "", false
	}
	id = unquoteSQL(fields[fieldID])
	if id == "" {
		return TicketLineage{}, "", false
	}
	return TicketLineage{
		CreatedAt: unquoteSQL(fields[fieldCreatedAt]),
		Title:     unquoteSQL(fields[fieldTitle]),
		Hash:      unquoteSQL(fields[fieldHash]),
	}, id, true
}

// splitValuesTuple splits the inside of a VALUES (...) tuple into its
// top-level fields, given the text immediately after the opening paren.
// It is quote- and paren-aware: commas, parens and the marker text inside
// a single-quoted literal (e.g. a description) are not field separators.
// ok is false if the tuple's closing paren is never reached.
func splitValuesTuple(s string) (fields []string, ok bool) {
	var b strings.Builder
	depth := 1 // the opening '(' has already been consumed by the caller
	inStr := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			if c == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					b.WriteByte(c)
					b.WriteByte(s[i+1])
					i++
					continue
				}
				inStr = false
			}
			b.WriteByte(c)
			continue
		}
		switch c {
		case '\'':
			inStr = true
			b.WriteByte(c)
		case '(':
			depth++
			b.WriteByte(c)
		case ')':
			depth--
			if depth == 0 {
				fields = append(fields, strings.TrimSpace(b.String()))
				return fields, true
			}
			b.WriteByte(c)
		case ',':
			if depth == 1 {
				fields = append(fields, strings.TrimSpace(b.String()))
				b.Reset()
			} else {
				b.WriteByte(c)
			}
		default:
			b.WriteByte(c)
		}
	}
	return fields, false
}

// unquoteSQL turns a serialised SQL field back into its Go string value:
// NULL → "", a single-quoted literal has its quotes stripped and ''
// un-escaped, anything else (a number) is returned as-is.
func unquoteSQL(field string) string {
	field = strings.TrimSpace(field)
	if field == sqlNull {
		return ""
	}
	if len(field) >= 2 && field[0] == '\'' && field[len(field)-1] == '\'' {
		return strings.ReplaceAll(field[1:len(field)-1], "''", "'")
	}
	return field
}

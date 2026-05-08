// Package changelog handles per-table monthly SQL files used for
// changelog-based replication of pql planning state. See D-15..D-18.
package changelog

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// sqlStr renders a string as a single-quoted SQL literal, doubling
// any embedded single quotes.
func sqlStr(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// sqlNullStr renders a sql.NullString as either a quoted string or
// the bare NULL keyword.
func sqlNullStr(n sql.NullString) string {
	if !n.Valid {
		return "NULL"
	}
	return sqlStr(n.String)
}

// sqlNullInt renders a sql.NullInt64 as decimal or NULL.
func sqlNullInt(n sql.NullInt64) string {
	if !n.Valid {
		return "NULL"
	}
	return strconv.FormatInt(n.Int64, 10)
}

// monthOf returns the YYYY-MM bucket for a SQLite-style timestamp
// ("2006-01-02 15:04:05") by slicing the leading 7 characters. The
// timestamp format is fixed by the schema's DEFAULT (datetime('now'))
// and by every write path that calls datetime('now') explicitly, so a
// substring is sufficient — no time.Parse round-trip needed.
func monthOf(updatedAt string) (string, error) {
	if len(updatedAt) < 7 || updatedAt[4] != '-' {
		return "", fmt.Errorf("changelog: cannot bucket %q (expected YYYY-MM-DD…)", updatedAt)
	}
	return updatedAt[:7], nil
}

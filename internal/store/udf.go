package store

import (
	"database/sql/driver"
	"fmt"

	"modernc.org/sqlite"
)

// SQLite (the embedded one we get via modernc) ships a small core of
// scalar functions; PQL's compiled SQL needs a couple more to keep
// derived columns expressible without leaving SQL. Each register-here is
// a deterministic, side-effect-free pure function so the planner can
// fold and cache.

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("reverse", 1, reverseFn)
}

// reverseFn returns its single string argument with characters reversed.
// Pure-Go [] rune flip — handles multi-byte UTF-8 correctly. Used by the
// DSL compiler's basename/dirname derivations on files.path.
func reverseFn(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if args[0] == nil {
		return nil, nil
	}
	var s string
	switch v := args[0].(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return nil, fmt.Errorf("reverse: expected text, got %T", args[0])
	}
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes), nil
}

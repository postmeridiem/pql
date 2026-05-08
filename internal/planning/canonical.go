package planning

import (
	"crypto/md5" //nolint:gosec // G501: MD5 is used as a non-security content hash; see Hash docstring.
	"encoding/hex"
	"fmt"
	"strconv"
)

// CanonicalVersion identifies the canonicalisation rules used to compute
// a row's hash. Bump when the projection rules in this file or the
// per-table projection helpers in internal/planning/repo change. Each
// row records the version it was hashed under, so re-hashing under a new
// version can be done lazily without invalidating other rows in the
// transition window.
const CanonicalVersion = 1

// Canonical encoding sentinels. fieldSep separates field values; the
// leading 0x00 byte in nullSentinel cannot appear in a SQLite TEXT
// value, so collisions with real strings are impossible.
const (
	fieldSep     = "\x1f" // ASCII unit separator
	nullSentinel = "\x00NULL\x00"
)

// Hash returns the canonical hash of the given projected row values as
// a lowercase 32-character hex string. Uses MD5 (crypto/md5, stdlib)
// for compactness and speed — this is a content integrity / sort
// tiebreaker / LWW tiebreaker, not a security primitive.
//
// Per-table projection helpers (TicketCanonical, DecisionCanonical,
// etc., in internal/planning/repo) build the []any in fixed column
// order and prepend the canonical version so every hash inputs the
// version it was computed under.
func Hash(values []any) string {
	sum := md5.Sum(CanonicalBytes(values)) //nolint:gosec // G401: non-security content hash; collision-resistance, not cryptographic strength, is required.
	return hex.EncodeToString(sum[:])
}

// CanonicalBytes serialises a row's projected values into a
// deterministic byte sequence suitable for hashing. The caller fixes
// the column order; this function only encodes whatever it receives,
// stably.
//
// Supported value types:
//   - string, *string (nil → NULL)
//   - int, int64, *int, *int64 (nil → NULL)
//   - bool (encoded "0" or "1")
//   - nil (NULL)
//
// Any other type panics — projection helpers should never produce
// unsupported types.
func CanonicalBytes(values []any) []byte {
	var out []byte
	for i, v := range values {
		if i > 0 {
			out = append(out, fieldSep...)
		}
		out = append(out, canonicalValue(v)...)
	}
	return out
}

func canonicalValue(v any) string {
	switch x := v.(type) {
	case nil:
		return nullSentinel
	case string:
		return x
	case *string:
		if x == nil {
			return nullSentinel
		}
		return *x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case *int:
		if x == nil {
			return nullSentinel
		}
		return strconv.Itoa(*x)
	case *int64:
		if x == nil {
			return nullSentinel
		}
		return strconv.FormatInt(*x, 10)
	case bool:
		if x {
			return "1"
		}
		return "0"
	default:
		panic(fmt.Sprintf("planning: canonical: unsupported type %T", v))
	}
}

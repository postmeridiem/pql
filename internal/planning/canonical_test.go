package planning

import (
	"bytes"
	"strings"
	"testing"
)

func TestHash_RoundTripDeterminism(t *testing.T) {
	a := []any{1, "T-1", "task", (*string)(nil), "title", (*string)(nil),
		"backlog", "medium", (*string)(nil), (*string)(nil), (*string)(nil),
		"2026-05-08", "2026-05-08"}
	b := []any{1, "T-1", "task", (*string)(nil), "title", (*string)(nil),
		"backlog", "medium", (*string)(nil), (*string)(nil), (*string)(nil),
		"2026-05-08", "2026-05-08"}

	if Hash(a) != Hash(b) {
		t.Errorf("identical projections produced different hashes: %s vs %s",
			Hash(a), Hash(b))
	}
}

func TestHash_LowercaseHex32(t *testing.T) {
	h := Hash([]any{"x"})
	if len(h) != 32 {
		t.Errorf("hash length = %d, want 32 (md5 hex)", len(h))
	}
	if h != strings.ToLower(h) {
		t.Errorf("hash should be lowercase hex: %s", h)
	}
	for _, r := range h {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Errorf("non-hex char in hash: %s", h)
		}
	}
}

func TestHash_NullVsEmptyString(t *testing.T) {
	withNil := Hash([]any{1, "id", (*string)(nil)})
	emptyStr := ""
	withEmpty := Hash([]any{1, "id", &emptyStr})
	if withNil == withEmpty {
		t.Errorf("NULL and empty string produced identical hash %s", withNil)
	}
}

func TestHash_PointerVsDirectString(t *testing.T) {
	s := "value"
	direct := Hash([]any{1, "id", "value"})
	pointer := Hash([]any{1, "id", &s})
	if direct != pointer {
		t.Errorf("string and *string with same value differ: %s vs %s", direct, pointer)
	}
}

func TestHash_VersionAffectsResult(t *testing.T) {
	v1 := Hash([]any{1, "T-1", "title"})
	v2 := Hash([]any{2, "T-1", "title"})
	if v1 == v2 {
		t.Errorf("different versions produced identical hash %s", v1)
	}
}

func TestHash_FieldOrderMatters(t *testing.T) {
	ab := Hash([]any{1, "a", "b"})
	ba := Hash([]any{1, "b", "a"})
	if ab == ba {
		t.Errorf("reordered fields produced identical hash %s", ab)
	}
}

func TestHash_BoolEncoding(t *testing.T) {
	tt := Hash([]any{1, true})
	ff := Hash([]any{1, false})
	if tt == ff {
		t.Errorf("true and false hashed identically: %s", tt)
	}
	// "1" and "0" string equivalents must NOT collide with bool form,
	// because canonicalValue normalises bool to "1"/"0" without
	// distinguishing them from the corresponding strings — this is
	// intentional and documented; the test asserts the current rule.
	one := Hash([]any{1, "1"})
	if tt != one {
		t.Errorf("bool true should encode as \"1\": %s vs %s", tt, one)
	}
}

func TestHash_IntEncoding(t *testing.T) {
	a := Hash([]any{1, 42})
	b := Hash([]any{1, int64(42)})
	if a != b {
		t.Errorf("int and int64 should encode identically: %s vs %s", a, b)
	}
}

func TestCanonicalBytes_FieldSeparator(t *testing.T) {
	// Field separator must keep "ab" and "a" + "b" distinct, otherwise
	// a row {"ab", ""} and {"a", "b"} would collide.
	abEmpty := CanonicalBytes([]any{"ab", ""})
	aB := CanonicalBytes([]any{"a", "b"})
	if bytes.Equal(abEmpty, aB) {
		t.Errorf("ambiguous canonical bytes: %q == %q", abEmpty, aB)
	}
}

func TestHash_DeletedAtChangesHash(t *testing.T) {
	ts := "2026-05-08T10:00:00Z"
	live := Hash([]any{1, "T-1", "task", "title", (*string)(nil)})
	stub := Hash([]any{1, "T-1", "task", "title", &ts})
	if live == stub {
		t.Errorf("setting deleted_at didn't change hash: %s", live)
	}
}

func TestCanonicalValue_PanicsOnUnknownType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for unsupported type, got none")
		}
	}()
	_ = canonicalValue(1.5)
}

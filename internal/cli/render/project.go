package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// FieldsOf returns the JSON field names of T's struct fields in
// declaration order, following anonymous embedded structs the same way
// encoding/json does. Fields without a json tag or tagged "-" are
// skipped. Used to validate --fields input against the row type.
func FieldsOf[T any]() []string {
	var zero T
	t := reflect.TypeOf(zero)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	var names []string
	for _, f := range reflect.VisibleFields(t) {
		if f.Anonymous || !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if name == "" {
			name = f.Name
		}
		names = append(names, name)
	}
	return names
}

// Project narrows rows to the requested JSON fields, preserving the
// order the caller listed them. Each returned element is a complete
// JSON object ready for Render ([]json.RawMessage round-trips through
// the encoder, including --pretty re-indentation).
//
// A requested field that a given row omits (omitempty on a nil value)
// is skipped for that row, matching the un-projected shape. A field
// name that doesn't exist on T at all is an error — callers surface it
// as a usage error so typos don't silently return empty objects.
func Project[T any](rows []T, fields []string) ([]json.RawMessage, error) {
	valid := FieldsOf[T]()
	validSet := make(map[string]bool, len(valid))
	for _, f := range valid {
		validSet[f] = true
	}
	for _, f := range fields {
		if !validSet[f] {
			sort.Strings(valid)
			return nil, fmt.Errorf("unknown field %q (valid: %s)", f, strings.Join(valid, ", "))
		}
	}

	out := make([]json.RawMessage, 0, len(rows))
	for i, row := range rows {
		full, err := json.Marshal(row)
		if err != nil {
			return nil, fmt.Errorf("project row %d: %w", i, err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(full, &m); err != nil {
			return nil, fmt.Errorf("project row %d: %w", i, err)
		}
		var buf bytes.Buffer
		buf.WriteByte('{')
		first := true
		for _, f := range fields {
			v, ok := m[f]
			if !ok {
				continue
			}
			if !first {
				buf.WriteByte(',')
			}
			first = false
			k, _ := json.Marshal(f)
			buf.Write(k)
			buf.WriteByte(':')
			buf.Write(v)
		}
		buf.WriteByte('}')
		out = append(out, json.RawMessage(buf.Bytes()))
	}
	return out, nil
}

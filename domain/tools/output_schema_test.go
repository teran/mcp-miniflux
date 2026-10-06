package tools

import (
	"testing"
)

// CONTRACT — the generic output schema must be PERMISSIVE (S09). Every tool
// declares a non-nil output schema, but the output is REFINED by the
// application layer, so the schema itself must accept any object that the
// application layer produces (Miniflux-shaped structs with arbitrary fields,
// nested objects, arrays). It must NOT be a strict
// `additionalProperties:false` empty-object schema, otherwise any structured
// tool output fails validation with
// "data must NOT have additional properties" (live bug).
//
// These tests are the regression for that bug: they FAIL against the current
// code (where outputSchema() hardcodes additionalProperties:false) and must
// pass once the generic output schema becomes permissive.

// TestOutputSchemaPermissive validates a Miniflux-shaped structured object
// against each generic-output tool's OutputSchema and asserts it is ACCEPTED.
func TestOutputSchemaPermissive(t *testing.T) {
	// A representative mix of generic-output tools: read, write/update, delete.
	generic := []Handler{
		&GetMe{},
		&ListCategories{},
		&GetCounters{},
		&GetFeed{},
		&FlushHistory{},
		&CreateCategory{},
	}

	// Arbitrary Miniflux-shaped output: flat scalar fields, a nested object,
	// and an array. A permissive schema must accept all of these.
	output := map[string]any{
		"id":         1,
		"title":      "x",
		"feed_count": 3,
		"user":       map[string]any{"id": 1, "username": "t"},
		"entries":    []any{map[string]any{"id": 9, "title": "e"}},
	}

	for _, h := range generic {
		t.Run(h.Name(), func(t *testing.T) {
			schema := h.OutputSchema()
			if schema == nil {
				t.Fatal("OutputSchema() must not be nil (S09)")
			}
			if err := resolveSchema(t, schema).Validate(output); err != nil {
				t.Errorf("%s: permissive output schema must accept structured output, got error: %v", h.Name(), err)
			}
		})
	}
}

// TestOutputSchemaPermissiveEmptyObject asserts that even a bare object
// (application layer may emit an empty struct result) validates fine.
func TestOutputSchemaPermissiveEmptyObject(t *testing.T) {
	for _, h := range []Handler{&GetMe{}, &FlushHistory{}} {
		t.Run(h.Name(), func(t *testing.T) {
			if err := resolveSchema(t, h.OutputSchema()).Validate(map[string]any{}); err != nil {
				t.Errorf("%s: permissive output schema must accept an empty object, got error: %v", h.Name(), err)
			}
		})
	}
}

// TestGenericOutputSchemaIsNotStrict asserts the CONTRACT that the generic
// output schema does NOT set additionalProperties:false. Only the typed
// discover_subscriptions output schema may be strict (covered separately by
// TestDiscoverOutputSchemaTyped).
func TestGenericOutputSchemaIsNotStrict(t *testing.T) {
	generic := []Handler{
		&GetMe{}, &ListCategories{}, &GetCounters{}, &GetFeed{},
		&FlushHistory{}, &CreateCategory{}, &ListEntries{}, &GetEntry{},
	}

	for _, h := range generic {
		t.Run(h.Name(), func(t *testing.T) {
			m := *(h.OutputSchema())
			if v, ok := m["additionalProperties"]; ok && v == false {
				t.Errorf("%s: generic output schema must be permissive, but additionalProperties=false", h.Name())
			}
		})
	}
}

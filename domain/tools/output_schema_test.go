package tools

import (
	"testing"
)

// CONTRACT — every tool must declare an explicit, strictly-typed output schema
// (M07/S09). The previous permissive `{"type":"object"}` schema (no
// `properties`, `additionalProperties` defaulted to true) is the ROOT CAUSE of
// a live MCP-client conformance bug: clients that validate a tool's
// StructuredContent against its declared outputSchema reject the result with
// "data must NOT have additional properties" (opencode). The correct, spec-
// compliant fix (M07) is that every generic tool's OutputSchema() declares a
// non-empty `properties` map that exactly matches the structured content its
// handler produces.
//
// These tests are the regression for that bug: they FAIL against the current
// code (where every generic tool returns the bare permissive `{"type":"object"}`
// with no `properties`) and must pass once every tool declares a typed output
// schema.

// genericOutputTools lists every tool except discover_subscriptions, which
// already declares a correct typed output schema (covered separately by
// TestDiscoverOutputSchemaTyped in contract_test.go and TestOutputSchemaTyped).
var genericOutputTools = []Handler{
	&ListFeeds{},
	&GetFeed{},
	&ListCategories{},
	&ListEntries{},
	&GetEntry{},
	&GetFeedEntries{},
	&GetCounters{},
	&GetMe{},
	&ExportOPML{},
	&CreateFeed{},
	&UpdateFeed{},
	&RefreshFeed{},
	&CreateCategory{},
	&UpdateCategory{},
	&RefreshCategory{},
	&MarkFeedEntriesRead{},
	&MarkCategoryEntriesRead{},
	&UpdateEntries{},
	&ToggleEntryBookmark{},
	&UpdateEntry{},
	&ImportOPML{},
	&DeleteFeed{},
	&DeleteCategory{},
	&FlushHistory{},
}

// TestGenericOutputSchemaDeclaresTypedProperties asserts the core contract:
// the generic output schema must NOT be the bare permissive {"type":"object"}
// and MUST declare a non-empty `properties` map. This fails against the
// current code (all generic tools return {"type":"object"} with no
// properties) and passes once every tool declares a typed output schema.
func TestGenericOutputSchemaDeclaresTypedProperties(t *testing.T) {
	for _, h := range genericOutputTools {
		t.Run(h.Name(), func(t *testing.T) {
			schema := h.OutputSchema()
			if schema == nil {
				t.Fatal("OutputSchema() must not be nil (S09)")
			}
			m := *schema
			props, ok := m["properties"].(map[string]any)
			if !ok {
				t.Fatalf("%s: output schema must declare a properties map (M07/S09), got: %v", h.Name(), m)
			}
			if len(props) == 0 {
				t.Fatalf("%s: output schema must declare a non-empty properties map (M07/S09), got: %v", h.Name(), m)
			}
		})
	}
}

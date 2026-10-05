package tools

import (
	"testing"
)

// CONTRACT — package `tools` (SPEC §4, §6.2; S02/S03/S08/S09/S12).
//
// The developer must define a `Handler` interface and one concrete struct per
// tool below. This test file is written FIRST (TDD, red state) and will not
// compile until they exist. Exact contract (definition-only; execution is wired
// in the application layer via the dmf.Client port):
//
//	type Handler interface {
//		Name() string
//		InputSchema() *map[string]any
//		OutputSchema() *map[string]any
//		Annotations() mcp.ToolAnnotations
//		Instructions() string
//	}
//
// The concrete tool structs (zero value must expose all definition methods —
// Name/InputSchema/OutputSchema/Annotations/Instructions — without any
// injected dependency, so they are safe to instantiate in tests):
//
//	Read (S03 first, readOnlyHint:true):
//		*ListFeeds, *GetFeed, *ListCategories, *ListEntries, *GetEntry,
//		*GetFeedEntries, *GetCounters, *GetMe, *ExportOPML, *DiscoverSubscriptions
//	Write/update (readOnlyHint:false):
//		*CreateFeed, *UpdateFeed, *RefreshFeed, *CreateCategory, *UpdateCategory,
//		*RefreshCategory, *MarkFeedEntriesRead, *MarkCategoryEntriesRead,
//		*UpdateEntries, *ToggleEntryBookmark, *UpdateEntry, *ImportOPML
//	Delete (destructiveHint:true, HITL S12):
//		*DeleteFeed, *DeleteCategory, *FlushHistory
//
// Note: SPEC §4 text lists 25 tools (10 read + 12 write + 3 delete). The
// enumeration below is authoritative and used for both registration order
// (S03) and per-tool assertions.

// expectedTool names in exact registration order (read → write/update → delete).
var expectedToolOrder = []string{
	// Read
	"list_feeds", "get_feed", "list_categories", "list_entries", "get_entry",
	"get_feed_entries", "get_counters", "get_me", "export_opml",
	"discover_subscriptions",
	// Write/update
	"create_feed", "update_feed", "refresh_feed", "create_category",
	"update_category", "refresh_category", "mark_feed_entries_read",
	"mark_category_entries_read", "update_entries", "toggle_entry_bookmark",
	"update_entry", "import_opml",
	// Delete
	"delete_feed", "delete_category", "flush_history",
}

// annotationExpectation captures the SPEC §4 annotation contract per tool.
type annotationExpectation struct {
	readOnly      bool
	idempotent    bool
	openWorld     bool
	openWorldOn   bool // whether OpenWorldHint should be explicitly true
	destructive   bool
	destructiveOn bool
}

var expectedAnnotations = map[string]annotationExpectation{
	// Read: readOnly:true, openWorld:false, idempotent:false, destructive:false
	"list_feeds":             {readOnly: true},
	"get_feed":               {readOnly: true},
	"list_categories":        {readOnly: true},
	"list_entries":           {readOnly: true},
	"get_entry":              {readOnly: true},
	"get_feed_entries":       {readOnly: true},
	"get_counters":           {readOnly: true},
	"get_me":                 {readOnly: true},
	"export_opml":            {readOnly: true},
	"discover_subscriptions": {readOnly: true, openWorld: true, openWorldOn: true},
	// Write/update: readOnly:false, destructive:false
	"create_feed":                {idempotent: true},
	"update_feed":                {idempotent: true},
	"refresh_feed":               {idempotent: true},
	"create_category":            {idempotent: true},
	"update_category":            {idempotent: true},
	"refresh_category":           {idempotent: true},
	"mark_feed_entries_read":     {idempotent: true},
	"mark_category_entries_read": {idempotent: true},
	"update_entries":             {idempotent: true},
	"toggle_entry_bookmark":      {idempotent: false}, // toggles — NOT idempotent (X02/N25)
	"update_entry":               {idempotent: true},
	"import_opml":                {idempotent: true},
	// Delete: readOnly:false, destructive:true, idempotent:true (S12/HITL)
	"delete_feed":     {idempotent: true, destructive: true, destructiveOn: true},
	"delete_category": {idempotent: true, destructive: true, destructiveOn: true},
	"flush_history":   {idempotent: true, destructive: true, destructiveOn: true},
}

// allTools returns one instance of every tool in registration order (S03).
// The slice is typed as Handler, so the compiler enforces that every struct
// implements the full Handler interface.
func allTools() []Handler {
	return []Handler{
		&ListFeeds{}, &GetFeed{}, &ListCategories{}, &ListEntries{}, &GetEntry{},
		&GetFeedEntries{}, &GetCounters{}, &GetMe{}, &ExportOPML{},
		&DiscoverSubscriptions{},
		&CreateFeed{}, &UpdateFeed{}, &RefreshFeed{}, &CreateCategory{},
		&UpdateCategory{}, &RefreshCategory{}, &MarkFeedEntriesRead{},
		&MarkCategoryEntriesRead{}, &UpdateEntries{}, &ToggleEntryBookmark{},
		&UpdateEntry{}, &ImportOPML{},
		&DeleteFeed{}, &DeleteCategory{}, &FlushHistory{},
	}
}

// TestToolSetIsCompleteAndOrdered verifies the exact tool surface (SPEC §4)
// and that registration order is read → write/update → delete (S03).
func TestToolSetIsCompleteAndOrdered(t *testing.T) {
	tools := allTools()
	if len(tools) != len(expectedToolOrder) {
		t.Fatalf("tool surface has %d tools, want %d", len(tools), len(expectedToolOrder))
	}
	for i, h := range tools {
		if got := h.Name(); got != expectedToolOrder[i] {
			t.Errorf("tool[%d].Name() = %q, want %q (registration order must be read → write/update → delete, S03)", i, got, expectedToolOrder[i])
		}
	}
}

// TestNoDuplicateToolNames guards against two tools exposing the same name.
func TestNoDuplicateToolNames(t *testing.T) {
	seen := map[string]bool{}
	for _, h := range allTools() {
		if seen[h.Name()] {
			t.Errorf("duplicate tool name %q", h.Name())
		}
		seen[h.Name()] = true
	}
}

func boolPtr(b bool) *bool { return &b }

// derefBool returns the pointer's value, or fallback if the pointer is nil.
func derefBool(p *bool, fallback bool) bool {
	if p == nil {
		return fallback
	}
	return *p
}

// TestAnnotationsMatchSpec asserts the full annotation contract per tool
// (M04): readOnly / idempotent / openWorld / destructive hints and a title.
func TestAnnotationsMatchSpec(t *testing.T) {
	for _, h := range allTools() {
		name := h.Name()
		exp, ok := expectedAnnotations[name]
		if !ok {
			t.Errorf("tool %q has no expected-annotation entry — update expectedAnnotations", name)
			continue
		}
		a := h.Annotations()

		if a.ReadOnlyHint != exp.readOnly {
			t.Errorf("%s: readOnlyHint = %v, want %v", name, a.ReadOnlyHint, exp.readOnly)
		}
		if a.IdempotentHint != exp.idempotent {
			t.Errorf("%s: idempotentHint = %v, want %v (SPEC X02/N25)", name, a.IdempotentHint, exp.idempotent)
		}
		if got := derefBool(a.OpenWorldHint, false); got != exp.openWorld {
			t.Errorf("%s: openWorldHint = %v, want %v", name, got, exp.openWorld)
		}
		if a.OpenWorldHint != nil && *a.OpenWorldHint != exp.openWorldOn {
			t.Errorf("%s: openWorldHint explicitly set to %v, want explicit=%v", name, *a.OpenWorldHint, exp.openWorldOn)
		}
		if got := derefBool(a.DestructiveHint, false); got != exp.destructive {
			t.Errorf("%s: destructiveHint = %v, want %v (S12/HITL)", name, got, exp.destructive)
		}
		if a.Title == "" {
			t.Errorf("%s: annotations title must be non-empty (M04)", name)
		}
	}
}

// TestInstructionsNonEmpty: every tool must carry a human-readable instruction
// string for the model (M04/M05).
func TestInstructionsNonEmpty(t *testing.T) {
	for _, h := range allTools() {
		if got := h.Instructions(); got == "" {
			t.Errorf("%s: Instructions() must be non-empty", h.Name())
		}
	}
}

// TestInputSchemaObjectContract (S08): every inputSchema is a JSON object with
// additionalProperties:false and strict types.
func TestInputSchemaObjectContract(t *testing.T) {
	for _, h := range allTools() {
		s := h.InputSchema()
		if s == nil {
			t.Errorf("%s: InputSchema() must not be nil", h.Name())
			continue
		}
		m := *s
		if m["type"] != "object" {
			t.Errorf("%s: inputSchema type = %v, want \"object\"", h.Name(), m["type"])
		}
		if m["additionalProperties"] != false {
			t.Errorf("%s: inputSchema additionalProperties = %v, want false (S08)", h.Name(), m["additionalProperties"])
		}
	}
}

// TestOutputSchemaExists (S09): every tool declares an output schema.
func TestOutputSchemaExists(t *testing.T) {
	for _, h := range allTools() {
		if h.OutputSchema() == nil {
			t.Errorf("%s: OutputSchema() must not be nil (S09)", h.Name())
		}
	}
}

// TestRequiredFields asserts the SPEC §4 required-field contract on tools with
// mandatory inputs.
func TestRequiredFields(t *testing.T) {
	require := map[string][]string{
		"get_feed":                   {"feed_id"},
		"get_feed_entries":           {"feed_id"},
		"discover_subscriptions":     {"url"},
		"create_feed":                {"feed_url"},
		"update_feed":                {"feed_id"},
		"refresh_feed":               {"feed_id"},
		"create_category":            {"title"},
		"update_category":            {"category_id"},
		"refresh_category":           {"category_id"},
		"mark_feed_entries_read":     {"feed_id"},
		"mark_category_entries_read": {"category_id"},
		"update_entries":             {"entry_ids"},
		"toggle_entry_bookmark":      {"entry_id"},
		"update_entry":               {"entry_id"},
		"delete_feed":                {"feed_id"},
		"delete_category":            {"category_id"},
	}

	for _, h := range allTools() {
		req, ok := require[h.Name()]
		if !ok {
			continue
		}
		m := *(h.InputSchema())
		got := toStringSet(m["required"])
		for _, field := range req {
			if !got[field] {
				t.Errorf("%s: inputSchema missing required field %q (required=%v)", h.Name(), field, m["required"])
			}
		}
	}
}

// TestListEntriesLimits asserts the strict limit cap from SPEC §4.1
// (limit default 100, max 1000).
func TestListEntriesLimits(t *testing.T) {
	h := &ListEntries{}
	m := *(h.InputSchema())
	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatalf("list_entries: inputSchema properties not an object: %v", m["properties"])
	}
	lim, ok := props["limit"].(map[string]any)
	if !ok {
		t.Fatalf("list_entries: properties.limit missing: %v", props["limit"])
	}
	if max, ok := lim["maximum"].(float64); ok && max > 1000 {
		t.Errorf("list_entries: limit maximum = %v, must be <= 1000", max)
	} else if !ok {
		t.Errorf("list_entries: properties.limit.maximum missing")
	}
	if def, ok := lim["default"].(float64); ok && def != 100 {
		t.Errorf("list_entries: limit default = %v, want 100", def)
	}
}

// TestCreateFeedHasSecretPassword asserts create_feed declares the optional
// `password` property (S02 — the secret credential for the feed). Secret
// redaction itself is enforced at the struct layer (domain/miniflux); here we
// lock that the credential is a first-class input.
func TestCreateFeedHasSecretPassword(t *testing.T) {
	m := *(NewCreateFeedForTest().InputSchema())
	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatalf("create_feed: inputSchema properties not an object")
	}
	if _, ok := props["password"]; !ok {
		t.Errorf("create_feed: inputSchema must declare a password property (S02)")
	}
	if _, ok := props["feed_url"]; !ok {
		t.Errorf("create_feed: inputSchema must declare feed_url")
	}
}

// convenience constructors for the few tools referenced by name in tests.
// These delegate to the zero-value struct so the developer only needs to
// implement the Handler interface on the zero value.
func NewCreateFeedForTest() Handler { return &CreateFeed{} }

func toStringSet(v any) map[string]bool {
	out := map[string]bool{}
	switch arr := v.(type) {
	case []string:
		for _, s := range arr {
			out[s] = true
		}
	case []any:
		for _, item := range arr {
			if s, ok := item.(string); ok {
				out[s] = true
			}
		}
	}
	return out
}

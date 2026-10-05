package tools

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

// CONTRACT — this file validates tool input against each tool's inputSchema
// exactly the way the go-sdk v1.8.0 does internally (see mcp/server.go
// `applySchema` → `jsonschema.Schema.Resolve` + `Resolved.Validate`). It
// proves that invalid arguments (missing required, extra unknown fields with
// additionalProperties:false, out-of-range values) are rejected — the S08
// InvalidParams contract — and that valid arguments pass.

// resolveSchema turns a *map[string]any schema into a resolved validator.
func resolveSchema(t *testing.T, schema *map[string]any) *jsonschema.Resolved {
	t.Helper()
	if schema == nil {
		t.Fatal("schema is nil")
	}
	b, err := json.Marshal(*schema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("unmarshal into jsonschema.Schema: %v", err)
	}
	resolved, err := s.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		t.Fatalf("resolve schema: %v", err)
	}
	return resolved
}

func validateArgs(t *testing.T, h Handler, args map[string]any) error {
	t.Helper()
	return resolveSchema(t, h.InputSchema()).Validate(args)
}

func TestValidationRejectsInvalidArgs(t *testing.T) {
	cases := []struct {
		name string
		h    Handler
		args map[string]any
	}{
		{
			name: "get_feed missing required feed_id",
			h:    &GetFeed{},
			args: map[string]any{},
		},
		{
			name: "get_feed extra unknown field (additionalProperties:false)",
			h:    &GetFeed{},
			args: map[string]any{"feed_id": 1, "bogus": 2},
		},
		{
			name: "get_feed wrong type (string for int)",
			h:    &GetFeed{},
			args: map[string]any{"feed_id": "not-an-int"},
		},
		{
			name: "create_feed missing required feed_url",
			h:    &CreateFeed{},
			args: map[string]any{"title": "x"},
		},
		{
			name: "create_feed extra unknown field",
			h:    &CreateFeed{},
			args: map[string]any{"feed_url": "https://x", "bogus": 1},
		},
		{
			name: "delete_feed missing required feed_id",
			h:    &DeleteFeed{},
			args: map[string]any{},
		},
		{
			name: "discover_subscriptions missing required url",
			h:    &DiscoverSubscriptions{},
			args: map[string]any{},
		},
		{
			name: "update_entries missing required entry_ids",
			h:    &UpdateEntries{},
			args: map[string]any{"status": "read"},
		},
		{
			name: "toggle_entry_bookmark missing required entry_id",
			h:    &ToggleEntryBookmark{},
			args: map[string]any{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateArgs(t, tc.h, tc.args); err == nil {
				t.Errorf("expected InvalidParams for %s, got nil error", tc.name)
			}
		})
	}
}

func TestValidationAcceptsValidArgs(t *testing.T) {
	cases := []struct {
		name string
		h    Handler
		args map[string]any
	}{
		{name: "get_feed", h: &GetFeed{}, args: map[string]any{"feed_id": 42}},
		{name: "create_feed", h: &CreateFeed{}, args: map[string]any{"feed_url": "https://example.com/feed.xml"}},
		{name: "create_feed with secret password", h: &CreateFeed{}, args: map[string]any{"feed_url": "https://example.com/feed.xml", "username": "u", "password": "p"}},
		{name: "list_entries defaults", h: &ListEntries{}, args: map[string]any{}},
		{name: "list_entries filters", h: &ListEntries{}, args: map[string]any{"status": "unread", "limit": 100, "starred": false, "category_id": 3}},
		{name: "delete_feed", h: &DeleteFeed{}, args: map[string]any{"feed_id": 1, "confirm": true}},
		{name: "flush_history", h: &FlushHistory{}, args: map[string]any{"confirm": true}},
		{name: "discover_subscriptions", h: &DiscoverSubscriptions{}, args: map[string]any{"url": "https://example.com"}},
		{name: "update_entries", h: &UpdateEntries{}, args: map[string]any{"entry_ids": []int{1, 2}, "status": "read"}},
		{name: "toggle_entry_bookmark", h: &ToggleEntryBookmark{}, args: map[string]any{"entry_id": 7}},
		{name: "no-input read tool", h: &GetMe{}, args: map[string]any{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateArgs(t, tc.h, tc.args); err != nil {
				t.Errorf("expected valid args for %s, got error: %v", tc.name, err)
			}
		})
	}
}

// TestValidationRejectsOutOfRangeLimit proves the strict limit cap (SPEC §4.1:
// limit max 1000) is enforced by the schema, not just documented.
func TestValidationRejectsOutOfRangeLimit(t *testing.T) {
	if err := validateArgs(t, &ListEntries{}, map[string]any{"limit": 2000}); err == nil {
		t.Error("expected InvalidParams for list_entries limit=2000 (> max 1000)")
	}
	// At the boundary it must pass.
	if err := validateArgs(t, &ListEntries{}, map[string]any{"limit": 1000}); err != nil {
		t.Errorf("expected valid for limit=1000 (boundary), got: %v", err)
	}
}

// TestNoInputToolsRejectAnyArgument: tools with no inputs (get_me, get_counters,
// list_categories, export_opml, flush_history) must reject any supplied
// argument because additionalProperties:false.
func TestNoInputToolsRejectAnyArgument(t *testing.T) {
	noInput := []Handler{
		&GetMe{}, &GetCounters{}, &ListCategories{}, &ExportOPML{},
	}
	for _, h := range noInput {
		if err := validateArgs(t, h, map[string]any{"foo": "bar"}); err == nil {
			t.Errorf("%s: expected InvalidParams for unexpected argument", h.Name())
		}
		// But empty args must be accepted (the common case).
		if err := validateArgs(t, h, map[string]any{}); err != nil {
			t.Errorf("%s: empty args must be valid, got: %v", h.Name(), err)
		}
	}
}

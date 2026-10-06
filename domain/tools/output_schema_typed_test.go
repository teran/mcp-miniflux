package tools

import (
	"encoding/json"
	"testing"
)

// CONTRACT — every tool's OutputSchema() must ACCEPT the exact structured
// content its handler produces (M07/S09). This is validated with the SAME
// path the go-sdk v1.8.0 uses for tool output: the handler's value is
// marshalled to JSON, unmarshalled into an `any`, then validated against the
// tool's resolved output schema (mcp/tool.go applySchema). It proves each
// declared schema is complete AND correctly typed for the handler's real
// output.
//
// This guards the mutation cases the properties-presence test alone cannot:
// a schema that declares the right properties but types them wrong (e.g.
// `feeds` as a string instead of an array), or that omits a field the handler
// actually emits, will reject the representative output here.
//
// Note: jsonschema-go's Resolved.Validate refuses struct instances, so each
// representative value is reduced to its JSON object form (map[string]any)
// exactly as the go-sdk does before validation.

// asObject reduces a realistic representative value v to the map[string]any JSON
// object the go-sdk validates a tool's StructuredContent against. It round-trips
// through JSON exactly as the go-sdk does before validation. (For the plain-map
// representatives below this is a pass-through, kept to mirror that path.)
func asObject(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal representative output: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal representative output into map: %v", err)
	}
	return m
}

func TestOutputSchemaTyped(t *testing.T) {
	// Representative structured outputs, as plain map[string]any literals that
	// produce EXACTLY the JSON object each handler emits (M07/S09). These are
	// written by hand to match the handler's marshalled output without pulling
	// the domain types into this package (go-arch-lint forbids domain -> domain
	// dependencies; see .go-arch-lint.yml).

	// Feed is redacted (username/password zeroed) per S02; the struct has no
	// omitempty, so both keys still appear in the emitted JSON as empty strings
	// and the schema must declare them.
	feed := map[string]any{
		"id":          1,
		"user_id":     2,
		"feed_url":    "https://example.com/feed.xml",
		"site_url":    "https://example.com",
		"title":       "Example",
		"category":    map[string]any{"id": 3, "title": "tech"},
		"status":      "subscribed",
		"error_count": 0,
		"username":    "",
		"password":    "",
	}

	category := map[string]any{
		"id":          3,
		"title":       "tech",
		"feed_count":  5,
		"entry_count": 100,
	}

	// time.Time values marshal to RFC3339 strings.
	entry := map[string]any{
		"id":           9,
		"user_id":      2,
		"feed_id":      3,
		"status":       "unread",
		"starred":      false,
		"title":        "Post",
		"url":          "https://example.com/post",
		"comments_url": "https://example.com/post#comments",
		"published_at": "2024-01-02T03:04:05Z",
		"created_at":   "2024-01-02T03:04:05Z",
		"content":      "body",
	}

	feedEntries := map[string]any{
		"total":   1,
		"entries": []any{entry},
	}

	counters := map[string]any{
		"feeds":  map[string]any{"3": 5},
		"totals": map[string]any{"unread": 10, "read": 20},
	}

	me := map[string]any{
		"id":       1,
		"username": "reader",
		"is_admin": true,
		"theme":    "sans",
	}

	ok := map[string]any{"ok": true}

	cases := []struct {
		name   string
		h      Handler
		output any
	}{
		{name: "list_feeds", h: &ListFeeds{}, output: map[string]any{"feeds": []any{feed}}},
		{name: "get_feed", h: &GetFeed{}, output: feed},
		{name: "list_categories", h: &ListCategories{}, output: map[string]any{"categories": []any{category}}},
		{name: "list_entries", h: &ListEntries{}, output: feedEntries},
		{name: "get_entry", h: &GetEntry{}, output: entry},
		{name: "get_feed_entries", h: &GetFeedEntries{}, output: feedEntries},
		{name: "get_counters", h: &GetCounters{}, output: counters},
		{name: "get_me", h: &GetMe{}, output: me},
		{name: "export_opml", h: &ExportOPML{}, output: map[string]any{"opml": "<opml version=\"1.0\"><head/></opml>"}},
		{name: "discover_subscriptions", h: &DiscoverSubscriptions{}, output: map[string]any{"feeds": []any{map[string]any{"url": "https://a.example/rss", "title": "A", "type": "rss"}}}},
		{name: "create_feed", h: &CreateFeed{}, output: feed},
		{name: "update_feed", h: &UpdateFeed{}, output: feed},
		{name: "refresh_feed", h: &RefreshFeed{}, output: ok},
		{name: "create_category", h: &CreateCategory{}, output: category},
		{name: "update_category", h: &UpdateCategory{}, output: ok},
		{name: "refresh_category", h: &RefreshCategory{}, output: ok},
		{name: "mark_feed_entries_read", h: &MarkFeedEntriesRead{}, output: ok},
		{name: "mark_category_entries_read", h: &MarkCategoryEntriesRead{}, output: ok},
		{name: "update_entries", h: &UpdateEntries{}, output: ok},
		{name: "toggle_entry_bookmark", h: &ToggleEntryBookmark{}, output: ok},
		{name: "update_entry", h: &UpdateEntry{}, output: entry},
		{name: "import_opml", h: &ImportOPML{}, output: ok},
		{name: "delete_feed", h: &DeleteFeed{}, output: ok},
		{name: "delete_category", h: &DeleteCategory{}, output: ok},
		{name: "flush_history", h: &FlushHistory{}, output: ok},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := asObject(t, tc.output)
			if err := resolveSchema(t, tc.h.OutputSchema()).Validate(obj); err != nil {
				t.Errorf("%s: OutputSchema() must accept the handler's structured output %v, got error: %v", tc.name, obj, err)
			}
		})
	}
}

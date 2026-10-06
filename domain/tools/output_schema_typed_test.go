package tools

import (
	"encoding/json"
	"testing"
	"time"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
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

// asObject reduces a realistic domain value v to the map[string]any JSON
// object the go-sdk validates a tool's StructuredContent against.
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
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	// Realistic domain values matching exactly what each handler returns.
	// Feed is redacted (username/password zeroed) per S02; both keys are still
	// present in the emitted JSON as empty strings, so the schema must declare
	// them.
	feed := dmf.Feed{
		ID:         1,
		UserID:     2,
		FeedURL:    "https://example.com/feed.xml",
		SiteURL:    "https://example.com",
		Title:      "Example",
		Category:   dmf.CategoryRef{ID: 3, Title: "tech"},
		Status:     "subscribed",
		ErrorCount: 0,
		// Username/Password redacted (S02).
	}

	category := dmf.Category{ID: 3, Title: "tech", FeedCount: 5, EntryCount: 100}

	entry := dmf.Entry{
		ID:          9,
		UserID:      2,
		FeedID:      3,
		Status:      "unread",
		Starred:     false,
		Title:       "Post",
		URL:         "https://example.com/post",
		CommentsURL: "https://example.com/post#comments",
		PublishedAt: ts,
		CreatedAt:   ts,
		Content:     "body",
	}

	feedEntries := dmf.FeedEntries{Total: 1, Entries: []dmf.Entry{entry}}

	counters := dmf.Counters{
		Feeds:  map[string]int{"3": 5},
		Totals: dmf.CounterTotals{Unread: 10, Read: 20},
	}

	me := dmf.Me{ID: 1, Username: "reader", IsAdmin: true, Theme: "sans"}

	ok := map[string]any{"ok": true}

	cases := []struct {
		name   string
		h      Handler
		output any
	}{
		{name: "list_feeds", h: &ListFeeds{}, output: map[string]any{"feeds": []dmf.Feed{feed}}},
		{name: "get_feed", h: &GetFeed{}, output: feed},
		{name: "list_categories", h: &ListCategories{}, output: map[string]any{"categories": []dmf.Category{category}}},
		{name: "list_entries", h: &ListEntries{}, output: feedEntries},
		{name: "get_entry", h: &GetEntry{}, output: entry},
		{name: "get_feed_entries", h: &GetFeedEntries{}, output: feedEntries},
		{name: "get_counters", h: &GetCounters{}, output: counters},
		{name: "get_me", h: &GetMe{}, output: me},
		{name: "export_opml", h: &ExportOPML{}, output: map[string]any{"opml": "<opml version=\"1.0\"><head/></opml>"}},
		{name: "discover_subscriptions", h: &DiscoverSubscriptions{}, output: map[string]any{"feeds": []dmf.DiscoveryResult{{URL: "https://a.example/rss", Title: "A", Type: "rss"}}}},
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

package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// listEntriesFilterProps returns the shared entry-filter properties used by
// list_entries and get_feed_entries (SPEC §4.1).
func listEntriesFilterProps() map[string]any {
	return map[string]any{
		"status":    stringEnumProp("unread", "read", "removed"),
		"order":     stringEnumProp("id", "status", "published_at", "category_title"),
		"direction": stringEnumProp("asc", "desc"),
		"limit": map[string]any{
			"type":    "integer",
			"default": float64(entryLimitDefault),
			"maximum": float64(entryLimitMax),
		},
		"offset":  integerProp(),
		"search":  stringProp(),
		"starred": booleanProp(),
		"before":  stringProp(),
		"after":   stringProp(),
	}
}

// ListFeeds lists all feed subscriptions, optionally filtered by category.
type ListFeeds struct{}

// Name returns the MCP tool name.
func (ListFeeds) Name() string { return "list_feeds" }

// InputSchema returns the tool's input JSON schema.
func (ListFeeds) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"category_id": integerProp(),
		"limit":       map[string]any{"type": "integer"},
		"offset":      integerProp(),
	}, nil)
}

// OutputSchema returns the tool's output JSON schema.
func (ListFeeds) OutputSchema() *map[string]any {
	return listSchema("feeds", *objectSchema(feedProps(), nil))
}

// Annotations returns the tool's MCP annotations.
func (ListFeeds) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:         "List feeds",
		ReadOnlyHint:  true,
		OpenWorldHint: ptrBool(false),
	}
}

// Instructions returns the tool's usage instructions.
func (ListFeeds) Instructions() string {
	return "Returns the user's feed subscriptions. Use to enumerate feeds or to look up a feed id " +
		"before calling get_feed/update_feed/delete_feed. Filter by category_id to scope to one " +
		"category. Read-only."
}

// GetFeed returns a single feed by id.
type GetFeed struct{}

// Name returns the MCP tool name.
func (GetFeed) Name() string { return "get_feed" }

// InputSchema returns the tool's input JSON schema.
func (GetFeed) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"feed_id": integerProp()}, []string{"feed_id"})
}

// OutputSchema returns the tool's output JSON schema.
func (GetFeed) OutputSchema() *map[string]any { return feedSchema() }

// Annotations returns the tool's MCP annotations.
func (GetFeed) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:         "Get feed",
		ReadOnlyHint:  true,
		OpenWorldHint: ptrBool(false),
	}
}

// Instructions returns the tool's usage instructions.
func (GetFeed) Instructions() string {
	return "Returns one feed by id. Use the id from list_feeds. Read-only."
}

// ListCategories lists categories with per-category entry counts.
type ListCategories struct{}

// Name returns the MCP tool name.
func (ListCategories) Name() string { return "list_categories" }

// InputSchema returns the tool's input JSON schema.
func (ListCategories) InputSchema() *map[string]any {
	return objectSchema(nil, nil)
}

// OutputSchema returns the tool's output JSON schema.
func (ListCategories) OutputSchema() *map[string]any {
	return listSchema("categories", *objectSchema(categoryProps(), nil))
}

// Annotations returns the tool's MCP annotations.
func (ListCategories) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:         "List categories",
		ReadOnlyHint:  true,
		OpenWorldHint: ptrBool(false),
	}
}

// Instructions returns the tool's usage instructions.
func (ListCategories) Instructions() string {
	return "Lists categories with counts. Use category ids with mark_category_entries_read/delete_category. Read-only."
}

// ListEntries lists entries across the whole account with filters.
type ListEntries struct{}

// Name returns the MCP tool name.
func (ListEntries) Name() string { return "list_entries" }

// InputSchema returns the tool's input JSON schema.
func (ListEntries) InputSchema() *map[string]any {
	props := listEntriesFilterProps()
	props["category_id"] = integerProp()

	return objectSchema(props, nil)
}

// OutputSchema returns the tool's output JSON schema.
func (ListEntries) OutputSchema() *map[string]any { return feedEntriesSchema() }

// Annotations returns the tool's MCP annotations.
func (ListEntries) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:         "List entries",
		ReadOnlyHint:  true,
		OpenWorldHint: ptrBool(false),
	}
}

// Instructions returns the tool's usage instructions.
func (ListEntries) Instructions() string {
	return "Returns entries with the given filters. This is the main 'what is in my queue' tool. " +
		"Use status to target unread/read/removed, search for full-text, starred for bookmarks, " +
		"category_id to scope, and before/after for time windows. Read-only."
}

// GetEntry returns a single entry by id.
type GetEntry struct{}

// Name returns the MCP tool name.
func (GetEntry) Name() string { return "get_entry" }

// InputSchema returns the tool's input JSON schema.
func (GetEntry) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"entry_id": integerProp()}, []string{"entry_id"})
}

// OutputSchema returns the tool's output JSON schema.
func (GetEntry) OutputSchema() *map[string]any { return entrySchema() }

// Annotations returns the tool's MCP annotations.
func (GetEntry) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:         "Get entry",
		ReadOnlyHint:  true,
		OpenWorldHint: ptrBool(false),
	}
}

// Instructions returns the tool's usage instructions.
func (GetEntry) Instructions() string {
	return "Returns one entry by id, including its full content. Use the id from list_entries/get_feed_entries. Read-only."
}

// GetFeedEntries lists entries of a single feed with filters.
type GetFeedEntries struct{}

// Name returns the MCP tool name.
func (GetFeedEntries) Name() string { return "get_feed_entries" }

// InputSchema returns the tool's input JSON schema.
func (GetFeedEntries) InputSchema() *map[string]any {
	props := listEntriesFilterProps()
	props["feed_id"] = integerProp()

	return objectSchema(props, []string{"feed_id"})
}

// OutputSchema returns the tool's output JSON schema.
func (GetFeedEntries) OutputSchema() *map[string]any { return feedEntriesSchema() }

// Annotations returns the tool's MCP annotations.
func (GetFeedEntries) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:         "Get feed entries",
		ReadOnlyHint:  true,
		OpenWorldHint: ptrBool(false),
	}
}

// Instructions returns the tool's usage instructions.
func (GetFeedEntries) Instructions() string {
	return "Lists entries for one feed. Use to read the contents of a specific subscription. " +
		"The nested feed.checked_at reflects the last time Miniflux fetched this feed — use it " +
		"to judge whether entries may be stale. Read-only."
}

// GetCounters returns feed-level unread counts.
type GetCounters struct{}

// Name returns the MCP tool name.
func (GetCounters) Name() string { return "get_counters" }

// InputSchema returns the tool's input JSON schema.
func (GetCounters) InputSchema() *map[string]any { return objectSchema(nil, nil) }

// OutputSchema returns the tool's output JSON schema.
func (GetCounters) OutputSchema() *map[string]any { return objectSchema(countersProps(), nil) }

// Annotations returns the tool's MCP annotations.
func (GetCounters) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:         "Get counters",
		ReadOnlyHint:  true,
		OpenWorldHint: ptrBool(false),
	}
}

// Instructions returns the tool's usage instructions.
func (GetCounters) Instructions() string {
	return "Returns per-feed and total unread/read counters. Useful for summarizing what needs attention. Read-only."
}

// GetMe returns the current user profile.
type GetMe struct{}

// Name returns the MCP tool name.
func (GetMe) Name() string { return "get_me" }

// InputSchema returns the tool's input JSON schema.
func (GetMe) InputSchema() *map[string]any { return objectSchema(nil, nil) }

// OutputSchema returns the tool's output JSON schema.
func (GetMe) OutputSchema() *map[string]any { return objectSchema(meProps(), nil) }

// Annotations returns the tool's MCP annotations.
func (GetMe) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:         "Get current user",
		ReadOnlyHint:  true,
		OpenWorldHint: ptrBool(false),
	}
}

// Instructions returns the tool's usage instructions.
func (GetMe) Instructions() string {
	return "Returns the authenticated user's profile. The only user-related tool; user management is " +
		"intentionally out of scope (least privilege). Read-only."
}

// ExportOPML exports the whole subscription set as OPML.
type ExportOPML struct{}

// Name returns the MCP tool name.
func (ExportOPML) Name() string { return "export_opml" }

// InputSchema returns the tool's input JSON schema.
func (ExportOPML) InputSchema() *map[string]any { return objectSchema(nil, nil) }

// OutputSchema returns the tool's output JSON schema.
func (ExportOPML) OutputSchema() *map[string]any { return objectSchema(opmlProps(), nil) }

// Annotations returns the tool's MCP annotations.
func (ExportOPML) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:         "Export OPML",
		ReadOnlyHint:  true,
		OpenWorldHint: ptrBool(false),
	}
}

// Instructions returns the tool's usage instructions.
func (ExportOPML) Instructions() string {
	return "Returns the user's full subscription set as OPML XML. Use for backup/migration. " +
		"Read-only. The returned document is trusted local data; it is returned structurally as the " +
		"opml field."
}

// DiscoverSubscriptions probes a URL and returns candidate feeds. This is the
// single open-world tool (SPEC §4.1, S07/S10): its output is untrusted data
// returned structurally.
type DiscoverSubscriptions struct{}

// Name returns the MCP tool name.
func (DiscoverSubscriptions) Name() string { return "discover_subscriptions" }

// InputSchema returns the tool's input JSON schema.
func (DiscoverSubscriptions) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"url": stringProp()}, []string{"url"})
}

// OutputSchema returns the tool's output JSON schema.
func (DiscoverSubscriptions) OutputSchema() *map[string]any {
	candidate := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url":   stringProp(),
			"title": stringProp(),
			"type":  stringProp(),
		},
	}

	return objectSchema(map[string]any{
		"feeds": map[string]any{
			"type":  "array",
			"items": candidate,
		},
	}, nil)
}

// Annotations returns the tool's MCP annotations.
func (DiscoverSubscriptions) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:         "Discover subscriptions",
		ReadOnlyHint:  true,
		OpenWorldHint: ptrBool(true),
	}
}

// Instructions returns the tool's usage instructions.
func (DiscoverSubscriptions) Instructions() string {
	return "Takes an arbitrary URL and asks Miniflux to detect the feed(s) it exposes. This is an " +
		"open-world tool: the URL and the returned candidates are untrusted external data. It is " +
		"isolated — it cannot read any local data and returns only the candidate list, " +
		"structurally, never raw free-form text. Use before create_feed to confirm a feed URL. " +
		"Read-only with respect to the account."
}

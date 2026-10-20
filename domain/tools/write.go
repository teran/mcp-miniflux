package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreateFeed subscribes to a feed (idempotent via search-before-create).
type CreateFeed struct{}

// Name returns the MCP tool name.
func (CreateFeed) Name() string { return "create_feed" }

// InputSchema returns the tool's input JSON schema.
func (CreateFeed) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"feed_url":    stringProp(),
		"category_id": integerProp(),
		"title":       stringProp(),
		"username":    stringProp(),
		"password":    stringProp(),
	}, []string{"feed_url"})
}

// OutputSchema returns the tool's output JSON schema.
func (CreateFeed) OutputSchema() *map[string]any { return feedSchema() }

// Annotations returns the tool's MCP annotations.
func (CreateFeed) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Create feed",
		IdempotentHint: true,
	}
}

// Instructions returns the tool's usage instructions.
func (CreateFeed) Instructions() string {
	return "Subscribes to the feed at feed_url. Idempotent: the server first searches existing feeds " +
		"by URL; if the feed already exists it returns the existing feed instead of creating a " +
		"duplicate. Safe to retry. Use discover_subscriptions first to confirm the URL. Optional " +
		"HTTP credentials are passed to Miniflux and never returned/logged."
}

// UpdateFeed updates a feed's metadata/credentials.
type UpdateFeed struct{}

// Name returns the MCP tool name.
func (UpdateFeed) Name() string { return "update_feed" }

// InputSchema returns the tool's input JSON schema.
func (UpdateFeed) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"feed_id":       integerProp(),
		"title":         stringProp(),
		"category_id":   integerProp(),
		"site_url":      stringProp(),
		"username":      stringProp(),
		"password":      stringProp(),
		"user_agent":    stringProp(),
		"scraper_rules": stringProp(),
		"rewrite_rules": stringProp(),
		"crawler":       booleanProp(),
	}, []string{"feed_id"})
}

// OutputSchema returns the tool's output JSON schema.
func (UpdateFeed) OutputSchema() *map[string]any { return feedSchema() }

// Annotations returns the tool's MCP annotations.
func (UpdateFeed) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Update feed",
		IdempotentHint: true,
	}
}

// Instructions returns the tool's usage instructions.
func (UpdateFeed) Instructions() string {
	return "Updates an existing feed's settings. Only the provided fields are changed. Setting " +
		"fields to explicit values is idempotent. Does not delete."
}

// RefreshFeed forces a refresh of one feed.
type RefreshFeed struct{}

// Name returns the MCP tool name.
func (RefreshFeed) Name() string { return "refresh_feed" }

// InputSchema returns the tool's input JSON schema.
func (RefreshFeed) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"feed_id": integerProp()}, []string{"feed_id"})
}

// OutputSchema returns the tool's output JSON schema.
func (RefreshFeed) OutputSchema() *map[string]any { return okSchema() }

// Annotations returns the tool's MCP annotations.
func (RefreshFeed) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Refresh feed",
		IdempotentHint: true,
	}
}

// Instructions returns the tool's usage instructions.
func (RefreshFeed) Instructions() string {
	return "Triggers an asynchronous background refresh of this feed's entries in Miniflux. " +
		"The call returns immediately; entries may not reflect the refresh until Miniflux " +
		"completes it — verify feed.checked_at and re-read if they still look stale. " +
		"Repeated refresh is idempotent (Miniflux coalesces refreshes)."
}

// CreateCategory creates a category (idempotent via search-before-create by title).
type CreateCategory struct{}

// Name returns the MCP tool name.
func (CreateCategory) Name() string { return "create_category" }

// InputSchema returns the tool's input JSON schema.
func (CreateCategory) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"title": stringProp()}, []string{"title"})
}

// OutputSchema returns the tool's output JSON schema.
func (CreateCategory) OutputSchema() *map[string]any { return categorySchema() }

// Annotations returns the tool's MCP annotations.
func (CreateCategory) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Create category",
		IdempotentHint: true,
	}
}

// Instructions returns the tool's usage instructions.
func (CreateCategory) Instructions() string {
	return "Creates a category with the given title. Idempotent: the handler first checks for an " +
		"existing category with the same title and returns it instead of creating a duplicate. " +
		"Safe to retry."
}

// UpdateCategory renames/reparents a category.
type UpdateCategory struct{}

// Name returns the MCP tool name.
func (UpdateCategory) Name() string { return "update_category" }

// InputSchema returns the tool's input JSON schema.
func (UpdateCategory) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"category_id": integerProp(),
		"title":       stringProp(),
	}, []string{"category_id"})
}

// OutputSchema returns the tool's output JSON schema.
func (UpdateCategory) OutputSchema() *map[string]any { return okSchema() }

// Annotations returns the tool's MCP annotations.
func (UpdateCategory) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Update category",
		IdempotentHint: true,
	}
}

// Instructions returns the tool's usage instructions.
func (UpdateCategory) Instructions() string {
	return "Updates a category's title. Setting a title is idempotent."
}

// RefreshCategory forces refresh of all feeds in a category.
type RefreshCategory struct{}

// Name returns the MCP tool name.
func (RefreshCategory) Name() string { return "refresh_category" }

// InputSchema returns the tool's input JSON schema.
func (RefreshCategory) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"category_id": integerProp()}, []string{"category_id"})
}

// OutputSchema returns the tool's output JSON schema.
func (RefreshCategory) OutputSchema() *map[string]any { return okSchema() }

// Annotations returns the tool's MCP annotations.
func (RefreshCategory) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Refresh category",
		IdempotentHint: true,
	}
}

// Instructions returns the tool's usage instructions.
func (RefreshCategory) Instructions() string {
	return "Triggers an asynchronous background refresh of every feed in the category. " +
		"The call returns immediately; entries may not reflect the refresh until Miniflux " +
		"completes it — verify feed.checked_at and re-read if they still look stale. " +
		"Idempotent."
}

// MarkFeedEntriesRead marks all entries of a feed as read.
type MarkFeedEntriesRead struct{}

// Name returns the MCP tool name.
func (MarkFeedEntriesRead) Name() string { return "mark_feed_entries_read" }

// InputSchema returns the tool's input JSON schema.
func (MarkFeedEntriesRead) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"feed_id": integerProp()}, []string{"feed_id"})
}

// OutputSchema returns the tool's output JSON schema.
func (MarkFeedEntriesRead) OutputSchema() *map[string]any { return okSchema() }

// Annotations returns the tool's MCP annotations.
func (MarkFeedEntriesRead) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Mark feed entries read",
		IdempotentHint: true,
	}
}

// Instructions returns the tool's usage instructions.
func (MarkFeedEntriesRead) Instructions() string {
	return "Marks every entry in the feed as read. Setting read-state is idempotent — safe to retry."
}

// MarkCategoryEntriesRead marks all entries of a category as read.
type MarkCategoryEntriesRead struct{}

// Name returns the MCP tool name.
func (MarkCategoryEntriesRead) Name() string { return "mark_category_entries_read" }

// InputSchema returns the tool's input JSON schema.
func (MarkCategoryEntriesRead) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"category_id": integerProp()}, []string{"category_id"})
}

// OutputSchema returns the tool's output JSON schema.
func (MarkCategoryEntriesRead) OutputSchema() *map[string]any { return okSchema() }

// Annotations returns the tool's MCP annotations.
func (MarkCategoryEntriesRead) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Mark category entries read",
		IdempotentHint: true,
	}
}

// Instructions returns the tool's usage instructions.
func (MarkCategoryEntriesRead) Instructions() string {
	return "Marks every entry in the category as read. Idempotent."
}

// UpdateEntries bulk-sets status/starred on a set of entries.
type UpdateEntries struct{}

// Name returns the MCP tool name.
func (UpdateEntries) Name() string { return "update_entries" }

// InputSchema returns the tool's input JSON schema.
func (UpdateEntries) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"entry_ids": integerArrayProp(entryLimitMax),
		"status":    stringEnumProp("unread", "read", "removed"),
		"starred":   booleanProp(),
	}, []string{"entry_ids"})
}

// OutputSchema returns the tool's output JSON schema.
func (UpdateEntries) OutputSchema() *map[string]any { return okSchema() }

// Annotations returns the tool's MCP annotations.
func (UpdateEntries) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Update entries (bulk)",
		IdempotentHint: true,
	}
}

// Instructions returns the tool's usage instructions.
func (UpdateEntries) Instructions() string {
	return "Bulk-applies a status and/or starred flag to the given entry ids. Setting state to " +
		"explicit values is idempotent. Prefer this over per-entry calls for batch operations."
}

// ToggleEntryBookmark toggles the starred/bookmark state of one entry. NOT
// idempotent (X02/N25).
type ToggleEntryBookmark struct{}

// Name returns the MCP tool name.
func (ToggleEntryBookmark) Name() string { return "toggle_entry_bookmark" }

// InputSchema returns the tool's input JSON schema.
func (ToggleEntryBookmark) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"entry_id": integerProp()}, []string{"entry_id"})
}

// OutputSchema returns the tool's output JSON schema.
func (ToggleEntryBookmark) OutputSchema() *map[string]any { return okSchema() }

// Annotations returns the tool's MCP annotations.
func (ToggleEntryBookmark) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Toggle entry bookmark",
		IdempotentHint: false,
	}
}

// Instructions returns the tool's usage instructions.
func (ToggleEntryBookmark) Instructions() string {
	return "Flips the starred state of one entry. Not idempotent — each call toggles state, so " +
		"calling twice returns it to the original value. Use update_entries with an explicit " +
		"starred value when you want to set (not toggle) state."
}

// UpdateEntry updates an entry's title/content.
type UpdateEntry struct{}

// Name returns the MCP tool name.
func (UpdateEntry) Name() string { return "update_entry" }

// InputSchema returns the tool's input JSON schema.
func (UpdateEntry) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"entry_id": integerProp(),
		"title":    stringProp(),
		"content":  stringProp(),
		"url":      stringProp(),
	}, []string{"entry_id"})
}

// OutputSchema returns the tool's output JSON schema.
func (UpdateEntry) OutputSchema() *map[string]any { return entrySchema() }

// Annotations returns the tool's MCP annotations.
func (UpdateEntry) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Update entry",
		IdempotentHint: true,
	}
}

// Instructions returns the tool's usage instructions.
func (UpdateEntry) Instructions() string {
	return "Updates an entry's title/content/url. Setting fields to values is idempotent. Does not change read/starred state."
}

// ImportOPML imports subscriptions from an OPML document.
type ImportOPML struct{}

// Name returns the MCP tool name.
func (ImportOPML) Name() string { return "import_opml" }

// InputSchema returns the tool's input JSON schema.
func (ImportOPML) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"opml": stringProp()}, []string{"opml"})
}

// OutputSchema returns the tool's output JSON schema.
func (ImportOPML) OutputSchema() *map[string]any { return okSchema() }

// Annotations returns the tool's MCP annotations.
func (ImportOPML) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Import OPML",
		IdempotentHint: true,
	}
}

// Instructions returns the tool's usage instructions.
func (ImportOPML) Instructions() string {
	return "Imports feed subscriptions from OPML. Idempotent: Miniflux returns 200 for feeds that " +
		"already exist, so re-importing the same OPML does not create duplicates. The document is " +
		"sent to Miniflux and not stored locally."
}

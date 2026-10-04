package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreateFeed subscribes to a feed (idempotent via search-before-create).
type CreateFeed struct{}

func (CreateFeed) Name() string { return "create_feed" }

func (CreateFeed) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"feed_url":    stringProp(),
		"category_id": integerProp(),
		"title":       stringProp(),
		"username":    stringProp(),
		"password":    stringProp(),
	}, []string{"feed_url"})
}

func (CreateFeed) OutputSchema() *map[string]any { return outputSchema() }

func (CreateFeed) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Create feed",
		IdempotentHint: true,
	}
}

func (CreateFeed) Instructions() string {
	return "Subscribes to the feed at feed_url. Idempotent: the server first searches existing feeds by URL; if the feed already exists it returns the existing feed instead of creating a duplicate. Safe to retry. Use discover_subscriptions first to confirm the URL. Optional HTTP credentials are passed to Miniflux and never returned/logged."
}

func (CreateFeed) Call(ctx context.Context, args map[string]any) (Result, error) {
	return nil, errNotImplemented
}

// UpdateFeed updates a feed's metadata/credentials.
type UpdateFeed struct{}

func (UpdateFeed) Name() string { return "update_feed" }

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

func (UpdateFeed) OutputSchema() *map[string]any { return outputSchema() }

func (UpdateFeed) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Update feed",
		IdempotentHint: true,
	}
}

func (UpdateFeed) Instructions() string {
	return "Updates an existing feed's settings. Only the provided fields are changed. Setting fields to explicit values is idempotent. Does not delete."
}

func (UpdateFeed) Call(ctx context.Context, args map[string]any) (Result, error) {
	return nil, errNotImplemented
}

// RefreshFeed forces a refresh of one feed.
type RefreshFeed struct{}

func (RefreshFeed) Name() string { return "refresh_feed" }

func (RefreshFeed) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"feed_id": integerProp()}, []string{"feed_id"})
}

func (RefreshFeed) OutputSchema() *map[string]any { return outputSchema() }

func (RefreshFeed) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Refresh feed",
		IdempotentHint: true,
	}
}

func (RefreshFeed) Instructions() string {
	return "Triggers Miniflux to refresh this feed's entries. Repeated refresh is idempotent (Miniflux coalesces refreshes)."
}

func (RefreshFeed) Call(ctx context.Context, args map[string]any) (Result, error) {
	return nil, errNotImplemented
}

// CreateCategory creates a category (idempotent via search-before-create by title).
type CreateCategory struct{}

func (CreateCategory) Name() string { return "create_category" }

func (CreateCategory) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"title": stringProp()}, []string{"title"})
}

func (CreateCategory) OutputSchema() *map[string]any { return outputSchema() }

func (CreateCategory) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Create category",
		IdempotentHint: true,
	}
}

func (CreateCategory) Instructions() string {
	return "Creates a category with the given title. Idempotent: the handler first checks for an existing category with the same title and returns it instead of creating a duplicate. Safe to retry."
}

func (CreateCategory) Call(ctx context.Context, args map[string]any) (Result, error) {
	return nil, errNotImplemented
}

// UpdateCategory renames/reparents a category.
type UpdateCategory struct{}

func (UpdateCategory) Name() string { return "update_category" }

func (UpdateCategory) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"category_id": integerProp(),
		"title":       stringProp(),
	}, []string{"category_id"})
}

func (UpdateCategory) OutputSchema() *map[string]any { return outputSchema() }

func (UpdateCategory) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Update category",
		IdempotentHint: true,
	}
}

func (UpdateCategory) Instructions() string {
	return "Updates a category's title. Setting a title is idempotent."
}

func (UpdateCategory) Call(ctx context.Context, args map[string]any) (Result, error) {
	return nil, errNotImplemented
}

// RefreshCategory forces refresh of all feeds in a category.
type RefreshCategory struct{}

func (RefreshCategory) Name() string { return "refresh_category" }

func (RefreshCategory) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"category_id": integerProp()}, []string{"category_id"})
}

func (RefreshCategory) OutputSchema() *map[string]any { return outputSchema() }

func (RefreshCategory) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Refresh category",
		IdempotentHint: true,
	}
}

func (RefreshCategory) Instructions() string {
	return "Triggers a refresh of every feed in the category. Idempotent."
}

func (RefreshCategory) Call(ctx context.Context, args map[string]any) (Result, error) {
	return nil, errNotImplemented
}

// MarkFeedEntriesRead marks all entries of a feed as read.
type MarkFeedEntriesRead struct{}

func (MarkFeedEntriesRead) Name() string { return "mark_feed_entries_read" }

func (MarkFeedEntriesRead) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"feed_id": integerProp()}, []string{"feed_id"})
}

func (MarkFeedEntriesRead) OutputSchema() *map[string]any { return outputSchema() }

func (MarkFeedEntriesRead) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Mark feed entries read",
		IdempotentHint: true,
	}
}

func (MarkFeedEntriesRead) Instructions() string {
	return "Marks every entry in the feed as read. Setting read-state is idempotent — safe to retry."
}

func (MarkFeedEntriesRead) Call(ctx context.Context, args map[string]any) (Result, error) {
	return nil, errNotImplemented
}

// MarkCategoryEntriesRead marks all entries of a category as read.
type MarkCategoryEntriesRead struct{}

func (MarkCategoryEntriesRead) Name() string { return "mark_category_entries_read" }

func (MarkCategoryEntriesRead) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"category_id": integerProp()}, []string{"category_id"})
}

func (MarkCategoryEntriesRead) OutputSchema() *map[string]any { return outputSchema() }

func (MarkCategoryEntriesRead) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Mark category entries read",
		IdempotentHint: true,
	}
}

func (MarkCategoryEntriesRead) Instructions() string {
	return "Marks every entry in the category as read. Idempotent."
}

func (MarkCategoryEntriesRead) Call(ctx context.Context, args map[string]any) (Result, error) {
	return nil, errNotImplemented
}

// UpdateEntries bulk-sets status/starred on a set of entries.
type UpdateEntries struct{}

func (UpdateEntries) Name() string { return "update_entries" }

func (UpdateEntries) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"entry_ids": integerArrayProp(1000),
		"status":    stringEnumProp("unread", "read", "removed"),
		"starred":   booleanProp(),
	}, []string{"entry_ids"})
}

func (UpdateEntries) OutputSchema() *map[string]any { return outputSchema() }

func (UpdateEntries) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Update entries (bulk)",
		IdempotentHint: true,
	}
}

func (UpdateEntries) Instructions() string {
	return "Bulk-applies a status and/or starred flag to the given entry ids. Setting state to explicit values is idempotent. Prefer this over per-entry calls for batch operations."
}

func (UpdateEntries) Call(ctx context.Context, args map[string]any) (Result, error) {
	return nil, errNotImplemented
}

// ToggleEntryBookmark toggles the starred/bookmark state of one entry. NOT
// idempotent (X02/N25).
type ToggleEntryBookmark struct{}

func (ToggleEntryBookmark) Name() string { return "toggle_entry_bookmark" }

func (ToggleEntryBookmark) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"entry_id": integerProp()}, []string{"entry_id"})
}

func (ToggleEntryBookmark) OutputSchema() *map[string]any { return outputSchema() }

func (ToggleEntryBookmark) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Toggle entry bookmark",
		IdempotentHint: false,
	}
}

func (ToggleEntryBookmark) Instructions() string {
	return "Flips the starred state of one entry. Not idempotent — each call toggles state, so calling twice returns it to the original value. Use update_entries with an explicit starred value when you want to set (not toggle) state."
}

func (ToggleEntryBookmark) Call(ctx context.Context, args map[string]any) (Result, error) {
	return nil, errNotImplemented
}

// UpdateEntry updates an entry's title/content.
type UpdateEntry struct{}

func (UpdateEntry) Name() string { return "update_entry" }

func (UpdateEntry) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"entry_id": integerProp(),
		"title":    stringProp(),
		"content":  stringProp(),
		"url":      stringProp(),
	}, []string{"entry_id"})
}

func (UpdateEntry) OutputSchema() *map[string]any { return outputSchema() }

func (UpdateEntry) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Update entry",
		IdempotentHint: true,
	}
}

func (UpdateEntry) Instructions() string {
	return "Updates an entry's title/content/url. Setting fields to values is idempotent. Does not change read/starred state."
}

func (UpdateEntry) Call(ctx context.Context, args map[string]any) (Result, error) {
	return nil, errNotImplemented
}

// ImportOPML imports subscriptions from an OPML document.
type ImportOPML struct{}

func (ImportOPML) Name() string { return "import_opml" }

func (ImportOPML) InputSchema() *map[string]any {
	return objectSchema(map[string]any{"opml": stringProp()}, []string{"opml"})
}

func (ImportOPML) OutputSchema() *map[string]any { return outputSchema() }

func (ImportOPML) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:          "Import OPML",
		IdempotentHint: true,
	}
}

func (ImportOPML) Instructions() string {
	return "Imports feed subscriptions from OPML. Idempotent: Miniflux returns 200 for feeds that already exist, so re-importing the same OPML does not create duplicates. The document is sent to Miniflux and not stored locally."
}

func (ImportOPML) Call(ctx context.Context, args map[string]any) (Result, error) {
	return nil, errNotImplemented
}

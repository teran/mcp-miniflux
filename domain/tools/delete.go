package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DeleteFeed permanently removes a feed subscription (HITL, S12).
type DeleteFeed struct{}

func (DeleteFeed) Name() string { return "delete_feed" }

func (DeleteFeed) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"feed_id": integerProp(),
		"confirm": booleanProp(),
	}, []string{"feed_id", "confirm"})
}

func (DeleteFeed) OutputSchema() *map[string]any { return okSchema() }

func (DeleteFeed) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:           "Delete feed",
		DestructiveHint: ptrBool(true),
		IdempotentHint:  true,
	}
}

func (DeleteFeed) Instructions() string {
	return "Permanently deletes a feed and its entries. Irreversible — require human confirmation. Delete is idempotent (deleting a missing feed succeeds)."
}

// DeleteCategory permanently removes a category (HITL, S12).
type DeleteCategory struct{}

func (DeleteCategory) Name() string { return "delete_category" }

func (DeleteCategory) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"category_id": integerProp(),
		"confirm":     booleanProp(),
	}, []string{"category_id", "confirm"})
}

func (DeleteCategory) OutputSchema() *map[string]any { return okSchema() }

func (DeleteCategory) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:           "Delete category",
		DestructiveHint: ptrBool(true),
		IdempotentHint:  true,
	}
}

func (DeleteCategory) Instructions() string {
	return "Permanently deletes a category. Irreversible — require human confirmation. Deleting a missing category succeeds (idempotent)."
}

// FlushHistory purges history (removed/older entries) from Miniflux (HITL, S12).
type FlushHistory struct{}

func (FlushHistory) Name() string { return "flush_history" }

func (FlushHistory) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"before":  stringProp(),
		"confirm": booleanProp(),
	}, []string{"confirm"})
}

func (FlushHistory) OutputSchema() *map[string]any { return okSchema() }

func (FlushHistory) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:           "Flush history",
		DestructiveHint: ptrBool(true),
		IdempotentHint:  true,
	}
}

func (FlushHistory) Instructions() string {
	return "Purges old/removed entry history from Miniflux. Irreversible — require human confirmation. Idempotent (flushing an already-clean history is a no-op)."
}

package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DeleteFeed permanently removes a feed subscription (HITL, S12).
type DeleteFeed struct{}

// Name returns the MCP tool name.
func (DeleteFeed) Name() string { return "delete_feed" }

// InputSchema returns the tool's input JSON schema.
func (DeleteFeed) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"feed_id": integerProp(),
		"confirm": booleanProp(),
	}, []string{"feed_id", "confirm"})
}

// OutputSchema returns the tool's output JSON schema.
func (DeleteFeed) OutputSchema() *map[string]any { return okSchema() }

// Annotations returns the tool's MCP annotations.
func (DeleteFeed) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:           "Delete feed",
		DestructiveHint: ptrBool(true),
		IdempotentHint:  true,
	}
}

// Instructions returns the tool's usage instructions.
func (DeleteFeed) Instructions() string {
	return "Permanently deletes a feed and its entries. Irreversible — require human " +
		"confirmation. Delete is idempotent (deleting a missing feed succeeds)."
}

// DeleteCategory permanently removes a category (HITL, S12).
type DeleteCategory struct{}

// Name returns the MCP tool name.
func (DeleteCategory) Name() string { return "delete_category" }

// InputSchema returns the tool's input JSON schema.
func (DeleteCategory) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"category_id": integerProp(),
		"confirm":     booleanProp(),
	}, []string{"category_id", "confirm"})
}

// OutputSchema returns the tool's output JSON schema.
func (DeleteCategory) OutputSchema() *map[string]any { return okSchema() }

// Annotations returns the tool's MCP annotations.
func (DeleteCategory) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:           "Delete category",
		DestructiveHint: ptrBool(true),
		IdempotentHint:  true,
	}
}

// Instructions returns the tool's usage instructions.
func (DeleteCategory) Instructions() string {
	return "Permanently deletes a category. Irreversible — require human confirmation. Deleting a " +
		"missing category succeeds (idempotent)."
}

// FlushHistory purges history (removed/older entries) from Miniflux (HITL, S12).
type FlushHistory struct{}

// Name returns the MCP tool name.
func (FlushHistory) Name() string { return "flush_history" }

// InputSchema returns the tool's input JSON schema.
func (FlushHistory) InputSchema() *map[string]any {
	return objectSchema(map[string]any{
		"before":  stringProp(),
		"confirm": booleanProp(),
	}, []string{"confirm"})
}

// OutputSchema returns the tool's output JSON schema.
func (FlushHistory) OutputSchema() *map[string]any { return okSchema() }

// Annotations returns the tool's MCP annotations.
func (FlushHistory) Annotations() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{
		Title:           "Flush history",
		DestructiveHint: ptrBool(true),
		IdempotentHint:  true,
	}
}

// Instructions returns the tool's usage instructions.
func (FlushHistory) Instructions() string {
	return "Purges old/removed entry history from Miniflux. Irreversible — require human " +
		"confirmation. Idempotent (flushing an already-clean history is a no-op)."
}

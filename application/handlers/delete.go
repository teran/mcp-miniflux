package handlers

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// DeleteFeedHandler implements the delete_feed use case (SPEC §4.3). It is a
// destructive, human-confirmable (HITL, S12) tool that refuses to run without
// confirm:true.
type DeleteFeedHandler struct {
	Client dmf.Client
}

func (h DeleteFeedHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	if err := requireConfirm(args); err != nil {
		return nil, err
	}
	id, err := asInt("feed_id", args["feed_id"])
	if err != nil {
		return nil, err
	}
	if err := h.Client.DeleteFeed(ctx, id); err != nil {
		return upstreamErr(err), nil
	}
	return okResult(), nil
}

// DeleteCategoryHandler implements the delete_category use case (SPEC §4.3).
// Destructive, human-confirmable (HITL, S12).
type DeleteCategoryHandler struct {
	Client dmf.Client
}

func (h DeleteCategoryHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	if err := requireConfirm(args); err != nil {
		return nil, err
	}
	id, err := asInt("category_id", args["category_id"])
	if err != nil {
		return nil, err
	}
	if err := h.Client.DeleteCategory(ctx, id); err != nil {
		return upstreamErr(err), nil
	}
	return okResult(), nil
}

// FlushHistoryHandler implements the flush_history use case (SPEC §4.3).
// Destructive, human-confirmable (HITL, S12).
type FlushHistoryHandler struct {
	Client dmf.Client
}

func (h FlushHistoryHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	if err := requireConfirm(args); err != nil {
		return nil, err
	}
	before, err := timeArg("before", args)
	if err != nil {
		return nil, err
	}
	if err := h.Client.FlushHistory(ctx, before); err != nil {
		return upstreamErr(err), nil
	}
	return okResult(), nil
}

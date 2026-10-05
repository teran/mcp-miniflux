package handlers

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// ListFeedsHandler implements the list_feeds use case (SPEC §4.1).
type ListFeedsHandler struct {
	Client dmf.Client
}

func (h ListFeedsHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	var categoryID *int
	if v, ok := args["category_id"]; ok {
		id, err := asInt("category_id", v)
		if err != nil {
			return nil, err
		}
		categoryID = &id
	}
	limit, err := intArg("limit", args, 0)
	if err != nil {
		return nil, err
	}
	offset, err := intArg("offset", args, 0)
	if err != nil {
		return nil, err
	}

	feeds, err := h.Client.ListFeeds(ctx, categoryID, limit, offset)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(map[string]any{"feeds": feeds})
}

// GetFeedHandler implements the get_feed use case (SPEC §4.1).
type GetFeedHandler struct {
	Client dmf.Client
}

func (h GetFeedHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	id, err := asInt("feed_id", args["feed_id"])
	if err != nil {
		return nil, err
	}
	feed, err := h.Client.GetFeed(ctx, id)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(feed)
}

// ListCategoriesHandler implements the list_categories use case (SPEC §4.1).
type ListCategoriesHandler struct {
	Client dmf.Client
}

func (h ListCategoriesHandler) Call(ctx context.Context, _ map[string]any) (*mcp.CallToolResult, error) {
	cats, err := h.Client.ListCategories(ctx)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(map[string]any{"categories": cats})
}

// ListEntriesHandler implements the list_entries use case (SPEC §4.1).
type ListEntriesHandler struct {
	Client dmf.Client
}

func (h ListEntriesHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	filter, err := entryFilter(args, true)
	if err != nil {
		return nil, err
	}
	out, err := h.Client.ListEntries(ctx, filter)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(out)
}

// GetEntryHandler implements the get_entry use case (SPEC §4.1).
type GetEntryHandler struct {
	Client dmf.Client
}

func (h GetEntryHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	id, err := asInt("entry_id", args["entry_id"])
	if err != nil {
		return nil, err
	}
	entry, err := h.Client.GetEntry(ctx, id)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(entry)
}

// GetFeedEntriesHandler implements the get_feed_entries use case (SPEC §4.1).
type GetFeedEntriesHandler struct {
	Client dmf.Client
}

func (h GetFeedEntriesHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	feedID, err := asInt("feed_id", args["feed_id"])
	if err != nil {
		return nil, err
	}
	filter, err := entryFilter(args, false)
	if err != nil {
		return nil, err
	}
	out, err := h.Client.GetFeedEntries(ctx, feedID, filter)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(out)
}

// GetCountersHandler implements the get_counters use case (SPEC §4.1).
type GetCountersHandler struct {
	Client dmf.Client
}

func (h GetCountersHandler) Call(ctx context.Context, _ map[string]any) (*mcp.CallToolResult, error) {
	counters, err := h.Client.GetCounters(ctx)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(counters)
}

// GetMeHandler implements the get_me use case (SPEC §4.1).
type GetMeHandler struct {
	Client dmf.Client
}

func (h GetMeHandler) Call(ctx context.Context, _ map[string]any) (*mcp.CallToolResult, error) {
	me, err := h.Client.GetMe(ctx)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(me)
}

// ExportOPMLHandler implements the export_opml use case (SPEC §4.1). The OPML
// document is trusted local data returned structurally as the opml field.
type ExportOPMLHandler struct {
	Client dmf.Client
}

func (h ExportOPMLHandler) Call(ctx context.Context, _ map[string]any) (*mcp.CallToolResult, error) {
	doc, err := h.Client.ExportOPML(ctx)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(map[string]any{"opml": doc})
}

// DiscoverSubscriptionsHandler implements the discover_subscriptions use case
// (SPEC §4.1, S07/S10). It validates the url is an absolute http(s) URL (S11)
// and returns the untrusted candidates as structured content — never raw
// free-form text.
type DiscoverSubscriptionsHandler struct {
	Client dmf.Client
}

func (h DiscoverSubscriptionsHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	target, err := stringArg("url", args)
	if err != nil {
		return nil, err
	}
	if target == "" {
		return nil, invalidParams("url is required")
	}
	if err := absoluteHTTPURL(target); err != nil {
		return nil, err
	}

	candidates, err := h.Client.Discover(ctx, target)
	if err != nil {
		return upstreamErr(err), nil
	}
	redacted := dmf.Redact(candidates)
	data, err := marshalJSON(redacted)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(data)}},
		StructuredContent: redacted,
	}, nil
}

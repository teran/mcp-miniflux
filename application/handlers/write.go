package handlers

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// CreateFeedHandler implements the create_feed use case (SPEC §4.2). It is
// idempotent via search-before-create: existing feeds are searched by URL and
// the existing feed is returned instead of creating a duplicate.
type CreateFeedHandler struct {
	Client dmf.Client
}

func (h CreateFeedHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	req := dmf.CreateFeedRequest{}
	var err error
	if req.FeedURL, err = stringArg("feed_url", args); err != nil {
		return nil, err
	}
	if req.FeedURL == "" {
		return nil, invalidParams("feed_url is required")
	}
	if v, ok := args["category_id"]; ok {
		id, aerr := asInt("category_id", v)
		if aerr != nil {
			return nil, aerr
		}
		req.CategoryID = &id
	}
	if req.Title, err = stringArg("title", args); err != nil {
		return nil, err
	}
	if req.Username, err = stringArg("username", args); err != nil {
		return nil, err
	}
	if req.Password, err = stringArg("password", args); err != nil {
		return nil, err
	}

	// Search-before-create idempotency (SPEC §4.2): return an existing feed
	// with the same URL instead of creating a duplicate.
	feeds, err := h.Client.ListFeeds(ctx, nil, 0, 0)
	if err != nil {
		return upstreamErr(err), nil
	}
	for _, f := range feeds {
		if f.FeedURL == req.FeedURL {
			return jsonResult(f)
		}
	}

	feed, err := h.Client.CreateFeed(ctx, req)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(feed)
}

// UpdateFeedHandler implements the update_feed use case (SPEC §4.2).
type UpdateFeedHandler struct {
	Client dmf.Client
}

func (h UpdateFeedHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	id, err := asInt("feed_id", args["feed_id"])
	if err != nil {
		return nil, err
	}
	req := dmf.UpdateFeedRequest{}
	if req.Title, err = stringArg("title", args); err != nil {
		return nil, err
	}
	if v, ok := args["category_id"]; ok {
		cid, aerr := asInt("category_id", v)
		if aerr != nil {
			return nil, aerr
		}
		req.CategoryID = &cid
	}
	if req.SiteURL, err = stringArg("site_url", args); err != nil {
		return nil, err
	}
	if req.Username, err = stringArg("username", args); err != nil {
		return nil, err
	}
	if req.Password, err = stringArg("password", args); err != nil {
		return nil, err
	}
	if req.UserAgent, err = stringArg("user_agent", args); err != nil {
		return nil, err
	}
	if req.ScraperRules, err = stringArg("scraper_rules", args); err != nil {
		return nil, err
	}
	if req.RewriteRules, err = stringArg("rewrite_rules", args); err != nil {
		return nil, err
	}
	if req.Crawler, err = boolArg("crawler", args); err != nil {
		return nil, err
	}

	feed, err := h.Client.UpdateFeed(ctx, id, req)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(feed)
}

// RefreshFeedHandler implements the refresh_feed use case (SPEC §4.2).
type RefreshFeedHandler struct {
	Client dmf.Client
}

func (h RefreshFeedHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	id, err := asInt("feed_id", args["feed_id"])
	if err != nil {
		return nil, err
	}
	if err := h.Client.RefreshFeed(ctx, id); err != nil {
		return upstreamErr(err), nil
	}
	return okResult(), nil
}

// CreateCategoryHandler implements the create_category use case (SPEC §4.2). It
// is idempotent via search-before-create by title.
type CreateCategoryHandler struct {
	Client dmf.Client
}

func (h CreateCategoryHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	title, err := stringArg("title", args)
	if err != nil {
		return nil, err
	}
	if title == "" {
		return nil, invalidParams("title is required")
	}

	cats, err := h.Client.ListCategories(ctx)
	if err != nil {
		return upstreamErr(err), nil
	}
	for _, c := range cats {
		if c.Title == title {
			return jsonResult(c)
		}
	}

	cat, err := h.Client.CreateCategory(ctx, title)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(cat)
}

// UpdateCategoryHandler implements the update_category use case (SPEC §4.2).
type UpdateCategoryHandler struct {
	Client dmf.Client
}

func (h UpdateCategoryHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	id, err := asInt("category_id", args["category_id"])
	if err != nil {
		return nil, err
	}
	title, err := stringArg("title", args)
	if err != nil {
		return nil, err
	}
	if err := h.Client.UpdateCategory(ctx, id, title); err != nil {
		return upstreamErr(err), nil
	}
	return okResult(), nil
}

// RefreshCategoryHandler implements the refresh_category use case (SPEC §4.2).
type RefreshCategoryHandler struct {
	Client dmf.Client
}

func (h RefreshCategoryHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	id, err := asInt("category_id", args["category_id"])
	if err != nil {
		return nil, err
	}
	if err := h.Client.RefreshCategory(ctx, id); err != nil {
		return upstreamErr(err), nil
	}
	return okResult(), nil
}

// MarkFeedEntriesReadHandler implements mark_feed_entries_read (SPEC §4.2).
type MarkFeedEntriesReadHandler struct {
	Client dmf.Client
}

func (h MarkFeedEntriesReadHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	id, err := asInt("feed_id", args["feed_id"])
	if err != nil {
		return nil, err
	}
	if err := h.Client.MarkFeedEntriesRead(ctx, id); err != nil {
		return upstreamErr(err), nil
	}
	return okResult(), nil
}

// MarkCategoryEntriesReadHandler implements mark_category_entries_read
// (SPEC §4.2).
type MarkCategoryEntriesReadHandler struct {
	Client dmf.Client
}

func (h MarkCategoryEntriesReadHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	id, err := asInt("category_id", args["category_id"])
	if err != nil {
		return nil, err
	}
	if err := h.Client.MarkCategoryEntriesRead(ctx, id); err != nil {
		return upstreamErr(err), nil
	}
	return okResult(), nil
}

// UpdateEntriesHandler implements the update_entries bulk use case (SPEC §4.2).
type UpdateEntriesHandler struct {
	Client dmf.Client
}

func (h UpdateEntriesHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	raw, ok := args["entry_ids"].([]any)
	if !ok || len(raw) == 0 {
		return nil, invalidParams("entry_ids must be a non-empty array of integers")
	}
	if len(raw) > 1000 {
		return nil, invalidParams("entry_ids must not exceed 1000 entries")
	}
	ids := make([]int, 0, len(raw))
	for _, item := range raw {
		id, err := asInt("entry_ids", item)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	req := dmf.UpdateEntriesRequest{EntryIDs: ids}
	var err error
	if req.Status, err = stringArg("status", args); err != nil {
		return nil, err
	}
	if req.Starred, err = boolArg("starred", args); err != nil {
		return nil, err
	}

	if err := h.Client.UpdateEntries(ctx, req); err != nil {
		return upstreamErr(err), nil
	}
	return okResult(), nil
}

// ToggleEntryBookmarkHandler implements the toggle_entry_bookmark use case
// (SPEC §4.2). Deliberately NOT idempotent (X02/N25).
type ToggleEntryBookmarkHandler struct {
	Client dmf.Client
}

func (h ToggleEntryBookmarkHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	id, err := asInt("entry_id", args["entry_id"])
	if err != nil {
		return nil, err
	}
	if err := h.Client.ToggleEntryBookmark(ctx, id); err != nil {
		return upstreamErr(err), nil
	}
	return okResult(), nil
}

// UpdateEntryHandler implements the update_entry use case (SPEC §4.2).
type UpdateEntryHandler struct {
	Client dmf.Client
}

func (h UpdateEntryHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	id, err := asInt("entry_id", args["entry_id"])
	if err != nil {
		return nil, err
	}
	req := dmf.UpdateEntryRequest{}
	if req.Title, err = stringArg("title", args); err != nil {
		return nil, err
	}
	if req.Content, err = stringArg("content", args); err != nil {
		return nil, err
	}
	if req.URL, err = stringArg("url", args); err != nil {
		return nil, err
	}

	entry, err := h.Client.UpdateEntry(ctx, id, req)
	if err != nil {
		return upstreamErr(err), nil
	}
	return jsonResult(entry)
}

// ImportOPMLHandler implements the import_opml use case (SPEC §4.2). It relies
// on Miniflux returning 200 for already-existing entries for idempotency.
type ImportOPMLHandler struct {
	Client dmf.Client
}

func (h ImportOPMLHandler) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	doc, err := stringArg("opml", args)
	if err != nil {
		return nil, err
	}
	if doc == "" {
		return nil, invalidParams("opml is required")
	}
	if err := h.Client.ImportOPML(ctx, doc); err != nil {
		return upstreamErr(err), nil
	}
	return okResult(), nil
}

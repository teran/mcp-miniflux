package handlers

import (
	"context"
	"strings"
	"testing"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

func TestCreateFeedHandlerExistingReturnsExisting(t *testing.T) {
	var created bool
	c := &fakeClient{
		listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) {
			return []dmf.Feed{{ID: 4, FeedURL: "https://x/feed", Title: "existing"}}, nil
		},
		createFeed: func(context.Context, dmf.CreateFeedRequest) (*dmf.Feed, error) {
			created = true
			return &dmf.Feed{ID: 9}, nil
		},
	}
	h := CreateFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_url": "https://x/feed"})
	out := okRes(t, res, err)
	if created {
		t.Error("create_feed created a duplicate despite existing feed (search-before-create idempotency)")
	}
	if !strings.Contains(out, `"id":4`) {
		t.Errorf("expected existing feed returned: %s", out)
	}
}

func TestCreateFeedHandlerCreates(t *testing.T) {
	var got dmf.CreateFeedRequest
	c := &fakeClient{
		listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) {
			return nil, nil
		},
		createFeed: func(_ context.Context, req dmf.CreateFeedRequest) (*dmf.Feed, error) {
			got = req
			return &dmf.Feed{ID: 1, FeedURL: req.FeedURL, Password: req.Password}, nil
		},
	}
	h := CreateFeedHandler{Client: c}
	res, err := h.Call(tokCtx(t), map[string]any{"feed_url": "https://x/feed", "password": "hunter2"})
	out := okRes(t, res, err)
	if got.FeedURL != "https://x/feed" {
		t.Errorf("CreateFeed got url %q", got.FeedURL)
	}
	if got.Password != "hunter2" {
		t.Errorf("CreateFeed password not forwarded")
	}
	if strings.Contains(out, "hunter2") {
		t.Errorf("output leaked feed password (S02): %s", out)
	}
}

func TestCreateFeedHandlerMissingURL(t *testing.T) {
	h := CreateFeedHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{}); err == nil {
		t.Error("expected InvalidParams for missing feed_url")
	}
}

func TestCreateFeedHandlerBadCategory(t *testing.T) {
	h := CreateFeedHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"feed_url": "https://x", "category_id": "z"}); err == nil {
		t.Error("expected InvalidParams for bad category_id")
	}
}

func TestCreateFeedHandlerUpstream(t *testing.T) {
	c := &fakeClient{
		listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) { return nil, errSentinel },
	}
	h := CreateFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_url": "https://x"})
	if err != nil {
		t.Fatalf("want IsError, got %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestUpdateFeedHandler(t *testing.T) {
	var gotID int
	var got dmf.UpdateFeedRequest
	c := &fakeClient{updateFeed: func(_ context.Context, id int, req dmf.UpdateFeedRequest) (*dmf.Feed, error) {
		gotID, got = id, req
		return &dmf.Feed{ID: id, Password: req.Password}, nil
	}}
	h := UpdateFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(3), "title": "t", "password": "p"})
	out := okRes(t, res, err)
	if gotID != 3 || got.Title != "t" || got.Password != "p" {
		t.Errorf("UpdateFeed got id=%d req=%+v", gotID, got)
	}
	if strings.Contains(out, `"password":"p"`) {
		t.Errorf("output leaked password (S02): %s", out)
	}
}

func TestUpdateFeedHandlerBadID(t *testing.T) {
	h := UpdateFeedHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"feed_id": "x"}); err == nil {
		t.Error("expected InvalidParams")
	}
}

func TestRefreshFeedHandler(t *testing.T) {
	var gotID int
	c := &fakeClient{refreshFeed: func(_ context.Context, id int) error { gotID = id; return nil }}
	h := RefreshFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(2)})
	okRes(t, res, err)
	if gotID != 2 {
		t.Errorf("refresh id = %d, want 2", gotID)
	}
}

func TestRefreshFeedHandlerUpstream(t *testing.T) {
	c := &fakeClient{refreshFeed: func(context.Context, int) error { return errSentinel }}
	h := RefreshFeedHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"feed_id": float64(1)})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestCreateCategoryHandlerExisting(t *testing.T) {
	var created bool
	c := &fakeClient{
		listCategories: func(context.Context) ([]dmf.Category, error) {
			return []dmf.Category{{ID: 2, Title: "tech"}}, nil
		},
		createCategory: func(context.Context, string) (*dmf.Category, error) {
			created = true
			return &dmf.Category{ID: 99}, nil
		},
	}
	h := CreateCategoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"title": "tech"})
	out := okRes(t, res, err)
	if created {
		t.Error("create_category created duplicate despite existing title (idempotency)")
	}
	if !strings.Contains(out, `"id":2`) {
		t.Errorf("expected existing category: %s", out)
	}
}

func TestCreateCategoryHandlerCreates(t *testing.T) {
	var got string
	c := &fakeClient{
		listCategories: func(context.Context) ([]dmf.Category, error) { return nil, nil },
		createCategory: func(_ context.Context, title string) (*dmf.Category, error) {
			got = title
			return &dmf.Category{ID: 1, Title: title}, nil
		},
	}
	h := CreateCategoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"title": "news"})
	out := okRes(t, res, err)
	if got != "news" {
		t.Errorf("create category title = %q", got)
	}
	if !strings.Contains(out, `"title":"news"`) {
		t.Errorf("missing category: %s", out)
	}
}

func TestCreateCategoryHandlerMissingTitle(t *testing.T) {
	h := CreateCategoryHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{}); err == nil {
		t.Error("expected InvalidParams for missing title")
	}
}

func TestUpdateCategoryHandler(t *testing.T) {
	var gotID int
	var gotTitle string
	c := &fakeClient{updateCategory: func(_ context.Context, id int, title string) error {
		gotID, gotTitle = id, title
		return nil
	}}
	h := UpdateCategoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"category_id": float64(5), "title": "x"})
	okRes(t, res, err)
	if gotID != 5 || gotTitle != "x" {
		t.Errorf("updateCategory got id=%d title=%q", gotID, gotTitle)
	}
}

func TestUpdateCategoryHandlerUpstream(t *testing.T) {
	c := &fakeClient{updateCategory: func(context.Context, int, string) error { return errSentinel }}
	h := UpdateCategoryHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"category_id": float64(1), "title": "x"})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestRefreshCategoryHandler(t *testing.T) {
	var gotID int
	c := &fakeClient{refreshCategory: func(_ context.Context, id int) error { gotID = id; return nil }}
	h := RefreshCategoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"category_id": float64(3)})
	okRes(t, res, err)
	if gotID != 3 {
		t.Errorf("refresh category id = %d", gotID)
	}
}

func TestMarkFeedEntriesReadHandler(t *testing.T) {
	var gotID int
	c := &fakeClient{markFeedRead: func(_ context.Context, id int) error { gotID = id; return nil }}
	h := MarkFeedEntriesReadHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(8)})
	okRes(t, res, err)
	if gotID != 8 {
		t.Errorf("mark feed read id = %d", gotID)
	}
}

func TestMarkCategoryEntriesReadHandler(t *testing.T) {
	var gotID int
	c := &fakeClient{markCatRead: func(_ context.Context, id int) error { gotID = id; return nil }}
	h := MarkCategoryEntriesReadHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"category_id": float64(6)})
	okRes(t, res, err)
	if gotID != 6 {
		t.Errorf("mark category read id = %d", gotID)
	}
}

func TestUpdateEntriesHandler(t *testing.T) {
	var got dmf.UpdateEntriesRequest
	c := &fakeClient{updateEntries: func(_ context.Context, req dmf.UpdateEntriesRequest) error {
		got = req
		return nil
	}}
	h := UpdateEntriesHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{
		"entry_ids": []any{float64(1), float64(2)},
		"status":    "read",
		"starred":   true,
	})
	okRes(t, res, err)
	if len(got.EntryIDs) != 2 || got.EntryIDs[0] != 1 || got.EntryIDs[1] != 2 || got.Status != "read" || got.Starred == nil || !*got.Starred {
		t.Errorf("UpdateEntries req = %+v", got)
	}
}

func TestUpdateEntriesHandlerEmptyIDs(t *testing.T) {
	h := UpdateEntriesHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"entry_ids": []any{}}); err == nil {
		t.Error("expected InvalidParams for empty entry_ids")
	}
}

func TestUpdateEntriesHandlerBadItem(t *testing.T) {
	h := UpdateEntriesHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"entry_ids": []any{"x"}}); err == nil {
		t.Error("expected InvalidParams for non-integer entry id")
	}
}

func TestUpdateEntriesHandlerTooMany(t *testing.T) {
	ids := make([]any, 1001)
	for i := range ids {
		ids[i] = float64(i)
	}
	h := UpdateEntriesHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"entry_ids": ids}); err == nil {
		t.Error("expected InvalidParams for >1000 entry_ids")
	}
}

func TestToggleEntryBookmarkHandler(t *testing.T) {
	var gotID int
	c := &fakeClient{toggleBookmark: func(_ context.Context, id int) error { gotID = id; return nil }}
	h := ToggleEntryBookmarkHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"entry_id": float64(11)})
	okRes(t, res, err)
	if gotID != 11 {
		t.Errorf("toggle bookmark id = %d", gotID)
	}
}

func TestToggleEntryBookmarkHandlerBadID(t *testing.T) {
	h := ToggleEntryBookmarkHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"entry_id": "x"}); err == nil {
		t.Error("expected InvalidParams")
	}
}

func TestUpdateEntryHandler(t *testing.T) {
	var gotID int
	var got dmf.UpdateEntryRequest
	c := &fakeClient{updateEntry: func(_ context.Context, id int, req dmf.UpdateEntryRequest) (*dmf.Entry, error) {
		gotID, got = id, req
		return &dmf.Entry{ID: id, Title: req.Title}, nil
	}}
	h := UpdateEntryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"entry_id": float64(4), "title": "n", "content": "c"})
	out := okRes(t, res, err)
	if gotID != 4 || got.Title != "n" || got.Content != "c" {
		t.Errorf("UpdateEntry got id=%d req=%+v", gotID, got)
	}
	if !strings.Contains(out, `"title":"n"`) {
		t.Errorf("missing updated entry: %s", out)
	}
}

func TestUpdateEntryHandlerUpstream(t *testing.T) {
	c := &fakeClient{updateEntry: func(context.Context, int, dmf.UpdateEntryRequest) (*dmf.Entry, error) {
		return nil, errSentinel
	}}
	h := UpdateEntryHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"entry_id": float64(1)})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestImportOPMLHandler(t *testing.T) {
	var got string
	c := &fakeClient{importOPML: func(_ context.Context, doc string) error { got = doc; return nil }}
	h := ImportOPMLHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"opml": "<opml/>"})
	okRes(t, res, err)
	if got != "<opml/>" {
		t.Errorf("import opml doc = %q", got)
	}
}

func TestImportOPMLHandlerMissing(t *testing.T) {
	h := ImportOPMLHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{}); err == nil {
		t.Error("expected InvalidParams for missing opml")
	}
}

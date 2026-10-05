package handlers

import (
	"context"
	"testing"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

func TestCreateFeedURLWrongType(t *testing.T) {
	h := CreateFeedHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"feed_url": 5}); err == nil {
		t.Error("expected InvalidParams for non-string feed_url")
	}
}

func TestCreateFeedUsernameWrongType(t *testing.T) {
	c := &fakeClient{listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) { return nil, nil }}
	h := CreateFeedHandler{Client: c}
	if _, err := h.Call(context.Background(), map[string]any{"feed_url": "https://x", "username": 5}); err == nil {
		t.Error("expected InvalidParams for non-string username")
	}
}

func TestCreateFeedPasswordWrongType(t *testing.T) {
	c := &fakeClient{listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) { return nil, nil }}
	h := CreateFeedHandler{Client: c}
	if _, err := h.Call(context.Background(), map[string]any{"feed_url": "https://x", "password": 5}); err == nil {
		t.Error("expected InvalidParams for non-string password")
	}
}

func TestCreateFeedUpstreamOnCreate(t *testing.T) {
	c := &fakeClient{
		listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) { return nil, nil },
		createFeed: func(context.Context, dmf.CreateFeedRequest) (*dmf.Feed, error) {
			return nil, errSentinel
		},
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

func TestUpdateFeedStringErrors(t *testing.T) {
	fields := []string{"site_url", "username", "password", "user_agent", "scraper_rules", "rewrite_rules"}
	for _, field := range fields {
		h := UpdateFeedHandler{Client: &fakeClient{}}
		args := map[string]any{"feed_id": float64(1), field: 123}
		if _, err := h.Call(context.Background(), args); err == nil {
			t.Errorf("expected InvalidParams for non-string %s", field)
		}
	}
}

func TestRefreshFeedBadID(t *testing.T) {
	h := RefreshFeedHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"feed_id": "x"}); err == nil {
		t.Error("expected InvalidParams")
	}
}

func TestCreateCategoryTitleWrongType(t *testing.T) {
	h := CreateCategoryHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"title": 5}); err == nil {
		t.Error("expected InvalidParams for non-string title")
	}
}

func TestCreateCategoryUpstream(t *testing.T) {
	c := &fakeClient{
		listCategories: func(context.Context) ([]dmf.Category, error) { return nil, errSentinel },
	}
	h := CreateCategoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("want IsError, got %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestUpdateCategoryTitleWrongType(t *testing.T) {
	h := UpdateCategoryHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"category_id": float64(1), "title": 5}); err == nil {
		t.Error("expected InvalidParams for non-string title")
	}
}

func TestRefreshCategoryBadID(t *testing.T) {
	h := RefreshCategoryHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"category_id": "x"}); err == nil {
		t.Error("expected InvalidParams")
	}
}

func TestRefreshCategoryUpstream(t *testing.T) {
	c := &fakeClient{refreshCategory: func(context.Context, int) error { return errSentinel }}
	h := RefreshCategoryHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"category_id": float64(1)})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestMarkFeedEntriesReadBadID(t *testing.T) {
	h := MarkFeedEntriesReadHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"feed_id": "x"}); err == nil {
		t.Error("expected InvalidParams")
	}
}

func TestMarkFeedEntriesReadUpstream(t *testing.T) {
	c := &fakeClient{markFeedRead: func(context.Context, int) error { return errSentinel }}
	h := MarkFeedEntriesReadHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"feed_id": float64(1)})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestMarkCategoryEntriesReadBadID(t *testing.T) {
	h := MarkCategoryEntriesReadHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"category_id": "x"}); err == nil {
		t.Error("expected InvalidParams")
	}
}

func TestMarkCategoryEntriesReadUpstream(t *testing.T) {
	c := &fakeClient{markCatRead: func(context.Context, int) error { return errSentinel }}
	h := MarkCategoryEntriesReadHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"category_id": float64(1)})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestToggleEntryBookmarkUpstream(t *testing.T) {
	c := &fakeClient{toggleBookmark: func(context.Context, int) error { return errSentinel }}
	h := ToggleEntryBookmarkHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"entry_id": float64(1)})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestUpdateEntryStringErrors(t *testing.T) {
	fields := []string{"title", "content", "url"}
	for _, field := range fields {
		h := UpdateEntryHandler{Client: &fakeClient{}}
		if _, err := h.Call(context.Background(), map[string]any{"entry_id": float64(1), field: 5}); err == nil {
			t.Errorf("expected InvalidParams for non-string %s", field)
		}
	}
}

func TestImportOPMLWrongType(t *testing.T) {
	h := ImportOPMLHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"opml": 5}); err == nil {
		t.Error("expected InvalidParams for non-string opml")
	}
}

func TestImportOPMLUpstream(t *testing.T) {
	c := &fakeClient{importOPML: func(context.Context, string) error { return errSentinel }}
	h := ImportOPMLHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"opml": "<x/>"})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestUpdateEntriesUpstream(t *testing.T) {
	c := &fakeClient{updateEntries: func(context.Context, dmf.UpdateEntriesRequest) error { return errSentinel }}
	h := UpdateEntriesHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"entry_ids": []any{float64(1)}})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestUpdateCategoryBadID(t *testing.T) {
	h := UpdateCategoryHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"category_id": "x", "title": "t"}); err == nil {
		t.Error("expected InvalidParams")
	}
}

func TestEntryFilterStatusWrongType(t *testing.T) {
	if _, err := entryFilter(map[string]any{"status": 5}, false); err == nil {
		t.Error("expected InvalidParams for non-string status")
	}
}

func TestEntryFilterOrderWrongType(t *testing.T) {
	if _, err := entryFilter(map[string]any{"order": 5}, false); err == nil {
		t.Error("expected InvalidParams for non-string order")
	}
}

func TestEntryFilterDirectionWrongType(t *testing.T) {
	if _, err := entryFilter(map[string]any{"direction": 5}, false); err == nil {
		t.Error("expected InvalidParams for non-string direction")
	}
}

func TestEntryFilterOffsetWrongType(t *testing.T) {
	if _, err := entryFilter(map[string]any{"offset": "x"}, false); err == nil {
		t.Error("expected InvalidParams for bad offset")
	}
}

func TestEntryFilterSearchWrongType(t *testing.T) {
	if _, err := entryFilter(map[string]any{"search": 5}, false); err == nil {
		t.Error("expected InvalidParams for non-string search")
	}
}

func TestEntryFilterAfterWrongType(t *testing.T) {
	if _, err := entryFilter(map[string]any{"after": "nope"}, false); err == nil {
		t.Error("expected InvalidParams for bad after")
	}
}

func TestEntryFilterStarredWrongType(t *testing.T) {
	if _, err := entryFilter(map[string]any{"starred": "x"}, false); err == nil {
		t.Error("expected InvalidParams for bad starred")
	}
}

func TestDeleteCategoryBadID(t *testing.T) {
	h := DeleteCategoryHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"category_id": "x", "confirm": true}); err == nil {
		t.Error("expected InvalidParams for bad category_id")
	}
}

func TestCreateCategoryCreateUpstream(t *testing.T) {
	c := &fakeClient{
		listCategories: func(context.Context) ([]dmf.Category, error) { return nil, nil },
		createCategory: func(context.Context, string) (*dmf.Category, error) {
			return nil, errSentinel
		},
	}
	h := CreateCategoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("want IsError, got %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestUpdateEntryBadID(t *testing.T) {
	h := UpdateEntryHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"entry_id": "x"}); err == nil {
		t.Error("expected InvalidParams")
	}
}

func TestDiscoverSubscriptionsURLConditionWrongType(t *testing.T) {
	h := DiscoverSubscriptionsHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"url": 123}); err == nil {
		t.Error("expected InvalidParams for non-string url")
	}
}

func TestCreateFeedListFeedsEmptyNil(t *testing.T) {
	// listFeeds returning nil (no existing feeds) still proceeds to create.
	var created bool
	c := &fakeClient{
		listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) { return nil, nil },
		createFeed: func(context.Context, dmf.CreateFeedRequest) (*dmf.Feed, error) {
			created = true
			return &dmf.Feed{ID: 1}, nil
		},
	}
	h := CreateFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_url": "https://x"})
	okRes(t, res, err)
	if !created {
		t.Error("expected feed to be created when no existing feeds")
	}
}

func TestCreateCategoryListEmptyNil(t *testing.T) {
	var created bool
	c := &fakeClient{
		listCategories: func(context.Context) ([]dmf.Category, error) { return nil, nil },
		createCategory: func(context.Context, string) (*dmf.Category, error) {
			created = true
			return &dmf.Category{ID: 1}, nil
		},
	}
	h := CreateCategoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"title": "x"})
	okRes(t, res, err)
	if !created {
		t.Error("expected category to be created when no existing categories")
	}
}

func TestDeleteFeedFractionalID(t *testing.T) {
	h := DeleteFeedHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"feed_id": 1.5, "confirm": true}); err == nil {
		t.Error("expected InvalidParams for fractional feed_id")
	}
}

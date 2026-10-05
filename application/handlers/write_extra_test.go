package handlers

import (
	"context"
	"strings"
	"testing"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

func TestUpdateFeedHandlerAllFields(t *testing.T) {
	var got dmf.UpdateFeedRequest
	c := &fakeClient{updateFeed: func(_ context.Context, _ int, req dmf.UpdateFeedRequest) (*dmf.Feed, error) {
		got = req
		return &dmf.Feed{ID: 1}, nil
	}}
	h := UpdateFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{
		"feed_id":       float64(1),
		"title":         "t",
		"category_id":   float64(2),
		"site_url":      "https://s",
		"username":      "u",
		"password":      "p",
		"user_agent":    "ua",
		"scraper_rules": "sr",
		"rewrite_rules": "rr",
		"crawler":       true,
	})
	okRes(t, res, err)
	if got.Title != "t" || got.CategoryID == nil || *got.CategoryID != 2 || got.SiteURL != "https://s" ||
		got.Username != "u" || got.Password != "p" || got.UserAgent != "ua" ||
		got.ScraperRules != "sr" || got.RewriteRules != "rr" || got.Crawler == nil || !*got.Crawler {
		t.Errorf("UpdateFeed req not fully populated: %+v", got)
	}
}

func TestUpdateFeedHandlerRedactsPassword(t *testing.T) {
	c := &fakeClient{updateFeed: func(context.Context, int, dmf.UpdateFeedRequest) (*dmf.Feed, error) {
		return &dmf.Feed{ID: 1, Password: "secretpw"}, nil
	}}
	h := UpdateFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(1)})
	out := okRes(t, res, err)
	if strings.Contains(out, "secretpw") {
		t.Errorf("output leaked password (S02): %s", out)
	}
}

func TestUpdateFeedHandlerBadCrawler(t *testing.T) {
	h := UpdateFeedHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"feed_id": float64(1), "crawler": "x"}); err == nil {
		t.Error("expected InvalidParams for bad crawler")
	}
}

func TestUpdateFeedHandlerBadCategory(t *testing.T) {
	h := UpdateFeedHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"feed_id": float64(1), "category_id": "x"}); err == nil {
		t.Error("expected InvalidParams for bad category_id")
	}
}

func TestUpdateFeedHandlerBadTitle(t *testing.T) {
	h := UpdateFeedHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"feed_id": float64(1), "title": 5}); err == nil {
		t.Error("expected InvalidParams for non-string title")
	}
}

func TestUpdateFeedHandlerUpstream(t *testing.T) {
	c := &fakeClient{updateFeed: func(context.Context, int, dmf.UpdateFeedRequest) (*dmf.Feed, error) {
		return nil, errSentinel
	}}
	h := UpdateFeedHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"feed_id": float64(1)})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestCreateFeedHandlerBadTitle(t *testing.T) {
	c := &fakeClient{
		listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) { return nil, nil },
	}
	h := CreateFeedHandler{Client: c}
	if _, err := h.Call(context.Background(), map[string]any{"feed_url": "https://x", "title": 5}); err == nil {
		t.Error("expected InvalidParams for non-string title")
	}
}

func TestCreateFeedHandlerCategorySet(t *testing.T) {
	var got dmf.CreateFeedRequest
	c := &fakeClient{
		listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) { return nil, nil },
		createFeed: func(_ context.Context, req dmf.CreateFeedRequest) (*dmf.Feed, error) {
			got = req
			return &dmf.Feed{ID: 1}, nil
		},
	}
	h := CreateFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_url": "https://x", "category_id": float64(7)})
	okRes(t, res, err)
	if got.CategoryID == nil || *got.CategoryID != 7 {
		t.Errorf("CreateFeed category not set: %+v", got)
	}
}

func TestUpdateEntriesHandlerBadStatus(t *testing.T) {
	h := UpdateEntriesHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"entry_ids": []any{float64(1)}, "status": 5}); err == nil {
		t.Error("expected InvalidParams for non-string status")
	}
}

func TestUpdateEntriesHandlerBadStarred(t *testing.T) {
	h := UpdateEntriesHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"entry_ids": []any{float64(1)}, "starred": "x"}); err == nil {
		t.Error("expected InvalidParams for non-boolean starred")
	}
}

func TestListFeedsHandlerBadLimit(t *testing.T) {
	h := ListFeedsHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"limit": "x"}); err == nil {
		t.Error("expected InvalidParams for bad limit")
	}
}

func TestListFeedsHandlerBadOffset(t *testing.T) {
	h := ListFeedsHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"offset": "x"}); err == nil {
		t.Error("expected InvalidParams for bad offset")
	}
}

func TestGetFeedEntriesHandlerFilterError(t *testing.T) {
	h := GetFeedEntriesHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"feed_id": float64(1), "limit": "x"}); err == nil {
		t.Error("expected InvalidParams for bad limit")
	}
}

func TestGetFeedEntriesHandlerUpstream(t *testing.T) {
	c := &fakeClient{getFeedEntries: func(context.Context, int, dmf.EntryFilter) (dmf.FeedEntries, error) {
		return dmf.FeedEntries{}, errSentinel
	}}
	h := GetFeedEntriesHandler{Client: c}
	res, _ := h.Call(context.Background(), map[string]any{"feed_id": float64(1)})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestGetCountersHandlerUpstream(t *testing.T) {
	c := &fakeClient{getCounters: func(context.Context) (*dmf.Counters, error) { return nil, errSentinel }}
	h := GetCountersHandler{Client: c}
	res, _ := h.Call(context.Background(), nil)
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestGetMeHandlerUpstream(t *testing.T) {
	c := &fakeClient{getMe: func(context.Context) (*dmf.Me, error) { return nil, errSentinel }}
	h := GetMeHandler{Client: c}
	res, _ := h.Call(context.Background(), nil)
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestListCategoriesTokenThreaded(t *testing.T) {
	c := &fakeClient{listCategories: func(context.Context) ([]dmf.Category, error) {
		return nil, nil
	}}
	h := ListCategoriesHandler{Client: c}
	res, err := h.Call(tokCtx(t), nil)
	okRes(t, res, err)
	if c.token != "tok-123" {
		t.Errorf("token not threaded: %q", c.token)
	}
}

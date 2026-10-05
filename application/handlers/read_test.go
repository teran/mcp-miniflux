package handlers

import (
	"context"
	"strings"
	"testing"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

func TestListFeedsHandlerHappy(t *testing.T) {
	c := &fakeClient{
		listFeeds: func(_ context.Context, _ *int, _, _ int) ([]dmf.Feed, error) {
			return []dmf.Feed{{ID: 1, Title: "a", Password: "secret"}}, nil
		},
	}
	h := ListFeedsHandler{Client: c}
	res, err := h.Call(tokCtx(t), map[string]any{"category_id": float64(2)})
	out := okRes(t, res, err)
	if c.token != "tok-123" {
		t.Errorf("token not threaded to client: %q", c.token)
	}
	if strings.Contains(out, "secret") {
		t.Errorf("output leaked feed password (S02): %s", out)
	}
	if !strings.Contains(out, `"title":"a"`) {
		t.Errorf("missing feed in output: %s", out)
	}
}

func TestListFeedsHandlerBadCategory(t *testing.T) {
	h := ListFeedsHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"category_id": "x"}); err == nil {
		t.Error("expected InvalidParams for non-integer category_id")
	}
}

func TestListFeedsHandlerUpstreamError(t *testing.T) {
	h := ListFeedsHandler{Client: &fakeClient{listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) {
		return nil, errSentinel
	}}}
	res, err := h.Call(context.Background(), nil)
	if err != nil {
		t.Fatalf("upstream error should map to IsError result, got error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError result for upstream failure")
	}
}

func TestGetFeedHandler(t *testing.T) {
	c := &fakeClient{getFeed: func(_ context.Context, id int) (*dmf.Feed, error) {
		return &dmf.Feed{ID: id, Title: "f", Password: "p"}, nil
	}}
	h := GetFeedHandler{Client: c}
	res, err := h.Call(tokCtx(t), map[string]any{"feed_id": float64(7)})
	out := okRes(t, res, err)
	if strings.Contains(out, `"password":"p"`) {
		t.Errorf("output leaked feed password (S02): %s", out)
	}
	if !strings.Contains(out, `"id":7`) {
		t.Errorf("missing feed id in output: %s", out)
	}
}

func TestGetFeedHandlerBadID(t *testing.T) {
	h := GetFeedHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"feed_id": "nope"}); err == nil {
		t.Error("expected InvalidParams for non-integer feed_id")
	}
}

func TestGetFeedHandlerUpstream(t *testing.T) {
	h := GetFeedHandler{Client: &fakeClient{getFeed: func(context.Context, int) (*dmf.Feed, error) {
		return nil, errSentinel
	}}}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(1)})
	if err != nil {
		t.Fatalf("want IsError result, got error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestListCategoriesHandler(t *testing.T) {
	c := &fakeClient{listCategories: func(context.Context) ([]dmf.Category, error) {
		return []dmf.Category{{ID: 1, Title: "tech"}}, nil
	}}
	h := ListCategoriesHandler{Client: c}
	res, err := h.Call(context.Background(), nil)
	out := okRes(t, res, err)
	if !strings.Contains(out, `"title":"tech"`) {
		t.Errorf("missing category: %s", out)
	}
}

func TestListCategoriesHandlerUpstream(t *testing.T) {
	h := ListCategoriesHandler{Client: &fakeClient{listCategories: func(context.Context) ([]dmf.Category, error) {
		return nil, errSentinel
	}}}
	res, _ := h.Call(context.Background(), nil)
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestListEntriesHandlerHappy(t *testing.T) {
	c := &fakeClient{listEntries: func(_ context.Context, f dmf.EntryFilter) (dmf.FeedEntries, error) {
		if f.Limit != 100 {
			t.Errorf("limit = %d, want default 100", f.Limit)
		}
		return dmf.FeedEntries{Total: 1, Entries: []dmf.Entry{{ID: 3, Title: "e"}}}, nil
	}}
	h := ListEntriesHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"status": "unread", "starred": true})
	out := okRes(t, res, err)
	if !strings.Contains(out, `"title":"e"`) {
		t.Errorf("missing entry: %s", out)
	}
}

func TestListEntriesHandlerLimitTooHigh(t *testing.T) {
	h := ListEntriesHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"limit": float64(5000)}); err == nil {
		t.Error("expected InvalidParams for limit > 1000")
	}
}

func TestListEntriesHandlerBadStarred(t *testing.T) {
	h := ListEntriesHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"starred": "yes"}); err == nil {
		t.Error("expected InvalidParams for non-boolean starred")
	}
}

func TestListEntriesHandlerUpstream(t *testing.T) {
	h := ListEntriesHandler{Client: &fakeClient{listEntries: func(context.Context, dmf.EntryFilter) (dmf.FeedEntries, error) {
		return dmf.FeedEntries{}, errSentinel
	}}}
	res, _ := h.Call(context.Background(), nil)
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestGetEntryHandler(t *testing.T) {
	c := &fakeClient{getEntry: func(_ context.Context, id int) (*dmf.Entry, error) {
		return &dmf.Entry{ID: id, Title: "x"}, nil
	}}
	h := GetEntryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"entry_id": float64(9)})
	out := okRes(t, res, err)
	if !strings.Contains(out, `"id":9`) {
		t.Errorf("missing entry: %s", out)
	}
}

func TestGetEntryHandlerBadID(t *testing.T) {
	h := GetEntryHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"entry_id": nil}); err == nil {
		t.Error("expected InvalidParams")
	}
}

func TestGetEntryHandlerUpstream(t *testing.T) {
	h := GetEntryHandler{Client: &fakeClient{getEntry: func(context.Context, int) (*dmf.Entry, error) {
		return nil, errSentinel
	}}}
	res, _ := h.Call(context.Background(), map[string]any{"entry_id": float64(1)})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestGetFeedEntriesHandler(t *testing.T) {
	c := &fakeClient{getFeedEntries: func(_ context.Context, feedID int, _ dmf.EntryFilter) (dmf.FeedEntries, error) {
		return dmf.FeedEntries{Total: 1, Entries: []dmf.Entry{{ID: 1, FeedID: feedID}}}, nil
	}}
	h := GetFeedEntriesHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(5)})
	out := okRes(t, res, err)
	if !strings.Contains(out, `"feed_id":5`) {
		t.Errorf("missing feed entry: %s", out)
	}
}

func TestGetFeedEntriesHandlerBadID(t *testing.T) {
	h := GetFeedEntriesHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{}); err == nil {
		t.Error("expected InvalidParams for missing feed_id")
	}
}

func TestGetCountersHandler(t *testing.T) {
	c := &fakeClient{getCounters: func(context.Context) (*dmf.Counters, error) {
		return &dmf.Counters{Totals: dmf.CounterTotals{Unread: 3}}, nil
	}}
	h := GetCountersHandler{Client: c}
	res, err := h.Call(context.Background(), nil)
	out := okRes(t, res, err)
	if !strings.Contains(out, `"unread":3`) {
		t.Errorf("missing counters: %s", out)
	}
}

func TestGetMeHandler(t *testing.T) {
	c := &fakeClient{getMe: func(context.Context) (*dmf.Me, error) {
		return &dmf.Me{ID: 1, Username: "u"}, nil
	}}
	h := GetMeHandler{Client: c}
	res, err := h.Call(context.Background(), nil)
	out := okRes(t, res, err)
	if !strings.Contains(out, `"username":"u"`) {
		t.Errorf("missing me: %s", out)
	}
}

func TestExportOPMLHandler(t *testing.T) {
	c := &fakeClient{exportOPML: func(context.Context) (string, error) {
		return "<opml/>", nil
	}}
	h := ExportOPMLHandler{Client: c}
	res, err := h.Call(context.Background(), nil)
	out := okRes(t, res, err)
	if !strings.Contains(out, `"opml":`) {
		t.Errorf("missing opml field: %s", out)
	}
}

func TestExportOPMLHandlerUpstream(t *testing.T) {
	h := ExportOPMLHandler{Client: &fakeClient{exportOPML: func(context.Context) (string, error) {
		return "", errSentinel
	}}}
	res, _ := h.Call(context.Background(), nil)
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

func TestDiscoverSubscriptionsHappy(t *testing.T) {
	c := &fakeClient{discover: func(_ context.Context, url string) ([]dmf.DiscoveryResult, error) {
		if url != "https://example.com/feed" {
			t.Errorf("discover url = %q", url)
		}
		return []dmf.DiscoveryResult{{URL: "https://example.com/rss", Title: "Example", Type: "rss"}}, nil
	}}
	h := DiscoverSubscriptionsHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"url": "https://example.com/feed"})
	okRes(t, res, err)
	if res.StructuredContent == nil {
		t.Fatal("expected structured content (S07)")
	}
	cands, ok := res.StructuredContent.([]dmf.DiscoveryResult)
	if !ok {
		t.Fatalf("structured content type %T, want []dmf.DiscoveryResult", res.StructuredContent)
	}
	if !strings.Contains(cands[0].URL, "rss") {
		t.Errorf("structured content missing candidate: %v", cands)
	}
}

func TestDiscoverSubscriptionsRelativeURL(t *testing.T) {
	h := DiscoverSubscriptionsHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"url": "/relative"}); err == nil {
		t.Error("expected InvalidParams for relative URL (S11)")
	}
}

func TestDiscoverSubscriptionsBadScheme(t *testing.T) {
	h := DiscoverSubscriptionsHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{"url": "ftp://x/y"}); err == nil {
		t.Error("expected InvalidParams for non-http scheme (S11)")
	}
}

func TestDiscoverSubscriptionsMissingURL(t *testing.T) {
	h := DiscoverSubscriptionsHandler{Client: &fakeClient{}}
	if _, err := h.Call(context.Background(), map[string]any{}); err == nil {
		t.Error("expected InvalidParams for missing url")
	}
}

func TestDiscoverSubscriptionsUpstream(t *testing.T) {
	h := DiscoverSubscriptionsHandler{Client: &fakeClient{discover: func(context.Context, string) ([]dmf.DiscoveryResult, error) {
		return nil, errSentinel
	}}}
	res, _ := h.Call(context.Background(), map[string]any{"url": "https://example.com"})
	if !res.IsError {
		t.Error("expected IsError result")
	}
}

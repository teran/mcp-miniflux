package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// CONTRACT — feed freshness reaches tool output. list_feeds / get_feed emit a
// Feed object and get_feed_entries emits a Feed nested inside each entry
// (Miniflux nests the feed object in every entry). The freshness signal
// (checked_at / next_check_at) must pass through to the JSON response.

var (
	feedFreshChecked = time.Date(2026, 10, 1, 12, 30, 45, 0, time.UTC)
	feedFreshNext    = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
)

func TestListFeedsHandlerExposesFreshness(t *testing.T) {
	c := &fakeClient{listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) {
		return []dmf.Feed{{ID: 1, CheckedAt: feedFreshChecked, NextCheckAt: feedFreshNext}}, nil
	}}
	h := ListFeedsHandler{Client: c}
	res, err := h.Call(context.Background(), nil)
	assertFeedFreshness(t, okRes(t, res, err))
}

func TestGetFeedHandlerExposesFreshness(t *testing.T) {
	c := &fakeClient{getFeed: func(context.Context, int) (*dmf.Feed, error) {
		return &dmf.Feed{ID: 1, CheckedAt: feedFreshChecked, NextCheckAt: feedFreshNext}, nil
	}}
	h := GetFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(1)})
	assertFeedFreshness(t, okRes(t, res, err))
}

// TestGetFeedEntriesHandlerExposesNestedFeedFreshness verifies the freshness
// signal carried by the feed object nested inside each entry of
// get_feed_entries reaches the JSON response.
func TestGetFeedEntriesHandlerExposesNestedFeedFreshness(t *testing.T) {
	c := &fakeClient{getFeedEntries: func(_ context.Context, feedID int, _ dmf.EntryFilter) (dmf.FeedEntries, error) {
		return dmf.FeedEntries{Total: 1, Entries: []dmf.Entry{{
			ID: 1, FeedID: feedID,
			Feed: dmf.Feed{ID: feedID, CheckedAt: feedFreshChecked, NextCheckAt: feedFreshNext},
		}}}, nil
	}}
	h := GetFeedEntriesHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(5)})
	assertFeedFreshness(t, okRes(t, res, err))
}

func assertFeedFreshness(t *testing.T, out string) {
	t.Helper()
	if !strings.Contains(out, `"checked_at":"2026-10-01T12:30:45Z"`) {
		t.Errorf("output missing checked_at: %s", out)
	}
	if !strings.Contains(out, `"next_check_at":"2026-10-02T00:00:00Z"`) {
		t.Errorf("output missing next_check_at: %s", out)
	}
}

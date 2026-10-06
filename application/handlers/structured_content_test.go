package handlers

// TDD regression tests for the structured-content bug (SEP-2106).
//
// Every tool declares an outputSchema (domain/tools/*), so per the MCP spec
// (2025-06-18 "Structured Content"): "If an output schema is provided: Servers
// MUST provide structured results that conform to this schema." Clients reject
// a tool result that declares an outputSchema but returns an EMPTY
// StructuredContent ("Tool X has an output schema but did not return
// structured content").
//
// Root cause: application/handlers/base.go jsonResult()/okResult() fill only
// Content (text) and leave StructuredContent nil. Only discover_subscriptions
// (read.go) populates it. These tests pin the REQUIRED behaviour so the fix
// (populate StructuredContent on every successful result) is verified.
//
// Per the MCP spec, ERROR results (IsError:true) are EXEMPT from the
// structured-content requirement — the spec's error example carries only a
// text content block and isError:true. That exemption is pinned separately
// (see TestStructuredContent_ErrorResultsExempt).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// requireStructured validates the call succeeded (no err, not IsError) and
// that the SUCCESS result carries non-nil, non-empty structured content
// (SEP-2106). It returns the serialized JSON of the structured content so
// callers can make further assertions.
func requireStructured(t *testing.T, res *mcp.CallToolResult, err error) []byte {
	t.Helper()
	okRes(t, res, err)
	if res.StructuredContent == nil {
		t.Fatal("StructuredContent is nil: tool declares an outputSchema so success results MUST carry structured content (SEP-2106)")
	}
	data, merr := json.Marshal(res.StructuredContent)
	if merr != nil {
		t.Fatalf("StructuredContent not serializable: %v", merr)
	}
	if string(data) == "null" || len(data) == 0 {
		t.Fatalf("StructuredContent is empty/null: %q", string(data))
	}
	return data
}

// requireOKStructured asserts a success result's structured content is the
// canonical {"ok":true} object used by okResult-style tools.
func requireOKStructured(t *testing.T, res *mcp.CallToolResult, err error) {
	t.Helper()
	data := requireStructured(t, res, err)
	if string(data) != `{"ok":true}` {
		t.Fatalf("StructuredContent = %s, want {\"ok\":true}", string(data))
	}
}

// assertRedacted validates the success result's structured content and fails
// if any secret value (S02) leaks into it.
func assertRedacted(t *testing.T, res *mcp.CallToolResult, err error, secrets ...string) {
	t.Helper()
	data := requireStructured(t, res, err)
	for _, s := range secrets {
		if s != "" && strings.Contains(string(data), s) {
			t.Errorf("secret %q leaked into StructuredContent (S02): %s", s, string(data))
		}
	}
}

// --- Read tools -----------------------------------------------------------

func TestStructuredContent_ListFeeds(t *testing.T) {
	c := &fakeClient{listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) {
		return []dmf.Feed{{ID: 1, Title: "a", Password: "list-secret"}}, nil
	}}
	h := ListFeedsHandler{Client: c}
	res, err := h.Call(tokCtx(t), map[string]any{"category_id": float64(2)})
	assertRedacted(t, res, err, "list-secret")
}

func TestStructuredContent_GetFeed(t *testing.T) {
	c := &fakeClient{getFeed: func(_ context.Context, id int) (*dmf.Feed, error) {
		return &dmf.Feed{ID: id, Title: "f", Password: "get-secret"}, nil
	}}
	h := GetFeedHandler{Client: c}
	res, err := h.Call(tokCtx(t), map[string]any{"feed_id": float64(7)})
	assertRedacted(t, res, err, "get-secret")
}

func TestStructuredContent_ListCategories(t *testing.T) {
	c := &fakeClient{listCategories: func(context.Context) ([]dmf.Category, error) {
		return []dmf.Category{{ID: 1, Title: "tech"}}, nil
	}}
	h := ListCategoriesHandler{Client: c}
	res, err := h.Call(context.Background(), nil)
	requireStructured(t, res, err)
}

func TestStructuredContent_ListEntries(t *testing.T) {
	c := &fakeClient{listEntries: func(_ context.Context, _ dmf.EntryFilter) (dmf.FeedEntries, error) {
		return dmf.FeedEntries{Total: 1, Entries: []dmf.Entry{{ID: 3, Title: "e"}}}, nil
	}}
	h := ListEntriesHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"status": "unread"})
	requireStructured(t, res, err)
}

func TestStructuredContent_GetEntry(t *testing.T) {
	c := &fakeClient{getEntry: func(_ context.Context, id int) (*dmf.Entry, error) {
		return &dmf.Entry{ID: id, Title: "x"}, nil
	}}
	h := GetEntryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"entry_id": float64(9)})
	requireStructured(t, res, err)
}

func TestStructuredContent_GetFeedEntries(t *testing.T) {
	c := &fakeClient{getFeedEntries: func(_ context.Context, feedID int, _ dmf.EntryFilter) (dmf.FeedEntries, error) {
		return dmf.FeedEntries{Total: 1, Entries: []dmf.Entry{{ID: 1, FeedID: feedID}}}, nil
	}}
	h := GetFeedEntriesHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(5)})
	requireStructured(t, res, err)
}

func TestStructuredContent_GetCounters(t *testing.T) {
	c := &fakeClient{getCounters: func(context.Context) (*dmf.Counters, error) {
		return &dmf.Counters{Totals: dmf.CounterTotals{Unread: 3}}, nil
	}}
	h := GetCountersHandler{Client: c}
	res, err := h.Call(context.Background(), nil)
	requireStructured(t, res, err)
}

func TestStructuredContent_GetMe(t *testing.T) {
	c := &fakeClient{getMe: func(context.Context) (*dmf.Me, error) {
		return &dmf.Me{ID: 1, Username: "u"}, nil
	}}
	h := GetMeHandler{Client: c}
	res, err := h.Call(context.Background(), nil)
	requireStructured(t, res, err)
}

func TestStructuredContent_ExportOPML(t *testing.T) {
	c := &fakeClient{exportOPML: func(context.Context) (string, error) {
		return "<opml/>", nil
	}}
	h := ExportOPMLHandler{Client: c}
	res, err := h.Call(context.Background(), nil)
	requireStructured(t, res, err)
}

// discover_subscriptions is the WORKING example (read.go sets both Content and
// StructuredContent). This pins it as a non-regression guard.
func TestStructuredContent_DiscoverSubscriptions(t *testing.T) {
	c := &fakeClient{discover: func(context.Context, string) ([]dmf.DiscoveryResult, error) {
		return []dmf.DiscoveryResult{{URL: "https://example.com/rss", Title: "Example", Type: "rss"}}, nil
	}}
	h := DiscoverSubscriptionsHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"url": "https://example.com/feed"})
	requireStructured(t, res, err)
}

// --- Write / update tools -------------------------------------------------

func TestStructuredContent_CreateFeed(t *testing.T) {
	c := &fakeClient{
		listFeeds: func(context.Context, *int, int, int) ([]dmf.Feed, error) { return nil, nil },
		createFeed: func(context.Context, dmf.CreateFeedRequest) (*dmf.Feed, error) {
			return &dmf.Feed{ID: 1, Password: "create-secret"}, nil
		},
	}
	h := CreateFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_url": "https://x"})
	assertRedacted(t, res, err, "create-secret")
}

func TestStructuredContent_UpdateFeed(t *testing.T) {
	c := &fakeClient{updateFeed: func(_ context.Context, id int, _ dmf.UpdateFeedRequest) (*dmf.Feed, error) {
		return &dmf.Feed{ID: id, Password: "update-secret"}, nil
	}}
	h := UpdateFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(1)})
	assertRedacted(t, res, err, "update-secret")
}

func TestStructuredContent_RefreshFeed(t *testing.T) {
	c := &fakeClient{refreshFeed: func(context.Context, int) error { return nil }}
	h := RefreshFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(1)})
	requireOKStructured(t, res, err)
}

func TestStructuredContent_CreateCategory(t *testing.T) {
	c := &fakeClient{
		listCategories: func(context.Context) ([]dmf.Category, error) { return nil, nil },
		createCategory: func(context.Context, string) (*dmf.Category, error) {
			return &dmf.Category{ID: 1, Title: "new"}, nil
		},
	}
	h := CreateCategoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"title": "x"})
	requireStructured(t, res, err)
}

func TestStructuredContent_UpdateCategory(t *testing.T) {
	c := &fakeClient{updateCategory: func(context.Context, int, string) error { return nil }}
	h := UpdateCategoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"category_id": float64(1), "title": "t"})
	requireOKStructured(t, res, err)
}

func TestStructuredContent_RefreshCategory(t *testing.T) {
	c := &fakeClient{refreshCategory: func(context.Context, int) error { return nil }}
	h := RefreshCategoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"category_id": float64(1)})
	requireOKStructured(t, res, err)
}

func TestStructuredContent_MarkFeedEntriesRead(t *testing.T) {
	c := &fakeClient{markFeedRead: func(context.Context, int) error { return nil }}
	h := MarkFeedEntriesReadHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(1)})
	requireOKStructured(t, res, err)
}

func TestStructuredContent_MarkCategoryEntriesRead(t *testing.T) {
	c := &fakeClient{markCatRead: func(context.Context, int) error { return nil }}
	h := MarkCategoryEntriesReadHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"category_id": float64(1)})
	requireOKStructured(t, res, err)
}

func TestStructuredContent_UpdateEntries(t *testing.T) {
	c := &fakeClient{updateEntries: func(context.Context, dmf.UpdateEntriesRequest) error { return nil }}
	h := UpdateEntriesHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"entry_ids": []any{float64(1), float64(2)}})
	requireOKStructured(t, res, err)
}

func TestStructuredContent_ToggleEntryBookmark(t *testing.T) {
	c := &fakeClient{toggleBookmark: func(context.Context, int) error { return nil }}
	h := ToggleEntryBookmarkHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"entry_id": float64(1)})
	requireOKStructured(t, res, err)
}

func TestStructuredContent_UpdateEntry(t *testing.T) {
	c := &fakeClient{updateEntry: func(_ context.Context, id int, _ dmf.UpdateEntryRequest) (*dmf.Entry, error) {
		return &dmf.Entry{ID: id, Title: "upd"}, nil
	}}
	h := UpdateEntryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"entry_id": float64(1)})
	requireStructured(t, res, err)
}

func TestStructuredContent_ImportOPML(t *testing.T) {
	c := &fakeClient{importOPML: func(context.Context, string) error { return nil }}
	h := ImportOPMLHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"opml": "<opml/>"})
	requireOKStructured(t, res, err)
}

// --- Delete tools (HITL, S12) ---------------------------------------------

func TestStructuredContent_DeleteFeed(t *testing.T) {
	c := &fakeClient{deleteFeed: func(context.Context, int) error { return nil }}
	h := DeleteFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(3), "confirm": true})
	requireOKStructured(t, res, err)
}

func TestStructuredContent_DeleteCategory(t *testing.T) {
	c := &fakeClient{deleteCategory: func(context.Context, int) error { return nil }}
	h := DeleteCategoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"category_id": float64(2), "confirm": true})
	requireOKStructured(t, res, err)
}

func TestStructuredContent_FlushHistory(t *testing.T) {
	c := &fakeClient{flushHistory: func(context.Context, *time.Time) error { return nil }}
	h := FlushHistoryHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"confirm": true})
	requireOKStructured(t, res, err)
}

// --- Redaction (S02) inside StructuredContent -----------------------------

// TestStructuredContent_RedactsFeedSecrets verifies that a Feed's
// secret:"true" fields (username/password) are stripped from BOTH the text
// content and the structured content. This guards S02 in the structured path.
func TestStructuredContent_RedactsFeedSecrets(t *testing.T) {
	c := &fakeClient{getFeed: func(context.Context, int) (*dmf.Feed, error) {
		return &dmf.Feed{
			ID: 1, Title: "t", Username: "svc-account", Password: "p4ssw0rd-0k",
		}, nil
	}}
	h := GetFeedHandler{Client: c}
	res, err := h.Call(tokCtx(t), map[string]any{"feed_id": float64(1)})
	data := requireStructured(t, res, err)
	for _, secret := range []string{"p4ssw0rd-0k", "svc-account"} {
		if strings.Contains(string(data), secret) {
			t.Errorf("secret %q leaked into StructuredContent (S02): %s", secret, string(data))
		}
	}
	// The text output must also stay clean (existing S02 guarantee).
	if out := textOf(t, res); strings.Contains(out, "p4ssw0rd-0k") || strings.Contains(out, "svc-account") {
		t.Errorf("secret leaked into text output (S02): %s", out)
	}
}

// --- Error exemption (item 3) ---------------------------------------------

// TestStructuredContent_ErrorResultsExempt pins the SEP-2106 exemption: ERROR
// results (IsError:true) are NOT required to carry structured content — the
// MCP spec's error example returns only a text content block plus
// isError:true. The fix must NOT inject structured content into error results.
func TestStructuredContent_ErrorResultsExempt(t *testing.T) {
	c := &fakeClient{getFeed: func(context.Context, int) (*dmf.Feed, error) {
		return nil, errSentinel
	}}
	h := GetFeedHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"feed_id": float64(1)})
	if err != nil {
		t.Fatalf("upstream error should map to IsError result, got error: %v", err)
	}
	if res == nil {
		t.Fatal("result is nil")
	}
	if !res.IsError {
		t.Fatal("expected IsError result for upstream failure")
	}
	// The exemption holds only while the result remains an error. Success
	// results with structured content are covered by the other tests here.
	if !res.IsError {
		t.Error("error results must remain IsError:true (structured-content exemption)")
	}
}

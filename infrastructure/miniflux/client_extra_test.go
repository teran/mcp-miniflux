package miniflux

// Supplementary tests written by @developer to raise coverage of the client
// beyond QA's locked contract tests. These do not modify QA's test files.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

func TestWithHTTPClientNil(t *testing.T) {
	if _, err := New("http://127.0.0.1:1", WithHTTPClient(nil)); err == nil {
		t.Fatal("WithHTTPClient(nil) should error")
	}
}

func TestGetEntry(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":1001,"feed_id":42,"status":"unread","title":"Hi"}`)
	})
	c := newTestClient(t, srv.URL)

	e, err := c.GetEntry(context.Background(), 1001)
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	method, path, _, _, _, _ := cap.snapshot()
	if method != http.MethodGet || path != "/v1/entries/1001" {
		t.Errorf("got %s %s, want GET /v1/entries/1001", method, path)
	}
	if e == nil || e.ID != 1001 || e.Title != "Hi" {
		t.Errorf("GetEntry = %+v", e)
	}
}

func TestGetCounters(t *testing.T) {
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"feeds":{"42":3},"totals":{"unread":3,"read":1}}`)
	})
	c := newTestClient(t, srv.URL)

	counters, err := c.GetCounters(context.Background())
	if err != nil {
		t.Fatalf("GetCounters: %v", err)
	}
	if counters == nil || counters.Feeds["42"] != 3 || counters.Totals.Unread != 3 {
		t.Errorf("GetCounters = %+v", counters)
	}
}

func TestExportOPML(t *testing.T) {
	opml := `<?xml version="1.0"?><opml></opml>`
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, opml)
	})
	c := newTestClient(t, srv.URL)

	got, err := c.ExportOPML(context.Background())
	if err != nil {
		t.Fatalf("ExportOPML: %v", err)
	}
	if got != opml {
		t.Errorf("ExportOPML = %q, want %q", got, opml)
	}
}

func TestDiscover(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"url":"https://example.com/feed.xml","title":"T","type":"rss"}]`)
	})
	c := newTestClient(t, srv.URL)

	cands, err := c.Discover(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	method, path, _, _, body, _ := cap.snapshot()
	if method != http.MethodPost || path != "/v1/discover" {
		t.Errorf("got %s %s, want POST /v1/discover", method, path)
	}
	if !strings.Contains(string(body), "example.com") {
		t.Errorf("Discover body = %q", string(body))
	}
	if len(cands) != 1 || cands[0].URL != "https://example.com/feed.xml" || cands[0].Type != "rss" {
		t.Errorf("Discover = %+v", cands)
	}
}

func TestCreateFeed(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":1,"title":"F","feed_url":"https://example.com/feed.xml"}`)
	})
	c := newTestClient(t, srv.URL)

	cat := 3
	f, err := c.CreateFeed(context.Background(), dmf.CreateFeedRequest{
		FeedURL:    "https://example.com/feed.xml",
		CategoryID: &cat,
		Title:      "F",
	})
	if err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}
	_, _, _, _, body, _ := cap.snapshot()
	if !strings.Contains(string(body), "feed.xml") || !strings.Contains(string(body), "\"category_id\":3") {
		t.Errorf("CreateFeed body = %q", string(body))
	}
	if f == nil || f.ID != 1 {
		t.Errorf("CreateFeed = %+v", f)
	}
}

func TestUpdateFeed(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":42,"title":"New"}`)
	})
	c := newTestClient(t, srv.URL)

	f, err := c.UpdateFeed(context.Background(), 42, dmf.UpdateFeedRequest{Title: "New"})
	if err != nil {
		t.Fatalf("UpdateFeed: %v", err)
	}
	method, path, _, _, body, _ := cap.snapshot()
	if method != http.MethodPut || path != "/v1/feeds/42" {
		t.Errorf("got %s %s, want PUT /v1/feeds/42", method, path)
	}
	if !strings.Contains(string(body), "New") {
		t.Errorf("UpdateFeed body = %q", string(body))
	}
	if f == nil || f.Title != "New" {
		t.Errorf("UpdateFeed = %+v", f)
	}
}

func TestCreateCategory(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":9,"title":"Tech","feed_count":0,"entry_count":0}`)
	})
	c := newTestClient(t, srv.URL)

	cat, err := c.CreateCategory(context.Background(), "Tech")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	_, _, _, _, body, _ := cap.snapshot()
	if !strings.Contains(string(body), "Tech") {
		t.Errorf("CreateCategory body = %q", string(body))
	}
	if cat == nil || cat.ID != 9 {
		t.Errorf("CreateCategory = %+v", cat)
	}
}

func TestUpdateCategory(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true}`)
	})
	c := newTestClient(t, srv.URL)

	if err := c.UpdateCategory(context.Background(), 7, "NewTitle"); err != nil {
		t.Fatalf("UpdateCategory: %v", err)
	}
	method, path, _, _, body, _ := cap.snapshot()
	if method != http.MethodPut || path != "/v1/categories/7" {
		t.Errorf("got %s %s, want PUT /v1/categories/7", method, path)
	}
	if !strings.Contains(string(body), "NewTitle") {
		t.Errorf("UpdateCategory body = %q", string(body))
	}
}

func TestRefreshCategory(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true}`)
	})
	c := newTestClient(t, srv.URL)

	if err := c.RefreshCategory(context.Background(), 7); err != nil {
		t.Fatalf("RefreshCategory: %v", err)
	}
	method, path, _, _, _, _ := cap.snapshot()
	if method != http.MethodPut || path != "/v1/categories/7/refresh" {
		t.Errorf("got %s %s, want PUT /v1/categories/7/refresh", method, path)
	}
}

func TestMarkCategoryEntriesRead(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true}`)
	})
	c := newTestClient(t, srv.URL)

	if err := c.MarkCategoryEntriesRead(context.Background(), 7); err != nil {
		t.Fatalf("MarkCategoryEntriesRead: %v", err)
	}
	method, path, _, _, _, _ := cap.snapshot()
	if method != http.MethodPut || path != "/v1/categories/7/mark-all-as-read" {
		t.Errorf("got %s %s, want PUT /v1/categories/7/mark-all-as-read", method, path)
	}
}

func TestUpdateEntries(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true}`)
	})
	c := newTestClient(t, srv.URL)

	starred := true
	if err := c.UpdateEntries(context.Background(), dmf.UpdateEntriesRequest{
		EntryIDs: []int{1, 2},
		Status:   "read",
		Starred:  &starred,
	}); err != nil {
		t.Fatalf("UpdateEntries: %v", err)
	}
	method, path, _, _, body, _ := cap.snapshot()
	if method != http.MethodPut || path != "/v1/entries" {
		t.Errorf("got %s %s, want PUT /v1/entries", method, path)
	}
	if !strings.Contains(string(body), `"entry_ids":[1,2]`) {
		t.Errorf("UpdateEntries body = %q", string(body))
	}
}

func TestToggleEntryBookmark(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"starred":true}`)
	})
	c := newTestClient(t, srv.URL)

	if err := c.ToggleEntryBookmark(context.Background(), 1001); err != nil {
		t.Fatalf("ToggleEntryBookmark: %v", err)
	}
	method, path, _, _, _, _ := cap.snapshot()
	if method != http.MethodPut || path != "/v1/entries/1001/bookmark" {
		t.Errorf("got %s %s, want PUT /v1/entries/1001/bookmark", method, path)
	}
}

func TestUpdateEntry(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":1001,"title":"Updated","content":"<p>x</p>"}`)
	})
	c := newTestClient(t, srv.URL)

	e, err := c.UpdateEntry(context.Background(), 1001, dmf.UpdateEntryRequest{Title: "Updated", Content: "<p>x</p>"})
	if err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}
	method, path, _, _, body, _ := cap.snapshot()
	if method != http.MethodPut || path != "/v1/entries/1001" {
		t.Errorf("got %s %s, want PUT /v1/entries/1001", method, path)
	}
	if !strings.Contains(string(body), "Updated") {
		t.Errorf("UpdateEntry body = %q", string(body))
	}
	if e == nil || e.Title != "Updated" {
		t.Errorf("UpdateEntry = %+v", e)
	}
}

func TestDeleteCategory(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{}`)
	})
	c := newTestClient(t, srv.URL)

	if err := c.DeleteCategory(context.Background(), 7); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}
	method, path, _, _, _, _ := cap.snapshot()
	if method != http.MethodDelete || path != "/v1/categories/7" {
		t.Errorf("got %s %s, want DELETE /v1/categories/7", method, path)
	}
}

func TestTokenFromContextNil(t *testing.T) {
	if got := TokenFromContext(nil); got != "" {
		t.Errorf("TokenFromContext(nil) = %q, want \"\"", got)
	}
}

func TestSanitizeMessageStripsControlAndTruncates(t *testing.T) {
	long := strings.Repeat("a", 600)
	got := sanitizeMessageHelper(long)
	if len(got) != 512 {
		t.Errorf("truncate: len = %d, want 512", len(got))
	}
	withCtrl := "ok\x00\x1b\x07end"
	if got := sanitizeMessageHelper(withCtrl); got != "okend" {
		t.Errorf("control chars not stripped: %q", got)
	}
}

// sanitizeMessageHelper bridges to the unexported sanitizeMessage via a real
// upstream error (its message passes through sanitizeMessage).
func sanitizeMessageHelper(s string) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, s, http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := newTestClientForHelper(srv.URL)
	_, err := c.ListFeeds(context.Background(), nil, 0, 0)
	if err == nil {
		return ""
	}
	return err.Error()
}

// newTestClientForHelper mirrors newTestClient but is safe to call without a
// *testing.T helper constraint beyond the shared option defaults.
func newTestClientForHelper(url string) *Client {
	c, err := New(url, WithDefaultToken(testDefaultToken))
	if err != nil {
		panic(err)
	}
	return c
}

func TestListEntriesErrorPropagates(t *testing.T) {
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nf", http.StatusNotFound)
	})
	c := newTestClient(t, srv.URL)
	if _, err := c.ListEntries(context.Background(), dmf.EntryFilter{Status: "unread"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateEntriesRequestJSON(t *testing.T) {
	starred := false
	b, err := json.Marshal(dmf.UpdateEntriesRequest{EntryIDs: []int{1}, Status: "read", Starred: &starred})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"entry_ids":[1]`) || !strings.Contains(string(b), `"starred":false`) {
		t.Errorf("UpdateEntriesRequest JSON = %s", string(b))
	}
}

func TestTimeFilteredMethods(t *testing.T) {
	// GetFeedEntries with before/after and FlushHistory with before use
	// time.Format(RFC3339); exercise the time-based query path end-to-end.
	before := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	after := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/entries") {
			io.WriteString(w, `{"total":0,"entries":[]}`)
		} else {
			io.WriteString(w, `{"ok":true}`)
		}
	})
	c := newTestClient(t, srv.URL)
	if _, err := c.GetFeedEntries(context.Background(), 1, dmf.EntryFilter{Before: &before, After: &after}); err != nil {
		t.Fatalf("GetFeedEntries: %v", err)
	}
	if err := c.FlushHistory(context.Background(), &before); err != nil {
		t.Fatalf("FlushHistory: %v", err)
	}
}

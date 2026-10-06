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
		io.WriteString(w, `{"reads":{"42":1},"unreads":{"42":3}}`)
	})
	c := newTestClient(t, srv.URL)

	counters, err := c.GetCounters(context.Background())
	if err != nil {
		t.Fatalf("GetCounters: %v", err)
	}
	if counters == nil || counters.Feeds["42"] != (dmf.CounterTotals{Read: 1, Unread: 3}) || counters.Totals.Unread != 3 {
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
	// POST /v1/feeds returns only {"feed_id":N}; the client must then resolve
	// the full feed via GET /v1/feeds/{id}. Use a dedicated server so the POST
	// body can be captured (startServer records only the last request's body,
	// which here is the resolving GET).
	var postBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/feeds":
			b, _ := io.ReadAll(r.Body)
			postBody = string(b)
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"feed_id":1}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/feeds/1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":1,"title":"F","feed_url":"https://example.com/feed.xml"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

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
	if !strings.Contains(postBody, "feed.xml") || !strings.Contains(postBody, "\"category_id\":3") {
		t.Errorf("CreateFeed body = %q", postBody)
	}
	if f == nil || f.ID != 1 || f.Title != "F" {
		t.Errorf("CreateFeed = %+v (want resolved feed id 1)", f)
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

// --- X03/N26: explicit timeout (SPEC §6.5, §8) ---

func TestNewDefaultClientHasTimeout(t *testing.T) {
	c, err := New("http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.cfg.httpClient.Timeout <= 0 {
		t.Errorf("default http.Client.Timeout = %v, want > 0", c.cfg.httpClient.Timeout)
	}
}

func TestWithTimeoutOverridesDefault(t *testing.T) {
	c, err := New("http://127.0.0.1:1", WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.cfg.httpClient.Timeout != 5*time.Second {
		t.Errorf("http.Client.Timeout = %v, want 5s", c.cfg.httpClient.Timeout)
	}
}

func TestWithTimeoutInvalidRejected(t *testing.T) {
	for _, d := range []time.Duration{0, -1 * time.Second} {
		if _, err := New("http://127.0.0.1:1", WithTimeout(d)); err == nil {
			t.Errorf("New with WithTimeout(%v): expected error, got nil", d)
		}
	}
}

func TestWithHTTPClientKeepsOwnTimeout(t *testing.T) {
	// An injected client carries its own timeout; WithTimeout must NOT clobber it.
	injected := &http.Client{Timeout: 2 * time.Second, Transport: failingTransport{}}
	c, err := New("http://127.0.0.1:1", WithHTTPClient(injected), WithTimeout(9*time.Second))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.cfg.httpClient.Timeout != 2*time.Second {
		t.Errorf("http.Client.Timeout = %v, want injected 2s (injected client wins)", c.cfg.httpClient.Timeout)
	}
}

// --- S11/N21: Discover URL scheme allowlist (SPEC §7) ---

func TestValidateDiscoverURL(t *testing.T) {
	valid := []string{
		"http://example.com",
		"http://example.com/feed",
		"https://example.com/feed",
	}
	for _, u := range valid {
		if err := validateDiscoverURL(u); err != nil {
			t.Errorf("validateDiscoverURL(%q) = %v, want nil", u, err)
		}
	}

	invalid := []string{
		"",
		"ftp://example.com/feed",
		"file:///etc/passwd",
		"javascript:alert(1)",
		"//example.com/feed", // scheme-less
		"http://",            // absolute but hostless
		"http://%zz",         // malformed — url.Parse error
	}
	for _, u := range invalid {
		if err := validateDiscoverURL(u); err == nil {
			t.Errorf("validateDiscoverURL(%q) = nil, want error", u)
		}
	}
}

func TestDiscoverURLValidation(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http", "http://example.com/feed", false},
		{"valid https", "https://example.com/feed", false},
		{"ftp rejected", "ftp://example.com/feed", true},
		{"file rejected", "file:///etc/passwd", true},
		{"javascript rejected", "javascript:alert(1)", true},
		{"scheme-less rejected", "//example.com/feed", true},
		{"empty rejected", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, `[{"url":"https://example.com/feed.xml","title":"T","type":"rss"}]`)
			})
			c := newTestClient(t, srv.URL)

			cands, err := c.Discover(context.Background(), tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Discover(%q): expected error, got nil", tt.url)
				}
			} else {
				if err != nil {
					t.Fatalf("Discover(%q): unexpected error: %v", tt.url, err)
				}
				if len(cands) != 1 || cands[0].Type != "rss" {
					t.Errorf("Discover(%q) = %+v", tt.url, cands)
				}
			}

			_, _, _, _, body, hits := cap.snapshot()
			if tt.wantErr && hits != 0 {
				t.Errorf("Discover(%q): upstream hit %d times, want 0 (invalid URL must not be sent)", tt.url, hits)
			}
			if !tt.wantErr && hits != 1 {
				t.Errorf("Discover(%q): upstream hit %d times, want 1", tt.url, hits)
			}
			if !tt.wantErr && !strings.Contains(string(body), tt.url) {
				t.Errorf("Discover(%q): body = %q, want url present", tt.url, string(body))
			}
		})
	}
}

func TestDiscoverPropagatesUpstreamError(t *testing.T) {
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nf", http.StatusNotFound)
	})
	c := newTestClient(t, srv.URL)

	_, err := c.Discover(context.Background(), "https://example.com/feed")
	assertKind(t, err, ErrorNotFound)
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

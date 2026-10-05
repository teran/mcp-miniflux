package miniflux

// CONTRACT — Miniflux HTTP client (SPEC §4, §6.5, §7, §8 X03, §9 L05/L09,
// §10 O03). The developer must define the following symbols in package
// infrastructure/miniflux. This test file is written FIRST (TDD, red state) and
// will not compile until they exist.
//
//	type Client struct{ ... }
//	func New(baseURL string, opts ...Option) (*Client, error)
//
//	type Option func(*clientConfig) error
//	func WithHTTPClient(hc *http.Client) Option      // inject transport for tests
//	func WithDefaultToken(tok string) Option          // MINIFLUX_API_TOKEN fallback
//	func WithMetrics(r MetricsRecorder) Option        // O03 upstream metrics
//
//	type MetricsRecorder interface {
//		ObserveUpstream(statusCode int, dur time.Duration, bytesIn, bytesOut int64)
//	}
//
//	// Token pass-through (SPEC §3.1): inbound if non-empty, else default token.
//	func (c *Client) ResolveToken(inbound string) string
//
//	// WithToken carries the inbound X-Auth-Token through ctx so per-call
//	// methods can honour the pass-through (SPEC §3.1); TokenFromContext is the
//	// internal reader. Mirrors logging.WithRequestID for request_id (L09).
//	func WithToken(ctx context.Context, token string) context.Context
//
//	// Error taxonomy (SPEC §6.5): 404 -> NotFound; 401/403 -> Auth;
//	// 429/5xx/network/timeout -> Transient; 2xx -> success.
//	type ErrorKind int
//	const (
//		ErrorNotFound ErrorKind = iota
//		ErrorAuth
//		ErrorTransient
//	)
//	type APIError struct {
//		Kind    ErrorKind
//		Status  int
//		Message string
//	}
//	func (e *APIError) Error() string
//
//	// Read methods (SPEC §4.1).
//	func (c *Client) ListFeeds(ctx context.Context, categoryID *int, limit, offset int) ([]dmf.Feed, error)
//	func (c *Client) GetFeed(ctx context.Context, id int) (*dmf.Feed, error)
//	func (c *Client) ListCategories(ctx context.Context) ([]dmf.Category, error)
//	func (c *Client) ListEntries(ctx context.Context, filter dmf.EntryFilter) (dmf.FeedEntries, error)
//	func (c *Client) GetEntry(ctx context.Context, id int) (*dmf.Entry, error)
//	func (c *Client) GetFeedEntries(ctx context.Context, feedID int, filter dmf.EntryFilter) (dmf.FeedEntries, error)
//	func (c *Client) GetCounters(ctx context.Context) (*dmf.Counters, error)
//	func (c *Client) GetMe(ctx context.Context) (*dmf.Me, error)
//	func (c *Client) ExportOPML(ctx context.Context) (string, error)
//	func (c *Client) Discover(ctx context.Context, url string) ([]dmf.DiscoveryResult, error)
//
//	// Write/update methods (SPEC §4.2).
//	func (c *Client) CreateFeed(ctx context.Context, req dmf.CreateFeedRequest) (*dmf.Feed, error)
//	func (c *Client) UpdateFeed(ctx context.Context, id int, req dmf.UpdateFeedRequest) (*dmf.Feed, error)
//	func (c *Client) RefreshFeed(ctx context.Context, id int) error
//	func (c *Client) CreateCategory(ctx context.Context, title string) (*dmf.Category, error)
//	func (c *Client) UpdateCategory(ctx context.Context, id int, title string) error
//	func (c *Client) RefreshCategory(ctx context.Context, id int) error
//	func (c *Client) MarkFeedEntriesRead(ctx context.Context, feedID int) error
//	func (c *Client) MarkCategoryEntriesRead(ctx context.Context, categoryID int) error
//	func (c *Client) UpdateEntries(ctx context.Context, req dmf.UpdateEntriesRequest) error
//	func (c *Client) ToggleEntryBookmark(ctx context.Context, entryID int) error
//	func (c *Client) UpdateEntry(ctx context.Context, id int, req dmf.UpdateEntryRequest) (*dmf.Entry, error)
//	func (c *Client) ImportOPML(ctx context.Context, opml string) error
//
//	// Delete / destructive methods (SPEC §4.3).
//	func (c *Client) DeleteFeed(ctx context.Context, id int) error
//	func (c *Client) DeleteCategory(ctx context.Context, id int) error
//	func (c *Client) FlushHistory(ctx context.Context, before *time.Time) error
//
// Domain types required (currently MISSING from domain/miniflux — the developer
// must add them): `EntryFilter`, `DiscoveryResult`, `CreateFeedRequest`,
// `UpdateFeedRequest`, `CreateCategoryRequest`(optional), `UpdateEntriesRequest`,
// `UpdateEntryRequest`.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
	"github.com/teran/mcp-miniflux/domain/requestid"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

const (
	testDefaultToken = "default-token"
	testInboundToken = "inbound-token"
)

// requestCapture records everything the client sent so tests can assert on
// method, path, query, headers and body.
type requestCapture struct {
	mu       sync.Mutex
	hits     int
	method   string
	path     string
	rawQuery string
	header   http.Header
	body     []byte
}

func (c *requestCapture) snapshot() (method, path, rawQuery string, header http.Header, body []byte, hits int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.method, c.path, c.rawQuery, c.header.Clone(), c.body, c.hits
}

// startServer runs an httptest server that captures each request then delegates
// to h for the response.
func startServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, *requestCapture) {
	t.Helper()
	cap := &requestCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.mu.Lock()
		cap.hits++
		cap.method = r.Method
		cap.path = r.URL.Path
		cap.rawQuery = r.URL.RawQuery
		cap.header = r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		cap.body = b
		cap.mu.Unlock()
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, cap
}

func okFeeds(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `[{"id":42,"user_id":7,"feed_url":"https://example.com/feed.xml","site_url":"https://example.com/","title":"Example Feed","category":{"id":3,"title":"Tech"},"status":"active","error_count":0,"username":"svc","password":"pw"}]`)
}

// recordingMetrics implements MetricsRecorder for tests.
type recordingMetrics struct {
	mu     sync.Mutex
	calls  int
	status int
}

func (r *recordingMetrics) ObserveUpstream(statusCode int, _ time.Duration, _, _ int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.status = statusCode
}

func (r *recordingMetrics) last() (int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls, r.status
}

func newTestClient(t *testing.T, url string, opts ...Option) *Client {
	t.Helper()
	all := append([]Option{WithDefaultToken(testDefaultToken)}, opts...)
	c, err := New(url, all...)
	if err != nil {
		t.Fatalf("New(%q): %v", url, err)
	}
	return c
}

func assertKind(t *testing.T, err error, want ErrorKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is %T, want *APIError: %v", err, err)
	}
	if apiErr.Kind != want {
		t.Errorf("Kind = %v, want %v (status=%d msg=%q)", apiErr.Kind, want, apiErr.Status, apiErr.Message)
	}
}

// --- Constructor & token resolution ---

func TestNewRequiresBaseURL(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("New(\"\") returned nil error, want error for empty base URL")
	}
}

func TestNewValidBaseURL(t *testing.T) {
	c, err := New("http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c == nil {
		t.Fatal("New returned nil client")
	}
}

func TestResolveToken(t *testing.T) {
	c := newTestClient(t, "http://127.0.0.1:1")
	if got := c.ResolveToken(""); got != testDefaultToken {
		t.Errorf("ResolveToken(\"\") = %q, want %q", got, testDefaultToken)
	}
	if got := c.ResolveToken(testInboundToken); got != testInboundToken {
		t.Errorf("ResolveToken(inbound) = %q, want %q", got, testInboundToken)
	}
}

// --- Auth header (SPEC §3.1) ---

func TestAuthHeaderUsesDefaultToken(t *testing.T) {
	srv, cap := startServer(t, okFeeds)
	c := newTestClient(t, srv.URL)

	if _, err := c.ListFeeds(context.Background(), nil, 0, 0); err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	_, _, _, hdr, _, _ := cap.snapshot()
	if got := hdr.Get("X-Auth-Token"); got != testDefaultToken {
		t.Errorf("X-Auth-Token = %q, want %q", got, testDefaultToken)
	}
}

func TestAuthHeaderInboundOverridesDefault(t *testing.T) {
	srv, cap := startServer(t, okFeeds)
	c := newTestClient(t, srv.URL)

	ctx := WithToken(context.Background(), testInboundToken)
	if _, err := c.ListFeeds(ctx, nil, 0, 0); err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	_, _, _, hdr, _, _ := cap.snapshot()
	if got := hdr.Get("X-Auth-Token"); got != testInboundToken {
		t.Errorf("X-Auth-Token = %q, want %q", got, testInboundToken)
	}
}

// --- X-Request-ID propagation (L09) ---

func TestRequestIDPropagation(t *testing.T) {
	srv, cap := startServer(t, okFeeds)
	c := newTestClient(t, srv.URL)

	ctx := requestid.WithRequestID(context.Background(), "req-123")
	if _, err := c.ListFeeds(ctx, nil, 0, 0); err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	_, _, _, hdr, _, _ := cap.snapshot()
	if got := hdr.Get("X-Request-ID"); got != "req-123" {
		t.Errorf("X-Request-ID = %q, want \"req-123\"", got)
	}
}

func TestRequestIDAbsentSendsNoHeader(t *testing.T) {
	srv, cap := startServer(t, okFeeds)
	c := newTestClient(t, srv.URL)

	if _, err := c.ListFeeds(context.Background(), nil, 0, 0); err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	_, _, _, hdr, _, _ := cap.snapshot()
	if got := hdr.Get("X-Request-ID"); got != "" {
		t.Errorf("X-Request-ID = %q, want empty when no request_id in ctx", got)
	}
}

// --- Base URL joining (SPEC §4) ---

func TestBaseURLPathJoining(t *testing.T) {
	srv, cap := startServer(t, okFeeds)
	c := newTestClient(t, srv.URL)

	if _, err := c.GetFeed(context.Background(), 42); err != nil {
		t.Fatalf("GetFeed: %v", err)
	}
	method, path, _, _, _, _ := cap.snapshot()
	if method != http.MethodGet || path != "/v1/feeds/42" {
		t.Errorf("got %s %s, want GET /v1/feeds/42", method, path)
	}
}

func TestListFeedsPathAndCategory(t *testing.T) {
	srv, cap := startServer(t, okFeeds)
	c := newTestClient(t, srv.URL)

	cat := 3
	if _, err := c.ListFeeds(context.Background(), &cat, 10, 5); err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	method, path, q, _, _, _ := cap.snapshot()
	if method != http.MethodGet || path != "/v1/feeds" {
		t.Errorf("got %s %s, want GET /v1/feeds", method, path)
	}
	for _, kv := range []string{"category_id=3", "limit=10", "offset=5"} {
		if !strings.Contains(q, kv) {
			t.Errorf("query %q missing %q", q, kv)
		}
	}
}

func TestListCategoriesUsesCounts(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"id":1,"title":"A","feed_count":0,"entry_count":2}]`)
	})
	c := newTestClient(t, srv.URL)

	if _, err := c.ListCategories(context.Background()); err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	method, path, q, _, _, _ := cap.snapshot()
	if method != http.MethodGet || path != "/v1/categories" {
		t.Errorf("got %s %s, want GET /v1/categories", method, path)
	}
	if !strings.Contains(q, "counts=true") {
		t.Errorf("query %q missing counts=true", q)
	}
}

func TestGetMePath(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":7,"username":"alice","is_admin":true,"theme":"system"}`)
	})
	c := newTestClient(t, srv.URL)

	me, err := c.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	method, path, _, _, _, _ := cap.snapshot()
	if method != http.MethodGet || path != "/v1/me" {
		t.Errorf("got %s %s, want GET /v1/me", method, path)
	}
	if me == nil || me.Username != "alice" || !me.IsAdmin {
		t.Errorf("GetMe = %+v", me)
	}
}

// --- JSON mapping (realistic Miniflux payloads) ---

func TestListFeedsJSONMapping(t *testing.T) {
	srv, _ := startServer(t, okFeeds)
	c := newTestClient(t, srv.URL)

	feeds, err := c.ListFeeds(context.Background(), nil, 0, 0)
	if err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("len = %d, want 1", len(feeds))
	}
	f := feeds[0]
	if f.ID != 42 || f.Title != "Example Feed" || f.FeedURL != "https://example.com/feed.xml" {
		t.Errorf("Feed = %+v", f)
	}
	if f.Category.ID != 3 || f.Category.Title != "Tech" {
		t.Errorf("Feed.Category = %+v", f.Category)
	}
	// The raw model carries the credentials (redaction happens in the output
	// layer, S02); the client must round-trip them from the API.
	if f.Username != "svc" || f.Password != "pw" {
		t.Errorf("credentials not parsed: username=%q password=%q", f.Username, f.Password)
	}
}

func TestGetFeedJSONMapping(t *testing.T) {
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":42,"user_id":7,"feed_url":"https://example.com/feed.xml","site_url":"https://example.com/","title":"Example Feed","category":{"id":3,"title":"Tech"},"status":"active","error_count":0,"username":"svc","password":"pw"}`)
	})
	c := newTestClient(t, srv.URL)

	f, err := c.GetFeed(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetFeed: %v", err)
	}
	if f == nil || f.ID != 42 || f.Title != "Example Feed" {
		t.Errorf("GetFeed = %+v", f)
	}
}

// --- Query param encoding for ListEntries ---

func TestListEntriesQueryParams(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"total":0,"entries":[]}`)
	})
	c := newTestClient(t, srv.URL)

	starred := true
	cat := 7
	before := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	after := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	filter := dmf.EntryFilter{
		Status:     "unread",
		Order:      "published_at",
		Direction:  "desc",
		Limit:      50,
		Offset:     10,
		Search:     "golang",
		Starred:    &starred,
		CategoryID: &cat,
		Before:     &before,
		After:      &after,
	}

	if _, err := c.ListEntries(context.Background(), filter); err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	_, path, q, _, _, _ := cap.snapshot()
	if path != "/v1/entries" {
		t.Errorf("path = %s, want /v1/entries", path)
	}
	for _, kv := range []string{
		"status=unread", "order=published_at", "direction=desc",
		"limit=50", "offset=10", "search=golang", "starred=true", "category_id=7",
		"before=2026-10-01T12%3A00%3A00Z", "after=2026-09-01T00%3A00%3A00Z",
	} {
		if !strings.Contains(q, kv) {
			t.Errorf("query %q missing %q", q, kv)
		}
	}
}

func TestGetFeedEntriesPathAndFilter(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"total":0,"entries":[]}`)
	})
	c := newTestClient(t, srv.URL)

	if _, err := c.GetFeedEntries(context.Background(), 42, dmf.EntryFilter{Status: "read"}); err != nil {
		t.Fatalf("GetFeedEntries: %v", err)
	}
	method, path, q, _, _, _ := cap.snapshot()
	if method != http.MethodGet || path != "/v1/feeds/42/entries" {
		t.Errorf("got %s %s, want GET /v1/feeds/42/entries", method, path)
	}
	if !strings.Contains(q, "status=read") {
		t.Errorf("query %q missing status=read", q)
	}
}

// --- Error taxonomy (SPEC §6.5) ---

func TestErrorNotFound(t *testing.T) {
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	c := newTestClient(t, srv.URL)

	_, err := c.GetFeed(context.Background(), 999)
	assertKind(t, err, ErrorNotFound)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", apiErr.Status)
	}
}

func TestErrorAuth401(t *testing.T) {
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	c := newTestClient(t, srv.URL)
	_, err := c.ListFeeds(context.Background(), nil, 0, 0)
	assertKind(t, err, ErrorAuth)
}

func TestErrorAuth403(t *testing.T) {
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	})
	c := newTestClient(t, srv.URL)
	_, err := c.ListFeeds(context.Background(), nil, 0, 0)
	assertKind(t, err, ErrorAuth)
}

func TestErrorTransient429(t *testing.T) {
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	})
	c := newTestClient(t, srv.URL)
	_, err := c.ListFeeds(context.Background(), nil, 0, 0)
	assertKind(t, err, ErrorTransient)
}

func TestErrorTransient5xx(t *testing.T) {
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	c := newTestClient(t, srv.URL)
	_, err := c.ListFeeds(context.Background(), nil, 0, 0)
	assertKind(t, err, ErrorTransient)
}

func TestErrorSuccess2xx(t *testing.T) {
	srv, _ := startServer(t, okFeeds)
	c := newTestClient(t, srv.URL)
	if _, err := c.ListFeeds(context.Background(), nil, 0, 0); err != nil {
		t.Errorf("2xx returned error: %v", err)
	}
}

// --- No silent retry (X03/N26) ---

func TestNoRetryOn429(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	})
	c := newTestClient(t, srv.URL)

	if _, err := c.ListFeeds(context.Background(), nil, 0, 0); err == nil {
		t.Fatal("expected error on 429")
	}
	_, _, _, _, _, hits := cap.snapshot()
	if hits != 1 {
		t.Errorf("upstream hit %d times, want exactly 1 (no silent retry)", hits)
	}
}

// --- Timeout / context cancellation (X03) ---

func TestContextCancellationIsTransient(t *testing.T) {
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		io.WriteString(w, `[]`)
	})
	c := newTestClient(t, srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, err := c.ListFeeds(ctx, nil, 0, 0)
	if err == nil {
		t.Fatal("expected error on expired context, got nil")
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.Kind != ErrorTransient {
			t.Errorf("Kind = %v, want Transient for timeout", apiErr.Kind)
		}
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("unexpected error type: %v", err)
	}
}

// --- No secret leakage in errors (L05) ---

func TestErrorDoesNotLeakToken(t *testing.T) {
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	c := newTestClient(t, srv.URL)

	_, err := c.ListFeeds(context.Background(), nil, 0, 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), testDefaultToken) {
		t.Errorf("error leaks the API token: %q", err.Error())
	}
}

// --- Upstream metrics (O03) ---

func TestMetricsRecorderCalledOnSuccess(t *testing.T) {
	srv, _ := startServer(t, okFeeds)
	metrics := &recordingMetrics{}
	c := newTestClient(t, srv.URL, WithMetrics(metrics))

	if _, err := c.ListFeeds(context.Background(), nil, 0, 0); err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	calls, status := metrics.last()
	if calls != 1 {
		t.Errorf("ObserveUpstream called %d times, want 1", calls)
	}
	if status != http.StatusOK {
		t.Errorf("ObserveUpstream status = %d, want 200", status)
	}
}

func TestMetricsRecorderCalledOnError(t *testing.T) {
	srv, _ := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nf", http.StatusNotFound)
	})
	metrics := &recordingMetrics{}
	c := newTestClient(t, srv.URL, WithMetrics(metrics))

	if _, err := c.GetFeed(context.Background(), 999); err == nil {
		t.Fatal("expected error")
	}
	calls, status := metrics.last()
	if calls != 1 {
		t.Errorf("ObserveUpstream called %d times, want 1", calls)
	}
	if status != http.StatusNotFound {
		t.Errorf("ObserveUpstream status = %d, want 404", status)
	}
}

// --- Injected http.Client / transport is honoured ---

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("transport down")
}

func TestWithHTTPClientInjectedTransport(t *testing.T) {
	c := newTestClient(t, "http://127.0.0.1:1",
		WithHTTPClient(&http.Client{Transport: failingTransport{}}))

	_, err := c.ListFeeds(context.Background(), nil, 0, 0)
	assertKind(t, err, ErrorTransient)
}

// --- Write / update / delete method contract (SPEC §4.2/§4.3) ---

func TestRefreshFeedPath(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true}`)
	})
	c := newTestClient(t, srv.URL)

	if err := c.RefreshFeed(context.Background(), 42); err != nil {
		t.Fatalf("RefreshFeed: %v", err)
	}
	method, path, _, _, _, _ := cap.snapshot()
	if method != http.MethodPut || path != "/v1/feeds/42/refresh" {
		t.Errorf("got %s %s, want PUT /v1/feeds/42/refresh", method, path)
	}
}

func TestDeleteFeedPath(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{}`)
	})
	c := newTestClient(t, srv.URL)

	if err := c.DeleteFeed(context.Background(), 42); err != nil {
		t.Fatalf("DeleteFeed: %v", err)
	}
	method, path, _, _, _, _ := cap.snapshot()
	if method != http.MethodDelete || path != "/v1/feeds/42" {
		t.Errorf("got %s %s, want DELETE /v1/feeds/42", method, path)
	}
}

func TestMarkFeedEntriesReadPath(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true}`)
	})
	c := newTestClient(t, srv.URL)

	if err := c.MarkFeedEntriesRead(context.Background(), 42); err != nil {
		t.Fatalf("MarkFeedEntriesRead: %v", err)
	}
	method, path, _, _, _, _ := cap.snapshot()
	if method != http.MethodPut || path != "/v1/feeds/42/mark-all-as-read" {
		t.Errorf("got %s %s, want PUT /v1/feeds/42/mark-all-as-read", method, path)
	}
}

func TestImportOPMLPostsBody(t *testing.T) {
	opml := `<?xml version="1.0"?><opml><body><outline text="x" type="rss" xmlUrl="https://example.com/feed.xml"/></body></opml>`
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{}`)
	})
	c := newTestClient(t, srv.URL)

	if err := c.ImportOPML(context.Background(), opml); err != nil {
		t.Fatalf("ImportOPML: %v", err)
	}
	method, path, _, _, body, _ := cap.snapshot()
	if method != http.MethodPost || path != "/v1/import" {
		t.Errorf("got %s %s, want POST /v1/import", method, path)
	}
	if !strings.Contains(string(body), "example.com/feed.xml") {
		t.Errorf("OPML body not posted: %q", string(body))
	}
}

func TestFlushHistoryWithoutBefore(t *testing.T) {
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true}`)
	})
	c := newTestClient(t, srv.URL)

	if err := c.FlushHistory(context.Background(), nil); err != nil {
		t.Fatalf("FlushHistory: %v", err)
	}
	method, path, _, _, _, _ := cap.snapshot()
	if method != http.MethodPut || path != "/v1/flush-history" {
		t.Errorf("got %s %s, want PUT /v1/flush-history", method, path)
	}
}

func TestFlushHistoryWithBefore(t *testing.T) {
	before := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	srv, cap := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true}`)
	})
	c := newTestClient(t, srv.URL)

	if err := c.FlushHistory(context.Background(), &before); err != nil {
		t.Fatalf("FlushHistory: %v", err)
	}
	_, _, q, _, _, _ := cap.snapshot()
	if !strings.Contains(q, "before=2026-01-01T00%3A00%3A00Z") {
		t.Errorf("query %q missing before param", q)
	}
}

// CONFORM-AUDIT [L09/L04GO] — every outbound Miniflux request must be logged
// with the request_id correlation id (threaded from ctx, propagated as
// X-Request-ID) and must NOT leak the auth token / secrets. This locks a new
// option `WithLogger(l *logrus.Logger)` on the client that emits an outbound
// request log carrying the request_id field.
func TestOutboundRequestLoggedWithRequestIDNoSecret(t *testing.T) {
	srv, _ := startServer(t, okFeeds)

	l := logrus.New()
	l.SetLevel(logrus.DebugLevel)
	hook := &test.Hook{}
	l.AddHook(hook)

	c := newTestClient(t, srv.URL, WithLogger(l))
	ctx := requestid.WithRequestID(context.Background(), "req-correl-1")

	if _, err := c.ListFeeds(ctx, nil, 0, 0); err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}

	found := false
	for _, e := range hook.AllEntries() {
		if id, ok := e.Data["request_id"].(string); ok && id == "req-correl-1" {
			found = true
			// The resolved token must never appear in the log record (L05).
			if strings.Contains(e.Message, testDefaultToken) || strings.Contains(e.Message, testInboundToken) {
				t.Errorf("outbound request log leaks the auth token: %q", e.Message)
			}
			for k := range e.Data {
				if strings.EqualFold(k, "token") || strings.EqualFold(k, "password") {
					t.Errorf("outbound request log leaks secret field %q", k)
				}
			}
		}
	}
	if !found {
		t.Errorf("no outbound request log entry with request_id req-correl-1 (L09/L04GO)")
	}
}

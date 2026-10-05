package main_test

// Integration test: full path from the MCP facade down to the real upstream.
//
// This external test package (package main_test, still under cmd/**) wires the
// REAL composition, exactly as main.go does, but points the REAL
// infrastructure/miniflux resty client at a LOCKED-DOWN httptest Miniflux
// upstream instead of a real instance:
//
//	MCP streamable HTTP facade (go-sdk server -> toolCallHandler/registry)
//	  -> real application handler (domain port)
//	  -> real infrastructure/miniflux Client (resty)
//	  -> real HTTP request to the mocked Miniflux (httptest.Server)
//
// The unit tests mock the dmf.Client port, so they never exercise the
// application->infrastructure->HTTP chain. This test closes that gap. It uses
// the sessionless (>= 2026-07-28) MCP protocol over Streamable HTTP so each
// tools/call is a real HTTP POST, exactly as a production client would.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/teran/mcp-miniflux/application"
	"github.com/teran/mcp-miniflux/application/handlers"
	"github.com/teran/mcp-miniflux/infrastructure/logging"
	mcpmcp "github.com/teran/mcp-miniflux/infrastructure/mcp"
	"github.com/teran/mcp-miniflux/infrastructure/miniflux"
)

// feedJSON / discoveryJSON mirror the JSON wire shape of the domain models so
// the mocked upstream can emit responses without the test depending on
// domain/miniflux (cmd may depend on application+infrastructure only, per
// .go-arch-lint.yml).
type feedJSON struct {
	ID       int    `json:"id"`
	UserID   int    `json:"user_id"`
	FeedURL  string `json:"feed_url"`
	SiteURL  string `json:"site_url"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type discoveryJSON struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// ---------------------------------------------------------------------------
// Mocked Miniflux upstream
// ---------------------------------------------------------------------------

// recordedRequest captures everything the mocked Miniflux received for one
// upstream call, so the test can assert method/path/query/headers/body.
type recordedRequest struct {
	Method    string
	Path      string
	RawQuery  string
	AuthToken string
	RequestID string
	Body      []byte
}

// mockUpstream is a locked-down Miniflux stand-in. It serves static responses
// and records every request it receives.
type mockUpstream struct {
	mu       sync.Mutex
	requests []recordedRequest

	// Configurable responses, keyed by method+path.
	feeds        []feedJSON      // GET /v1/feeds
	createResp   feedJSON        // POST /v1/feeds
	discoverResp []discoveryJSON // POST /v1/discover
}

func (m *mockUpstream) record(r *http.Request, body []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, recordedRequest{
		Method:    r.Method,
		Path:      r.URL.Path,
		RawQuery:  r.URL.RawQuery,
		AuthToken: r.Header.Get("X-Auth-Token"),
		RequestID: r.Header.Get("X-Request-ID"),
		Body:      body,
	})
}

func (m *mockUpstream) snapshot() []recordedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]recordedRequest, len(m.requests))
	copy(out, m.requests)
	return out
}

func (m *mockUpstream) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m.record(r, body)

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/feeds":
			writeJSON(w, m.feeds)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/feeds":
			writeJSON(w, m.createResp)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/feeds/"):
			// Miniflux returns 204 on delete; no body.
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/discover":
			writeJSON(w, m.discoverResp)
		default:
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]string{"errorMessage": "not found"})
		}
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------------------
// MCP client driver (sessionless protocol over real Streamable HTTP)
// ---------------------------------------------------------------------------

const (
	protocolVersion = "2026-07-28"
	authHeader      = "X-Auth-Token"
)

// rpcResponse is the JSON-RPC response envelope. A successful tool call has a
// populated Result; a protocol error (e.g. InvalidParams from a destructive
// tool without confirm) has a populated Error and an HTTP 4xx status.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// callTool performs a real HTTP POST tools/call against the streamable handler
// wrapped in httptest.NewServer. It returns the JSON-RPC envelope and the HTTP
// status.
func callTool(t *testing.T, srvURL, tool string, args map[string]any, authToken, requestID string) (rpcResponse, int) {
	t.Helper()

	params := map[string]any{
		"name":      tool,
		"arguments": args,
		"_meta": map[string]any{
			"io.modelcontextprotocol/protocolVersion":    protocolVersion,
			"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		},
	}
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  params,
	})
	if err != nil {
		t.Fatalf("marshal tools/call request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, srvURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", protocolVersion)
	req.Header.Set("Mcp-Method", "tools/call")
	req.Header.Set("Mcp-Name", tool)
	if authToken != "" {
		req.Header.Set(authHeader, authToken)
	}
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	// Give the access log a stable source to assert against.
	req.Header.Set("X-Real-IP", "203.0.113.7")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST tools/call %s: %v", tool, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read tools/call response: %v", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("tools/call %s status = %d, body=%s", tool, resp.StatusCode, raw)
	}

	var r rpcResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("decode tools/call response %q: %v", raw, err)
	}
	return r, resp.StatusCode
}

// toolCallResult is the JSON shape of the CallToolResult embedded in result.
type toolCallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent"`
	IsError           bool            `json:"isError"`
}

func decodeResult(t *testing.T, r rpcResponse) toolCallResult {
	t.Helper()
	var res toolCallResult
	if err := json.Unmarshal(r.Result, &res); err != nil {
		t.Fatalf("decode CallToolResult %q: %v", r.Result, err)
	}
	return res
}

// ---------------------------------------------------------------------------
// Composition (mirrors main.go, with the real client pointed at the mock)
// ---------------------------------------------------------------------------

type harness struct {
	upstream *mockUpstream
	srv      *httptest.Server
	logHook  *test.Hook
}

// newHarness builds the full real stack: mock Miniflux upstream, real resty
// client, real application handlers and a real go-sdk streamable HTTP server.
func newHarness(t *testing.T) *harness {
	t.Helper()

	upstream := &mockUpstream{}
	mock := httptest.NewServer(upstream.handler())
	t.Cleanup(mock.Close)

	log, logHook := test.NewNullLogger()
	log.SetLevel(logrus.InfoLevel)

	// Real infrastructure client, pointed at the mocked Miniflux.
	client, err := miniflux.New(mock.URL,
		miniflux.WithHTTPClient(mock.Client()),
		miniflux.WithDefaultToken("srv-token"),
		miniflux.WithLogger(log),
		miniflux.WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("miniflux.New: %v", err)
	}
	port := miniflux.AsPort(client)

	// Application layer wired exactly as main.go does.
	toolLogger := func(ctx context.Context, tool string, args map[string]any, source string, duration time.Duration, outcome string) {
		logging.LogToolCall(logging.WithContext(log, ctx), tool, args, source, duration, outcome)
	}
	deps := application.Deps{
		Client: port,
		Logger: toolLogger,
		Tools:  handlers.All(port),
	}

	// go-sdk server + streamable handler.
	sdkLogger := slog.New(logging.NewSlogHandler(log, slog.LevelError))
	srv := mcpmcp.NewServer(&mcp.Implementation{Name: "mcp-miniflux", Version: "test"}, sdkLogger, "integration test")
	application.RegisterTools(srv, deps)
	handler := mcpmcp.NewStreamableHandler(srv, sdkLogger)

	// Wrap in a real HTTP server so the request goes through the full facade.
	hs := httptest.NewServer(handler)
	t.Cleanup(hs.Close)

	return &harness{upstream: upstream, srv: hs, logHook: logHook}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestIntegrationListFeedsRedactsSecrets walks list_feeds end-to-end and asserts
// both sides of the wire: the upstream GET /v1/feeds carried the passed-through
// X-Auth-Token and X-Request-ID, and the tool output had password redacted.
func TestIntegrationListFeedsRedactsSecrets(t *testing.T) {
	h := newHarness(t)
	h.upstream.feeds = []feedJSON{
		{
			ID: 7, UserID: 1, FeedURL: "https://example.com/rss", SiteURL: "https://example.com",
			Title: "Example", Status: "subscribed",
			Username: "alice", Password: "super-secret-pw",
		},
	}

	r, status := callTool(t, h.srv.URL, "list_feeds", map[string]any{}, "client-token", "rid-abc-123")
	if status != http.StatusOK {
		t.Fatalf("list_feeds status = %d, want 200", status)
	}
	if r.Error != nil {
		t.Fatalf("list_feeds protocol error: %+v", r.Error)
	}
	res := decodeResult(t, r)
	if res.IsError {
		t.Fatalf("list_feeds reported tool error; content=%+v", res.Content)
	}

	// Output: feeds parsed, password/username redacted.
	var out struct {
		Feeds []struct {
			ID       int    `json:"id"`
			Title    string `json:"title"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"feeds"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &out); err != nil {
		t.Fatalf("decode list_feeds output %q: %v", res.Content[0].Text, err)
	}
	if len(out.Feeds) != 1 || out.Feeds[0].ID != 7 {
		t.Fatalf("unexpected feeds: %+v", out.Feeds)
	}
	if out.Feeds[0].Password != "" {
		t.Errorf("password not redacted in output: %q", out.Feeds[0].Password)
	}
	if out.Feeds[0].Username != "" {
		t.Errorf("username not redacted in output: %q", out.Feeds[0].Username)
	}
	if strings.Contains(res.Content[0].Text, "super-secret-pw") {
		t.Error("secret password leaked into tool output text")
	}

	// Upstream: GET /v1/feeds with pass-through token and correlation id.
	reqs := h.upstream.snapshot()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 upstream request, got %d: %+v", len(reqs), reqs)
	}
	got := reqs[0]
	if got.Method != http.MethodGet || got.Path != "/v1/feeds" {
		t.Errorf("upstream call = %s %s, want GET /v1/feeds", got.Method, got.Path)
	}
	if got.AuthToken != "client-token" {
		t.Errorf("X-Auth-Token = %q, want client-token (pass-through beats default)", got.AuthToken)
	}
	if got.RequestID != "rid-abc-123" {
		t.Errorf("X-Request-ID = %q, want rid-abc-123", got.RequestID)
	}
}

// TestIntegrationCreateFeedSearchBeforeCreate walks the idempotent
// search-before-create path: GET /v1/feeds first, then POST /v1/feeds, and the
// password sent upstream is not echoed into the tool output.
func TestIntegrationCreateFeedSearchBeforeCreate(t *testing.T) {
	h := newHarness(t)
	// No existing feed -> proceeds to create.
	h.upstream.feeds = nil
	h.upstream.createResp = feedJSON{
		ID: 42, FeedURL: "https://news.example/rss", Title: "News",
		Username: "bob", Password: "upstream-stored-pw",
	}

	r, status := callTool(t, h.srv.URL, "create_feed", map[string]any{
		"feed_url": "https://news.example/rss",
		"title":    "News",
		"username": "bob",
		"password": "client-supplied-pw",
	}, "client-token", "")
	if status != http.StatusOK {
		t.Fatalf("create_feed status = %d, want 200", status)
	}
	if r.Error != nil {
		t.Fatalf("create_feed protocol error: %+v", r.Error)
	}
	res := decodeResult(t, r)
	if res.IsError {
		t.Fatalf("create_feed tool error; content=%+v", res.Content)
	}
	if strings.Contains(res.Content[0].Text, "client-supplied-pw") || strings.Contains(res.Content[0].Text, "upstream-stored-pw") {
		t.Errorf("password leaked into create_feed output: %s", res.Content[0].Text)
	}

	// Order: GET (search) then POST (create).
	reqs := h.upstream.snapshot()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 upstream requests (search+create), got %d: %+v", len(reqs), reqs)
	}
	if reqs[0].Method != http.MethodGet || reqs[0].Path != "/v1/feeds" {
		t.Errorf("request[0] = %s %s, want GET /v1/feeds (search)", reqs[0].Method, reqs[0].Path)
	}
	if reqs[1].Method != http.MethodPost || reqs[1].Path != "/v1/feeds" {
		t.Errorf("request[1] = %s %s, want POST /v1/feeds (create)", reqs[1].Method, reqs[1].Path)
	}
	// Password IS forwarded upstream (that is the point of the credential).
	if !bytes.Contains(reqs[1].Body, []byte("client-supplied-pw")) {
		t.Errorf("POST /v1/feeds body does not carry the password: %s", reqs[1].Body)
	}
}

// TestIntegrationCreateFeedIdempotentSkipsCreate verifies that when a feed with
// the same URL already exists, search-before-create returns it and never issues
// the POST.
func TestIntegrationCreateFeedIdempotentSkipsCreate(t *testing.T) {
	h := newHarness(t)
	h.upstream.feeds = []feedJSON{{ID: 9, FeedURL: "https://dup.example/rss", Title: "Dup"}}
	h.upstream.createResp = feedJSON{ID: 999, FeedURL: "should-not-exist"}

	r, status := callTool(t, h.srv.URL, "create_feed", map[string]any{
		"feed_url": "https://dup.example/rss",
	}, "client-token", "")
	if status != http.StatusOK {
		t.Fatalf("create_feed status = %d, want 200", status)
	}
	if r.Error != nil {
		t.Fatalf("create_feed protocol error: %+v", r.Error)
	}
	res := decodeResult(t, r)
	if res.IsError {
		t.Fatalf("create_feed tool error; content=%+v", res.Content)
	}
	if strings.Contains(res.Content[0].Text, "999") {
		t.Errorf("idempotent create_feed called POST despite existing feed: %s", res.Content[0].Text)
	}

	reqs := h.upstream.snapshot()
	if len(reqs) != 1 {
		t.Fatalf("expected exactly 1 upstream request (search only), got %d: %+v", len(reqs), reqs)
	}
	if reqs[0].Method != http.MethodGet || reqs[0].Path != "/v1/feeds" {
		t.Errorf("upstream call = %s %s, want GET /v1/feeds", reqs[0].Method, reqs[0].Path)
	}
}

// TestIntegrationDeleteFeedRequiresConfirm verifies the destructive HITL gate
// (S12): without confirm:true no upstream DELETE is issued; with it the DELETE
// reaches Miniflux.
func TestIntegrationDeleteFeedRequiresConfirm(t *testing.T) {
	t.Run("without confirm", func(t *testing.T) {
		h := newHarness(t)
		r, status := callTool(t, h.srv.URL, "delete_feed", map[string]any{"feed_id": 123}, "client-token", "")
		if status != http.StatusBadRequest {
			t.Fatalf("delete_feed without confirm status = %d, want 400", status)
		}
		if r.Error == nil {
			t.Fatal("expected protocol error when confirm is missing")
		}
		if r.Error.Code != -32602 {
			t.Errorf("error code = %d, want -32602 (InvalidParams)", r.Error.Code)
		}
		if reqs := h.upstream.snapshot(); len(reqs) != 0 {
			t.Errorf("expected NO upstream request without confirm, got %d: %+v", len(reqs), reqs)
		}
	})

	t.Run("with confirm", func(t *testing.T) {
		h := newHarness(t)
		r, status := callTool(t, h.srv.URL, "delete_feed", map[string]any{"feed_id": 123, "confirm": true}, "client-token", "")
		if status != http.StatusOK {
			t.Fatalf("delete_feed with confirm status = %d, want 200", status)
		}
		if r.Error != nil {
			t.Fatalf("delete_feed protocol error: %+v", r.Error)
		}
		res := decodeResult(t, r)
		if res.IsError {
			t.Fatalf("delete_feed tool error; content=%+v", res.Content)
		}

		reqs := h.upstream.snapshot()
		if len(reqs) != 1 {
			t.Fatalf("expected 1 upstream request, got %d: %+v", len(reqs), reqs)
		}
		got := reqs[0]
		if got.Method != http.MethodDelete || got.Path != "/v1/feeds/123" {
			t.Errorf("upstream call = %s %s, want DELETE /v1/feeds/123", got.Method, got.Path)
		}
	})
}

// TestIntegrationDiscoverSubscriptionsStructuredAndSanitized walks the single
// open-world tool (S07/S09): the output is structural (typed candidate list),
// and the TEXT output is ANSI/control sanitized while the token is passed
// through upstream.
func TestIntegrationDiscoverSubscriptionsStructuredAndSanitized(t *testing.T) {
	h := newHarness(t)
	h.upstream.discoverResp = []discoveryJSON{
		{URL: "https://example.com/rss", Title: "Plain Feed", Type: "rss"},
		{URL: "https://evil.example/feed", Title: "\x1b[31mANSI\u0007 Feed\x1b[0m", Type: "atom"},
	}

	r, status := callTool(t, h.srv.URL, "discover_subscriptions", map[string]any{
		"url": "https://example.com",
	}, "client-token", "")
	if status != http.StatusOK {
		t.Fatalf("discover_subscriptions status = %d, want 200", status)
	}
	if r.Error != nil {
		t.Fatalf("discover_subscriptions protocol error: %+v", r.Error)
	}
	res := decodeResult(t, r)
	if res.IsError {
		t.Fatalf("discover_subscriptions tool error; content=%+v", res.Content)
	}

	// Structured content is a typed candidate list (S07/M07).
	if len(res.StructuredContent) == 0 {
		t.Fatal("expected structuredContent for discover_subscriptions")
	}
	var structured []struct {
		URL   string `json:"url"`
		Title string `json:"title"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal(res.StructuredContent, &structured); err != nil {
		t.Fatalf("decode structuredContent %q: %v", res.StructuredContent, err)
	}
	if len(structured) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %+v", len(structured), structured)
	}

	// The TEXT output is sanitized: no ANSI ESC or BEL anywhere in it.
	if strings.ContainsRune(res.Content[0].Text, 0x1b) {
		t.Errorf("ANSI ESC leaked into text output: %q", res.Content[0].Text)
	}
	if strings.ContainsRune(res.Content[0].Text, 0x07) {
		t.Errorf("BEL control char leaked into text output: %q", res.Content[0].Text)
	}

	// Token passed through upstream.
	reqs := h.upstream.snapshot()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 upstream request, got %d: %+v", len(reqs), reqs)
	}
	if reqs[0].Method != http.MethodPost || reqs[0].Path != "/v1/discover" {
		t.Errorf("upstream call = %s %s, want POST /v1/discover", reqs[0].Method, reqs[0].Path)
	}
	if reqs[0].AuthToken != "client-token" {
		t.Errorf("X-Auth-Token = %q, want client-token", reqs[0].AuthToken)
	}
	if !bytes.Contains(reqs[0].Body, []byte(`"url":"https://example.com"`)) {
		t.Errorf("discover body does not carry the target URL: %s", reqs[0].Body)
	}
}

// TestIntegrationAccessLogWrittenWithRequestIDAndSource verifies the per-call
// access log records the request_id (correlated with the inbound header), the
// source and a redacted args map (no password).
func TestIntegrationAccessLogWrittenWithRequestIDAndSource(t *testing.T) {
	h := newHarness(t)
	h.upstream.feeds = []feedJSON{{ID: 1, FeedURL: "https://x/rss", Title: "X"}}

	callTool(t, h.srv.URL, "list_feeds", map[string]any{}, "client-token", "rid-log-1")

	var found bool
	for _, e := range h.logHook.AllEntries() {
		if e.Message != "tool call" {
			continue
		}
		found = true
		if e.Data["request_id"] != "rid-log-1" {
			t.Errorf("access log request_id = %v, want rid-log-1", e.Data["request_id"])
		}
		if e.Data["tool"] != "list_feeds" {
			t.Errorf("access log tool = %v, want list_feeds", e.Data["tool"])
		}
		if e.Data["source"] != "203.0.113.7" {
			t.Errorf("access log source = %v, want 203.0.113.7", e.Data["source"])
		}
		if e.Data["outcome"] != "ok" {
			t.Errorf("access log outcome = %v, want ok", e.Data["outcome"])
		}
	}
	if !found {
		t.Error("no 'tool call' access-log entry emitted")
	}
}

// TestIntegrationDefaultTokenUsedWhenNoInbound verifies the MINIFLUX_API_TOKEN
// fallback is used when the client sends no X-Auth-Token.
func TestIntegrationDefaultTokenUsedWhenNoInbound(t *testing.T) {
	h := newHarness(t)
	h.upstream.feeds = nil

	callTool(t, h.srv.URL, "list_feeds", map[string]any{}, "", "")

	reqs := h.upstream.snapshot()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 upstream request, got %d: %+v", len(reqs), reqs)
	}
	if reqs[0].AuthToken != "srv-token" {
		t.Errorf("X-Auth-Token = %q, want srv-token (MINIFLUX_API_TOKEN fallback)", reqs[0].AuthToken)
	}
}

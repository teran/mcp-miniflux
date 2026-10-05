package application

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-miniflux/domain/app"
	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
	"github.com/teran/mcp-miniflux/domain/requestid"
	"github.com/teran/mcp-miniflux/domain/tools"
)

// fakeHandler is a configurable app.Handler used to exercise the registry
// wrapper (token threading, logging, error mapping) without a real use case.
type fakeHandler struct {
	// token captures the inbound token threaded into ctx.
	token string
	// err, when set, is returned from Call.
	err error
	// isErrResult, when set, makes Call return an IsError result.
	isErrResult bool
}

func (f *fakeHandler) Call(ctx context.Context, _ map[string]any) (*mcp.CallToolResult, error) {
	f.token = dmf.TokenFromContext(ctx)
	if f.err != nil {
		return nil, f.err
	}
	if f.isErrResult {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "boom"}}}, nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: `{"ok":true}`}}}, nil
}

// newReq builds a CallToolRequest with the given raw arguments and optional
// inbound X-Auth-Token header.
func newReq(t *testing.T, args string, token string) *mcp.CallToolRequest {
	t.Helper()
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(args)},
	}
	if token != "" {
		req.Extra = &mcp.RequestExtra{Header: http.Header{"X-Auth-Token": []string{token}}}
	}
	return req
}

func TestToolCallHandlerThreadsInboundToken(t *testing.T) {
	fh := &fakeHandler{}
	h := toolCallHandler("list_feeds", fh, nil)
	_, err := h(context.Background(), newReq(t, `{}`, "inbound-secret"))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if fh.token != "inbound-secret" {
		t.Errorf("handler saw token %q, want %q (pass-through, SPEC §3.1)", fh.token, "inbound-secret")
	}
}

func TestToolCallHandlerNoToken(t *testing.T) {
	fh := &fakeHandler{}
	h := toolCallHandler("get_me", fh, nil)
	if _, err := h(context.Background(), newReq(t, ``, "")); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if fh.token != "" {
		t.Errorf("handler saw token %q, want empty", fh.token)
	}
}

func TestToolCallHandlerInvalidArgs(t *testing.T) {
	fh := &fakeHandler{}
	h := toolCallHandler("list_feeds", fh, nil)
	_, err := h(context.Background(), newReq(t, `{not json`, ""))
	if err == nil {
		t.Fatal("expected InvalidParams error for malformed arguments")
	}
}

func TestToolCallHandlerLogsRedactedArgs(t *testing.T) {
	var gotTool, gotOutcome, gotSource string
	var gotArgs map[string]any
	logger := func(_ context.Context, tool string, redacted map[string]any, source string, _ time.Duration, outcome string) {
		gotTool, gotArgs, gotSource, gotOutcome = tool, redacted, source, outcome
	}

	fh := &fakeHandler{}
	h := toolCallHandler("create_feed", fh, logger)
	if _, err := h(context.Background(), newReq(t, `{"feed_url":"https://x","password":"hunter2"}`, "")); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if gotTool != "create_feed" {
		t.Errorf("log tool = %q, want create_feed", gotTool)
	}
	if gotOutcome != "ok" {
		t.Errorf("log outcome = %q, want ok", gotOutcome)
	}
	if gotSource != "mcp" {
		t.Errorf("log source = %q, want mcp", gotSource)
	}
	if _, present := gotArgs["password"]; present {
		t.Errorf("log args leaked password: %v (L05)", gotArgs)
	}
	if gotArgs["feed_url"] != "https://x" {
		t.Errorf("log args missing feed_url: %v", gotArgs)
	}
}

func TestToolCallHandlerLogsErrorOutcome(t *testing.T) {
	var gotOutcome string
	logger := func(_ context.Context, _ string, _ map[string]any, _ string, _ time.Duration, outcome string) {
		gotOutcome = outcome
	}
	fh := &fakeHandler{isErrResult: true}
	h := toolCallHandler("delete_feed", fh, logger)
	res, err := h(context.Background(), newReq(t, `{}`, ""))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError result")
	}
	if gotOutcome != "error" {
		t.Errorf("log outcome = %q, want error", gotOutcome)
	}
}

func TestToolCallHandlerNilLogger(t *testing.T) {
	fh := &fakeHandler{}
	h := toolCallHandler("get_me", fh, nil)
	if _, err := h(context.Background(), newReq(t, `{}`, "")); err != nil {
		t.Fatalf("handler error: %v", err)
	}
}

func TestToolCallHandlerReturnsHandlerError(t *testing.T) {
	fh := &fakeHandler{err: errors.New("boom")}
	h := toolCallHandler("get_me", fh, nil)
	_, err := h(context.Background(), newReq(t, `{}`, ""))
	if err == nil || err.Error() != "boom" {
		t.Errorf("expected handler error to propagate, got %v", err)
	}
}

// CONFORM-AUDIT [L09] — in stdio mode (or any transport without the HTTP
// request_id middleware) the context carries no request_id. The toolCallHandler
// must therefore GENERATE one when absent so every access-log record still
// carries a correlation id, and must PRESERVE an existing one.
func TestToolCallHandlerGeneratesRequestIDWhenAbsent(t *testing.T) {
	var gotID string
	logger := func(ctx context.Context, _ string, _ map[string]any, _ string, _ time.Duration, _ string) {
		gotID = requestid.RequestIDFromContext(ctx)
	}
	fh := &fakeHandler{}
	h := toolCallHandler("get_me", fh, logger)

	// Context with NO request_id (stdio path).
	if _, err := h(context.Background(), newReq(t, `{}`, "")); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if gotID == "" {
		t.Error("toolCallHandler did not generate a request_id when context lacked one (L09)")
	}
}

func TestToolCallHandlerPreservesExistingRequestID(t *testing.T) {
	var gotID string
	logger := func(ctx context.Context, _ string, _ map[string]any, _ string, _ time.Duration, _ string) {
		gotID = requestid.RequestIDFromContext(ctx)
	}
	fh := &fakeHandler{}
	h := toolCallHandler("get_me", fh, logger)

	ctx := requestid.WithRequestID(context.Background(), "existing-7")
	if _, err := h(ctx, newReq(t, `{}`, "")); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if gotID != "existing-7" {
		t.Errorf("request_id = %q, want preserved existing-7 (L09)", gotID)
	}
}

// CONFORM-AUDIT [L08] — the access-log `source` must be the resolved inbound
// client IP (X-Real-IP / X-Forwarded-For) rather than a hardcoded constant.
// This locks the end-to-end wiring: toolCallHandler must call resolveSource on
// the request headers and pass the result into the access logger.
func TestToolCallHandlerLogsResolvedSourceFromHeaders(t *testing.T) {
	var gotSource string
	logger := func(_ context.Context, _ string, _ map[string]any, source string, _ time.Duration, _ string) {
		gotSource = source
	}
	fh := &fakeHandler{}
	h := toolCallHandler("get_me", fh, logger)

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{}`)},
		Extra:  &mcp.RequestExtra{Header: http.Header{"X-Real-IP": []string{"203.0.113.42"}}},
	}
	if _, err := h(context.Background(), req); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if gotSource != "203.0.113.42" {
		t.Errorf("access-log source = %q, want resolved X-Real-IP 203.0.113.42 (L08)", gotSource)
	}
}

func TestToolCallHandlerNilRequest(t *testing.T) {
	fh := &fakeHandler{}
	h := toolCallHandler("get_me", fh, nil)
	if _, err := h(context.Background(), nil); err != nil {
		t.Fatalf("nil request should be tolerated: %v", err)
	}
}

func TestToolCallHandlerNilParams(t *testing.T) {
	fh := &fakeHandler{}
	h := toolCallHandler("get_me", fh, nil)
	req := &mcp.CallToolRequest{}
	if _, err := h(context.Background(), req); err != nil {
		t.Fatalf("nil params should be tolerated: %v", err)
	}
}

// expectedToolOrder mirrors domain/tools.expectedToolOrder (S03).
var expectedToolOrder = []string{
	"list_feeds", "get_feed", "list_categories", "list_entries", "get_entry",
	"get_feed_entries", "get_counters", "get_me", "export_opml",
	"discover_subscriptions",
	"create_feed", "update_feed", "refresh_feed", "create_category",
	"update_category", "refresh_category", "mark_feed_entries_read",
	"mark_category_entries_read", "update_entries", "toggle_entry_bookmark",
	"update_entry", "import_opml",
	"delete_feed", "delete_category", "flush_history",
}

// allDefs returns one definition instance per tool in registration order.
func allDefs() []app.Definition {
	return []app.Definition{
		&tools.ListFeeds{}, &tools.GetFeed{}, &tools.ListCategories{}, &tools.ListEntries{}, &tools.GetEntry{},
		&tools.GetFeedEntries{}, &tools.GetCounters{}, &tools.GetMe{}, &tools.ExportOPML{},
		&tools.DiscoverSubscriptions{},
		&tools.CreateFeed{}, &tools.UpdateFeed{}, &tools.RefreshFeed{}, &tools.CreateCategory{},
		&tools.UpdateCategory{}, &tools.RefreshCategory{}, &tools.MarkFeedEntriesRead{},
		&tools.MarkCategoryEntriesRead{}, &tools.UpdateEntries{}, &tools.ToggleEntryBookmark{},
		&tools.UpdateEntry{}, &tools.ImportOPML{},
		&tools.DeleteFeed{}, &tools.DeleteCategory{}, &tools.FlushHistory{},
	}
}

func TestBuildToolMapping(t *testing.T) {
	defs := allDefs()
	if len(defs) != len(expectedToolOrder) {
		t.Fatalf("got %d defs, want %d", len(defs), len(expectedToolOrder))
	}
	for i, def := range defs {
		if def.Name() != expectedToolOrder[i] {
			t.Errorf("def[%d].Name() = %q, want %q (order S03)", i, def.Name(), expectedToolOrder[i])
		}
		tool := buildTool(def)
		if tool.Name != def.Name() {
			t.Errorf("%s: built tool name = %q", def.Name(), tool.Name)
		}
		if tool.Description != def.Instructions() {
			t.Errorf("%s: description != instructions", def.Name())
		}
		if tool.Title == "" {
			t.Errorf("%s: tool title must be non-empty", def.Name())
		}
		if tool.Annotations == nil {
			t.Errorf("%s: annotations must be set", def.Name())
		}
		if tool.InputSchema == nil {
			t.Errorf("%s: input schema must be non-nil (AddTool panics otherwise)", def.Name())
		}
		if tool.OutputSchema == nil {
			t.Errorf("%s: output schema must be non-nil (S09)", def.Name())
		}
	}
}

func TestRegisterToolsDoesNotPanic(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	var tools []app.Tool
	for _, def := range allDefs() {
		tools = append(tools, app.Tool{Def: def, Handler: &fakeHandler{}})
	}
	// Any nil input schema would panic inside AddTool.
	RegisterTools(server, Deps{Tools: tools})
}

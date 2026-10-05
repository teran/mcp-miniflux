package mcp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-miniflux/domain/requestid"
)

// discardLogger returns a slog.Logger that writes nowhere.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestNewServerBuildsServer verifies NewServer constructs a usable go-sdk
// server wired with the given slog logger and instructions (L07/L03GO). The
// server identity is exercised through the initialize handshake in
// TestStreamableHandlerInitializes.
func TestNewServerBuildsServer(t *testing.T) {
	logger := discardLogger()
	srv := NewServer(&mcp.Implementation{Name: "mcp-miniflux", Version: "v1"}, logger, "test instructions")
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}

	// A tool registered after construction must be usable (the composition root
	// registers tools via application.RegisterTools after NewServer).
	srv.AddTool(
		&mcp.Tool{Name: "ping", Title: "Ping", InputSchema: map[string]any{"type": "object"}},
		func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		},
	)
}

// TestStreamableHandlerInitializes drives the full NewServer +
// NewStreamableHandler pipeline with a stateless initialize POST and verifies
// the server responds with its identity (name/version).
func TestStreamableHandlerInitializes(t *testing.T) {
	logger := discardLogger()
	srv := NewServer(&mcp.Implementation{Name: "mcp-miniflux", Version: "1.0.0"}, logger, "test instructions")
	h := NewStreamableHandler(srv, logger)

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "mcp-miniflux") {
		t.Errorf("initialize response missing server name; body=%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "1.0.0") {
		t.Errorf("initialize response missing server version; body=%s", rec.Body.String())
	}
}

// TestWithRequestIDUsesHeader verifies the middleware threads the inbound
// X-Request-ID header into ctx (L09).
func TestWithRequestIDUsesHeader(t *testing.T) {
	var got string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = requestid.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Request-ID", "abc-123")
	withRequestID(next).ServeHTTP(httptest.NewRecorder(), req)
	if got != "abc-123" {
		t.Errorf("request_id = %q, want abc-123 (header passthrough)", got)
	}
}

// TestWithRequestIDGeneratesWhenAbsent verifies the middleware generates a
// non-empty request_id when no X-Request-ID header is present (L09).
func TestWithRequestIDGeneratesWhenAbsent(t *testing.T) {
	var got string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = requestid.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	withRequestID(next).ServeHTTP(httptest.NewRecorder(), req)
	if got == "" {
		t.Error("expected a generated request_id, got empty")
	}
	if len(got) != 32 {
		t.Errorf("generated request_id length = %d, want 32 (hex of 16 bytes)", len(got))
	}
}

// TestNewRequestID verifies the generator always returns a 32-char hex id.
func TestNewRequestID(t *testing.T) {
	for i := 0; i < 8; i++ {
		id := newRequestID()
		if len(id) != 32 {
			t.Fatalf("newRequestID() length = %d, want 32", len(id))
		}
	}
}

// TestRunStdioCancelledContext verifies the stdio path returns promptly when
// the context is already cancelled, rather than blocking on stdin.
func TestRunStdioCancelledContext(t *testing.T) {
	logger := discardLogger()
	srv := NewServer(&mcp.Implementation{Name: "t", Version: "v"}, logger, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() { done <- RunStdio(ctx, srv) }()

	select {
	case err := <-done:
		// The stdio run loop returns context.Canceled when the context is
		// cancelled; either outcome (nil or context.Canceled) confirms it did
		// not block on stdin.
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("RunStdio on cancelled context returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunStdio did not return on a cancelled context")
	}
}

package main

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/teran/mcp-miniflux/infrastructure/logging"
)

// --- parseMode ---

func TestParseModeDefaultStdio(t *testing.T) {
	mode, err := parseMode(nil)
	if err != nil {
		t.Fatalf("parseMode: %v", err)
	}
	if mode != "stdio" {
		t.Errorf("default mode = %q, want stdio (M06)", mode)
	}
}

func TestParseModeHTTP(t *testing.T) {
	mode, err := parseMode([]string{"-mode", "http"})
	if err != nil {
		t.Fatalf("parseMode: %v", err)
	}
	if mode != "http" {
		t.Errorf("mode = %q, want http", mode)
	}
}

func TestParseModeInvalid(t *testing.T) {
	if _, err := parseMode([]string{"-mode", "bogus"}); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestParseModeUnknownFlag(t *testing.T) {
	if _, err := parseMode([]string{"-nope"}); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

// --- buildLogger ---

func TestBuildLoggerHTTPMode(t *testing.T) {
	l, err := buildLogger("http", "info", "text", "")
	if err != nil {
		t.Fatalf("buildLogger: %v", err)
	}
	if l == nil {
		t.Fatal("buildLogger returned nil logger")
	}
}

func TestBuildLoggerStdioMode(t *testing.T) {
	dir := t.TempDir()
	filename := dir + "/server.log"
	l, err := buildLogger("stdio", "info", "text", filename)
	if err != nil {
		t.Fatalf("buildLogger: %v", err)
	}
	if l == nil {
		t.Fatal("buildLogger returned nil logger")
	}
	t.Cleanup(func() { _ = os.Remove(filename) })
}

func TestBuildLoggerInvalidMode(t *testing.T) {
	if _, err := buildLogger("bogus", "", "", ""); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestBuildLoggerStdioMissingFilename(t *testing.T) {
	if _, err := buildLogger("stdio", "info", "text", ""); err == nil {
		t.Fatal("expected error for stdio mode without filename")
	}
}

// --- emitBanner ---

func TestEmitBannerLogsFirstLine(t *testing.T) {
	l, hook := test.NewNullLogger()
	emitBanner(l, "mcp-miniflux", "v1", "abc", "2026-10-05")
	entries := hook.AllEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	want := "Starting mcp-miniflux/v1 (commit: abc; built at 2026-10-05) ..."
	if entries[0].Message != want {
		t.Errorf("banner message = %q, want %q", entries[0].Message, want)
	}
}

func TestEmitBannerNilLoggerNoop(t *testing.T) {
	// Must not panic.
	emitBanner(nil, "mcp-miniflux", "v1", "abc", "ts")
}

// --- makeToolLogger ---

func TestMakeToolLoggerNil(t *testing.T) {
	if makeToolLogger(nil) != nil {
		t.Error("makeToolLogger(nil) should return nil (logging disabled)")
	}
}

func TestMakeToolLoggerEmitsWithRequestID(t *testing.T) {
	l, hook := test.NewNullLogger()
	lg := makeToolLogger(l)
	if lg == nil {
		t.Fatal("makeToolLogger(non-nil) returned nil")
	}

	ctx := logging.WithRequestID(context.Background(), "rid-1")
	lg(ctx, "create_feed", map[string]any{"feed_url": "https://x"}, "mcp", 5*time.Millisecond, "ok")

	entries := hook.AllEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Data["request_id"] != "rid-1" {
		t.Errorf("log entry missing request_id field; got %v", e.Data)
	}
	if e.Data["tool"] != "create_feed" {
		t.Errorf("log entry tool = %v, want create_feed", e.Data["tool"])
	}
	if e.Data["outcome"] != "ok" {
		t.Errorf("log entry outcome = %v, want ok", e.Data["outcome"])
	}
}

// --- serverImplementation ---

func TestServerImplementation(t *testing.T) {
	impl := serverImplementation()
	if impl.Name != "mcp-miniflux" {
		t.Errorf("Name = %q", impl.Name)
	}
	if impl.Title == "" {
		t.Error("Title must be non-empty")
	}
	if impl.Description == "" {
		t.Error("Description must be non-empty")
	}
	if impl.Version != appVersion {
		t.Errorf("Version = %q, want %q", impl.Version, appVersion)
	}
}

// --- run: config error paths ---

func TestRunMissingAPIURL(t *testing.T) {
	t.Setenv("MINIFLUX_API_URL", "")
	t.Setenv("MINIFLUX_API_TOKEN", "tok")
	err := run(context.Background(), []string{"-mode", "http"})
	if err == nil {
		t.Fatal("expected error when MINIFLUX_API_URL is missing")
	}
}

func TestRunMissingTokenStdio(t *testing.T) {
	t.Setenv("MINIFLUX_API_URL", "http://127.0.0.1:1")
	t.Setenv("MINIFLUX_API_TOKEN", "")
	err := run(context.Background(), []string{"-mode", "stdio"})
	if err == nil {
		t.Fatal("expected error when MINIFLUX_API_TOKEN is missing in stdio mode")
	}
}

// --- run: http mode graceful shutdown ---

func TestRunHTTPModeGracefulShutdown(t *testing.T) {
	t.Setenv("MINIFLUX_API_URL", "http://127.0.0.1:1")
	t.Setenv("MINIFLUX_API_TOKEN", "tok")
	t.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("INTERNAL_ADDR", "127.0.0.1:0")
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("LOG_LEVEL", "info")

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- run(ctx, []string{"-mode", "http"}) }()

	// Give the server a moment to come up, then request a graceful shutdown.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run(http) returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run(http) did not shut down after context cancellation")
	}
}

// --- serveHTTP ---

func TestServeHTTPGracefulShutdown(t *testing.T) {
	l := logrus.New()
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- serveHTTP(ctx, http.NotFoundHandler(), "127.0.0.1:0", l) }()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serveHTTP returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serveHTTP did not shut down after cancellation")
	}
}

func TestServeHTTPListenError(t *testing.T) {
	l := logrus.New()
	err := serveHTTP(context.Background(), http.NotFoundHandler(), "256.256.256.256:0", l)
	if err == nil {
		t.Fatal("expected ListenAndServe error for invalid address")
	}
}

package miniflux

// CONTRACT — Outbound User-Agent (SPEC §11.4 M10/N44). Every outbound request
// to Miniflux must carry an identifying User-Agent of the form
//   app-name/version (reference)
// e.g. `mcp-miniflux/1.0.0 (https://github.com/teran/mcp-miniflux)`.
//
// The reference URL is a constant for this public repo; app-name is constant;
// only the version is ldflags-injected at the composition root
// (cmd/mcp-miniflux/main.go appVersion). The version must therefore be threaded
// from the caller/composition root into the client. Following the existing
// Options pattern (WithDefaultToken/WithMetrics/WithLogger) the developer must
// define a new option in package infrastructure/miniflux:
//
//	// WithUserAgent sets the identifying User-Agent on every outbound request
//	// (M10/N44). An empty value is an error.
//	func WithUserAgent(ua string) Option
//
// This test file is written FIRST (TDD, red state) and will not compile until
// that symbol exists.

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestOutboundRequestSetsUserAgentHeader asserts the strongest form of M10: the
// outbound request carries a User-Agent header whose value is exactly the one
// configured via WithUserAgent. This fails against the current code, which sets
// no User-Agent (resty's default go-resty/... does not match).
func TestOutboundRequestSetsUserAgentHeader(t *testing.T) {
	srv, cap := startServer(t, okFeeds)
	ua := "mcp-miniflux/1.0.0 (https://github.com/teran/mcp-miniflux)"
	c := newTestClient(t, srv.URL, WithUserAgent(ua))

	if _, err := c.ListFeeds(context.Background(), nil, 0, 0); err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	_, _, _, hdr, _, _ := cap.snapshot()
	if got := hdr.Get("User-Agent"); got != ua {
		t.Errorf("M10: User-Agent = %q, want %q", got, ua)
	}
}

// TestUserAgentContainsAppNameAndReference asserts the M10 format pieces: the
// UA must identify the app by name and include the GitHub reference URL.
func TestUserAgentContainsAppNameAndReference(t *testing.T) {
	srv, cap := startServer(t, okFeeds)
	ua := "mcp-miniflux/0.9.0 (https://github.com/teran/mcp-miniflux)"
	c := newTestClient(t, srv.URL, WithUserAgent(ua))

	if _, err := c.ListFeeds(context.Background(), nil, 0, 0); err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	_, _, _, hdr, _, _ := cap.snapshot()
	got := hdr.Get("User-Agent")
	if !strings.Contains(got, "mcp-miniflux/") {
		t.Errorf("M10: User-Agent %q missing app name prefix %q", got, "mcp-miniflux/")
	}
	if !strings.Contains(got, "https://github.com/teran/mcp-miniflux") {
		t.Errorf("M10: User-Agent %q missing GitHub reference URL", got)
	}
}

// TestUserAgentVersionThreadedFromCaller pins N44: the version must be the
// caller-supplied value (ldflags-injected at the composition root), not a
// hardcoded constant buried in the client. A distinct, caller-chosen version
// must appear verbatim in the outbound UA.
func TestUserAgentVersionThreadedFromCaller(t *testing.T) {
	srv, cap := startServer(t, okFeeds)
	const version = "2.7.1"
	ua := fmt.Sprintf("mcp-miniflux/%s (https://github.com/teran/mcp-miniflux)", version)
	c := newTestClient(t, srv.URL, WithUserAgent(ua))

	if _, err := c.ListFeeds(context.Background(), nil, 0, 0); err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	_, _, _, hdr, _, _ := cap.snapshot()
	got := hdr.Get("User-Agent")
	if !strings.Contains(got, "mcp-miniflux/"+version) {
		t.Errorf("N44: User-Agent %q does not carry the caller-supplied version %q", got, version)
	}
}

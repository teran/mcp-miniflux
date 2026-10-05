package handlers

import (
	"context"
	"strings"
	"testing"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// CONFORM-AUDIT [S09] — discover_subscriptions is the single open-world tool
// (S07/S10): its input URL and the returned candidates are untrusted external
// data. Per S09/N23 the TEXT output must be routed through the ANSI/control
// sanitizer (textContent) — never a raw TextContent — and the structured
// content must be the redacted candidate list. This locks that:
//
//   - res.StructuredContent is the typed, redacted candidate list.
//   - The TextContent equals exactly what sanitizeText would produce for the
//     marshaled (already-redacted) payload — i.e. the handler MUST route its
//     text output through textContent()/sanitizeText, not build a raw
//     TextContent.
//   - No raw control/ANSI byte survives in the text output.
//
// Distinguishability: Go's encoding/json escapes bytes < 0x20 (so an ESC
// sequence becomes the literal "\u001b"), but it emits 0x7f (DEL) literally.
// By seeding a raw DEL byte into the candidate title, the sanitizer changes the
// text output (strips the DEL) while a raw TextContent would keep it — so this
// test FAILS if the handler bypasses textContent() and only passes once it
// routes through the sanitizer. This is what pins the fix (the ESC sequence is
// retained to lock the "no raw ESC byte" property as well).
func TestDiscoverSubscriptionsTextContentSanitized(t *testing.T) {
	cands := []dmf.DiscoveryResult{
		{URL: "https://example.com/rss", Title: "Example\x1b[31mRed\x1b[0m\x7f", Type: "rss"},
	}
	c := &fakeClient{discover: func(_ context.Context, url string) ([]dmf.DiscoveryResult, error) {
		return cands, nil
	}}
	h := DiscoverSubscriptionsHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"url": "https://example.com/feed"})
	okRes(t, res, err)

	// Structured content must be the typed, redacted candidate list (S07/M07).
	sc, ok := res.StructuredContent.([]dmf.DiscoveryResult)
	if !ok {
		t.Fatalf("structured content type = %T, want []dmf.DiscoveryResult (M07)", res.StructuredContent)
	}
	if len(sc) != 1 || sc[0].URL != "https://example.com/rss" {
		t.Errorf("structured content = %+v, want candidate list", sc)
	}

	// The marshaled JSON payload contains a literal DEL (0x7f) byte because the
	// candidate title is seeded with one and encoding/json emits 0x7f literally.
	// Sanitizing that payload must strip the DEL, so the handler's text content
	// must equal the sanitized form — a raw TextContent would keep the DEL.
	data, err := marshalJSON(dmf.Redact(cands))
	if err != nil {
		t.Fatalf("marshalJSON: %v", err)
	}
	if !strings.Contains(string(data), "\x7f") {
		t.Fatalf("test setup: marshaled payload must carry a raw DEL byte for the sanitizer to be observable (S09)")
	}
	want := sanitizeText(string(data))
	got := textOf(t, res)
	if got != want {
		t.Errorf("discover text content not routed through sanitizeText: got %q want %q (S09/N23)", got, want)
	}

	// No raw control/ANSI byte may survive in the text output.
	if strings.Contains(got, "\x1b") {
		t.Errorf("discover text output contains a raw ESC byte: %q", got)
	}
	for _, r := range got {
		if (r < 0x20 && r != '\t' && r != '\n') || r == 0x7f {
			t.Errorf("discover text output contains raw control byte U+%04X", r)
		}
	}
}

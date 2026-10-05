package handlers

import (
	"context"
	"strings"
	"testing"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
	"github.com/teran/mcp-miniflux/domain/tools"
)

// CONFORM-AUDIT [S09/M07] — discover_subscriptions is the single open-world
// tool (S07/S10) whose output is untrusted external data returned
// structurally. DiscoverSubscriptions.OutputSchema declares an OBJECT with a
// "feeds" array of {url,title,type} candidates. The actual structured content
// produced by the handler MUST match that declared shape (an object with a
// "feeds" array — never a bare top-level array). This test is a regression
// guard for the schema<->structuredContent mismatch that shipped a bare
// candidate array while the OutputSchema declared {"feeds": [...]}.
func TestDiscoverSubscriptionsOutputMatchesSchema(t *testing.T) {
	// The declared contract: top-level object with a "feeds" array.
	schema := *(new(tools.DiscoverSubscriptions).OutputSchema())
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("outputSchema must declare properties with a feeds array (M07): %v", schema)
	}
	if _, ok := props["feeds"]; !ok {
		t.Fatalf("outputSchema must declare a feeds property (M07): %v", props)
	}

	c := &fakeClient{discover: func(_ context.Context, url string) ([]dmf.DiscoveryResult, error) {
		return []dmf.DiscoveryResult{
			{URL: "https://a.example/rss", Title: "A", Type: "rss"},
			{URL: "https://b.example/atom", Title: "B", Type: "atom"},
		}, nil
	}}
	h := DiscoverSubscriptionsHandler{Client: c}
	res, err := h.Call(context.Background(), map[string]any{"url": "https://example.com"})
	okRes(t, res, err)

	// The actual structured output must be an OBJECT (per the schema) wrapping
	// the candidates under "feeds", not a bare top-level array.
	obj, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T, want map[string]any object (schema says object with feeds)", res.StructuredContent)
	}
	feeds, ok := obj["feeds"]
	if !ok {
		t.Fatalf("structured content missing feeds key (schema says object with feeds): %v", obj)
	}
	cands, ok := feeds.([]dmf.DiscoveryResult)
	if !ok {
		t.Fatalf("structured content.feeds type = %T, want []dmf.DiscoveryResult (schema says array of {url,title,type})", feeds)
	}
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %+v", len(cands), cands)
	}
	for i, cand := range cands {
		if cand.URL == "" || cand.Title == "" || cand.Type == "" {
			t.Errorf("candidate[%d] must carry url/title/type fields (M07): %+v", i, cand)
		}
	}

	// The text output must present the same object shape as JSON {"feeds":[...]}.
	text := textOf(t, res)
	if !strings.Contains(text, `"feeds"`) {
		t.Errorf("text output must be the {\"feeds\":[...]} object, got: %s", text)
	}
	if strings.HasPrefix(strings.TrimSpace(text), "[") {
		t.Errorf("text output must not be a bare top-level array, got: %s", text)
	}
}

package logging

import "testing"

// TestBanner locks the startup banner format (SPEC §9 L06, §11.4 B05): it must
// be the first-line shape "Starting {app}/{version} (commit: {commit}; built at
// {ts}) ...".
func TestBanner(t *testing.T) {
	got := Banner("mcp-miniflux", "v1.0.0", "abc123", "2026-10-05T00:00:00Z")
	want := "Starting mcp-miniflux/v1.0.0 (commit: abc123; built at 2026-10-05T00:00:00Z) ..."
	if got != want {
		t.Errorf("Banner() = %q, want %q", got, want)
	}
}

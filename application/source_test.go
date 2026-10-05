package application

import (
	"net/http"
	"testing"
)

// CONFORM-AUDIT [L08] — the access-log `source` field must reflect the actual
// inbound client IP when the request carries X-Real-IP or X-Forwarded-For,
// falling back to a default identifier otherwise. This locks the contract:
//
//	func resolveSource(hdr http.Header) string
//
// Resolution order:
//  1. X-Real-IP header value (trimmed) — the canonical trusted client IP.
//  2. Otherwise X-Forwarded-For — the leftmost (original client) entry, trimmed;
//     a comma-separated list yields its first IP.
//  3. Otherwise the fallback default (the constant "mcp").
func TestResolveSourceUsesXRealIP(t *testing.T) {
	hdr := http.Header{}
	hdr.Set("X-Real-IP", "203.0.113.9")
	if got := resolveSource(hdr); got != "203.0.113.9" {
		t.Errorf("resolveSource = %q, want 203.0.113.9 (X-Real-IP)", got)
	}
}

func TestResolveSourceXRealIPBeatsForwardedFor(t *testing.T) {
	hdr := http.Header{}
	hdr.Set("X-Real-IP", "203.0.113.9")
	hdr.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.1")
	if got := resolveSource(hdr); got != "203.0.113.9" {
		t.Errorf("resolveSource = %q, want 203.0.113.9 (X-Real-IP must take precedence)", got)
	}
}

func TestResolveSourceUsesXForwardedFor(t *testing.T) {
	hdr := http.Header{}
	hdr.Set("X-Forwarded-For", "198.51.100.7")
	if got := resolveSource(hdr); got != "198.51.100.7" {
		t.Errorf("resolveSource = %q, want 198.51.100.7 (X-Forwarded-For)", got)
	}
}

func TestResolveSourceForwardedForTakesLeftmost(t *testing.T) {
	hdr := http.Header{}
	hdr.Set("X-Forwarded-For", " 198.51.100.7, 10.0.0.1, 172.16.0.2 ")
	if got := resolveSource(hdr); got != "198.51.100.7" {
		t.Errorf("resolveSource = %q, want leftmost 198.51.100.7", got)
	}
}

func TestResolveSourceFallback(t *testing.T) {
	if got := resolveSource(http.Header{}); got != "mcp" {
		t.Errorf("resolveSource(empty) = %q, want fallback %q", got, "mcp")
	}
}

func TestResolveSourceIgnoresEmptyHeaders(t *testing.T) {
	hdr := http.Header{}
	hdr.Set("X-Real-IP", "   ")
	hdr.Set("X-Forwarded-For", "")
	if got := resolveSource(hdr); got != "mcp" {
		t.Errorf("resolveSource(blank headers) = %q, want fallback %q", got, "mcp")
	}
}

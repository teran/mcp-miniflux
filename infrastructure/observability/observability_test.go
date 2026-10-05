package observability

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestNewObservabilityMuxMetrics(t *testing.T) {
	srv := httptest.NewServer(NewObservabilityMux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("GET /metrics Content-Type = %q, want text/plain", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /metrics body: %v", err)
	}
	text := string(body)
	// O02: default registry carries the Go runtime/memstats + process collectors.
	for _, want := range []string{"go_goroutines", "go_memstats_alloc_bytes", "process_cpu_seconds_total"} {
		if !strings.Contains(text, want) {
			t.Errorf("GET /metrics body does not contain %q", want)
		}
	}
}

func TestNewObservabilityMuxProbes(t *testing.T) {
	srv := httptest.NewServer(NewObservabilityMux())
	defer srv.Close()

	for _, path := range []string{"/healthz", "/readyz", "/startupz"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", path, resp.StatusCode)
		}
		if string(body) != "ok" {
			t.Errorf("GET %s body = %q, want \"ok\"", path, string(body))
		}
	}
}

func TestNewObservabilityMuxPprof(t *testing.T) {
	srv := httptest.NewServer(NewObservabilityMux())
	defer srv.Close()

	// Index page lists available profiles.
	resp, err := http.Get(srv.URL + "/debug/pprof/")
	if err != nil {
		t.Fatalf("GET /debug/pprof/: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /debug/pprof/ status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Types of profiles available") {
		t.Errorf("GET /debug/pprof/ body does not list profiles")
	}

	// Specific pprof endpoints respond.
	for _, path := range []string{
		"/debug/pprof/cmdline",
		"/debug/pprof/symbol",
		"/debug/pprof/trace",
	} {
		r, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		if r.StatusCode != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", path, r.StatusCode)
		}
	}
}

// freeAddr returns a "host:port" that was free at reservation time.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func TestStartObservabilityServerBindAndShutdown(t *testing.T) {
	addr := freeAddr(t)
	log := logrus.New()
	log.SetOutput(io.Discard)

	srv := StartObservabilityServer(addr, log)

	// Poll until the background listener serves a request.
	base := "http://" + addr
	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("observability server never became ready on %s", addr)
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := srv.Shutdown(nil); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

// TestStartObservabilityServerExposesInstrumentedMetrics verifies that the O02
// instrumenter vectors registered by StartObservabilityServer appear on the
// default registry served at /metrics. It does not need the live server; it
// reuses the same registration path to prove the vectors exist and are idempotent.
func TestStartObservabilityServerExposesInstrumentedMetrics(t *testing.T) {
	addr := freeAddr(t)
	log := logrus.New()
	log.SetOutput(io.Discard)

	srv := StartObservabilityServer(addr, log)
	defer func() {
		_ = srv.Shutdown(nil)
	}()

	// Calling StartObservabilityServer twice must not panic (idempotent
	// registration with the default registry).
	srv2 := StartObservabilityServer(freeAddr(t), log)
	defer func() { _ = srv2.Shutdown(nil) }()

	mux := NewObservabilityMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	text := string(body)
	for _, want := range []string{
		"http_request_duration_seconds",
		"http_response_size_bytes",
		"http_requests_total",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("/metrics does not contain instrumenter metric %q", want)
		}
	}
}

// Package observability serves the internal observability endpoint (:8081,
// INTERNAL_ADDR): Prometheus metrics, pprof and the startup/readiness/liveness
// probes, plus upstream (Miniflux) response metrics.
//
// O01/N32: the observability listener is on a SEPARATE address from the MCP app
// listener (:8080, LISTEN_ADDR). Prometheus metrics, pprof and probes are served
// ONLY here and are never part of the MCP JSON-RPC flow.
package observability

import (
	"context"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// DefaultAddr is the default observability listen address (INTERNAL_ADDR,
// O01/O04). It is distinct from the MCP app default of :8080.
const DefaultAddr = ":8081"

// NewObservabilityMux builds the HTTP mux served on the observability port
// (O01):
//
//   - /metrics        — Prometheus scrape endpoint (O02). Serves the real
//     exposition from the default prometheus registry, which already carries
//     the Go runtime/memstats + process collectors.
//   - /debug/pprof/... — Go pprof profiles (index, cmdline, profile, symbol,
//     trace) for on-demand debugging.
//   - /healthz        — liveness probe.
//   - /readyz         — readiness probe.
//   - /startupz       — startup probe.
//
// Per A02/N35 the probes reflect PROCESS/listener liveness ONLY, never upstream
// reachability: no outbound request is made from a probe. Readiness and startup
// are local-state based and always report "ok" while the process is up.
func NewObservabilityMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler()) // O02: REAL Prometheus exposition
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.HandleFunc("/healthz", okHandler)  // liveness
	mux.HandleFunc("/readyz", okHandler)   // readiness
	mux.HandleFunc("/startupz", okHandler) // startup
	return mux
}

// okHandler answers the liveness/readiness/startup probes. It always returns
// 200 "ok" because, per A02/N35, probes reflect process/listener liveness and
// never upstream reachability.
func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// newInstrumentedServer builds the instrumented observability *http.Server
// bound to addr (O02: real mux wrapped with promhttp.InstrumentHandler* so the
// standard Go HTTP request metrics are collected). The instrumenter vectors are
// registered with the default registry idempotently so repeated calls (e.g. in
// tests) do not panic.
func newInstrumentedServer(addr string, log *logrus.Logger) *http.Server {
	requestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_request_duration_seconds",
		Help: "HTTP request duration in seconds for the observability endpoint.",
	}, []string{"method", "code"})
	responseSize := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_response_size_bytes",
		Help: "HTTP response size in bytes for the observability endpoint.",
	}, []string{"method", "code"})
	requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests to the observability endpoint.",
	}, []string{"code", "method"})

	register := func(c prometheus.Collector) {
		if err := prometheus.DefaultRegisterer.Register(c); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				log.Errorf("register observability metrics: %v", err)
			}
		}
	}
	register(requestDuration)
	register(responseSize)
	register(requestsTotal)

	instrumented := promhttp.InstrumentHandlerDuration(
		requestDuration,
		promhttp.InstrumentHandlerResponseSize(
			responseSize,
			promhttp.InstrumentHandlerCounter(
				requestsTotal,
				NewObservabilityMux(), // O02: the REAL mux holding all routes.
			),
		),
	)

	return &http.Server{
		Addr:              addr,
		Handler:           instrumented,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// StartObservabilityServer binds the observability mux (metrics + pprof +
// probes) to addr (default :8081, O01/O04) and serves it in a background
// goroutine. It returns the *http.Server so the composition root can shut it
// down. Metrics/pprof/probes are served here — never on the MCP app listener.
//
// O02: the real mux (which holds all routes, including /metrics) is wrapped
// with promhttp.InstrumentHandler* so the standard Go HTTP request metrics
// (per-request latency, response size, status-code counter) are collected.
func StartObservabilityServer(addr string, log *logrus.Logger) *http.Server {
	srv := newInstrumentedServer(addr, log)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("observability server on %s: %v", addr, err)
		}
	}()
	return srv
}

// RunObservabilityServer runs the observability listener (metrics + pprof +
// probes) on addr and blocks until ctx is cancelled, then gracefully shuts the
// *http.Server down and returns nil (O01: SIGTERM/SIGINT graceful shutdown).
// If the listener fails to bind or serve, the error is returned.
func RunObservabilityServer(ctx context.Context, addr string, log *logrus.Logger) error {
	srv := newInstrumentedServer(addr, log)

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("observability server on %s: %v", addr, err)
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-serveErr:
		return err
	}
}

// Package observability serves the internal observability endpoint (:8081):
// Prometheus metrics, pprof and the startup/readiness/liveness probes, plus
// upstream (Miniflux) response metrics.
//
// client_golang is blank-imported here only to pin the exact dependency
// version in go.mod while the package skeleton is empty; real usage lands with
// the metrics server in a later phase.
package observability

import _ "github.com/prometheus/client_golang/prometheus"

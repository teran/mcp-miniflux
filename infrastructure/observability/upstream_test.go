package observability

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// metricsRecorder mirrors the miniflux.MetricsRecorder port (O03). We do not
// import infrastructure/miniflux here to keep the layer edge clean
// (infrastructure -> infrastructure is not an allowed edge); conformance is
// structural, so satisfying this local mirror proves the port is implemented.
type metricsRecorder interface {
	ObserveUpstream(statusCode int, dur time.Duration, bytesIn, bytesOut int64)
}

// Compile-time check that UpstreamRecorder satisfies the miniflux port (O03).
var _ metricsRecorder = (*UpstreamRecorder)(nil)

func TestUpstreamRecorderSatisfiesInterface(t *testing.T) {
	var rec metricsRecorder = NewUpstreamMetrics("miniflux", "upstream")
	if rec == nil {
		t.Fatal("NewUpstreamMetrics returned nil")
	}
}

func TestNewUpstreamMetricsObserve(t *testing.T) {
	// Use a fresh registry so the test is isolated from the default one.
	reg := prometheus.NewRegistry()
	rec := NewUpstreamMetrics("miniflux", "upstream")
	reg.MustRegister(rec)

	rec.ObserveUpstream(200, 100*time.Millisecond, 512, 2048)
	rec.ObserveUpstream(200, 200*time.Millisecond, 256, 1024)
	rec.ObserveUpstream(500, 50*time.Millisecond, 64, 128)

	want := `
# HELP miniflux_upstream_request_duration_seconds Upstream (Miniflux) request duration in seconds.
# TYPE miniflux_upstream_request_duration_seconds histogram
miniflux_upstream_request_duration_seconds_bucket{le="0.005"} 0
miniflux_upstream_request_duration_seconds_bucket{le="0.01"} 0
miniflux_upstream_request_duration_seconds_bucket{le="0.025"} 0
miniflux_upstream_request_duration_seconds_bucket{le="0.05"} 1
miniflux_upstream_request_duration_seconds_bucket{le="0.1"} 2
miniflux_upstream_request_duration_seconds_bucket{le="0.25"} 3
miniflux_upstream_request_duration_seconds_bucket{le="0.5"} 3
miniflux_upstream_request_duration_seconds_bucket{le="1"} 3
miniflux_upstream_request_duration_seconds_bucket{le="2.5"} 3
miniflux_upstream_request_duration_seconds_bucket{le="5"} 3
miniflux_upstream_request_duration_seconds_bucket{le="10"} 3
miniflux_upstream_request_duration_seconds_bucket{le="+Inf"} 3
miniflux_upstream_request_duration_seconds_sum 0.35000000000000003
miniflux_upstream_request_duration_seconds_count 3
# HELP miniflux_upstream_request_size_bytes Upstream (Miniflux) request size in bytes.
# TYPE miniflux_upstream_request_size_bytes histogram
miniflux_upstream_request_size_bytes_bucket{le="100"} 1
miniflux_upstream_request_size_bytes_bucket{le="250"} 1
miniflux_upstream_request_size_bytes_bucket{le="500"} 2
miniflux_upstream_request_size_bytes_bucket{le="1000"} 3
miniflux_upstream_request_size_bytes_bucket{le="+Inf"} 3
miniflux_upstream_request_size_bytes_sum 832
miniflux_upstream_request_size_bytes_count 3
# HELP miniflux_upstream_response_size_bytes Upstream (Miniflux) response size in bytes.
# TYPE miniflux_upstream_response_size_bytes histogram
miniflux_upstream_response_size_bytes_bucket{le="128"} 1
miniflux_upstream_response_size_bytes_bucket{le="256"} 1
miniflux_upstream_response_size_bytes_bucket{le="512"} 1
miniflux_upstream_response_size_bytes_bucket{le="1024"} 2
miniflux_upstream_response_size_bytes_bucket{le="2048"} 3
miniflux_upstream_response_size_bytes_bucket{le="+Inf"} 3
miniflux_upstream_response_size_bytes_sum 3200
miniflux_upstream_response_size_bytes_count 3
# HELP miniflux_upstream_responses_total Upstream (Miniflux) responses, labelled by upstream status code.
# TYPE miniflux_upstream_responses_total counter
miniflux_upstream_responses_total{status="200"} 2
miniflux_upstream_responses_total{status="500"} 1
`
	if err := testutil.CollectAndCompare(reg, strings.NewReader(want)); err != nil {
		t.Fatalf("upstream metrics mismatch:\n%v", err)
	}
}

func TestNewUpstreamMetricsCounter(t *testing.T) {
	rec := NewUpstreamMetrics("test", "counter")
	reg := prometheus.NewRegistry()
	reg.MustRegister(rec)

	rec.ObserveUpstream(429, time.Millisecond, 10, 20)
	if got := testutil.ToFloat64(rec.responseStatus.WithLabelValues("429")); got != 1 {
		t.Errorf("responses_total{status=429} = %v, want 1", got)
	}
	if got := testutil.ToFloat64(rec.responseStatus.WithLabelValues("200")); got != 0 {
		t.Errorf("responses_total{status=200} = %v, want 0", got)
	}
}

package observability

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// UpstreamRecorder records upstream (Miniflux) response metrics (O03). Because
// this server is a proxying/passthrough server, it exposes upstream latency,
// request/response size and a status-code counter labelled by the upstream
// status code. It implements infrastructure/miniflux.MetricsRecorder and can be
// wired as miniflux.WithMetrics(recorder).
type UpstreamRecorder struct {
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.HistogramVec
	responseSize    *prometheus.HistogramVec
	responseStatus  *prometheus.CounterVec
}

// NewUpstreamMetrics constructs and registers an UpstreamRecorder with the
// default registry (so its series appear on /metrics) and returns it. The
// metric family names are prefixed by namespace and subsystem. Registration is
// idempotent: repeated construction with the same names does not panic.
func NewUpstreamMetrics(namespace, subsystem string) *UpstreamRecorder {
	u := &UpstreamRecorder{
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "request_duration_seconds",
			Help:      "Upstream (Miniflux) request duration in seconds.",
		}, nil),
		requestSize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "request_size_bytes",
			Help:      "Upstream (Miniflux) request size in bytes.",
			Buckets:   []float64{100, 250, 500, 1000},
		}, nil),
		responseSize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "response_size_bytes",
			Help:      "Upstream (Miniflux) response size in bytes.",
			Buckets:   []float64{128, 256, 512, 1024, 2048},
		}, nil),
		responseStatus: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "responses_total",
			Help:      "Upstream (Miniflux) responses, labelled by upstream status code.",
		}, []string{"status"}),
	}

	if err := prometheus.Register(u); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	return u
}

// ObserveUpstream records one upstream response (O03): latency, request and
// response sizes as histograms, and increments the status-code counter labelled
// by the upstream status. It implements miniflux.MetricsRecorder.
func (u *UpstreamRecorder) ObserveUpstream(statusCode int, dur time.Duration, bytesIn, bytesOut int64) {
	u.requestDuration.WithLabelValues().Observe(dur.Seconds())
	u.requestSize.WithLabelValues().Observe(float64(bytesIn))
	u.responseSize.WithLabelValues().Observe(float64(bytesOut))
	u.responseStatus.WithLabelValues(statusCodeString(statusCode)).Inc()
}

func statusCodeString(code int) string {
	return strconv.Itoa(code)
}

// Describe implements prometheus.Collector so the recorder can be registered on
// any registry (including a custom one in tests).
func (u *UpstreamRecorder) Describe(ch chan<- *prometheus.Desc) {
	u.requestDuration.Describe(ch)
	u.requestSize.Describe(ch)
	u.responseSize.Describe(ch)
	u.responseStatus.Describe(ch)
}

// Collect implements prometheus.Collector.
func (u *UpstreamRecorder) Collect(ch chan<- prometheus.Metric) {
	u.requestDuration.Collect(ch)
	u.requestSize.Collect(ch)
	u.responseSize.Collect(ch)
	u.responseStatus.Collect(ch)
}

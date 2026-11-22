package proxy

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var latencyBounds = [...]time.Duration{
	5 * time.Millisecond, 10 * time.Millisecond, 25 * time.Millisecond,
	50 * time.Millisecond, 100 * time.Millisecond, 250 * time.Millisecond,
	500 * time.Millisecond, time.Second, 2500 * time.Millisecond,
	5 * time.Second, 10 * time.Second,
}

type latencyHistogram struct {
	mu      sync.Mutex
	buckets [len(latencyBounds)]uint64
	count   uint64
	sumNS   uint64
}

func (h *latencyHistogram) observe(duration time.Duration) {
	if duration < 0 {
		duration = 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count++
	h.sumNS += uint64(duration)
	for index, bound := range latencyBounds {
		if duration <= bound {
			h.buckets[index]++
		}
	}
}

func (h *latencyHistogram) snapshot() ([len(latencyBounds)]uint64, uint64, uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.buckets, h.count, h.sumNS
}

type proxyMetrics struct {
	requests         atomic.Uint64
	upstreamRequests atomic.Uint64
	upstreamErrors   atomic.Uint64
	fallbacks        atomic.Uint64
	invalidCookies   atomic.Uint64
	logouts          atomic.Uint64
	requestDuration  latencyHistogram
	upstreamDuration latencyHistogram
}

func (m *proxyMetrics) writePrometheus(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	writeCounter := func(name string, value uint64) {
		_, _ = fmt.Fprintf(w, "# TYPE %s counter\n%s %d\n", name, name, value)
	}
	writeHistogram := func(name string, histogram *latencyHistogram) {
		_, _ = fmt.Fprintf(w, "# TYPE %s histogram\n", name)
		buckets, count, sumNS := histogram.snapshot()
		for index, bound := range latencyBounds {
			_, _ = fmt.Fprintf(w, "%s_bucket{le=%q} %d\n", name, strconv.FormatFloat(bound.Seconds(), 'f', -1, 64), buckets[index])
		}
		_, _ = fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n", name, count)
		_, _ = fmt.Fprintf(w, "%s_sum %g\n%s_count %d\n", name, float64(sumNS)/float64(time.Second), name, count)
	}

	writeCounter("idmux_requests_total", m.requests.Load())
	writeCounter("idmux_upstream_requests_total", m.upstreamRequests.Load())
	writeCounter("idmux_upstream_errors_total", m.upstreamErrors.Load())
	writeCounter("idmux_fallback_total", m.fallbacks.Load())
	writeCounter("idmux_invalid_cookies_total", m.invalidCookies.Load())
	writeCounter("idmux_logout_total", m.logouts.Load())
	writeHistogram("idmux_request_duration_seconds", &m.requestDuration)
	writeHistogram("idmux_upstream_duration_seconds", &m.upstreamDuration)
}

type measuredTransport struct {
	next    http.RoundTripper
	metrics *proxyMetrics
}

func (t measuredTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	started := time.Now()
	t.metrics.upstreamRequests.Add(1)
	response, err := t.next.RoundTrip(request)
	t.metrics.upstreamDuration.observe(time.Since(started))
	if response != nil && response.StatusCode >= http.StatusInternalServerError {
		recordUpstreamError(request, t.metrics)
	}
	return response, err
}

func recordUpstreamError(request *http.Request, metrics *proxyMetrics) {
	state, ok := request.Context().Value(requestStateKey{}).(*requestState)
	if ok {
		if state.upstreamError {
			return
		}
		state.upstreamError = true
	}
	metrics.upstreamErrors.Add(1)
}

var _ http.RoundTripper = measuredTransport{}

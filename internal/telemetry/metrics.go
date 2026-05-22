package telemetry

import (
	"sync"
	"time"
)

// MetricsCollector holds performance counters and histograms for the proxy.
type MetricsCollector struct {
	mu sync.RWMutex

	// Counters
	totalRequests   int64
	successRequests int64
	errorRequests   int64

	// TTFT (Time To First Token) tracking
	ttftCount    int64
	ttftTotalMs  float64
	ttftMinMs    float64
	ttftMaxMs    float64

	// Request duration tracking
	durationCount   int64
	durationTotalMs float64
	durationMinMs   float64
	durationMaxMs   float64
}

// NewMetricsCollector creates a new metrics collector with zeroed fields.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		ttftMinMs:     -1, // sentinel: unset
		ttftMaxMs:     -1,
		durationMinMs: -1,
		durationMaxMs: -1,
	}
}

// RecordRequest increments the total request counter.
func (m *MetricsCollector) RecordRequest() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalRequests++
}

// RecordSuccess increments the success counter.
func (m *MetricsCollector) RecordSuccess() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.successRequests++
}

// RecordError increments the error counter.
func (m *MetricsCollector) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorRequests++
}

// RecordTTFT records a single Time-To-First-Token observation.
func (m *MetricsCollector) RecordTTFT(d time.Duration) {
	ms := d.Seconds() * 1000

	m.mu.Lock()
	defer m.mu.Unlock()

	m.ttftCount++
	m.ttftTotalMs += ms

	if m.ttftMinMs < 0 || ms < m.ttftMinMs {
		m.ttftMinMs = ms
	}
	if m.ttftMaxMs < 0 || ms > m.ttftMaxMs {
		m.ttftMaxMs = ms
	}
}

// RecordDuration records a single request duration observation.
func (m *MetricsCollector) RecordDuration(d time.Duration) {
	ms := d.Seconds() * 1000

	m.mu.Lock()
	defer m.mu.Unlock()

	m.durationCount++
	m.durationTotalMs += ms

	if m.durationMinMs < 0 || ms < m.durationMinMs {
		m.durationMinMs = ms
	}
	if m.durationMaxMs < 0 || ms > m.durationMaxMs {
		m.durationMaxMs = ms
	}
}

// Snapshot returns a point-in-time copy of all metrics.
func (m *MetricsCollector) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap := MetricsSnapshot{
		TotalRequests:   m.totalRequests,
		SuccessRequests: m.successRequests,
		ErrorRequests:   m.errorRequests,
		TTFTCount:       m.ttftCount,
		DurationCount:   m.durationCount,
	}

	if m.ttftCount > 0 {
		snap.TTFTAvgMs = m.ttftTotalMs / float64(m.ttftCount)
		snap.TTFTMinMs = m.ttftMinMs
		snap.TTFTMaxMs = m.ttftMaxMs
	}

	if m.durationCount > 0 {
		snap.DurationAvgMs = m.durationTotalMs / float64(m.durationCount)
		snap.DurationMinMs = m.durationMinMs
		snap.DurationMaxMs = m.durationMaxMs
	}

	return snap
}

// MetricsSnapshot is a read-only view of metrics at a single point in time.
type MetricsSnapshot struct {
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	ErrorRequests   int64   `json:"error_requests"`
	TTFTCount       int64   `json:"ttft_count"`
	TTFTAvgMs       float64 `json:"ttft_avg_ms"`
	TTFTMinMs       float64 `json:"ttft_min_ms"`
	TTFTMaxMs       float64 `json:"ttft_max_ms"`
	DurationCount   int64   `json:"duration_count"`
	DurationAvgMs   float64 `json:"duration_avg_ms"`
	DurationMinMs   float64 `json:"duration_min_ms"`
	DurationMaxMs   float64 `json:"duration_max_ms"`
}

// RequestTimer is a helper that tracks both TTFT and total duration for a single request.
type RequestTimer struct {
	collector  *MetricsCollector
	start      time.Time
	ttftRecorded bool
}

// StartRequestTimer begins timing a new request and returns the timer.
// The caller must call RecordTTFT when the first token arrives and Stop when the request ends.
func (m *MetricsCollector) StartRequestTimer() *RequestTimer {
	m.RecordRequest()
	return &RequestTimer{
		collector: m,
		start:     time.Now(),
	}

}

// RecordTTFT marks the moment the first token was received.
func (rt *RequestTimer) RecordTTFT() {
	if rt.ttftRecorded {
		return
	}
	rt.ttftRecorded = true
	rt.collector.RecordTTFT(time.Since(rt.start))
}

// Stop marks the request as complete and records its total duration.
// If no TTFT was explicitly recorded, this also records TTFT == duration (fallback).
func (rt *RequestTimer) Stop(success bool) {
	duration := time.Since(rt.start)

	if !rt.ttftRecorded {
		rt.collector.RecordTTFT(duration)
	}

	rt.collector.RecordDuration(duration)

	if success {
		rt.collector.RecordSuccess()
	} else {
		rt.collector.RecordError()
	}
}

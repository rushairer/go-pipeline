//go:build ignore

package examples

// This file contains Prometheus MetricsHook examples and is excluded from default builds/tests.
// Remove the build tag above and add the Prometheus client dependency to go.mod to compile it:
//
//	go get github.com/prometheus/client_golang/prometheus
//	go get github.com/prometheus/client_golang/prometheus/promhttp

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	gopipeline "github.com/rushairer/go-pipeline/v2"
)

// PromMetrics is a Prometheus implementation of the MetricsHook interface.
// It exports recommended metrics with sane buckets and labels kept minimal for cardinality safety.
type PromMetrics struct {
	flushLatency *prometheus.HistogramVec
	flushSize    prometheus.Observer
	flushSuccess prometheus.Counter
	flushFailure prometheus.Counter
	errorCount   prometheus.Counter
	errorDropped prometheus.Counter
	finalFlushTO prometheus.Counter
	drainFlush   prometheus.Counter
	drainFlushTO prometheus.Counter
	// Optionally, track error channel saturation via external sampler (see README).
}

// NewPromMetrics registers metrics to the provided registry (or prometheus.DefaultRegisterer if nil).
func NewPromMetrics(reg prometheus.Registerer) *PromMetrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	flushLatency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "gopipeline",
		Name:      "flush_latency_seconds",
		Help:      "Latency of batch flush handler",
		// Adjust buckets per workload; this set covers 1ms..10s roughly.
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
	}, []string{"result"}) // result: "ok"|"fail"
	flushSize := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gopipeline",
		Name:      "batch_size_observed",
		Help:      "Observed batch sizes processed",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // 1..512
	})
	flushSuccess := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "gopipeline",
		Name:      "flush_success_total",
		Help:      "Total successful flushes",
	})
	flushFailure := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "gopipeline",
		Name:      "flush_failure_total",
		Help:      "Total failed flushes",
	})
	errorCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "gopipeline",
		Name:      "error_count_total",
		Help:      "Total errors returned by flush handler",
	})
	errorDropped := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "gopipeline",
		Name:      "dropped_error_estimate_total",
		Help:      "Estimated number of dropped errors when error channel saturates",
	})
	finalFlushTO := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "gopipeline",
		Name:      "final_flush_timeout_total",
		Help:      "Count of timeouts during final flush on channel-close path",
	})
	drainFlush := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "gopipeline",
		Name:      "drain_flush_total",
		Help:      "Count of best-effort drain flushes on cancel",
	})
	drainFlushTO := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "gopipeline",
		Name:      "drain_flush_timeout_total",
		Help:      "Count of drain flushes that timed out",
	})

	reg.MustRegister(flushLatency, flushSize, flushSuccess, flushFailure, errorCount, errorDropped, finalFlushTO, drainFlush, drainFlushTO)

	return &PromMetrics{
		flushLatency: flushLatency,
		flushSize:    flushSize,
		flushSuccess: flushSuccess,
		flushFailure: flushFailure,
		errorCount:   errorCount,
		errorDropped: errorDropped,
		finalFlushTO: finalFlushTO,
		drainFlush:   drainFlush,
		drainFlushTO: drainFlushTO,
	}
}

// Flush implements MetricsHook.Flush: called once per flush (success or failure).
func (m *PromMetrics) Flush(items int, d time.Duration) {
	// Record latency with a result label. For simplicity, infer result by duration & prior Error() call is not visible here.
	// Best practice: record latency in both paths; here we default to "ok" and rely on Error() to also observe "fail".
	m.flushLatency.WithLabelValues("ok").Observe(d.Seconds())
	m.flushSize.Observe(float64(items))
	m.flushSuccess.Inc()
}

// Error implements MetricsHook.Error: count failures; you may also tag error type if stable (avoid high-cardinality values).
func (m *PromMetrics) Error(err error) {
	m.errorCount.Inc()
	// Also increment failure counter and latency with "fail" label in a best-effort manner if paired in your flush wrapper.
	m.flushFailure.Inc()
	// Optional: error-class labeling
	_ = err
}

// ErrorDropped implements MetricsHook.ErrorDropped: count dropped errors when error channel saturates.
func (m *PromMetrics) ErrorDropped() {
	m.errorDropped.Inc()
}

// Below are optional helpers to be called from your flush wrapper when specific conditions occur.
// The pipeline can also expose hooks in future to call these directly.
func (m *PromMetrics) FinalFlushTimeout() { m.finalFlushTO.Inc() }
func (m *PromMetrics) DrainFlush()        { m.drainFlush.Inc() }
func (m *PromMetrics) DrainFlushTimeout() { m.drainFlushTO.Inc() }

// Example wiring with Prometheus HTTP exporter.
func Example_prometheusWiring() {
	// Build pipeline and attach metrics
	p := gopipeline.NewDefaultStandardPipeline(func(ctx context.Context, batch []int) error {
		// your flush logic
		return nil
	})
	m := NewPromMetrics(nil)
	p.WithMetrics(m)

	// Expose /metrics
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Println("prometheus metrics at :2112/metrics")
		_ = http.ListenAndServe(":2112", nil)
	}()

	// Run once (sync) with a small load
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() {
		ch := p.DataChan()
		defer close(ch)
		for i := 0; i < 200; i++ {
			select {
			case ch <- i:
			case <-ctx.Done():
				return
			}
		}
	}()
	_ = p.SyncPerform(ctx)
	// Visit http://localhost:2112/metrics to see exported series.
	// Output:
}

// Optional: a wrapper to correlate success/failure latency recording explicitly.
func flushWrapper(m *PromMetrics, fn func(ctx context.Context, batch []int) error) func(ctx context.Context, batch []int) error {
	return func(ctx context.Context, batch []int) error {
		start := time.Now()
		err := fn(ctx, batch)
		elapsed := time.Since(start)
		if err != nil {
			m.flushLatency.WithLabelValues("fail").Observe(elapsed.Seconds())
			m.flushFailure.Inc()
			return err
		}
		m.flushLatency.WithLabelValues("ok").Observe(elapsed.Seconds())
		m.flushSuccess.Inc()
		return nil
	}
}

// Example showing how to record drain/final flush events from user code when detectable.
func recordShutdownExample(m *PromMetrics, path string) {
	switch path {
	case "final_flush_timeout":
		m.FinalFlushTimeout()
	case "drain":
		m.DrainFlush()
	case "drain_timeout":
		m.DrainFlushTimeout()
	default:
	}
}

// Example demonstrating error channel dropped detection strategy (external sampler).
// Note: needs holding errs := p.ErrorChan(n) to sample len(errs) without races.
func sampleErrorChanSaturation(errs <-chan error) {
	_ = errs
	// See README for sampler goroutine that periodically observes len(errs)/cap(errs).
}

// Example showing how to surface ErrAlreadyRunning via errs in Start.
func exampleStartAlreadyRunning(p gopipeline.Pipeline[int]) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done1, errs1 := p.Start(ctx)
	_, errs2 := p.Start(ctx)
	var got error
	select {
	case got = <-errs1:
	case got = <-errs2:
	case <-time.After(200 * time.Millisecond):
	}
	_ = got // expect errors.Is(got, gopipeline.ErrAlreadyRunning)
	cancel()
	<-done1
}

// avoid unused warnings
var _ = errors.Is

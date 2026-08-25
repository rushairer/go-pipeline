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

// Flush implements MetricsHook.Flush: called once per flush, both on success and on failure.
// The interface does not expose the result, so only neutral stats (batch size) are recorded
// here. Failure counting happens in Error(); precise latency with an ok/fail result label and
// the success counter require wrapping the flush handler with flushWrapper (see below).
func (m *PromMetrics) Flush(items int, d time.Duration) {
	_ = d // duration is meaningful only when paired with a result; recorded in flushWrapper
	m.flushSize.Observe(float64(items))
}

// Error implements MetricsHook.Error: called by the pipeline when the flush handler returns
// an error. This is the failure counter; you may also tag error type if stable (avoid
// high-cardinality values).
func (m *PromMetrics) Error(err error) {
	m.errorCount.Inc()
	m.flushFailure.Inc()
	_ = err // optional: error-class labeling
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
	// Build pipeline and attach metrics. Wrap the flush handler with flushWrapper so
	// that success counting and ok/fail latency are recorded precisely.
	m := NewPromMetrics(nil)
	p := gopipeline.NewDefaultStandardPipeline(flushWrapper(m, func(ctx context.Context, batch []int) error {
		// your flush logic
		return nil
	}))
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

// flushWrapper wraps your flush handler to record precise per-result latency and the
// success counter. Failure is already counted by MetricsHook.Error(), so the failure
// branch must NOT increment flushFailure again (that would double-count failed flushes).
func flushWrapper(m *PromMetrics, fn func(ctx context.Context, batch []int) error) func(ctx context.Context, batch []int) error {
	return func(ctx context.Context, batch []int) error {
		start := time.Now()
		err := fn(ctx, batch)
		elapsed := time.Since(start)
		if err != nil {
			m.flushLatency.WithLabelValues("fail").Observe(elapsed.Seconds())
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
// Start (and thus ErrAlreadyRunning) lives on the concrete *StandardPipeline type, not on
// the Pipeline interface. ErrAlreadyRunning is delivered asynchronously (a second Start
// triggers a fire-and-forget AsyncPerform that reports it), so a single short select can
// miss it: poll until observed or timeout. Both Start calls share the same internal error
// channel.
func exampleStartAlreadyRunning(p *gopipeline.StandardPipeline[int]) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done1, errs1 := p.Start(ctx)
	_, errs2 := p.Start(ctx) // second Start: ErrAlreadyRunning arrives asynchronously

	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case err := <-errs1:
			if errors.Is(err, gopipeline.ErrAlreadyRunning) {
				cancel()
				<-done1
				return
			}
		case err := <-errs2:
			if errors.Is(err, gopipeline.ErrAlreadyRunning) {
				cancel()
				<-done1
				return
			}
		case <-timer.C:
			log.Printf("warning: did not observe ErrAlreadyRunning within timeout")
			return
		}
	}
}

// avoid unused warnings
var _ = errors.Is

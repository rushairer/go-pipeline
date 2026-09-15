package gopipeline_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

// contractMetrics is intentionally minimal: these tests only need to observe
// whether the non-blocking error path reports dropped error events.
type contractMetrics struct {
	dropped atomic.Int32
}

func (*contractMetrics) Flush(int, time.Duration) {}
func (*contractMetrics) Error(error)              {}
func (m *contractMetrics) ErrorDropped()          { m.dropped.Add(1) }

// TestAsyncPerformAllowsConcurrentFlushes locks the core throughput contract:
// AsyncPerform may dispatch multiple full batches concurrently. A slow batch
// must not force unrelated batches to execute serially while concurrency
// capacity remains available.
func TestAsyncPerformAllowsConcurrentFlushes(t *testing.T) {
	cfg := gopipeline.NewPipelineConfig().
		WithBufferSize(16).
		WithFlushSize(1).
		WithFlushInterval(24 * time.Hour).
		WithMaxConcurrentFlushes(2)

	started := make(chan struct{}, 2)
	release := make(chan struct{})

	p := gopipeline.NewStandardPipeline[int](cfg, func(ctx context.Context, batch []int) error {
		started <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done, _ := p.Start(ctx)
	ch := p.DataChan()
	ch <- 1
	ch <- 2

	// Both flushes must be able to enter before either is released. If the
	// implementation accidentally serializes async flushes, the second signal
	// never arrives and this test fails.
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("expected two concurrent async flushes; only %d started", i)
		}
	}

	close(release)
	close(ch)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pipeline run loop did not finish")
	}
}

// TestDoneSignalsRunLoopCompletionNotAsyncFlushJoin locks the documented Done
// semantics. Done is the completion signal for the pipeline run loop; it is not
// a global join barrier for async flush goroutines that were already dispatched.
func TestDoneSignalsRunLoopCompletionNotAsyncFlushJoin(t *testing.T) {
	cfg := gopipeline.NewPipelineConfig().
		WithBufferSize(8).
		WithFlushSize(1).
		WithFlushInterval(24 * time.Hour)

	flushStarted := make(chan struct{})
	flushDone := make(chan struct{})
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()

	p := gopipeline.NewStandardPipeline[int](cfg, func(ctx context.Context, batch []int) error {
		close(flushStarted)
		defer close(flushDone)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done, _ := p.Start(ctx)
	ch := p.DataChan()
	ch <- 1

	select {
	case <-flushStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("async flush did not start")
	}

	// The only full batch is already dispatched and deliberately blocked. Once
	// the input channel is closed, the run loop can finish without joining that
	// worker.
	close(ch)

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Done should signal run-loop completion without waiting for async flush join")
	}

	select {
	case <-flushDone:
		t.Fatal("async flush unexpectedly completed before it was released")
	default:
		// Expected: Done can close while an already-dispatched async flush remains in flight.
	}

	close(release)

	select {
	case <-flushDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("async flush did not finish after release")
	}
}

// TestErrorChannelSaturationDoesNotBlockPipeline locks the observability
// contract: ErrorChan is best-effort. A slow or absent error consumer must not
// become backpressure for the data-processing hot path.
func TestErrorChannelSaturationDoesNotBlockPipeline(t *testing.T) {
	cfg := gopipeline.NewPipelineConfig().
		WithBufferSize(32).
		WithFlushSize(1).
		WithFlushInterval(24 * time.Hour).
		WithMaxConcurrentFlushes(1)

	metrics := &contractMetrics{}
	p := gopipeline.NewStandardPipeline[int](cfg, func(ctx context.Context, batch []int) error {
		return errors.New("expected flush failure")
	}).WithMetrics(metrics)

	// Capacity one is intentional and this channel is deliberately never
	// consumed. The first error occupies the slot; later errors must be dropped
	// rather than block flush completion and therefore the pipeline.
	errs := p.ErrorChan(1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done, _ := p.Start(ctx)
	ch := p.DataChan()
	for i := 0; i < 8; i++ {
		ch <- i
	}
	close(ch)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pipeline blocked because ErrorChan was saturated")
	}

	if got := len(errs); got != 1 {
		t.Fatalf("expected saturated error channel to retain one error, got %d", got)
	}
	if got := metrics.dropped.Load(); got == 0 {
		t.Fatal("expected at least one dropped error event after ErrorChan saturation")
	}
}

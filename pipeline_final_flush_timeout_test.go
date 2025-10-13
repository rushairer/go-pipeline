package gopipeline

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestFinalFlushOnCloseTimeout verifies that on the channel-close path
// the final synchronous flush is bounded by FinalFlushOnCloseTimeout when set > 0.
// It constructs a flush function that would normally block longer than the timeout,
// and asserts that the total time to exit is close to (and not significantly above) the configured timeout.
// Note: The flush function must respect ctx.Done() to allow timely exit on timeout.
func TestFinalFlushOnCloseTimeout(t *testing.T) {
	// Timeout budget for the final flush on channel-close path
	const finalTimeout = 150 * time.Millisecond

	// A flush function that simulates long work but respects ctx
	// It loops until either the intended long duration passes OR ctx is done.
	longWork := 500 * time.Millisecond
	var calls int32
	flush := func(ctx context.Context, batch []int) error {
		atomic.AddInt32(&calls, 1)
		start := time.Now()
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				// Respect the timeout: return promptly
				return ctx.Err()
			case <-tick.C:
				if time.Since(start) >= longWork {
					return nil
				}
			}
		}
	}

	cfg := NewPipelineConfig().
		WithBufferSize(16).
		WithFlushSize(8).
		WithFlushInterval(24 * time.Hour). // avoid timer-driven flush
		WithFinalFlushOnCloseTimeout(finalTimeout)

	p := NewStandardPipeline[int](cfg, flush)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start async run
	errs := p.ErrorChan(16)
	doneCh := make(chan struct{})
	go func() {
		_ = p.AsyncPerform(ctx)
		close(doneCh)
	}()

	// Send fewer than FlushSize items so that only the final flush is triggered by closing channel.
	ch := p.DataChan()
	for i := 0; i < 3; i++ {
		select {
		case ch <- i:
		case <-ctx.Done():
			t.Fatalf("context canceled before sending test data: %v", ctx.Err())
		}
	}
	// Close to trigger final flush under timeout.
	start := time.Now()
	close(ch)

	// Wait for the run to finish (local doneCh)
	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("pipeline did not finish within expected window")
	}
	elapsed := time.Since(start)

	// We expect exactly one final flush call
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 final flush call, got %d", got)
	}

	// The flush would take ~500ms if not bounded, but with FinalFlushOnCloseTimeout=150ms
	// the total elapsed should be close to that timeout (allowing scheduling jitter).
	if elapsed < finalTimeout/2 || elapsed > finalTimeout+100*time.Millisecond {
		t.Fatalf("final flush elapsed not within expected bounds: got=%v, want around=%v (+100ms jitter)", elapsed, finalTimeout)
	}

	// Drain any error sent (implementation may return context.DeadlineExceeded on timeout)
	// Non-blocking drain with short timeout
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer drainCancel()
	for {
		select {
		case err, ok := <-errs:
			if !ok {
				break
			}
			_ = err // optional: could assert errors.Is(err, context.DeadlineExceeded)
		case <-drainCtx.Done():
			goto DONE
		}
	}
DONE:
}

// Control test: when FinalFlushOnCloseTimeout == 0, the final flush uses context.Background()
// and thus should not be force-canceled by timeout (we simulate shorter longWork to avoid slow tests).
func TestFinalFlushOnCloseTimeout_Disabled(t *testing.T) {
	// Use a shorter simulated work to keep the test time reasonable
	longWork := 120 * time.Millisecond
	var calls int32
	flush := func(ctx context.Context, batch []int) error {
		atomic.AddInt32(&calls, 1)
		// Since no timeout is applied (Background ctx), we just sleep and return.
		// Ignore ctx here to simulate an operation that doesn't observe cancellation.
		time.Sleep(longWork)
		return nil
	}

	cfg := NewPipelineConfig().
		WithBufferSize(16).
		WithFlushSize(8).
		WithFlushInterval(24 * time.Hour). // avoid timer-driven flush
		WithFinalFlushOnCloseTimeout(0)    // disabled

	p := NewStandardPipeline[int](cfg, flush)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	doneCh := make(chan struct{})
	go func() {
		_ = p.AsyncPerform(ctx)
		close(doneCh)
	}()

	ch := p.DataChan()
	for i := 0; i < 2; i++ {
		ch <- i
	}
	start := time.Now()
	close(ch)

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("pipeline did not finish within expected window")
	}
	elapsed := time.Since(start)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 final flush call, got %d", got)
	}

	// Expect to be around longWork (no timeout applied)
	if elapsed < longWork/2 || elapsed > longWork+80*time.Millisecond {
		t.Fatalf("final flush elapsed not within expected bounds (no-timeout case): got=%v, want around=%v", elapsed, longWork)
	}
}

// Optional assertion to ensure the flush function actually observes ctx cancel when timeout is set.
// This test uses an explicit context.DeadlineExceeded expectation if the implementation forwards ctx error.
func TestFinalFlushOnCloseTimeout_ErrorSurfaceOptional(t *testing.T) {
	const finalTimeout = 80 * time.Millisecond
	flush := func(ctx context.Context, batch []string) error {
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				// keep waiting
			}
		}
	}

	cfg := NewPipelineConfig().
		WithBufferSize(8).
		WithFlushSize(4).
		WithFlushInterval(24 * time.Hour).
		WithFinalFlushOnCloseTimeout(finalTimeout)

	p := NewStandardPipeline[string](cfg, flush)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errs := p.ErrorChan(8)
	doneCh := make(chan struct{})
	go func() {
		_ = p.AsyncPerform(ctx)
		close(doneCh)
	}()

	ch := p.DataChan()
	ch <- "a"
	close(ch)

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("pipeline did not finish within expected window")
	}

	// Try to read at least one error that may contain DeadlineExceeded
	timeout := time.NewTimer(100 * time.Millisecond)
	defer timeout.Stop()
	var gotErr error
READ:
	for {
		select {
		case err, ok := <-errs:
			if !ok {
				break READ
			}
			if err != nil {
				gotErr = err
				break READ
			}
		case <-timeout.C:
			break READ
		}
	}
	if gotErr != nil && !errors.Is(gotErr, context.DeadlineExceeded) {
		t.Fatalf("expected ctx deadline exceeded or nil, got %v", gotErr)
	}
}

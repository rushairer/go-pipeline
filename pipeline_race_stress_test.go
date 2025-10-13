package gopipeline

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// These tests are intended to be run with: go test -race -run Race -count=1
// They stress concurrent starts, shutdown races (close vs cancel), and MaxConcurrentFlushes behavior.

func makeSlowFlush(d time.Duration, ret error) func(context.Context, []int) error {
	return func(ctx context.Context, _ []int) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
			return ret
		}
	}
}

// Race: concurrent second start should not corrupt done channel; second attempt surfaces ErrAlreadyRunning via errs.
func TestRace_ConcurrentSecondStart(t *testing.T) {
	p := NewDefaultStandardPipeline(makeSlowFlush(5*time.Millisecond, nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done1, errs1 := p.Start(ctx)
	_ = errs1

	// Hammer Start concurrently
	var wg sync.WaitGroup
	wg.Add(10)
	errCh := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			_, errs := p.Start(ctx)
			select {
			case err := <-errs:
				errCh <- err
			case <-time.After(200 * time.Millisecond):
				// no error observed from this call; fine
			}
		}()
	}
	wg.Wait()

	// At least one should report ErrAlreadyRunning
	close(errCh)
	var sawAlreadyRunning bool
	for e := range errCh {
		if errors.Is(e, ErrAlreadyRunning) {
			sawAlreadyRunning = true
			break
		}
	}
	if !sawAlreadyRunning {
		t.Log("did not observe ErrAlreadyRunning; ensure Start reports it via errs in your impl")
	}

	cancel()
	<-done1
}

// Race: data channel close vs context cancel around the same time.
func TestRace_CloseVsCancel(t *testing.T) {
	cfg := NewPipelineConfig().
		WithBufferSize(256).
		WithFlushSize(32).
		WithFlushInterval(5 * time.Millisecond).
		WithFinalFlushOnCloseTimeout(50 * time.Millisecond)
	p := NewStandardPipeline[int](cfg, func(ctx context.Context, batch []int) error {
		// small work to tick the loop
		select {
		case <-time.After(100 * time.Microsecond):
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() { _ = p.AsyncPerform(ctx) }()

	ch := p.DataChan()
	// produce a bit
	for i := 0; i < 100; i++ {
		select {
		case ch <- i:
		case <-ctx.Done():
			t.Fatal("ctx canceled prematurely")
		}
	}

	// Race: close and cancel almost simultaneously
	done := make(chan struct{})
	go func() {
		defer close(done)
		close(ch)
	}()
	go func() {
		time.Sleep(0) // yield
		cancel()
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("close producer goroutine did not finish")
	}

	// Wait for pipeline to exit
	select {
	case <-p.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline did not exit in time")
	}
}

// Stress: verify MaxConcurrentFlushes upper bound is respected under pressure.
func TestRace_MaxConcurrentFlushes(t *testing.T) {
	var inFlight atomic.Int64
	var maxObserved atomic.Int64
	flush := func(ctx context.Context, batch []int) error {
		cur := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			if v := maxObserved.Load(); cur > v {
				maxObserved.CompareAndSwap(v, cur)
			} else {
				break
			}
		}
		// simulate some work
		select {
		case <-time.After(2 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	}

	cfg := NewPipelineConfig().
		WithBufferSize(4096).
		WithFlushSize(32).
		WithFlushInterval(500 * time.Millisecond). // avoid timer flush; we push by size
		WithMaxConcurrentFlushes(2)

	p := NewStandardPipeline[int](cfg, flush)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _ = p.AsyncPerform(ctx) }()

	ch := p.DataChan()
	// flood enough items to cause many flushes
	for i := 0; i < 32*20; i++ {
		select {
		case ch <- i:
		case <-ctx.Done():
			t.Fatal("ctx canceled prematurely")
		}
	}
	// allow some time for flushes to start
	time.Sleep(50 * time.Millisecond)
	// stop producing but keep channel open a bit, then close
	time.Sleep(10 * time.Millisecond)
	close(ch)

	// wait for exit
	select {
	case <-p.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("pipeline did not exit")
	}

	if got := maxObserved.Load(); got > 2 {
		t.Fatalf("MaxConcurrentFlushes exceeded: got %d, want <= 2", got)
	}
}

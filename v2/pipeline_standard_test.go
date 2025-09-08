package gopipeline_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

func TestStandardPipelineAsyncPerform(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			FlushSize:     32,
			BufferSize:    64,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			select {
			case <-ctx.Done():
			default:
				// AsyncPerform 时 可以是 flush无序更加明显
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
				//t.Log(batchData)
			}
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.AsyncPerform(ctx, flushError)

	// Add some data
	for i := 0; i < 6478017; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			t.Fatalf("Failed to add item: %v", err)
		}
	}

	select {
	case err, ok := <-flushError:
		if ok {
			t.Errorf("Expected no error, got %v", err)
		} else {
			t.Log("flushError closed")
		}
	case <-ctx.Done():
	}

	if processedCount != 6478017 {
		t.Errorf("Expected 6478017 processed items, got %d", processedCount)
	}
}

func TestStandardPipelineAsyncPerformWithFlushError(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			FlushSize:     32,
			BufferSize:    64,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			select {
			case <-ctx.Done():
			default:
				// AsyncPerform 时 可以是 flush无序更加明显
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
				//t.Log(batchData)
			}
			return newErrorWithData(batchData, errors.New("test error"))
		})

	var flushError = make(chan error, 1)
	go pipeline.AsyncPerform(ctx, flushError)

	// Add some data
	for i := 0; i < 6478017; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			t.Fatalf("Failed to add item: %v", err)
		}
	}

	for err := range flushError {
		t.Log(err)
		if e, ok := err.(errorWithData); ok {
			if e.Err.Error() != "test error" {
				t.Errorf("Expected error, got nil")
			}
		}
	}

	if processedCount != 6478017 {
		t.Errorf("Expected 6478017 processed items, got %d", processedCount)
	}
}

func TestStandardPipelineAsyncPerformTimeout(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			FlushSize:     32,
			BufferSize:    64,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			select {
			case <-ctx.Done():
			default:
				// SyncPerform 是有序的，即使sleep也不影响
				time.Sleep(time.Second * 6)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
				//t.Log(batchData)
			}
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.AsyncPerform(ctx, flushError)

	// Add some data
	for i := 0; i < 6478017; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			if errors.Is(err, gopipeline.ErrContextIsClosed) {
				t.Log("gopipeline.ErrContextIsClosed", err)
				break
			} else if errors.Is(err, gopipeline.ErrChannelIsClosed) {
				t.Log("gopipeline.ErrChannelIsClosed", err)
			} else {
				t.Fatalf("Failed to add item: %v", err)
			}
		}
	}

	select {
	case err, ok := <-flushError:
		if ok {
			t.Errorf("Expected no error, got %v", err)
		} else {
			t.Log("flushError closed")
		}
	case <-ctx.Done():
	}

	if processedCount == 6478017 {
		t.Errorf("Expected less than 6478017 processed items, got %d", processedCount)
	}
}

func TestStandardPipelineSyncPerform(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			FlushSize:     32,
			BufferSize:    64,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			select {
			case <-ctx.Done():
			default:
				// SyncPerform 是有序的，即使sleep也不影响
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
				//t.Log(batchData)
			}
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.SyncPerform(ctx, flushError)

	// Add some data
	for i := 0; i < 2000; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			t.Fatalf("Failed to add item: %v", err)
		}
	}

	select {
	case err, ok := <-flushError:
		if ok {
			t.Errorf("Expected no error, got %v", err)
		} else {
			t.Log("flushError closed")
		}
	case <-ctx.Done():
	}

	if processedCount != 2000 {
		t.Errorf("Expected 2000 processed items, got %d", processedCount)
	}
}

func TestStandardPipelineSyncPerformWithFlushError(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			FlushSize:     32,
			BufferSize:    64,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			select {
			case <-ctx.Done():
			default:
				// SyncPerform 是有序的，即使sleep也不影响
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
				//t.Log(batchData)
			}
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.SyncPerform(ctx, flushError)

	// Add some data
	for i := 0; i < 2000; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			t.Fatalf("Failed to add item: %v", err)
		}
	}

	for err := range flushError {
		t.Log(err)
		if e, ok := err.(errorWithData); ok {
			if e.Err.Error() != "test error" {
				t.Errorf("Expected error, got nil")
			}
		}
	}

	if processedCount != 2000 {
		t.Errorf("Expected 2000 processed items, got %d", processedCount)
	}
}

func TestStandardPipelineSyncPerformTimeout(t *testing.T) {
	var mux sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			FlushSize:     32,
			BufferSize:    64,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			select {
			case <-ctx.Done():
			default:
				// SyncPerform 是有序的，即使sleep也不影响
				time.Sleep(time.Millisecond * 1000)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
				//t.Log(batchData)
			}
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.SyncPerform(ctx, flushError)

	// Add some data
	for i := 0; i < 2000; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			if errors.Is(err, gopipeline.ErrContextIsClosed) {
				t.Log("gopipeline.ErrContextIsClosed", err)
				break
			} else if errors.Is(err, gopipeline.ErrChannelIsClosed) {
				t.Log("gopipeline.ErrChannelIsClosed", err)
			} else {
				t.Fatalf("Failed to add item: %v", err)
			}
		}
	}

	select {
	case err, ok := <-flushError:
		if ok {
			t.Errorf("Expected no error, got %v", err)
		} else {
			t.Log("flushError closed")
		}
	case <-ctx.Done():
	}

	if processedCount == 2000 {
		t.Errorf("Expected less than 2000 processed items, got %d", processedCount)
	}
}

type errorWithData struct {
	Data any
	Err  error
}

func (e errorWithData) Error() string {
	return e.Err.Error()
}

func newErrorWithData(data any, err error) errorWithData {
	return errorWithData{
		Data: data,
		Err:  err,
	}
}

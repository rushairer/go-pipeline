package gopipeline_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline"
)

func TestPipelineBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	var processedCount int
	pipeline := gopipeline.NewPipeline[int](
		gopipeline.PipelineConfig{
			FlushSize:     10,
			FlushInterval: time.Millisecond * 100,
			BufferSize:    20,
		},
		func(ctx context.Context, batchData []int) error {
			processedCount += len(batchData)
			return nil
		})

	go pipeline.AsyncPerform(ctx)

	for i := 0; i < 50; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			t.Fatalf("Failed to add item: %v", err)
		}
	}

	time.Sleep(time.Second)
	if processedCount != 50 {
		t.Errorf("Expected 50 processed items, got %d", processedCount)
	}
}

func TestPipelineErrorHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	expectedError := errors.New("process error")
	pipeline := gopipeline.NewPipeline[int](
		gopipeline.PipelineConfig{
			FlushSize:     500,
			FlushInterval: time.Millisecond * 100,
			BufferSize:    1000,
		},
		func(ctx context.Context, batchData []int) error {
			return expectedError
		})

	go pipeline.AsyncPerform(ctx)

	// Add some data
	for i := 0; i < 10; i++ {
		_ = pipeline.Add(ctx, i)
	}

	select {
	case err := <-pipeline.GetErrorChan():
		if err != expectedError {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	case <-time.After(time.Second * 10):
		t.Error("Timeout waiting for error")
	}
}

func TestPipelineConcurrent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	var processedCount int32
	var mu sync.Mutex
	processed := make(map[int]bool)

	pipeline := gopipeline.NewPipeline[int](
		gopipeline.PipelineConfig{
			FlushSize:     100,
			FlushInterval: time.Millisecond * 100,
			BufferSize:    1000,
		},
		func(ctx context.Context, batchData []int) error {
			mu.Lock()
			for _, item := range batchData {
				processed[item] = true
			}
			mu.Unlock()
			return nil
		})

	go pipeline.AsyncPerform(ctx)

	var wg sync.WaitGroup
	workers := 10
	itemsPerWorker := 100

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < itemsPerWorker; i++ {
				item := workerID*itemsPerWorker + i
				if err := pipeline.Add(ctx, item); err != nil {
					t.Errorf("Worker %d failed to add item: %v", workerID, err)
					return
				}
			}
		}(w)
	}

	wg.Wait()
	time.Sleep(time.Second)

	mu.Lock()
	processedCount = int32(len(processed))
	mu.Unlock()

	expectedCount := workers * itemsPerWorker
	if int(processedCount) != expectedCount {
		t.Errorf("Expected %d processed items, got %d", expectedCount, processedCount)
	}
}

func BenchmarkPipelineSmallBatch(b *testing.B) {
	ctx := context.Background()
	pipeline := gopipeline.NewPipeline[int](
		gopipeline.PipelineConfig{
			FlushSize:     100,
			FlushInterval: time.Millisecond * 100,
			BufferSize:    200,
		},
		func(ctx context.Context, batchData []int) error {
			return nil
		})

	go pipeline.AsyncPerform(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pipeline.Add(ctx, i)
	}
}

func BenchmarkPipelineLargeBatch(b *testing.B) {
	ctx := context.Background()
	pipeline := gopipeline.NewPipeline[[]string](
		gopipeline.PipelineConfig{
			FlushSize:     100000,
			FlushInterval: time.Second * 2,
			BufferSize:    200000,
		},
		func(ctx context.Context, batchData [][]string) error {
			return nil
		})

	go pipeline.AsyncPerform(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pipeline.Add(ctx, make([]string, 100))
	}
}

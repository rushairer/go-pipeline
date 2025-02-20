package gopipeline_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline"
)

type Obj struct {
	Key string
}

func (o Obj) GetKey() string {
	return o.Key
}

func TestPipelineDeduplicationBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	processed := make(map[string]bool)
	pipeline := gopipeline.NewPipelineDeduplication[Obj](
		gopipeline.PipelineConfig{
			FlushSize:     10,
			FlushInterval: time.Millisecond * 100,
			BufferSize:    20,
		},
		func(ctx context.Context, batchData map[string]Obj) error {
			for k := range batchData {
				processed[k] = true
			}
			return nil
		})

	go pipeline.SyncPerform(ctx)

	// Add duplicate items
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("%d", i%10) // Only 10 unique keys
		if err := pipeline.Add(ctx, Obj{Key: key}); err != nil {
			t.Fatalf("Failed to add item: %v", err)
		}
	}

	time.Sleep(time.Second)
	if len(processed) != 10 {
		t.Errorf("Expected 10 unique items, got %d", len(processed))
	}
}

func TestPipelineDeduplicationConcurrent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	var mu sync.Mutex
	processed := make(map[string]bool)
	pipeline := gopipeline.NewPipelineDeduplication[Obj](
		gopipeline.PipelineConfig{
			FlushSize:     100,
			FlushInterval: time.Millisecond * 100,
			BufferSize:    1000,
		},
		func(ctx context.Context, batchData map[string]Obj) error {
			mu.Lock()
			for k := range batchData {
				processed[k] = true
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
				// Each worker adds some duplicate items
				key := fmt.Sprintf("%d", i%50)
				if err := pipeline.Add(ctx, Obj{Key: key}); err != nil {
					t.Errorf("Worker %d failed to add item: %v", workerID, err)
					return
				}
			}
		}(w)
	}

	wg.Wait()
	time.Sleep(time.Second)

	mu.Lock()
	processedCount := len(processed)
	mu.Unlock()

	expectedCount := 50 // Since we're using i%50 for keys
	if processedCount != expectedCount {
		t.Errorf("Expected %d unique items, got %d", expectedCount, processedCount)
	}
}

func BenchmarkPipelineDeduplicationSmall(b *testing.B) {
	ctx := context.Background()
	pipeline := gopipeline.NewPipelineDeduplication[Obj](
		gopipeline.PipelineConfig{
			FlushSize:     100,
			FlushInterval: time.Millisecond * 100,
			BufferSize:    200,
		},
		func(ctx context.Context, batchData map[string]Obj) error {
			return nil
		})

	go pipeline.AsyncPerform(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pipeline.Add(ctx, Obj{Key: fmt.Sprintf("%d", i%100)})
	}
}

func BenchmarkPipelineDeduplicationLarge(b *testing.B) {
	ctx := context.Background()
	pipeline := gopipeline.NewPipelineDeduplication[Obj](
		gopipeline.PipelineConfig{
			FlushSize:     100000,
			FlushInterval: time.Second * 2,
			BufferSize:    200000,
		},
		func(ctx context.Context, batchData map[string]Obj) error {
			return nil
		})

	go pipeline.AsyncPerform(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pipeline.Add(ctx, Obj{Key: fmt.Sprintf("%d", i)})
	}
}

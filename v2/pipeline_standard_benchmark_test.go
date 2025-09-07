package gopipeline_test

import (
	"context"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

func BenchmarkStandardPipelineAsyncPerform(b *testing.B) {
	ctx := context.Background()
	var processedCount int

	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			FlushSize:     32,
			BufferSize:    64,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			time.Sleep(time.Microsecond * 100)
			processedCount += len(batchData)
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.AsyncPerform(ctx, flushError)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pipeline.Add(ctx, i)
	}
}

func BenchmarkStandardPipelineAsyncPerform2(b *testing.B) {
	ctx := context.Background()
	var processedCount int

	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			FlushSize:     512,
			BufferSize:    1024,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			time.Sleep(time.Microsecond * 100)
			processedCount += len(batchData)
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.AsyncPerform(ctx, flushError)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pipeline.Add(ctx, i)
	}
}

func BenchmarkStandardPipelineSyncPerform(b *testing.B) {
	ctx := context.Background()
	var processedCount int

	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			FlushSize:     32,
			BufferSize:    64,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			time.Sleep(time.Microsecond * 100)
			processedCount += len(batchData)
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.SyncPerform(ctx, flushError)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pipeline.Add(ctx, i)
	}
}

func BenchmarkStandardPipelineSyncPerform2(b *testing.B) {
	ctx := context.Background()
	var processedCount int

	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			FlushSize:     512,
			BufferSize:    1024,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			time.Sleep(time.Microsecond * 100)
			processedCount += len(batchData)
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.SyncPerform(ctx, flushError)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pipeline.Add(ctx, i)
	}
}

package gopipeline_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

type TestData struct {
	Name    string
	Address string
	Age     uint
}

func BenchmarkStandardPipelineAsyncPerform(b *testing.B) {
	ctx := context.Background()
	var processedCount int
	var processedCount2 int

	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			FlushSize:     32,
			BufferSize:    64,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []TestData) error {
			time.Sleep(time.Microsecond * 100)
			processedCount += len(batchData)
			for _, data := range batchData {
				select {
				case <-ctx.Done():
				default:
					data.Age = data.Age + 1
				}
			}
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.AsyncPerform(ctx, flushError)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pipeline.Add(ctx, TestData{Name: fmt.Sprintf("name:%d", i), Address: "Some address string", Age: 20})
		processedCount2 = processedCount2 + 1
	}

	select {
	case err, ok := <-flushError:
		if ok {
			b.Errorf("Expected no error, got %v", err)
		} else {
			b.Log("flushError closed")
		}
	default:
	}
	b.Log(processedCount, processedCount2)

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
		func(ctx context.Context, batchData []TestData) error {
			time.Sleep(time.Microsecond * 100)
			processedCount += len(batchData)
			for _, data := range batchData {
				select {
				case <-ctx.Done():
				default:
					data.Age = data.Age + 1
				}
			}
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.AsyncPerform(ctx, flushError)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pipeline.Add(ctx, TestData{Name: fmt.Sprintf("name:%d", i), Address: "Some address string", Age: 20})
	}

	b.Log(processedCount)
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
		func(ctx context.Context, batchData []TestData) error {
			time.Sleep(time.Microsecond * 100)
			processedCount += len(batchData)
			for _, data := range batchData {
				select {
				case <-ctx.Done():
				default:
					data.Age = data.Age + 1
				}
			}
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.SyncPerform(ctx, flushError)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pipeline.Add(ctx, TestData{Name: fmt.Sprintf("name:%d", i), Address: "Some address string", Age: 20})
	}

	b.Log(processedCount)
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
		func(ctx context.Context, batchData []TestData) error {
			time.Sleep(time.Microsecond * 100)
			processedCount += len(batchData)
			for _, data := range batchData {
				select {
				case <-ctx.Done():
				default:
					data.Age = data.Age + 1
				}
			}
			return nil
		})

	var flushError = make(chan error, 1)
	go pipeline.SyncPerform(ctx, flushError)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pipeline.Add(ctx, TestData{Name: fmt.Sprintf("name:%d", i), Address: "Some address string", Age: 20})
	}

	b.Log(processedCount)
}

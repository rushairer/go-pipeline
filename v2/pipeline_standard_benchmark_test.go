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

func BenchmarkDefaultStandardPipelineAsyncPerform(b *testing.B) {

	ctx := context.Background()

	pipeline := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batchData []TestData) error {
			time.Sleep(time.Microsecond * 100)

			for _, data := range batchData {
				data.Age = data.Age + 1
			}
			return nil
		})
	defer pipeline.Close()

	go pipeline.AsyncPerform(ctx)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pipeline.Add(ctx, TestData{Name: fmt.Sprintf("name:%d", i), Address: "Some address string", Age: 20})
	}

}

func BenchmarkCustomStandardPipelineAsyncPerform(b *testing.B) {
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
	defer pipeline.Close()

	go pipeline.AsyncPerform(ctx)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pipeline.Add(ctx, TestData{Name: fmt.Sprintf("name:%d", i), Address: "Some address string", Age: 20})
	}

	b.Log(processedCount)
}

func BenchmarkDefaultStandardPipelineSyncPerform(b *testing.B) {
	ctx := context.Background()
	var processedCount int

	pipeline := gopipeline.NewDefaultStandardPipeline(
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
	defer pipeline.Close()

	go pipeline.SyncPerform(ctx)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pipeline.Add(ctx, TestData{Name: fmt.Sprintf("name:%d", i), Address: "Some address string", Age: 20})
	}

	b.Log(processedCount)
}

func BenchmarkCustomStandardPipelineSyncPerform(b *testing.B) {
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
	defer pipeline.Close()

	go pipeline.SyncPerform(ctx)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pipeline.Add(ctx, TestData{Name: fmt.Sprintf("name:%d", i), Address: "Some address string", Age: 20})
	}

	b.Log(processedCount)
}

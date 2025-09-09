package gopipeline_test

import (
	"context"
	"fmt"
	"sync"
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	var processedCount int
	var mux sync.Mutex

	pipeline := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batchData []TestData) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				// 模拟处理时间
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				for i := range batchData {
					batchData[i].Age = batchData[i].Age + 1
				}
				mux.Unlock()
			}
			return nil
		})
	defer pipeline.Close()

	// 启动异步处理
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			b.Log(err)
		}
	}()

	// 监听错误
	go func() {
		for err := range pipeline.ErrorChan() {
			b.Errorf("Unexpected error: %v", err)
		}
	}()

	b.ResetTimer()

	// 基准测试主循环
	for i := 0; i < b.N; i++ {
		if err := pipeline.Add(ctx, TestData{
			Name:    fmt.Sprintf("name:%d", i),
			Address: "Some address string",
			Age:     20,
		}); err != nil {
			b.Errorf("Failed to add item: %v", err)
			break
		}
	}

	b.StopTimer()
}

func BenchmarkCustomStandardPipelineAsyncPerform(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	var processedCount int
	var mux sync.Mutex

	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    128,
			FlushSize:     256,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []TestData) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				// 模拟处理时间
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				for i := range batchData {
					batchData[i].Age = batchData[i].Age + 1
				}
				mux.Unlock()
			}
			return nil
		})
	defer pipeline.Close()

	// 启动异步处理
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			b.Log(err)
		}
	}()

	// 监听错误
	go func() {
		for err := range pipeline.ErrorChan() {
			b.Errorf("Unexpected error: %v", err)
		}
	}()

	b.ResetTimer()

	// 基准测试主循环
	for i := 0; i < b.N; i++ {
		if err := pipeline.Add(ctx, TestData{
			Name:    fmt.Sprintf("name:%d", i),
			Address: "Some address string",
			Age:     20,
		}); err != nil {
			b.Errorf("Failed to add item: %v", err)
			break
		}
	}

	b.StopTimer()
}

func BenchmarkDefaultStandardPipelineSyncPerform(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	var processedCount int
	var mux sync.Mutex

	pipeline := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batchData []TestData) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				// 模拟处理时间
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				for i := range batchData {
					batchData[i].Age = batchData[i].Age + 1
				}
				mux.Unlock()
			}
			return nil
		})
	defer pipeline.Close()

	// 启动同步处理
	go func() {
		if err := pipeline.SyncPerform(ctx); err != nil {
			b.Log(err)
		}
	}()

	// 监听错误
	go func() {
		for err := range pipeline.ErrorChan() {
			b.Errorf("Unexpected error: %v", err)
		}
	}()

	b.ResetTimer()

	// 基准测试主循环
	for i := 0; i < b.N; i++ {
		if err := pipeline.Add(ctx, TestData{
			Name:    fmt.Sprintf("name:%d", i),
			Address: "Some address string",
			Age:     20,
		}); err != nil {
			b.Errorf("Failed to add item: %v", err)
			break
		}
	}

	b.StopTimer()
}

func BenchmarkCustomStandardPipelineSyncPerform(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	var processedCount int
	var mux sync.Mutex

	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    512,
			FlushSize:     1024,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []TestData) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				// 模拟处理时间
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				for i := range batchData {
					batchData[i].Age = batchData[i].Age + 1
				}
				mux.Unlock()
			}
			return nil
		})
	defer pipeline.Close()

	// 启动同步处理
	go func() {
		if err := pipeline.SyncPerform(ctx); err != nil {
			b.Log(err)
		}
	}()

	// 监听错误
	go func() {
		for err := range pipeline.ErrorChan() {
			b.Errorf("Unexpected error: %v", err)
		}
	}()

	b.ResetTimer()

	// 基准测试主循环
	for i := 0; i < b.N; i++ {
		if err := pipeline.Add(ctx, TestData{
			Name:    fmt.Sprintf("name:%d", i),
			Address: "Some address string",
			Age:     20,
		}); err != nil {
			b.Errorf("Failed to add item: %v", err)
			break
		}
	}

	b.StopTimer()
}

// BenchmarkPipelineComparison 比较不同配置下的性能
func BenchmarkPipelineComparison(b *testing.B) {
	configs := []struct {
		name   string
		config gopipeline.PipelineConfig
	}{
		{
			name: "Small",
			config: gopipeline.PipelineConfig{
				BufferSize:    32,
				FlushSize:     64,
				FlushInterval: time.Millisecond * 50,
			},
		},
		{
			name: "Medium",
			config: gopipeline.PipelineConfig{
				BufferSize:    128,
				FlushSize:     256,
				FlushInterval: time.Millisecond * 100,
			},
		},
		{
			name: "Large",
			config: gopipeline.PipelineConfig{
				BufferSize:    512,
				FlushSize:     1024,
				FlushInterval: time.Millisecond * 200,
			},
		},
	}

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()

			var processedCount int
			var mux sync.Mutex

			pipeline := gopipeline.NewStandardPipeline(
				cfg.config,
				func(ctx context.Context, batchData []TestData) error {
					select {
					case <-ctx.Done():
						return nil
					default:
						// 模拟处理时间
						time.Sleep(time.Microsecond * 100)
						mux.Lock()
						processedCount += len(batchData)
						for i := range batchData {
							batchData[i].Age = batchData[i].Age + 1
						}
						mux.Unlock()
					}
					return nil
				})
			defer pipeline.Close()

			// 启动异步处理
			go func() {
				if err := pipeline.AsyncPerform(ctx); err != nil {
					b.Log(err)
				}
			}()

			// 监听错误
			go func() {
				for err := range pipeline.ErrorChan() {
					b.Errorf("Unexpected error: %v", err)
				}
			}()

			b.ResetTimer()

			// 基准测试主循环
			for i := 0; i < b.N; i++ {
				if err := pipeline.Add(ctx, TestData{
					Name:    fmt.Sprintf("name:%d", i),
					Address: "Some address string",
					Age:     20,
				}); err != nil {
					b.Errorf("Failed to add item: %v", err)
					break
				}
			}

			b.StopTimer()
		})
	}
}

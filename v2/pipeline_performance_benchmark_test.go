package gopipeline_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

type BenchmarkTestData struct {
	Name    string
	Address string
	Age     uint
}

// BenchmarkPipelineDataProcessing 测试管道的纯数据处理性能
func BenchmarkPipelineDataProcessing(b *testing.B) {
	var processedCount int64

	// 创建管道，使用最优配置
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    1000,
			FlushSize:     100,
			FlushInterval: time.Millisecond * 10,
		},
		func(ctx context.Context, batchData []BenchmarkTestData) error {
			// 模拟真实的数据处理工作
			for _, data := range batchData {
				// 简单的字符串操作
				_ = fmt.Sprintf("%s-%s-%d", data.Name, data.Address, data.Age)
			}
			atomic.AddInt64(&processedCount, int64(len(batchData)))
			return nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动管道
	go pipeline.AsyncPerform(ctx)
	dataChan := pipeline.DataChan()

	b.ResetTimer()
	b.ReportAllocs()

	// 发送数据并测量性能
	for i := 0; i < b.N; i++ {
		dataChan <- BenchmarkTestData{
			Name:    fmt.Sprintf("User%d", i%1000),
			Address: fmt.Sprintf("Address%d", i%500),
			Age:     uint(20 + i%50),
		}
	}

	b.StopTimer()

	// 关闭数据通道
	close(dataChan)

	// 等待处理完成
	deadline := time.Now().Add(time.Second * 10)
	for atomic.LoadInt64(&processedCount) < int64(b.N) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond * 10)
	}

	finalCount := atomic.LoadInt64(&processedCount)
	if finalCount < int64(b.N) {
		b.Logf("Warning: Only processed %d out of %d items", finalCount, b.N)
	}
}

// BenchmarkPipelineVsDirectProcessing 比较管道处理和直接处理的性能
func BenchmarkPipelineVsDirectProcessing(b *testing.B) {
	b.Run("Pipeline", func(b *testing.B) {
		var processedCount int64

		pipeline := gopipeline.NewStandardPipeline(
			gopipeline.PipelineConfig{
				BufferSize:    1000,
				FlushSize:     100,
				FlushInterval: time.Millisecond * 5,
			},
			func(ctx context.Context, batchData []BenchmarkTestData) error {
				for _, data := range batchData {
					_ = fmt.Sprintf("%s-%s-%d", data.Name, data.Address, data.Age)
				}
				atomic.AddInt64(&processedCount, int64(len(batchData)))
				return nil
			})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go pipeline.AsyncPerform(ctx)
		dataChan := pipeline.DataChan()

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			dataChan <- BenchmarkTestData{
				Name:    fmt.Sprintf("User%d", i%1000),
				Address: fmt.Sprintf("Address%d", i%500),
				Age:     uint(20 + i%50),
			}
		}

		b.StopTimer()
		close(dataChan)

		// 等待处理完成
		deadline := time.Now().Add(time.Second * 10)
		for atomic.LoadInt64(&processedCount) < int64(b.N) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond * 10)
		}
	})

	b.Run("Direct", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			data := BenchmarkTestData{
				Name:    fmt.Sprintf("User%d", i%1000),
				Address: fmt.Sprintf("Address%d", i%500),
				Age:     uint(20 + i%50),
			}
			_ = fmt.Sprintf("%s-%s-%d", data.Name, data.Address, data.Age)
		}
	})
}

// BenchmarkPipelineBatchSizes 测试不同批次大小的性能影响
func BenchmarkPipelineBatchSizes(b *testing.B) {
	batchSizes := []int{1, 10, 50, 100, 500}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize%d", batchSize), func(b *testing.B) {
			var processedCount int64

			pipeline := gopipeline.NewStandardPipeline(
				gopipeline.PipelineConfig{
					BufferSize:    uint32(batchSize * 10),
					FlushSize:     uint32(batchSize),
					FlushInterval: time.Millisecond * 5,
				},
				func(ctx context.Context, batchData []BenchmarkTestData) error {
					// 模拟批处理工作
					result := make([]string, len(batchData))
					for i, data := range batchData {
						result[i] = fmt.Sprintf("%s-%s-%d", data.Name, data.Address, data.Age)
					}
					atomic.AddInt64(&processedCount, int64(len(batchData)))
					return nil
				})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go pipeline.AsyncPerform(ctx)
			dataChan := pipeline.DataChan()

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				dataChan <- BenchmarkTestData{
					Name:    fmt.Sprintf("User%d", i),
					Address: fmt.Sprintf("Addr%d", i),
					Age:     uint(20 + i%50),
				}
			}

			b.StopTimer()
			close(dataChan)

			// 等待处理完成
			deadline := time.Now().Add(time.Second * 10)
			for atomic.LoadInt64(&processedCount) < int64(b.N) && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond * 10)
			}

			finalCount := atomic.LoadInt64(&processedCount)
			if finalCount > 0 {
				b.ReportMetric(float64(finalCount), "items_processed")
			}
		})
	}
}

// BenchmarkPipelineMemoryUsage 测试管道的内存使用效率
func BenchmarkPipelineMemoryUsage(b *testing.B) {
	var processedCount int64

	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    200,
			FlushSize:     50,
			FlushInterval: time.Millisecond * 10,
		},
		func(ctx context.Context, batchData []BenchmarkTestData) error {
			// 模拟内存密集型操作
			results := make([]map[string]interface{}, len(batchData))
			for i, data := range batchData {
				results[i] = map[string]interface{}{
					"name":    data.Name,
					"address": data.Address,
					"age":     data.Age,
					"id":      fmt.Sprintf("ID-%d", i),
				}
			}
			atomic.AddInt64(&processedCount, int64(len(batchData)))
			return nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pipeline.AsyncPerform(ctx)
	dataChan := pipeline.DataChan()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dataChan <- BenchmarkTestData{
			Name:    fmt.Sprintf("MemUser%d", i),
			Address: fmt.Sprintf("MemAddr%d", i),
			Age:     uint(25 + i%40),
		}
	}

	b.StopTimer()
	close(dataChan)

	// 等待处理完成
	deadline := time.Now().Add(time.Second * 10)
	for atomic.LoadInt64(&processedCount) < int64(b.N) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond * 10)
	}
}

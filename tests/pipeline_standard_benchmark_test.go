package gopipeline_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

type BenchmarkBenchmarkTestData struct {
	Name    string
	Address string
	Age     uint
}

// BenchmarkPipelineThroughput 测试管道的数据吞吐量
func BenchmarkPipelineThroughput(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var processedCount int64

	// 使用优化的配置来确保快速处理
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    uint32(b.N + 100),    // 足够大的缓冲区
			FlushSize:     10,                   // 小批次，快速处理
			FlushInterval: time.Millisecond * 1, // 很短的间隔
		},
		func(ctx context.Context, batchData []BenchmarkTestData) error {
			// 模拟最小的处理开销
			atomic.AddInt64(&processedCount, int64(len(batchData)))
			return nil
		})

	// 启动管道处理
	go pipeline.AsyncPerform(ctx)

	// 获取数据通道
	dataChan := pipeline.DataChan()

	b.ResetTimer()
	b.ReportAllocs()

	// 测量纯数据发送性能
	for i := 0; i < b.N; i++ {
		dataChan <- BenchmarkTestData{
			Name:    "BenchUser",
			Address: "BenchAddress",
			Age:     25,
		}
	}

	// 关闭通道并等待处理完成
	close(dataChan)

	// 等待所有数据被处理（带超时）
	timeout := time.After(time.Second * 30)
	for atomic.LoadInt64(&processedCount) < int64(b.N) {
		select {
		case <-timeout:
			b.Fatalf("Timeout waiting for processing completion. Processed: %d, Expected: %d",
				atomic.LoadInt64(&processedCount), b.N)
		default:
			time.Sleep(time.Millisecond * 1)
		}
	}

	b.StopTimer()
	cancel()
}

// BenchmarkPipelineLatency 测试管道的处理延迟
func BenchmarkPipelineLatency(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	processed := make(chan struct{}, b.N)

	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    1, // 最小缓冲，测试延迟
			FlushSize:     1, // 立即处理
			FlushInterval: time.Microsecond * 1,
		},
		func(ctx context.Context, batchData []BenchmarkTestData) error {
			// 每处理一个数据就发送信号
			for range batchData {
				select {
				case processed <- struct{}{}:
				case <-ctx.Done():
					return nil
				}
			}
			return nil
		})

	go pipeline.AsyncPerform(ctx)
	dataChan := pipeline.DataChan()

	b.ResetTimer()

	// 测量端到端延迟
	for i := 0; i < b.N; i++ {
		start := time.Now()

		dataChan <- BenchmarkTestData{
			Name:    "LatencyTest",
			Address: "TestAddr",
			Age:     30,
		}

		<-processed // 等待处理完成

		duration := time.Since(start)
		b.ReportMetric(float64(duration.Nanoseconds()), "ns/item")
	}

	b.StopTimer()
	close(dataChan)
	cancel()
}

// BenchmarkPipelineBatchEfficiency 测试不同批次大小的效率
func BenchmarkPipelineBatchEfficiency(b *testing.B) {
	batchSizes := []int{1, 10, 50, 100, 500, 1000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize%d", batchSize), func(b *testing.B) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var processedCount int64
			var batchCount int64

			pipeline := gopipeline.NewStandardPipeline(
				gopipeline.PipelineConfig{
					BufferSize:    uint32(batchSize * 2),
					FlushSize:     uint32(batchSize),
					FlushInterval: time.Millisecond * 10,
				},
				func(ctx context.Context, batchData []BenchmarkTestData) error {
					atomic.AddInt64(&processedCount, int64(len(batchData)))
					atomic.AddInt64(&batchCount, 1)
					return nil
				})

			go pipeline.AsyncPerform(ctx)
			dataChan := pipeline.DataChan()

			// 准备测试数据
			testData := make([]BenchmarkTestData, b.N)
			for i := 0; i < b.N; i++ {
				testData[i] = BenchmarkTestData{
					Name:    "BatchTest",
					Address: "BatchAddr",
					Age:     35,
				}
			}

			b.ResetTimer()

			// 发送所有测试数据
			for i := 0; i < b.N; i++ {
				dataChan <- testData[i]
			}

			close(dataChan)

			// 等待处理完成（带超时）
			timeout := time.After(time.Second * 30)
			for atomic.LoadInt64(&processedCount) < int64(b.N) {
				select {
				case <-timeout:
					b.Fatalf("Timeout waiting for processing completion. Processed: %d, Expected: %d",
						atomic.LoadInt64(&processedCount), b.N)
				default:
					time.Sleep(time.Millisecond * 10)
				}
			}

			b.StopTimer()

			// 报告批次效率指标
			finalBatchCount := atomic.LoadInt64(&batchCount)
			if finalBatchCount > 0 {
				avgBatchSize := float64(b.N) / float64(finalBatchCount)
				b.ReportMetric(avgBatchSize, "items/batch")
				b.ReportMetric(float64(finalBatchCount), "total_batches")
			}
		})
	}
}

// BenchmarkPipelineMemoryEfficiency 测试内存使用效率
func BenchmarkPipelineMemoryEfficiency(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var processedCount int64

	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    100,
			FlushSize:     50,
			FlushInterval: time.Millisecond * 5,
		},
		func(ctx context.Context, batchData []BenchmarkTestData) error {
			// 模拟一些内存操作
			result := make([]string, len(batchData))
			for i, data := range batchData {
				result[i] = fmt.Sprintf("%s-%s-%d", data.Name, data.Address, data.Age)
			}
			atomic.AddInt64(&processedCount, int64(len(batchData)))
			return nil
		})

	go pipeline.AsyncPerform(ctx)
	dataChan := pipeline.DataChan()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dataChan <- BenchmarkTestData{
			Name:    fmt.Sprintf("User%d", i%1000),
			Address: fmt.Sprintf("Addr%d", i%500),
			Age:     uint(20 + i%50),
		}
	}

	b.StopTimer()

	close(dataChan)

	for atomic.LoadInt64(&processedCount) < int64(b.N) {
		time.Sleep(time.Microsecond * 100)
	}
}

// BenchmarkPipelineAsyncVsSync 比较异步和同步处理性能
func BenchmarkPipelineAsyncVsSync(b *testing.B) {
	modes := []struct {
		name  string
		async bool
	}{
		{"Async", true},
		{"Sync", false},
	}

	for _, mode := range modes {
		b.Run(mode.name, func(b *testing.B) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var processedCount int64

			pipeline := gopipeline.NewStandardPipeline(
				gopipeline.PipelineConfig{
					BufferSize:    200,
					FlushSize:     100,
					FlushInterval: time.Millisecond * 10,
				},
				func(ctx context.Context, batchData []BenchmarkTestData) error {
					// 模拟一些计算工作
					sum := 0
					for _, data := range batchData {
						sum += int(data.Age)
					}
					atomic.AddInt64(&processedCount, int64(len(batchData)))
					return nil
				})

			if mode.async {
				go func() { _ = pipeline.AsyncPerform(ctx) }()
			} else {
				go func() { _ = pipeline.SyncPerform(ctx) }()
			}

			dataChan := pipeline.DataChan()

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				dataChan <- BenchmarkTestData{
					Name:    "PerfTest",
					Address: "PerfAddr",
					Age:     uint(25 + i%20),
				}
			}

			b.StopTimer()

			close(dataChan)

			for atomic.LoadInt64(&processedCount) < int64(b.N) {
				time.Sleep(time.Microsecond * 100)
			}
		})
	}
}

// BenchmarkPipelineConcurrentProducers 测试多生产者场景
func BenchmarkPipelineConcurrentProducers(b *testing.B) {
	producerCounts := []int{1, 2, 4, 8}

	for _, producerCount := range producerCounts {
		b.Run(fmt.Sprintf("Producers%d", producerCount), func(b *testing.B) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var processedCount int64

			pipeline := gopipeline.NewStandardPipeline(
				gopipeline.PipelineConfig{
					BufferSize:    500,
					FlushSize:     100,
					FlushInterval: time.Millisecond * 5,
				},
				func(ctx context.Context, batchData []BenchmarkTestData) error {
					atomic.AddInt64(&processedCount, int64(len(batchData)))
					return nil
				})

			go pipeline.AsyncPerform(ctx)
			dataChan := pipeline.DataChan()

			b.ResetTimer()

			// 启动多个生产者
			var wg sync.WaitGroup
			itemsPerProducer := b.N / producerCount

			for p := 0; p < producerCount; p++ {
				wg.Add(1)
				go func(producerID int) {
					defer wg.Done()
					for i := 0; i < itemsPerProducer; i++ {
						dataChan <- BenchmarkTestData{
							Name:    fmt.Sprintf("Producer%d-Item%d", producerID, i),
							Address: "ConcurrentAddr",
							Age:     uint(20 + i%30),
						}
					}
				}(p)
			}

			wg.Wait()

			b.StopTimer()

			close(dataChan)

			expectedItems := itemsPerProducer * producerCount
			for atomic.LoadInt64(&processedCount) < int64(expectedItems) {
				time.Sleep(time.Microsecond * 100)
			}
		})
	}
}

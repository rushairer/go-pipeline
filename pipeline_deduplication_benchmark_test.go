package gopipeline_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

type DedupBenchmarkTestData struct {
	ID      string
	Name    string
	Address string
	Age     uint
}

// GetKey 实现 UniqueKeyData 接口
func (d DedupBenchmarkTestData) GetKey() string {
	return d.ID
}

// BenchmarkDeduplicationPipelineThroughput 测试去重管道的吞吐量
func BenchmarkDeduplicationPipelineThroughput(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var processedCount int64

	pipeline := gopipeline.NewDeduplicationPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    uint32(b.N + 100),
			FlushSize:     10,
			FlushInterval: time.Millisecond * 1,
		},
		func(ctx context.Context, batchData map[string]DedupBenchmarkTestData) error {
			atomic.AddInt64(&processedCount, int64(len(batchData)))
			return nil
		})

	go pipeline.AsyncPerform(ctx)
	dataChan := pipeline.DataChan()

	b.ResetTimer()
	b.ReportAllocs()

	// 发送数据，包含重复项
	for i := 0; i < b.N; i++ {
		// 生成唯一的数据ID
		dataID := fmt.Sprintf("ID-%d", i)
		dataChan <- DedupBenchmarkTestData{
			ID:      dataID,
			Name:    fmt.Sprintf("User-%s", dataID),
			Address: "TestAddress",
			Age:     uint(20 + i%50),
		}
	}

	b.StopTimer()
	close(dataChan)

	// 等待处理完成
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
}

// BenchmarkDeduplicationEfficiency 测试去重效率
func BenchmarkDeduplicationEfficiency(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var processedCount int64
	var uniqueCount int64

	pipeline := gopipeline.NewDeduplicationPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    500,
			FlushSize:     100,
			FlushInterval: time.Millisecond * 5,
		},
		func(ctx context.Context, batchData map[string]DedupBenchmarkTestData) error {
			atomic.AddInt64(&processedCount, int64(len(batchData)))
			atomic.AddInt64(&uniqueCount, int64(len(batchData)))
			return nil
		})

	go pipeline.AsyncPerform(ctx)
	dataChan := pipeline.DataChan()

	b.ResetTimer()

	// 发送大量重复数据
	for i := 0; i < b.N; i++ {
		// 只有100个唯一ID，大量重复
		dataID := fmt.Sprintf("ID-%d", i%100)
		dataChan <- DedupBenchmarkTestData{
			ID:      dataID,
			Name:    fmt.Sprintf("User-%s", dataID),
			Address: "TestAddress",
			Age:     uint(20 + i%50),
		}
	}

	b.StopTimer()
	close(dataChan)

	// 等待处理完成
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

	// 报告去重效率
	duplicationRate := float64(b.N-int(uniqueCount)) / float64(b.N) * 100
	b.ReportMetric(duplicationRate, "duplication_rate_%")
	b.ReportMetric(float64(uniqueCount), "unique_items")
}

// BenchmarkDeduplicationMemoryUsage 测试去重管道的内存使用
func BenchmarkDeduplicationMemoryUsage(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var processedCount int64

	pipeline := gopipeline.NewDeduplicationPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    200,
			FlushSize:     50,
			FlushInterval: time.Millisecond * 10,
		},
		func(ctx context.Context, batchData map[string]DedupBenchmarkTestData) error {
			// 模拟内存密集型操作
			results := make([]map[string]interface{}, 0, len(batchData))
			for _, data := range batchData {
				results = append(results, map[string]interface{}{
					"id":      data.ID,
					"name":    data.Name,
					"address": data.Address,
					"age":     data.Age,
				})
			}
			atomic.AddInt64(&processedCount, int64(len(batchData)))
			return nil
		})

	go pipeline.AsyncPerform(ctx)
	dataChan := pipeline.DataChan()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dataID := fmt.Sprintf("ID-%d", i)
		dataChan <- DedupBenchmarkTestData{
			ID:      dataID,
			Name:    fmt.Sprintf("User-%s", dataID),
			Address: "MemoryTestAddr",
			Age:     uint(25 + i%40),
		}
	}

	b.StopTimer()
	close(dataChan)

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
}

// BenchmarkDeduplicationVsStandard 比较去重管道和标准管道的性能
func BenchmarkDeduplicationVsStandard(b *testing.B) {
	b.Run("Deduplication", func(b *testing.B) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var processedCount int64

		pipeline := gopipeline.NewDeduplicationPipeline(
			gopipeline.PipelineConfig{
				BufferSize:    500,
				FlushSize:     100,
				FlushInterval: time.Millisecond * 5,
			},
			func(ctx context.Context, batchData map[string]DedupBenchmarkTestData) error {
				atomic.AddInt64(&processedCount, int64(len(batchData)))
				return nil
			})

		go pipeline.AsyncPerform(ctx)
		dataChan := pipeline.DataChan()

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			dataID := fmt.Sprintf("ID-%d", i%100) // 大量重复
			dataChan <- DedupBenchmarkTestData{
				ID:      dataID,
				Name:    "TestUser",
				Address: "TestAddr",
				Age:     30,
			}
		}

		b.StopTimer()
		close(dataChan)

		for atomic.LoadInt64(&processedCount) < int64(b.N) {
			time.Sleep(time.Microsecond * 100)
		}
	})

	b.Run("Standard", func(b *testing.B) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var processedCount int64

		pipeline := gopipeline.NewStandardPipeline(
			gopipeline.PipelineConfig{
				BufferSize:    500,
				FlushSize:     100,
				FlushInterval: time.Millisecond * 5,
			},
			func(ctx context.Context, batchData []DedupBenchmarkTestData) error {
				atomic.AddInt64(&processedCount, int64(len(batchData)))
				return nil
			})

		go pipeline.AsyncPerform(ctx)
		dataChan := pipeline.DataChan()

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			dataID := fmt.Sprintf("ID-%d", i%100) // 同样的大量重复
			dataChan <- DedupBenchmarkTestData{
				ID:      dataID,
				Name:    "TestUser",
				Address: "TestAddr",
				Age:     30,
			}
		}

		b.StopTimer()
		close(dataChan)

		for atomic.LoadInt64(&processedCount) < int64(b.N) {
			time.Sleep(time.Microsecond * 100)
		}
	})
}

// BenchmarkDeduplicationBatchSizes 测试不同批次大小对去重性能的影响
func BenchmarkDeduplicationBatchSizes(b *testing.B) {
	batchSizes := []int{1, 10, 50, 100, 200}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize%d", batchSize), func(b *testing.B) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var processedCount int64

			pipeline := gopipeline.NewDeduplicationPipeline(
				gopipeline.PipelineConfig{
					BufferSize:    uint32(batchSize * 10),
					FlushSize:     uint32(batchSize),
					FlushInterval: time.Millisecond * 5,
				},
				func(ctx context.Context, batchData map[string]DedupBenchmarkTestData) error {
					atomic.AddInt64(&processedCount, int64(len(batchData)))
					return nil
				})

			go pipeline.AsyncPerform(ctx)
			dataChan := pipeline.DataChan()

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				dataID := fmt.Sprintf("ID-%d", i) // 生成唯一ID
				dataChan <- DedupBenchmarkTestData{
					ID:      dataID,
					Name:    fmt.Sprintf("User-%s", dataID),
					Address: "BatchTestAddr",
					Age:     uint(20 + i%30),
				}
			}

			b.StopTimer()
			close(dataChan)

			timeout := time.After(time.Second * 5)
			for atomic.LoadInt64(&processedCount) < int64(b.N) {
				select {
				case <-timeout:
					b.Fatalf("Timeout waiting for processing completion. Processed: %d, Expected: %d",
						atomic.LoadInt64(&processedCount), b.N)
				default:
					time.Sleep(time.Millisecond * 1)
				}
			}
		})
	}
}

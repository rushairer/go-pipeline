package gopipeline

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// 验证对象池功能的基本测试
func TestPoolBasicFunctionality(t *testing.T) {
	flushFunc := func(ctx context.Context, batch []string) error {
		t.Logf("Processing batch of size: %d", len(batch))
		return nil
	}

	pipeline := NewDefaultStandardPipelineWithPool(flushFunc)
	ctx := context.Background()
	done, errs := pipeline.Start(ctx)

	// 消费错误通道
	go func() {
		for err := range errs {
			t.Logf("Error: %v", err)
		}
	}()

	dataChan := pipeline.DataChan()

	// 发送一些数据
	for i := 0; i < 100; i++ {
		dataChan <- "test data"
	}

	close(dataChan)
	<-done

	// 检查池统计
	activeCount, poolSize := pipeline.GetPoolStats()
	t.Logf("Final stats - Active batches: %d, Pool size: %d", activeCount, poolSize)

	// 强制GC并再次检查
	pipeline.ForceGC()
	time.Sleep(time.Millisecond * 50)

	activeCount2, poolSize2 := pipeline.GetPoolStats()
	t.Logf("After GC - Active batches: %d, Pool size: %d", activeCount2, poolSize2)
}

// 测试异步处理场景下的内存安全性
func TestPoolAsyncSafety(t *testing.T) {
	var processedBatches [][]string
	
	flushFunc := func(ctx context.Context, batch []string) error {
		// 模拟异步处理
		go func() {
			// 延迟访问数据，测试是否会有竞争
			time.Sleep(time.Millisecond * 10)
			batchCopy := make([]string, len(batch))
			copy(batchCopy, batch)
			processedBatches = append(processedBatches, batchCopy)
			t.Logf("Async processed batch of size: %d", len(batchCopy))
		}()
		return nil
	}

	pipeline := NewDefaultStandardPipelineWithPool(flushFunc)
	ctx := context.Background()
	done, errs := pipeline.Start(ctx)

	go func() {
		for err := range errs {
			t.Logf("Error: %v", err)
		}
	}()

	dataChan := pipeline.DataChan()

	// 发送数据
	for i := 0; i < 200; i++ {
		dataChan <- "async test data"
	}

	close(dataChan)
	<-done

	// 等待异步处理完成
	time.Sleep(time.Millisecond * 100)

	t.Logf("Total processed batches: %d", len(processedBatches))
	
	// 强制GC
	pipeline.ForceGC()
	time.Sleep(time.Millisecond * 50)
	
	activeCount, poolSize := pipeline.GetPoolStats()
	t.Logf("Final stats - Active: %d, Pool: %d", activeCount, poolSize)
}

// 简单的性能对比
func TestPerformanceComparison(t *testing.T) {
	const iterations = 10
	const dataPerIteration = 100

	flushFunc := func(ctx context.Context, batch []int) error {
		return nil
	}

	// 测试标准管道
	t.Run("Standard", func(t *testing.T) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		start := time.Now()
		for i := 0; i < iterations; i++ {
			pipeline := NewDefaultStandardPipeline(flushFunc)
			ctx := context.Background()
			done, errs := pipeline.Start(ctx)

			go func() {
				for range errs {
				}
			}()

			dataChan := pipeline.DataChan()
			for j := 0; j < dataPerIteration; j++ {
				dataChan <- j
			}
			close(dataChan)
			<-done
		}
		duration := time.Since(start)

		runtime.GC()
		runtime.ReadMemStats(&m2)

		t.Logf("Standard - Duration: %v, Allocs: %d, TotalAlloc: %d bytes",
			duration, m2.Mallocs-m1.Mallocs, m2.TotalAlloc-m1.TotalAlloc)
	})

	// 测试带对象池的管道
	t.Run("WithPool", func(t *testing.T) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		start := time.Now()
		for i := 0; i < iterations; i++ {
			pipeline := NewDefaultStandardPipelineWithPool(flushFunc)
			ctx := context.Background()
			done, errs := pipeline.Start(ctx)

			go func() {
				for range errs {
				}
			}()

			dataChan := pipeline.DataChan()
			for j := 0; j < dataPerIteration; j++ {
				dataChan <- j
			}
			close(dataChan)
			<-done
		}
		duration := time.Since(start)

		runtime.GC()
		runtime.ReadMemStats(&m2)

		t.Logf("WithPool - Duration: %v, Allocs: %d, TotalAlloc: %d bytes",
			duration, m2.Mallocs-m1.Mallocs, m2.TotalAlloc-m1.TotalAlloc)
	})
}
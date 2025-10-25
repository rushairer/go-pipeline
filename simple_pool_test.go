package gopipeline

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// 简单的性能对比测试
func TestSimpleComparison(t *testing.T) {
	flushFunc := func(ctx context.Context, batch []int) error {
		// 模拟一些处理
		time.Sleep(time.Microsecond * 10)
		return nil
	}

	t.Run("Standard", func(t *testing.T) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		pipeline := NewDefaultStandardPipeline(flushFunc)
		ctx := context.Background()
		done, errs := pipeline.Start(ctx)

		go func() {
			for range errs {
			}
		}()

		dataChan := pipeline.DataChan()

		// 发送数据
		for i := 0; i < 1000; i++ {
			dataChan <- i
		}

		close(dataChan)
		<-done

		runtime.GC()
		runtime.ReadMemStats(&m2)

		t.Logf("Standard - Allocs: %d, TotalAlloc: %d bytes",
			m2.Mallocs-m1.Mallocs, m2.TotalAlloc-m1.TotalAlloc)
	})

	t.Run("WithPool", func(t *testing.T) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		pipeline := NewDefaultStandardPipelineWithPool(flushFunc)
		ctx := context.Background()
		done, errs := pipeline.Start(ctx)

		go func() {
			for range errs {
			}
		}()

		dataChan := pipeline.DataChan()

		// 发送数据
		for i := 0; i < 1000; i++ {
			dataChan <- i
		}

		close(dataChan)
		<-done

		runtime.GC()
		runtime.ReadMemStats(&m2)

		t.Logf("WithPool - Allocs: %d, TotalAlloc: %d bytes",
			m2.Mallocs-m1.Mallocs, m2.TotalAlloc-m1.TotalAlloc)

		// 显示池统计
		activeCount, poolSize := pipeline.GetPoolStats()
		t.Logf("Pool stats - Active: %d, Pool: %d", activeCount, poolSize)
	})
}

// 简单的基准测试
func BenchmarkSimpleStandard(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		return nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pipeline := NewDefaultStandardPipeline(flushFunc)
		ctx := context.Background()
		done, errs := pipeline.Start(ctx)

		go func() {
			for range errs {
			}
		}()

		dataChan := pipeline.DataChan()

		// 发送少量数据
		for j := 0; j < 10; j++ {
			dataChan <- j
		}

		close(dataChan)
		<-done
	}
}

func BenchmarkSimpleWithPool(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		return nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pipeline := NewDefaultStandardPipelineWithPool(flushFunc)
		ctx := context.Background()
		done, errs := pipeline.Start(ctx)

		go func() {
			for range errs {
			}
		}()

		dataChan := pipeline.DataChan()

		// 发送少量数据
		for j := 0; j < 10; j++ {
			dataChan <- j
		}

		close(dataChan)
		<-done
	}
}
package gopipeline

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// 长期运行的基准测试，更能体现对象池的优势
func BenchmarkLongRunning_Standard(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		// 模拟一些处理时间
		time.Sleep(time.Microsecond)
		return nil
	}

	pipeline := NewDefaultStandardPipeline(flushFunc)
	ctx := context.Background()
	done, errs := pipeline.Start(ctx)

	// 消费错误通道
	go func() {
		for range errs {
		}
	}()

	dataChan := pipeline.DataChan()

	b.ResetTimer()
	b.ReportAllocs()

	// 发送大量数据，触发多次 flush
	for i := 0; i < b.N; i++ {
		dataChan <- i
	}

	close(dataChan)
	<-done
}

func BenchmarkLongRunning_WithPool(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		// 模拟一些处理时间
		time.Sleep(time.Microsecond)
		return nil
	}

	pipeline := NewDefaultStandardPipelineWithPool(flushFunc)
	ctx := context.Background()
	done, errs := pipeline.Start(ctx)

	// 消费错误通道
	go func() {
		for range errs {
		}
	}()

	dataChan := pipeline.DataChan()

	b.ResetTimer()
	b.ReportAllocs()

	// 发送大量数据，触发多次 flush
	for i := 0; i < b.N; i++ {
		dataChan <- i
	}

	close(dataChan)
	<-done
}

// 高频创建销毁的场景
func BenchmarkHighFrequency_Standard(b *testing.B) {
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
		
		// 发送足够的数据触发多次 flush
		for j := 0; j < 200; j++ {
			dataChan <- j
		}
		
		close(dataChan)
		<-done
		
		// 每10次迭代强制GC一次
		if i%10 == 0 {
			runtime.GC()
		}
	}
}

func BenchmarkHighFrequency_WithPool(b *testing.B) {
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
		
		// 发送足够的数据触发多次 flush
		for j := 0; j < 200; j++ {
			dataChan <- j
		}
		
		close(dataChan)
		<-done
		
		// 每10次迭代强制GC一次，让终结器有机会运行
		if i%10 == 0 {
			runtime.GC()
			runtime.GC() // 调用两次确保终结器运行
		}
	}
}

// 内存压力测试
func TestMemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory pressure test in short mode")
	}

	flushFunc := func(ctx context.Context, batch []int) error {
		return nil
	}

	t.Run("Standard", func(t *testing.T) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		// 创建多个管道实例，模拟高负载
		for round := 0; round < 50; round++ {
			pipeline := NewDefaultStandardPipeline(flushFunc)
			ctx := context.Background()
			done, errs := pipeline.Start(ctx)

			go func() {
				for range errs {
				}
			}()

			dataChan := pipeline.DataChan()
			for i := 0; i < 500; i++ {
				dataChan <- i
			}
			close(dataChan)
			<-done

			if round%10 == 0 {
				runtime.GC()
			}
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		t.Logf("Standard - Rounds: 50, Allocs: %d, TotalAlloc: %d bytes, HeapAlloc: %d bytes",
			m2.Mallocs-m1.Mallocs, m2.TotalAlloc-m1.TotalAlloc, m2.HeapAlloc)
	})

	t.Run("WithPool", func(t *testing.T) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		// 创建多个管道实例，模拟高负载
		for round := 0; round < 50; round++ {
			pipeline := NewDefaultStandardPipelineWithPool(flushFunc)
			ctx := context.Background()
			done, errs := pipeline.Start(ctx)

			go func() {
				for range errs {
				}
			}()

			dataChan := pipeline.DataChan()
			for i := 0; i < 500; i++ {
				dataChan <- i
			}
			close(dataChan)
			<-done

			if round%10 == 0 {
				runtime.GC()
				runtime.GC() // 双重GC确保终结器运行
			}
		}

		runtime.GC()
		runtime.GC()
		runtime.ReadMemStats(&m2)

		t.Logf("WithPool - Rounds: 50, Allocs: %d, TotalAlloc: %d bytes, HeapAlloc: %d bytes",
			m2.Mallocs-m1.Mallocs, m2.TotalAlloc-m1.TotalAlloc, m2.HeapAlloc)
	})
}
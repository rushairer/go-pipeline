package gopipeline

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// 基准测试：对比所有版本的性能

// 标准版本
func BenchmarkOptimization_Standard(b *testing.B) {
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
		for j := 0; j < 100; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

// 原始 WithPool 版本
func BenchmarkOptimization_WithPool(b *testing.B) {
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
		for j := 0; j < 100; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

// 优化版本1：简化对象池
func BenchmarkOptimization_Optimized1(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		return nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pipeline := NewDefaultStandardPipelineOptimized1(flushFunc)
		ctx := context.Background()
		done, errs := pipeline.Start(ctx)

		go func() {
			for range errs {
			}
		}()

		dataChan := pipeline.DataChan()
		for j := 0; j < 100; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

// 优化版本2：轻量级终结器
func BenchmarkOptimization_Optimized2(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		return nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pipeline := NewDefaultStandardPipelineOptimized2(flushFunc)
		ctx := context.Background()
		done, errs := pipeline.Start(ctx)

		go func() {
			for range errs {
			}
		}()

		dataChan := pipeline.DataChan()
		for j := 0; j < 100; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

// 优化版本3：双池设计
func BenchmarkOptimization_Optimized3(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		return nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pipeline := NewDefaultStandardPipelineOptimized3(flushFunc)
		ctx := context.Background()
		done, errs := pipeline.Start(ctx)

		go func() {
			for range errs {
			}
		}()

		dataChan := pipeline.DataChan()
		for j := 0; j < 100; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

// 长期运行测试
func BenchmarkOptLongRunning_Standard(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		time.Sleep(time.Microsecond)
		return nil
	}

	pipeline := NewDefaultStandardPipeline(flushFunc)
	ctx := context.Background()
	done, errs := pipeline.Start(ctx)

	go func() {
		for range errs {
		}
	}()

	dataChan := pipeline.DataChan()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dataChan <- i
	}

	close(dataChan)
	<-done
}

func BenchmarkOptLongRunning_Optimized1(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		time.Sleep(time.Microsecond)
		return nil
	}

	pipeline := NewDefaultStandardPipelineOptimized1(flushFunc)
	ctx := context.Background()
	done, errs := pipeline.Start(ctx)

	go func() {
		for range errs {
		}
	}()

	dataChan := pipeline.DataChan()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dataChan <- i
	}

	close(dataChan)
	<-done
}

func BenchmarkOptLongRunning_Optimized3(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		time.Sleep(time.Microsecond)
		return nil
	}

	pipeline := NewDefaultStandardPipelineOptimized3(flushFunc)
	ctx := context.Background()
	done, errs := pipeline.Start(ctx)

	go func() {
		for range errs {
		}
	}()

	dataChan := pipeline.DataChan()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dataChan <- i
	}

	close(dataChan)
	<-done
}

// 内存压力测试
func TestOptimizationMemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory pressure test in short mode")
	}

	flushFunc := func(ctx context.Context, batch []int) error {
		return nil
	}

	testCases := []struct {
		name string
		createPipeline func() interface {
			Start(context.Context) (<-chan struct{}, <-chan error)
			DataChan() chan<- int
		}
	}{
		{
			name: "Standard",
			createPipeline: func() interface {
				Start(context.Context) (<-chan struct{}, <-chan error)
				DataChan() chan<- int
			} {
				return NewDefaultStandardPipeline(flushFunc)
			},
		},
		{
			name: "WithPool",
			createPipeline: func() interface {
				Start(context.Context) (<-chan struct{}, <-chan error)
				DataChan() chan<- int
			} {
				return NewDefaultStandardPipelineWithPool(flushFunc)
			},
		},
		{
			name: "Optimized1",
			createPipeline: func() interface {
				Start(context.Context) (<-chan struct{}, <-chan error)
				DataChan() chan<- int
			} {
				return NewDefaultStandardPipelineOptimized1(flushFunc)
			},
		},
		{
			name: "Optimized3",
			createPipeline: func() interface {
				Start(context.Context) (<-chan struct{}, <-chan error)
				DataChan() chan<- int
			} {
				return NewDefaultStandardPipelineOptimized3(flushFunc)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			// 运行多轮测试
			for round := 0; round < 20; round++ {
				pipeline := tc.createPipeline()
				ctx := context.Background()
				done, errs := pipeline.Start(ctx)

				go func() {
					for range errs {
					}
				}()

				dataChan := pipeline.DataChan()
				for i := 0; i < 200; i++ {
					dataChan <- i
				}
				close(dataChan)
				<-done

				if round%5 == 0 {
					runtime.GC()
				}
			}

			runtime.GC()
			runtime.ReadMemStats(&m2)

			t.Logf("%s - Rounds: 20, Allocs: %d, TotalAlloc: %d bytes, HeapAlloc: %d bytes",
				tc.name, m2.Mallocs-m1.Mallocs, m2.TotalAlloc-m1.TotalAlloc, m2.HeapAlloc)
		})
	}
}

// 异步处理测试
func BenchmarkOptAsyncProcessing_Standard(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		go func() {
			time.Sleep(time.Microsecond * 10)
			_ = len(batch)
		}()
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
		for j := 0; j < 100; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

func BenchmarkOptAsyncProcessing_Optimized3(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		go func() {
			time.Sleep(time.Microsecond * 10)
			_ = len(batch)
		}()
		return nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pipeline := NewDefaultStandardPipelineOptimized3(flushFunc)
		ctx := context.Background()
		done, errs := pipeline.Start(ctx)

		go func() {
			for range errs {
			}
		}()

		dataChan := pipeline.DataChan()
		for j := 0; j < 100; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}
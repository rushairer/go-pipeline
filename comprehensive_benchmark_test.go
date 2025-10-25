package gopipeline

import (
	"context"
	"testing"
	"time"
)

// 不同数据量的基准测试
func BenchmarkSmallBatch_Standard(b *testing.B) {
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
		// 小批次：5个数据
		for j := 0; j < 5; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

func BenchmarkSmallBatch_WithPool(b *testing.B) {
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
		// 小批次：5个数据
		for j := 0; j < 5; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

func BenchmarkMediumBatch_Standard(b *testing.B) {
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
		// 中等批次：100个数据
		for j := 0; j < 100; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

func BenchmarkMediumBatch_WithPool(b *testing.B) {
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
		// 中等批次：100个数据
		for j := 0; j < 100; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

func BenchmarkLargeBatch_Standard(b *testing.B) {
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
		// 大批次：1000个数据
		for j := 0; j < 1000; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

func BenchmarkLargeBatch_WithPool(b *testing.B) {
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
		// 大批次：1000个数据
		for j := 0; j < 1000; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

// 重复使用同一个管道实例的基准测试
func BenchmarkReusePipeline_Standard(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		return nil
	}

	pipeline := NewDefaultStandardPipeline(flushFunc)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		done, errs := pipeline.Start(ctx)

		go func() {
			for range errs {
			}
		}()

		dataChan := pipeline.DataChan()
		for j := 0; j < 50; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

func BenchmarkReusePipeline_WithPool(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		return nil
	}

	pipeline := NewDefaultStandardPipelineWithPool(flushFunc)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		done, errs := pipeline.Start(ctx)

		go func() {
			for range errs {
			}
		}()

		dataChan := pipeline.DataChan()
		for j := 0; j < 50; j++ {
			dataChan <- j
		}
		close(dataChan)
		<-done
	}
}

// 模拟异步处理的基准测试
func BenchmarkAsyncProcessing_Standard(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		// 模拟异步处理
		go func() {
			time.Sleep(time.Microsecond * 10)
			_ = len(batch) // 访问数据
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

func BenchmarkAsyncProcessing_WithPool(b *testing.B) {
	flushFunc := func(ctx context.Context, batch []int) error {
		// 模拟异步处理
		go func() {
			time.Sleep(time.Microsecond * 10)
			_ = len(batch) // 访问数据
		}()
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
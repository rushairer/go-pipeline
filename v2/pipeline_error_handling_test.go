package gopipeline_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

// TestPipelineWithoutErrorChan 测试用户不调用 ErrorChan() 方法的情况
func TestPipelineWithoutErrorChan(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	// 创建一个会产生错误的管道
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    10,
			FlushSize:     5,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			// 故意返回错误
			return fmt.Errorf("test error with batch size: %d", len(batchData))
		})

	// 启动管道处理（不调用 ErrorChan 方法）
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			t.Logf("Pipeline finished with: %v", err)
		}
	}()

	// 发送数据
	dataChan := pipeline.DataChan()
	go func() {
		defer close(dataChan)
		for i := 0; i < 10; i++ {
			select {
			case dataChan <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	// 等待处理完成
	<-ctx.Done()

	// 如果没有 panic，说明修复成功
	t.Log("Test passed: No panic when ErrorChan() is not called")
}

// TestPipelineWithErrorChan 测试用户调用 ErrorChan() 方法的情况
func TestPipelineWithErrorChan(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	errorCount := 0

	// 创建一个会产生错误的管道
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    10,
			FlushSize:     5,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			// 故意返回错误
			return fmt.Errorf("test error with batch size: %d", len(batchData))
		})

	// 启动管道处理
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			// 使用 t.Log 而不是 t.Logf 避免测试完成后的日志问题
			if ctx.Err() == nil {
				t.Logf("Pipeline finished with: %v", err)
			}
		}
	}()

	// 监听错误通道
	errorChan := pipeline.ErrorChan(10)
	go func() {
		for {
			select {
			case err, ok := <-errorChan:
				if !ok {
					return
				}
				errorCount++
				t.Logf("Received error: %v", err)
			case <-ctx.Done():
				return
			}
		}
	}()

	// 发送数据
	dataChan := pipeline.DataChan()
	go func() {
		defer close(dataChan)
		for i := 0; i < 10; i++ {
			select {
			case dataChan <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	// 等待处理完成
	<-ctx.Done()

	if errorCount == 0 {
		t.Error("Expected to receive errors, but got none")
	} else {
		t.Logf("Successfully received %d errors", errorCount)
	}
}

// TestPipelineErrorChanFullBuffer 测试错误通道缓冲区满的情况
func TestPipelineErrorChanFullBuffer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	// 创建一个会产生大量错误的管道
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    20,
			FlushSize:     1, // 每个数据都会触发一次处理
			FlushInterval: time.Millisecond * 10,
		},
		func(ctx context.Context, batchData []int) error {
			// 每次都返回错误
			return fmt.Errorf("test error with batch size: %d", len(batchData))
		})

	// 启动管道处理
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			t.Logf("Pipeline finished with: %v", err)
		}
	}()

	// 创建一个很小的错误通道缓冲区
	errorChan := pipeline.ErrorChan(2)
	receivedErrors := 0

	// 故意不及时消费错误通道，让它满
	go func() {
		time.Sleep(time.Millisecond * 500) // 延迟消费
		for {
			select {
			case err, ok := <-errorChan:
				if !ok {
					return
				}
				receivedErrors++
				t.Logf("Received error: %v", err)
			case <-ctx.Done():
				return
			}
		}
	}()

	// 发送大量数据
	dataChan := pipeline.DataChan()
	go func() {
		defer close(dataChan)
		for i := 0; i < 10; i++ {
			select {
			case dataChan <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	// 等待处理完成
	<-ctx.Done()

	// 应该收到一些错误，但不是全部（因为缓冲区满了会丢弃）
	t.Logf("Received %d errors (some may have been dropped due to full buffer)", receivedErrors)

	if receivedErrors == 0 {
		t.Error("Expected to receive at least some errors")
	}
}

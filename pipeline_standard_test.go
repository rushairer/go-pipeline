package gopipeline_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

// TestStandardPipelineAsyncPerform 测试异步执行管道操作
func TestStandardPipelineAsyncPerform(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	pipeline := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batchData []int) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				// AsyncPerform 时可以是 flush 无序更加明显
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
			}
			return nil
		})

	// 启动异步处理
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			t.Log("AsyncPerform finished with:", err)
		}
	}()

	// 监听错误
	errorChan := pipeline.ErrorChan(10)
	go func() {
		for err := range errorChan {
			t.Errorf("Expected no error, got %v", err)
		}
	}()

	// 使用新的 DataChan API 添加数据
	dataChan := pipeline.DataChan()
	go func() {
		defer close(dataChan)
		for i := 0; i < 1000; i++ {
			select {
			case dataChan <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()

	mux.Lock()
	finalCount := processedCount
	mux.Unlock()

	if finalCount != 1000 {
		t.Errorf("Expected 1000 processed items, got %d", finalCount)
	}
}

// TestStandardPipelineAsyncPerformWithFlushError 测试异步执行时的错误处理
func TestStandardPipelineAsyncPerformWithFlushError(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	var errorCount int
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    32,
			FlushSize:     64,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
			}
			return newErrorWithData(batchData, errors.New("test error"))
		})

	// 启动异步处理
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			t.Log("AsyncPerform finished with:", err)
		}
	}()

	// 监听错误
	errorChan := pipeline.ErrorChan(50)
	go func() {
		for err := range errorChan {
			if e, ok := err.(errorWithData); ok {
				if e.Err.Error() != "test error" {
					t.Errorf("Expected 'test error', got %v", e.Err.Error())
				}
				mux.Lock()
				errorCount++
				mux.Unlock()
			}
		}
	}()

	// 使用新的 DataChan API 添加数据
	dataChan := pipeline.DataChan()
	go func() {
		defer close(dataChan)
		for i := 0; i < 1000; i++ {
			select {
			case dataChan <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()

	mux.Lock()
	finalProcessedCount := processedCount
	finalErrorCount := errorCount
	mux.Unlock()

	if finalProcessedCount != 1000 {
		t.Errorf("Expected 1000 processed items, got %d", finalProcessedCount)
	}
	if finalErrorCount == 0 {
		t.Errorf("Expected some errors, got %d", finalErrorCount)
	}
}

// TestStandardPipelineAsyncPerformTimeout 测试异步执行超时场景
func TestStandardPipelineAsyncPerformTimeout(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	var processedCount int
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    32,
			FlushSize:     64,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				// 长时间 sleep 模拟超时场景
				time.Sleep(time.Second * 5)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
			}
			return nil
		})

	// 启动异步处理
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			t.Log("AsyncPerform finished with:", err)
		}
	}()

	// 监听错误
	errorChan := pipeline.ErrorChan(10)
	go func() {
		for err := range errorChan {
			t.Errorf("Expected no error, got %v", err)
		}
	}()

	// 使用新的 DataChan API 添加数据
	dataChan := pipeline.DataChan()
	go func() {
		defer close(dataChan)
		for i := 0; i < 1000; i++ {
			select {
			case dataChan <- i:
			case <-ctx.Done():
				t.Log("Data sending stopped due to context cancellation")
				return
			}
		}
	}()

	<-ctx.Done()

	// 由于超时，处理的数据应该少于总数
	mux.Lock()
	finalCount := processedCount
	mux.Unlock()

	if finalCount >= 1000 {
		t.Errorf("Expected less than 1000 processed items due to timeout, got %d", finalCount)
	}
}

// TestStandardPipelineSyncPerform 测试同步执行管道操作
func TestStandardPipelineSyncPerform(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    32,
			FlushSize:     64,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData []int) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				// SyncPerform 是有序的，即使 sleep 也不影响
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
			}
			return nil
		})

	// 启动同步处理
	go func() {
		if err := pipeline.SyncPerform(ctx); err != nil {
			t.Log("SyncPerform finished with:", err)
		}
	}()

	// 监听错误
	errorChan := pipeline.ErrorChan(10)
	go func() {
		for err := range errorChan {
			t.Errorf("Expected no error, got %v", err)
		}
	}()

	// 使用新的 DataChan API 添加数据
	dataChan := pipeline.DataChan()
	go func() {
		defer close(dataChan)
		for i := 0; i < 500; i++ {
			select {
			case dataChan <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()

	mux.Lock()
	finalCount := processedCount
	mux.Unlock()

	if finalCount != 500 {
		t.Errorf("Expected 500 processed items, got %d", finalCount)
	}
}

// TestStandardPipelineDataChanClosed 测试数据通道关闭后的行为
func TestStandardPipelineDataChanClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	var mux sync.Mutex
	var processedCount int
	pipeline := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batchData []int) error {
			mux.Lock()
			processedCount += len(batchData)
			mux.Unlock()
			return nil
		})

	// 启动异步处理
	performDone := make(chan error, 1)
	go func() {
		performDone <- pipeline.AsyncPerform(ctx)
	}()

	// 监听错误
	errorChan := pipeline.ErrorChan(10)
	go func() {
		for err := range errorChan {
			t.Errorf("Expected no error, got %v", err)
		}
	}()

	// 使用新的 DataChan API 添加数据并关闭通道
	dataChan := pipeline.DataChan()
	for i := 0; i < 100; i++ {
		dataChan <- i
	}
	close(dataChan) // 关闭数据通道

	// 等待处理完成
	select {
	case err := <-performDone:
		if err != nil && !errors.Is(err, gopipeline.ErrContextIsClosed) {
			t.Errorf("Expected ErrContextIsClosed or nil, got %v", err)
		}
	case <-time.After(time.Second * 1):
		// 应该在通道关闭后很快完成
	}

	mux.Lock()
	finalCount := processedCount
	mux.Unlock()

	if finalCount != 100 {
		t.Errorf("Expected 100 processed items, got %d", finalCount)
	}
}

// TestStandardPipelineErrorChanSize 测试错误通道大小配置
func TestStandardPipelineErrorChanSize(t *testing.T) {
	pipeline := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batchData []int) error {
			return errors.New("test error")
		})

	// 测试自定义错误通道大小
	errorChan := pipeline.ErrorChan(5)
	if cap(errorChan) != 5 {
		t.Errorf("Expected error channel capacity 5, got %d", cap(errorChan))
	}

	// 测试默认错误通道大小（传入0）
	errorChan2 := pipeline.ErrorChan(0)
	if cap(errorChan2) == 0 {
		t.Error("Expected non-zero default error channel capacity")
	}
}

// 辅助类型和函数
type errorWithData struct {
	Data any
	Err  error
}

func (e errorWithData) Error() string {
	return e.Err.Error()
}

func newErrorWithData(data any, err error) errorWithData {
	return errorWithData{
		Data: data,
		Err:  err,
	}
}

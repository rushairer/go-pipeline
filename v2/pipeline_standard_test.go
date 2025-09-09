package gopipeline_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

func TestStandardPipelineAsyncPerform(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	pipeline := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batchData []int) error {
			select {
			case <-ctx.Done():
			default:
				// AsyncPerform 时 可以是 flush无序更加明显
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
				//t.Log(batchData)
			}
			return nil
		})
	defer pipeline.Close()

	// 启动异步处理
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			t.Log(err)
		}
	}()

	// 监听错误
	go func() {
		for err := range pipeline.ErrorChan() {
			t.Errorf("Expected no error, got %v", err)
		}
	}()

	// 添加数据
	for i := 0; i < 6478017; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			t.Errorf("Failed to add item: %v", err)
		}
	}

	<-ctx.Done()

	if processedCount != 6478017 {
		t.Errorf("Expected 6478017 processed items, got %d", processedCount)
	}
}

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
				// AsyncPerform 时 可以是 flush无序更加明显
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
				//t.Log(batchData)
			}
			return newErrorWithData(batchData, errors.New("test error"))
		})
	defer pipeline.Close()

	// 启动异步处理
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			t.Log(err)
		}
	}()

	// 监听错误
	go func() {
		for err := range pipeline.ErrorChan() {
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

	// 添加数据
	for i := 0; i < 6478017; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			t.Errorf("Failed to add item: %v", err)
		}
	}

	<-ctx.Done()

	if processedCount != 6478017 {
		t.Errorf("Expected 6478017 processed items, got %d", processedCount)
	}
	if errorCount == 0 {
		t.Errorf("Expected some errors, got %d", errorCount)
	}
}

func TestStandardPipelineAsyncPerformTimeout(t *testing.T) {
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
				// 长时间sleep模拟超时场景
				time.Sleep(time.Second * 6)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
				//t.Log(batchData)
			}
			return nil
		})
	defer pipeline.Close()

	// 启动异步处理
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			t.Log(err)
		}
	}()

	// 监听错误
	go func() {
		for err := range pipeline.ErrorChan() {
			t.Errorf("Expected no error, got %v", err)
		}
	}()

	// 添加数据
	for i := 0; i < 6478017; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			if errors.Is(err, gopipeline.ErrContextIsClosed) {
				t.Log("gopipeline.ErrContextIsClosed", err)
				break
			} else if errors.Is(err, gopipeline.ErrChannelIsClosed) {
				t.Log("gopipeline.ErrChannelIsClosed", err)
			} else {
				t.Errorf("Failed to add item: %v", err)
			}
		}
	}

	<-ctx.Done()

	if processedCount == 6478017 {
		t.Errorf("Expected less than 6478017 processed items, got %d", processedCount)
	}
}

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
				// SyncPerform 是有序的，即使sleep也不影响
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
				//t.Log(batchData)
			}
			return nil
		})
	defer pipeline.Close()

	// 启动同步处理
	go func() {
		if err := pipeline.SyncPerform(ctx); err != nil {
			t.Log(err)
		}
	}()

	// 监听错误
	go func() {
		for err := range pipeline.ErrorChan() {
			t.Errorf("Expected no error, got %v", err)
		}
	}()

	// 添加数据
	for i := 0; i < 2000; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			t.Errorf("Failed to add item: %v", err)
		}
	}

	<-ctx.Done()

	if processedCount != 2000 {
		t.Errorf("Expected 2000 processed items, got %d", processedCount)
	}
}

func TestStandardPipelineSyncPerformWithFlushError(t *testing.T) {
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
				// SyncPerform 是有序的，即使sleep也不影响
				time.Sleep(time.Millisecond * 10)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
				//t.Log(batchData)
			}
			return newErrorWithData(batchData, errors.New("test error"))
		})
	defer pipeline.Close()

	// 启动同步处理
	go func() {
		if err := pipeline.SyncPerform(ctx); err != nil {
			t.Log(err)
		}
	}()

	// 监听错误
	go func() {
		for err := range pipeline.ErrorChan() {
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

	// 添加数据
	for i := 0; i < 2000; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			t.Errorf("Failed to add item: %v", err)
		}
	}

	<-ctx.Done()

	if processedCount != 2000 {
		t.Errorf("Expected 2000 processed items, got %d", processedCount)
	}
	if errorCount == 0 {
		t.Errorf("Expected some errors, got %d", errorCount)
	}
}

func TestStandardPipelineSyncPerformTimeout(t *testing.T) {
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
				// SyncPerform 是有序的，即使sleep也不影响
				time.Sleep(time.Millisecond * 1000)
				mux.Lock()
				processedCount += len(batchData)
				mux.Unlock()
				//t.Log(batchData)
			}
			return nil
		})
	defer pipeline.Close()

	// 启动同步处理
	go func() {
		if err := pipeline.SyncPerform(ctx); err != nil {
			t.Log(err)
		}
	}()

	// 监听错误
	go func() {
		for err := range pipeline.ErrorChan() {
			t.Errorf("Expected no error, got %v", err)
		}
	}()

	// 添加数据
	for i := 0; i < 2000; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			if errors.Is(err, gopipeline.ErrContextIsClosed) {
				break
			} else if errors.Is(err, gopipeline.ErrChannelIsClosed) {
				t.Log("gopipeline.ErrChannelIsClosed", err)
			} else {
				t.Errorf("Failed to add item: %v", err)
			}
		}
	}

	<-ctx.Done()

	if processedCount == 2000 {
		t.Errorf("Expected less than 2000 processed items, got %d", processedCount)
	}
}

func TestStandardPipelineWhenDataChanIsClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	pipeline := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batchData []int) error {
			return nil
		},
	)
	defer pipeline.Close()

	// 启动异步处理
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			t.Log(err)
		}
	}()

	// 监听错误
	go func() {
		for err := range pipeline.ErrorChan() {
			t.Errorf("Expected no error, got %v", err)
		}
	}()

	// 取消上下文
	cancel()

	// 尝试添加数据应该失败
	if err := pipeline.Add(ctx, 1); err == nil {
		t.Errorf("Expected ErrChannelIsClosed or ErrContextIsClosed, got %v", err)
	}
}

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

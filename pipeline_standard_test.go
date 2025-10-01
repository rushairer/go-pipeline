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

	doneChan := make(chan struct{})
	// 启动异步处理
	go func() {
		defer close(doneChan)

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

	<-doneChan
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
	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)

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

	<-doneChan
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
	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)

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

	<-doneChan
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
	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)

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

	<-doneChan
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

// TestStandardPipelineSyncPerformRestart 测试同步队列重复启动和前后独立性
func TestStandardPipelineSyncPerformRestart(t *testing.T) {
	var mux sync.Mutex
	var firstRunProcessed, secondRunProcessed int
	var processOrder []int

	pipeline := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    16,
			FlushSize:     32,
			FlushInterval: time.Millisecond * 50,
		},
		func(ctx context.Context, batchData []int) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				mux.Lock()
				processOrder = append(processOrder, batchData...)
				mux.Unlock()
				return nil
			}
		})

	// 第一次运行
	t.Log("开始第一次同步运行")
	ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel1()

	// 启动第一次同步处理
	performDone1 := make(chan error, 1)
	go func() {
		performDone1 <- pipeline.SyncPerform(ctx1)
	}()

	// 监听第一次运行的错误
	errorChan1 := pipeline.ErrorChan(10)
	go func() {
		for err := range errorChan1 {
			t.Errorf("First run: Expected no error, got %v", err)
		}
	}()

	// 第一次运行添加数据 - 不关闭通道，让 context 超时结束
	dataChan := pipeline.DataChan()
	dataSent1 := make(chan bool, 1)
	go func() {
		defer func() { dataSent1 <- true }()
		for i := 100; i < 200; i++ { // 使用100-199的数据
			select {
			case dataChan <- i:
			case <-ctx1.Done():
				return
			}
		}
	}()

	// 等待数据发送完成或超时
	select {
	case <-dataSent1:
		t.Log("第一次运行数据发送完成")
	case <-time.After(time.Millisecond * 500):
		t.Log("第一次运行数据发送超时，继续等待处理完成")
	}

	// 等待第一次运行完成
	select {
	case err := <-performDone1:
		if err != nil && !errors.Is(err, gopipeline.ErrContextIsClosed) {
			t.Errorf("First run failed: %v", err)
		}
	case <-time.After(time.Second * 2):
		t.Error("First run timed out")
	}

	// 记录第一次运行的结果
	mux.Lock()
	firstRunProcessed = len(processOrder)
	firstRunOrder := make([]int, len(processOrder))
	copy(firstRunOrder, processOrder)
	processOrder = nil // 清空处理顺序记录
	mux.Unlock()

	t.Logf("第一次运行处理了 %d 个项目", firstRunProcessed)

	// 等待一段时间确保第一次运行完全结束
	time.Sleep(time.Millisecond * 200)

	// 第二次运行
	t.Log("开始第二次同步运行")
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel2()

	// 启动第二次同步处理
	performDone2 := make(chan error, 1)
	go func() {
		performDone2 <- pipeline.SyncPerform(ctx2)
	}()

	// 监听第二次运行的错误
	errorChan2 := pipeline.ErrorChan(10)
	go func() {
		for err := range errorChan2 {
			t.Errorf("Second run: Expected no error, got %v", err)
		}
	}()

	// 第二次运行添加数据 - 复用同一个 dataChan，不关闭通道
	dataSent2 := make(chan bool, 1)
	go func() {
		defer func() { dataSent2 <- true }()
		for i := 200; i < 300; i++ { // 使用200-299的数据
			select {
			case dataChan <- i:
			case <-ctx2.Done():
				return
			}
		}
	}()

	// 等待数据发送完成或超时
	select {
	case <-dataSent2:
		t.Log("第二次运行数据发送完成")
	case <-time.After(time.Millisecond * 500):
		t.Log("第二次运行数据发送超时，继续等待处理完成")
	}

	// 等待第二次运行完成
	select {
	case err := <-performDone2:
		if err != nil && !errors.Is(err, gopipeline.ErrContextIsClosed) {
			t.Errorf("Second run failed: %v", err)
		}
	case <-time.After(time.Second * 2):
		t.Error("Second run timed out")
	}

	// 记录第二次运行的结果
	mux.Lock()
	secondRunProcessed = len(processOrder)
	secondRunOrder := make([]int, len(processOrder))
	copy(secondRunOrder, processOrder)
	mux.Unlock()

	t.Logf("第二次运行处理了 %d 个项目", secondRunProcessed)

	// 验证结果 - 由于使用超时机制，可能不会处理完所有数据，所以检查处理的数据是否大于0
	if firstRunProcessed == 0 {
		t.Errorf("第一次运行: 期望处理一些项目，实际处理了 %d 个", firstRunProcessed)
	}

	if secondRunProcessed == 0 {
		t.Errorf("第二次运行: 期望处理一些项目，实际处理了 %d 个", secondRunProcessed)
	}

	// 验证数据独立性：第一次运行的数据应该在100-199范围内
	for _, val := range firstRunOrder {
		if val < 100 || val >= 200 {
			t.Errorf("第一次运行包含了不应该的数据: %d", val)
		}
	}

	// 验证数据独立性：第二次运行的数据应该在200-299范围内
	for _, val := range secondRunOrder {
		if val < 200 || val >= 300 {
			t.Errorf("第二次运行包含了不应该的数据: %d", val)
		}
	}

	// 验证同步队列的有序性（在每次运行内部应该保持顺序）
	for i := 1; i < len(firstRunOrder); i++ {
		if firstRunOrder[i] < firstRunOrder[i-1] {
			t.Errorf("第一次运行数据顺序错误: %d 出现在 %d 之后", firstRunOrder[i], firstRunOrder[i-1])
		}
	}

	for i := 1; i < len(secondRunOrder); i++ {
		if secondRunOrder[i] < secondRunOrder[i-1] {
			t.Errorf("第二次运行数据顺序错误: %d 出现在 %d 之后", secondRunOrder[i], secondRunOrder[i-1])
		}
	}

	t.Log("同步队列重复启动测试通过：两次运行相互独立，数据处理正确且有序")
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

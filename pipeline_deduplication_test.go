package gopipeline_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

type DedupTestData struct {
	ID      string
	Name    string
	Address string
	Age     uint
}

// GetKey 实现 UniqueKeyData 接口
func (d DedupTestData) GetKey() string {
	return d.ID
}

// TestDeduplicationPipelineAsyncPerform 测试异步执行去重管道操作
func TestDeduplicationPipelineAsyncPerform(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	var uniqueKeys = make(map[string]bool)

	pipeline := gopipeline.NewDefaultDeduplicationPipeline(
		func(ctx context.Context, batchData map[string]DedupTestData) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				time.Sleep(time.Millisecond * 5)
				mux.Lock()
				processedCount += len(batchData)
				// 记录所有唯一键
				for key := range batchData {
					uniqueKeys[key] = true
				}
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

	// 使用新的 DataChan API 添加数据（包含重复项）
	dataChan := pipeline.DataChan()
	go func() {
		defer close(dataChan)
		for i := 0; i < 1000; i++ {
			// 每10个数据中产生2个重复项
			dataID := "ID-" + string(rune('A'+(i/10)%26))
			select {
			case dataChan <- DedupTestData{
				ID:      dataID,
				Name:    "TestUser",
				Address: "TestAddress",
				Age:     uint(20 + i%50),
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()

	mux.Lock()
	finalCount := processedCount
	finalUniqueCount := len(uniqueKeys)
	mux.Unlock()

	// 验证去重效果：处理的数据量应该等于唯一键的数量
	if finalCount != finalUniqueCount {
		t.Errorf("Expected processed count (%d) to equal unique count (%d)", finalCount, finalUniqueCount)
	}

	// 唯一键数量应该远小于总数据量（因为有重复）
	if finalUniqueCount >= 1000 {
		t.Errorf("Expected unique count to be less than 1000 due to deduplication, got %d", finalUniqueCount)
	}
}

// TestDeduplicationPipelineAsyncPerformWithFlushError 测试异步执行时的错误处理
func TestDeduplicationPipelineAsyncPerformWithFlushError(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	var errorCount int
	pipeline := gopipeline.NewDeduplicationPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    32,
			FlushSize:     16,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData map[string]DedupTestData) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				time.Sleep(time.Millisecond * 5)
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
		for i := 0; i < 500; i++ {
			dataID := "ID-" + string(rune('A'+(i/10)%26))
			select {
			case dataChan <- DedupTestData{
				ID:      dataID,
				Name:    "TestUser",
				Address: "TestAddress",
				Age:     uint(20 + i%50),
			}:
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

	if finalProcessedCount == 0 {
		t.Error("Expected some processed items, got 0")
	}
	if finalErrorCount == 0 {
		t.Error("Expected some errors, got 0")
	}
}

// TestDeduplicationPipelineSyncPerform 测试同步执行去重管道操作
func TestDeduplicationPipelineSyncPerform(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	var uniqueKeys = make(map[string]bool)

	pipeline := gopipeline.NewDeduplicationPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    32,
			FlushSize:     16,
			FlushInterval: time.Millisecond * 100,
		},
		func(ctx context.Context, batchData map[string]DedupTestData) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				time.Sleep(time.Millisecond * 5)
				mux.Lock()
				processedCount += len(batchData)
				for key := range batchData {
					uniqueKeys[key] = true
				}
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
		for i := 0; i < 300; i++ {
			dataID := "ID-" + strconv.Itoa(i)
			select {
			case dataChan <- DedupTestData{
				ID:      dataID,
				Name:    "TestUser",
				Address: "TestAddress",
				Age:     uint(20 + i%50),
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()

	mux.Lock()
	finalCount := processedCount
	finalUniqueCount := len(uniqueKeys)
	mux.Unlock()

	// 验证去重效果
	if finalCount != finalUniqueCount {
		t.Errorf("Expected processed count (%d) to equal unique count (%d)", finalCount, finalUniqueCount)
	}
}

// TestDeduplicationPipelineDataChanClosed 测试数据通道关闭后的行为
func TestDeduplicationPipelineDataChanClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	var mux sync.Mutex
	var processedCount int
	pipeline := gopipeline.NewDefaultDeduplicationPipeline(
		func(ctx context.Context, batchData map[string]DedupTestData) error {
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
	for i := 0; i < 50; i++ {
		dataID := "ID-" + strconv.Itoa(i)
		dataChan <- DedupTestData{
			ID:      dataID,
			Name:    "TestUser",
			Address: "TestAddress",
			Age:     uint(20 + i%50),
		}
	}
	close(dataChan) // 关闭数据通道

	// 等待处理完成
	select {
	case err := <-performDone:
		if err != nil && !errors.Is(err, gopipeline.ErrContextIsClosed) {
			t.Errorf("Expected ErrContextIsClosed or nil, got %v", err)
		}
	case <-time.After(time.Second * 5):
		t.Error("Expected processing to complete after channel close")
	}

	mux.Lock()
	finalCount := processedCount
	mux.Unlock()

	if finalCount <= 0 {
		t.Error("Expected some processed items, got 0")
	}
}

// TestDeduplicationPipelineErrorChanSize 测试错误通道大小配置
func TestDeduplicationPipelineErrorChanSize(t *testing.T) {
	pipeline := gopipeline.NewDefaultDeduplicationPipeline(
		func(ctx context.Context, batchData map[string]DedupTestData) error {
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

// TestDeduplicationPipelineCompleteDeduplication 测试完全去重场景
func TestDeduplicationPipelineCompleteDeduplication(t *testing.T) {
	var mux sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var processedCount int
	var uniqueKeys = make(map[string]bool)

	pipeline := gopipeline.NewDeduplicationPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    100,
			FlushSize:     50,
			FlushInterval: time.Millisecond * 50,
		},
		func(ctx context.Context, batchData map[string]DedupTestData) error {
			mux.Lock()
			processedCount += len(batchData)
			for key := range batchData {
				uniqueKeys[key] = true
			}
			mux.Unlock()
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

	// 发送大量重复数据（只有5个唯一ID）
	dataChan := pipeline.DataChan()
	go func() {
		defer close(dataChan)
		for i := 0; i < 1000; i++ {
			dataID := "ID-" + string(rune('A'+(i%5)))
			select {
			case dataChan <- DedupTestData{
				ID:      dataID,
				Name:    "TestUser",
				Address: "TestAddress",
				Age:     uint(20 + i%50),
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()

	mux.Lock()
	finalCount := processedCount
	finalUniqueCount := len(uniqueKeys)
	mux.Unlock()

	// 应该只有5个唯一项被处理
	if finalUniqueCount != 5 {
		t.Errorf("Expected exactly 5 unique items, got %d", finalUniqueCount)
	}
	if finalCount != 5 {
		t.Errorf("Expected exactly 5 processed items, got %d", finalCount)
	}
}

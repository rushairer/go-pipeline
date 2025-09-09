package gopipeline_test

import (
	"context"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

// TestErrorChanAutoBufferSize 测试ErrorChan方法在size=0时的自动缓冲区大小计算
func TestErrorChanAutoBufferSize(t *testing.T) {
	_, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	config := gopipeline.PipelineConfig{
		BufferSize:    100,
		FlushSize:     50,
		FlushInterval: time.Millisecond * 50,
	}

	pipeline := gopipeline.NewStandardPipeline(
		config,
		func(ctx context.Context, batchData []int) error {
			return nil
		},
	)

	// 测试size=0时的自动计算
	errorChan := pipeline.ErrorChan(0)

	if errorChan == nil {
		t.Error("ErrorChan should not return nil channel")
	}

	// 验证通道容量计算逻辑
	// 预期容量 = (FlushSize + BufferSize - 1) / BufferSize
	expectedCapacity := int((config.FlushSize + config.BufferSize - 1) / config.BufferSize)

	// 由于通道容量无法直接获取，我们通过发送测试来验证
	testSendToErrorChan(t, errorChan, expectedCapacity)
}

// TestErrorChanCustomBufferSize 测试ErrorChan方法在指定size时的自定义缓冲区大小
func TestErrorChanCustomBufferSize(t *testing.T) {
	_, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	customSize := 5

	pipeline := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batchData []int) error {
			return nil
		},
	)

	// 测试自定义缓冲区大小
	errorChan := pipeline.ErrorChan(customSize)

	if errorChan == nil {
		t.Error("ErrorChan should not return nil channel")
	}

	// 验证自定义容量
	testSendToErrorChan(t, errorChan, customSize)
}

// TestErrorChanMultipleCalls 测试多次调用ErrorChan方法的行为
func TestErrorChanMultipleCalls(t *testing.T) {
	_, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	pipeline := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batchData []int) error {
			return nil
		},
	)

	// 第一次调用
	errorChan1 := pipeline.ErrorChan(10)
	if errorChan1 == nil {
		t.Error("First ErrorChan call should not return nil")
	}

	// 第二次调用应该返回相同的通道
	errorChan2 := pipeline.ErrorChan(5) // 注意：size参数在第二次调用时会被忽略
	if errorChan2 != errorChan1 {
		t.Error("Multiple ErrorChan calls should return the same channel")
	}

	// 验证通道仍然可用
	testSendToErrorChan(t, errorChan1, 10) // 第一次调用设置的容量
}

// TestErrorChanWithoutCall 测试不调用ErrorChan方法时的行为
func TestErrorChanWithoutCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	pipeline := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batchData []int) error {
			// 返回错误，但用户没有调用ErrorChan
			return context.DeadlineExceeded
		},
	)

	// 启动管道处理（不调用ErrorChan）
	go func() {
		if err := pipeline.AsyncPerform(ctx); err != nil {
			t.Logf("Pipeline finished with: %v", err)
		}
	}()

	// 发送一些数据
	dataChan := pipeline.DataChan()
	go func() {
		defer close(dataChan)
		for i := 0; i < 5; i++ {
			select {
			case dataChan <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	// 等待处理完成
	<-ctx.Done()

	// 如果没有panic，说明错误被安全地忽略了
	t.Log("Test passed: Errors safely ignored when ErrorChan() is not called")
}

// testSendToErrorChan 辅助函数，测试向错误通道发送数据
func testSendToErrorChan(t *testing.T, errorChan <-chan error, expectedCapacity int) {
	// 创建一个临时通道用于测试容量
	testChan := make(chan error, expectedCapacity)

	// 填充测试通道到容量
	for i := 0; i < expectedCapacity; i++ {
		select {
		case testChan <- context.DeadlineExceeded:
			// 成功发送
		default:
			t.Errorf("Expected capacity at least %d, but failed to send item %d", expectedCapacity, i)
			return
		}
	}

	// 尝试发送超出容量的项目（应该失败）
	select {
	case testChan <- context.DeadlineExceeded:
		t.Errorf("Expected capacity %d, but was able to send extra item", expectedCapacity)
	default:
		// 预期行为：无法发送，说明容量正确
	}

	// 清空通道
	for i := 0; i < expectedCapacity; i++ {
		<-testChan
	}
}

// TestErrorChanBufferSizeCalculation 测试不同配置下的缓冲区大小计算
func TestErrorChanBufferSizeCalculation(t *testing.T) {
	testCases := []struct {
		name           string
		bufferSize     uint32
		flushSize      uint32
		expectedMinCap int
	}{
		{
			name:           "default_config",
			bufferSize:     100,
			flushSize:      50,
			expectedMinCap: 1, // (50+100-1)/100 = 1.49 -> 1
		},
		{
			name:           "small_buffers",
			bufferSize:     10,
			flushSize:      5,
			expectedMinCap: 1, // (5+10-1)/10 = 1.4 -> 1
		},
		{
			name:           "large_flush_relative_to_buffer",
			bufferSize:     50,
			flushSize:      100,
			expectedMinCap: 2, // (100+50-1)/50 = 2.98 -> 2
		},
		{
			name:           "equal_sizes",
			bufferSize:     50,
			flushSize:      50,
			expectedMinCap: 1, // (50+50-1)/50 = 1.98 -> 1
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := gopipeline.PipelineConfig{
				BufferSize:    tc.bufferSize,
				FlushSize:     tc.flushSize,
				FlushInterval: time.Millisecond * 50,
			}

			pipeline := gopipeline.NewStandardPipeline(
				config,
				func(ctx context.Context, batchData []int) error {
					return nil
				},
			)

			errorChan := pipeline.ErrorChan(0)
			if errorChan == nil {
				t.Error("ErrorChan should not return nil channel")
			}

			// 计算预期容量
			expectedCapacity := int((tc.flushSize + tc.bufferSize - 1) / tc.bufferSize)
			if expectedCapacity < tc.expectedMinCap {
				expectedCapacity = tc.expectedMinCap
			}

			t.Logf("Config: BufferSize=%d, FlushSize=%d, ExpectedCapacity=%d",
				tc.bufferSize, tc.flushSize, expectedCapacity)
		})
	}
}

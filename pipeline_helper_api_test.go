package gopipeline

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// 一个不会报错的 flush 函数，统计调用次数
func okFlush[T any](calls *int32) FlushStandardFunc[T] {
	return func(ctx context.Context, batch []T) error {
		atomic.AddInt32(calls, 1)
		return nil
	}
}

// 快速配置，降低测试时延
func quickConfig() PipelineConfig {
	return PipelineConfig{
		BufferSize:               16,
		FlushSize:                4,
		FlushInterval:            10 * time.Millisecond,
		DrainOnCancel:            true,
		DrainGracePeriod:         50 * time.Millisecond,
		MaxConcurrentFlushes:     0,
		UseMapReuse:              false,
		FinalFlushOnCloseTimeout: 50 * time.Millisecond,
	}.ValidateOrDefault()
}

func TestStart_ReturnsDoneAndErrs_SecondStartYieldsErrAlreadyRunning(t *testing.T) {
	var calls int32
	p := NewStandardPipeline[int](quickConfig(), okFlush[int](&calls))

	// Start 第一次
	ctx, cancel := context.WithCancel(context.Background())
	done1, errs := p.Start(ctx)
	if done1 == nil {
		t.Fatalf("Start should return a non-nil done channel")
	}

	// 立刻二次 Start，期望 ErrAlreadyRunning 进入错误通道
	_, _ = p.Start(ctx)

	// 从错误通道读到 ErrAlreadyRunning（给一些时间调度）
	gotErrAlreadyRunning := false
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()

readLoop:
	for {
		select {
		case err := <-errs:
			if errors.Is(err, ErrAlreadyRunning) {
				gotErrAlreadyRunning = true
				break readLoop
			}
			// 其他错误忽略，继续读取
		case <-timer.C:
			break readLoop
		}
	}
	if !gotErrAlreadyRunning {
		t.Fatalf("expected ErrAlreadyRunning on second Start, but not received")
	}

	// 投喂一些数据，触发至少一次 flush
	for i := 0; i < 8; i++ {
		p.DataChan() <- i
	}
	// 取消以结束
	cancel()

	// 等待 done 关闭
	select {
	case <-done1:
	case <-time.After(2 * time.Second):
		t.Fatalf("pipeline did not finish in time")
	}
}

func TestRun_BlocksUntilCancelAndReturnsContextError(t *testing.T) {
	var calls int32
	p := NewStandardPipeline[int](quickConfig(), okFlush[int](&calls))

	// 为确保错误通道容量被设置，传入一个自定义容量
	errChSize := 8

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 在另外的 goroutine 中稍后取消
	go func() {
		// 投喂一些数据，确保有 flush 发生
		for i := 0; i < 6; i++ {
			p.DataChan() <- i
		}
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := p.Run(ctx, errChSize)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("Run should return an error on cancel")
	}
	if !errors.Is(err, ErrContextIsClosed) {
		t.Fatalf("expected Run error to satisfy errors.Is(_, ErrContextIsClosed), got: %v", err)
	}
	// 合理的返回时间（避免无限阻塞）
	if elapsed > 3*time.Second {
		t.Fatalf("Run took too long to return on cancel: %v", elapsed)
	}

	// 至少应发生过一次 flush
	if atomic.LoadInt32(&calls) == 0 {
		t.Fatalf("expected at least one flush call, got 0")
	}
}

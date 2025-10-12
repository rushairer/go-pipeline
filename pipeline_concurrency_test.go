package gopipeline_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

// TestMaxConcurrentFlushesLimit 确认 MaxConcurrentFlushes=1 限制异步 flush 并发度
func TestMaxConcurrentFlushesLimit(t *testing.T) {
	var (
		current int32 // 当前并发中的 flush 计数
		maxSeen int32 // 观测到的最大并发
	)

	// 处理函数：每次 flush 持续一段时间，便于制造重叠 flush
	processDur := 120 * time.Millisecond

	cfg := gopipeline.NewPipelineConfig().
		WithBufferSize(256).
		WithFlushSize(16).
		WithFlushInterval(10 * time.Millisecond).
		WithMaxConcurrentFlushes(1) // 关键：限制为1

	p := gopipeline.NewStandardPipeline(cfg, func(ctx context.Context, batch []int) error {
		// 增加并更新并发观测
		cur := atomic.AddInt32(&current, 1)
		for {
			// 记录最大并发
			old := atomic.LoadInt32(&maxSeen)
			if cur > old {
				if atomic.CompareAndSwapInt32(&maxSeen, old, cur) {
					break
				}
				continue
			}
			break
		}

		// 模拟较长的处理时长，制造潜在重叠
		select {
		case <-time.After(processDur):
		case <-ctx.Done():
		}

		atomic.AddInt32(&current, -1)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 启动异步执行
	done := make(chan error, 1)
	go func() { done <- p.AsyncPerform(ctx) }()

	// 必须创建错误通道以避免内部错误阻塞（尽管当前用例不应产生错误）
	_ = p.ErrorChan(32)

	// 快速写入多批数据，触发多次 flush
	ch := p.DataChan()
	go func() {
		defer func() {
			// 不关闭通道，交由 ctx 结束
		}()
		for i := 0; i < 1000; i++ {
			select {
			case ch <- i:
			case <-ctx.Done():
				return
			}
			// 稍微放慢写入，配合 FlushInterval 触发
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// 等待上下文完成
	<-ctx.Done()
	// 等待异步执行退出
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Log("AsyncPerform did not complete promptly after context done")
	}

	if atomic.LoadInt32(&maxSeen) > 1 {
		t.Fatalf("Max concurrent flushes observed = %d, expected <= 1", atomic.LoadInt32(&maxSeen))
	}
}

// TestErrAlreadyRunning 确认同一实例并发启动多次 Perform 返回 ErrAlreadyRunning
func TestErrAlreadyRunning(t *testing.T) {
	cfg := gopipeline.NewPipelineConfig().
		WithBufferSize(64).
		WithFlushSize(16).
		WithFlushInterval(50 * time.Millisecond).
		WithMaxConcurrentFlushes(1)

	p := gopipeline.NewStandardPipeline(cfg, func(ctx context.Context, batch []int) error {
		// 轻量处理
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Millisecond):
			return nil
		}
	})

	ctx1, cancel1 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel1()

	// 首次启动（异步）
	started := make(chan struct{})
	go func() {
		close(started)
		_ = p.AsyncPerform(ctx1)
	}()

	<-started

	// 立即尝试再次启动（同步或异步均应返回 ErrAlreadyRunning）
	ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel2()

	err := p.SyncPerform(ctx2)
	if !errors.Is(err, gopipeline.ErrAlreadyRunning) {
		t.Fatalf("expected ErrAlreadyRunning, got: %v", err)
	}
}

// 附加：在测试文件内使用互斥或其他辅助，避免竞态（如上无共享写入无需）。
// 现有项目的其他测试已验证核心行为，这里仅覆盖新增能力。

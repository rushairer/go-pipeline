package gopipeline

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// 满足去重管道所需的唯一键接口
type user struct{ id string }

func (u user) GetKey() string { return u.id }

// 标准管道：默认 DrainOnCancel=false，cancel 不应 flush 未满批次
func TestStandard_Cancel_NoDrain_DoesNotFlush(t *testing.T) {
	var processed int64
	config := NewPipelineConfig().
		WithBufferSize(100).
		WithFlushSize(50).
		WithFlushInterval(50 * time.Millisecond) // 未满批次不应因 cancel 而 flush

	p := NewStandardPipeline[int](config, func(ctx context.Context, batch []int) error {
		atomic.AddInt64(&processed, int64(len(batch)))
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.AsyncPerform(ctx) }()

	ch := p.DataChan()
	// 只发送一个未满批次
	for i := 0; i < 10; i++ {
		ch <- i
	}

	// 不关闭通道，直接取消
	cancel()

	// 等待一小段时间以让 goroutine 有机会响应取消
	time.Sleep(50 * time.Millisecond)

	// 等待退出并断言错误
	select {
	case err := <-errCh:
		if !errors.Is(err, ErrContextIsClosed) {
			t.Fatalf("expected ErrContextIsClosed, got %v", err)
		}
		if errors.Is(err, ErrContextDrained) {
			t.Fatalf("should not be drained when DrainOnCancel=false, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for AsyncPerform to exit")
	}

	// 不应 flush 这 10 条
	if got := atomic.LoadInt64(&processed); got != 0 {
		t.Fatalf("expected 0 processed on cancel without drain, got %d", got)
	}
}

// 标准管道：DrainOnCancel=true + Grace，cancel 时尽力 flush 未满批次
func TestStandard_Cancel_WithDrain_FlushesPartial(t *testing.T) {
	var processed int64
	config := NewPipelineConfig().
		WithBufferSize(100).
		WithFlushSize(50).
		WithFlushInterval(10 * time.Second). // 很长间隔，确保不会被定时触发
		WithDrainOnCancel(true).
		WithDrainGracePeriod(200 * time.Millisecond)

	p := NewStandardPipeline[int](config, func(ctx context.Context, batch []int) error {
		atomic.AddInt64(&processed, int64(len(batch)))
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- p.AsyncPerform(ctx) }()

	ch := p.DataChan()
	for i := 0; i < 10; i++ {
		ch <- i
	}

	// 直接取消，触发 drain-on-cancel
	cancel()

	// 等待退出并断言错误组合
	select {
	case err := <-errCh:
		if !errors.Is(err, ErrContextIsClosed) {
			t.Fatalf("expected ErrContextIsClosed, got %v", err)
		}
		if !errors.Is(err, ErrContextDrained) {
			t.Fatalf("expected ErrContextDrained when DrainOnCancel=true, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for AsyncPerform to exit")
	}

	// 等待在宽限期内 flush 完成
	deadline := time.After(1 * time.Second)
	for {
		if atomic.LoadInt64(&processed) == 10 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting drain-on-cancel flush, processed=%d", atomic.LoadInt64(&processed))
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// 标准管道：关闭通道后必然 flush（即便 ctx 已取消）
func TestStandard_CloseChannel_AlwaysFlush(t *testing.T) {
	var processed int64
	config := NewPipelineConfig().
		WithBufferSize(100).
		WithFlushSize(50).
		WithFlushInterval(50 * time.Millisecond)

	p := NewStandardPipeline[int](config, func(ctx context.Context, batch []int) error {
		atomic.AddInt64(&processed, int64(len(batch)))
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.AsyncPerform(ctx) }()

	ch := p.DataChan()
	for i := 0; i < 10; i++ {
		ch <- i
	}
	// 先关闭通道，再取消 ctx；应保证 flush 发生（实现中用 context.Background() 执行最终 flush）
	close(ch)

	deadline := time.After(1 * time.Second)
	for {
		if atomic.LoadInt64(&processed) == 10 {
			// 等待退出并断言返回为 nil
			select {
			case err := <-errCh:
				if err != nil {
					t.Fatalf("expected nil on channel close, got %v", err)
				}
			case <-time.After(1 * time.Second):
				t.Fatal("timeout waiting for AsyncPerform to exit")
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting close-channel final flush, processed=%d", atomic.LoadInt64(&processed))
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// 去重管道：默认 DrainOnCancel=false，cancel 不应 flush 未满批次
func TestDedup_Cancel_NoDrain_DoesNotFlush(t *testing.T) {
	var processed int64

	config := NewPipelineConfig().
		WithBufferSize(100).
		WithFlushSize(50).
		WithFlushInterval(50 * time.Millisecond)

	p := NewDeduplicationPipeline[user](config, func(ctx context.Context, batch map[string]user) error {
		atomic.AddInt64(&processed, int64(len(batch)))
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- p.AsyncPerform(ctx) }()

	ch := p.DataChan()
	// 放入一些重复数据（有效唯一数 < 10）
	for i := 0; i < 10; i++ {
		ch <- user{id: fmt.Sprintf("%d", i%3)}
	}

	cancel()
	time.Sleep(50 * time.Millisecond)

	// 等待退出并断言错误
	select {
	case err := <-errCh:
		if !errors.Is(err, ErrContextIsClosed) {
			t.Fatalf("expected ErrContextIsClosed, got %v", err)
		}
		if errors.Is(err, ErrContextDrained) {
			t.Fatalf("should not be drained when DrainOnCancel=false (dedup), got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for AsyncPerform to exit")
	}

	if got := atomic.LoadInt64(&processed); got != 0 {
		t.Fatalf("expected 0 processed on cancel without drain (dedup), got %d", got)
	}
}

// 去重管道：DrainOnCancel=true + Grace，cancel 时尽力 flush 未满批次（按唯一键计数）
func TestDedup_Cancel_WithDrain_FlushesPartial(t *testing.T) {
	var processed int64

	config := NewPipelineConfig().
		WithBufferSize(100).
		WithFlushSize(50).
		WithFlushInterval(10 * time.Second).
		WithDrainOnCancel(true).
		WithDrainGracePeriod(200 * time.Millisecond)

	p := NewDeduplicationPipeline[user](config, func(ctx context.Context, batch map[string]user) error {
		atomic.AddInt64(&processed, int64(len(batch))) // 这里是唯一键数量
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- p.AsyncPerform(ctx) }()

	ch := p.DataChan()
	// 总共 10 条，但只有 3 个唯一键
	for i := 0; i < 10; i++ {
		ch <- user{id: fmt.Sprintf("%d", i%3)}
	}

	cancel()

	// 等待退出并断言错误组合
	select {
	case err := <-errCh:
		if !errors.Is(err, ErrContextIsClosed) {
			t.Fatalf("expected ErrContextIsClosed, got %v", err)
		}
		if !errors.Is(err, ErrContextDrained) {
			t.Fatalf("expected ErrContextDrained when DrainOnCancel=true (dedup), got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for AsyncPerform to exit")
	}

	deadline := time.After(1 * time.Second)
	for {
		if atomic.LoadInt64(&processed) == 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting drain-on-cancel flush (dedup), processed=%d", atomic.LoadInt64(&processed))
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// 去重管道：关闭通道后必然 flush（即便 ctx 已取消）
func TestDedup_CloseChannel_AlwaysFlush(t *testing.T) {
	var processed int64

	config := NewPipelineConfig().
		WithBufferSize(100).
		WithFlushSize(50).
		WithFlushInterval(50 * time.Millisecond)

	p := NewDeduplicationPipeline[user](config, func(ctx context.Context, batch map[string]user) error {
		atomic.AddInt64(&processed, int64(len(batch)))
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.AsyncPerform(ctx) }()

	ch := p.DataChan()
	for i := 0; i < 10; i++ {
		ch <- user{id: fmt.Sprintf("%d", i%3)}
	}
	close(ch)

	deadline := time.After(1 * time.Second)
	for {
		if atomic.LoadInt64(&processed) == 3 { // 3 个唯一键被 flush
			// 等待退出并断言返回为 nil
			select {
			case err := <-errCh:
				if err != nil {
					t.Fatalf("expected nil on channel close (dedup), got %v", err)
				}
			case <-time.After(1 * time.Second):
				t.Fatal("timeout waiting for AsyncPerform to exit")
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting close-channel final flush (dedup), processed=%d", atomic.LoadInt64(&processed))
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

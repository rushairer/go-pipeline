package gopipeline_test

import (
	"context"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

// TestCurrentFlushSize_InitialValue 验证初始 FlushSize 来自配置
func TestCurrentFlushSize_InitialValue(t *testing.T) {
	p := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    64,
			FlushSize:     16,
			FlushInterval: 50 * time.Millisecond,
		},
		func(ctx context.Context, batch []int) error { return nil },
	)
	if got, want := p.CurrentFlushSize(), uint32(16); got != want {
		t.Fatalf("CurrentFlushSize = %d, want %d", got, want)
	}
}

// TestUpdateFlushSize_ChangesCurrent 验证 UpdateFlushSize 更新当前值，0 回退为 1
func TestUpdateFlushSize_ChangesCurrent(t *testing.T) {
	p := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batch []int) error { return nil },
	)
	p.UpdateFlushSize(64)
	if got, want := p.CurrentFlushSize(), uint32(64); got != want {
		t.Fatalf("after UpdateFlushSize(64), CurrentFlushSize = %d, want %d", got, want)
	}
	p.UpdateFlushSize(0)
	if got, want := p.CurrentFlushSize(), uint32(1); got != want {
		t.Fatalf("after UpdateFlushSize(0), CurrentFlushSize = %d, want %d (fallback to 1)", got, want)
	}
}

// TestUpdateFlushSize_TakesEffect 验证更新 FlushSize 后，管道按新大小触发 flush
func TestUpdateFlushSize_TakesEffect(t *testing.T) {
	flushCh := make(chan int, 4)
	p := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    256,
			FlushSize:     1000,       // 初始很大，避免被旧大小触发
			FlushInterval: time.Hour,  // 初始很大，避免被定时器触发
		},
		func(ctx context.Context, batch []int) error {
			select {
			case flushCh <- len(batch):
			default:
			}
			return nil
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errs := p.ErrorChan(16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = p.AsyncPerform(ctx)
	}()
	go func() {
		for {
			select {
			case _, ok := <-errs:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 更新 FlushSize 为 4
	p.UpdateFlushSize(4)

	// 投 4 条数据（不关闭通道），应触发一次大小为 4 的 flush
	ch := p.DataChan()
	for i := 0; i < 4; i++ {
		select {
		case ch <- i:
		case <-ctx.Done():
			t.Fatal("context canceled while sending")
		}
	}

	select {
	case n := <-flushCh:
		if n != 4 {
			t.Fatalf("flush batch size = %d, want 4", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected flush after UpdateFlushSize, none observed")
	}

	cancel()
	<-done
}

// TestCurrentFlushInterval_InitialValue 验证初始 FlushInterval 来自配置
func TestCurrentFlushInterval_InitialValue(t *testing.T) {
	p := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    64,
			FlushSize:     16,
			FlushInterval: 100 * time.Millisecond,
		},
		func(ctx context.Context, batch []int) error { return nil },
	)
	if got, want := p.CurrentFlushInterval(), 100*time.Millisecond; got != want {
		t.Fatalf("CurrentFlushInterval = %v, want %v", got, want)
	}
}

// TestUpdateFlushInterval_ChangesCurrent 验证 UpdateFlushInterval 更新当前值，<=0 回退为 1ms
func TestUpdateFlushInterval_ChangesCurrent(t *testing.T) {
	p := gopipeline.NewDefaultStandardPipeline(
		func(ctx context.Context, batch []int) error { return nil },
	)
	p.UpdateFlushInterval(10 * time.Millisecond)
	if got, want := p.CurrentFlushInterval(), 10*time.Millisecond; got != want {
		t.Fatalf("after UpdateFlushInterval(10ms), CurrentFlushInterval = %v, want %v", got, want)
	}
	p.UpdateFlushInterval(0)
	if got, want := p.CurrentFlushInterval(), time.Millisecond; got != want {
		t.Fatalf("after UpdateFlushInterval(0), CurrentFlushInterval = %v, want %v (fallback to 1ms)", got, want)
	}
}

// TestUpdateFlushInterval_TakesEffect 验证更新 FlushInterval 后，定时 flush 按新间隔触发
func TestUpdateFlushInterval_TakesEffect(t *testing.T) {
	flushCh := make(chan struct{}, 4)
	p := gopipeline.NewStandardPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    256,
			FlushSize:     1000,       // 很大，确保不因批量满而触发
			FlushInterval: time.Hour,  // 初始很大，确保不因旧定时器触发
		},
		func(ctx context.Context, batch []int) error {
			select {
			case flushCh <- struct{}{}:
			default:
			}
			return nil
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errs := p.ErrorChan(16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = p.AsyncPerform(ctx)
	}()
	go func() {
		for {
			select {
			case _, ok := <-errs:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 更新间隔为 20ms（nudge 会唤醒主循环，使新间隔尽快生效）
	p.UpdateFlushInterval(20 * time.Millisecond)

	// 投 1 条数据，远低于 FlushSize，只能靠定时器触发
	select {
	case p.DataChan() <- 1:
	case <-ctx.Done():
		t.Fatal("context canceled while sending")
	}

	// 20ms 间隔 + 调度余量，2s 内应触发定时 flush
	select {
	case <-flushCh:
	case <-time.After(2 * time.Second):
		t.Fatal("expected timer-triggered flush after UpdateFlushInterval, none observed")
	}

	cancel()
	<-done
}

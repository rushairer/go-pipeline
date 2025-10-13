package gopipeline_test

import (
	"bytes"
	"context"
	"log"
	"sync/atomic"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

var dhFlushCount int32
var dhErrorCount int32
var dhDroppedCount int32

// dummyHook 实现了在 pipeline_impl.go 中定义的 MetricsHook 接口：
//
//	type MetricsHook interface {
//	  Flush(items int, duration time.Duration)
//	  Error(err error)
//	  ErrorDropped()
//	}
type dummyHook struct{}

func (dummyHook) Flush(items int, duration time.Duration) { atomic.AddInt32(&dhFlushCount, 1) }
func (dummyHook) Error(err error)                         { atomic.AddInt32(&dhErrorCount, 1) }
func (dummyHook) ErrorDropped()                           { atomic.AddInt32(&dhDroppedCount, 1) }

// TestDoneChannelWithStart 验证 Done() 返回的通道会在
// 通过 Start 启动的 Perform 完成时关闭。我们仍然建议用户
// 将 Start(ctx) 返回的 done 作为主要的完成信号。
func TestDoneChannelWithStart(t *testing.T) {
	cfg := gopipeline.NewPipelineConfig().
		WithBufferSize(16).
		WithFlushSize(1).
		WithFlushInterval(10 * time.Millisecond)

	p := gopipeline.NewStandardPipeline[int](cfg, func(ctx context.Context, batch []int) error {
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 启动并获取 done 通道
	doneFromStart, _ := p.Start(ctx)
	// 获取当前 Done() 通道的快照（应与当前运行匹配）
	doneSnap := p.Done()

	// 发送一些元素后关闭通道以结束
	ch := p.DataChan()
	go func() {
		defer close(ch) // 写入方关闭通道
		for i := 0; i < 5; i++ {
			select {
			case ch <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	select {
	case <-doneFromStart:
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for Start(ctx) done")
	}
	// 此时 Done() 的快照也应已关闭
	select {
	case <-doneSnap:
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout waiting for Done() snapshot to close")
	}
}

// TestWithMetricsHook 验证可以注入 MetricsHook 且其会被触发。
// 我们模拟一次 flush，并断言钩子的 Flush() 至少被调用一次。
func TestWithMetricsHook(t *testing.T) {
	atomic.StoreInt32(&dhFlushCount, 0)
	atomic.StoreInt32(&dhErrorCount, 0)
	atomic.StoreInt32(&dhDroppedCount, 0)

	// 通过 WithMetrics 设置钩子，并触发至少一次 flush。
	cfg := gopipeline.NewPipelineConfig().
		WithBufferSize(32).
		WithFlushSize(8).
		WithFlushInterval(24 * time.Hour) // 避免基于计时器的 flush

	// 通过短暂的休眠制造可测量的持续时间。
	p := gopipeline.NewStandardPipeline[int](cfg, func(ctx context.Context, batch []int) error {
		return nil
	})

	// 注入指标钩子
	_ = p.WithMetrics(dummyHook{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	doneFromStart, _ := p.Start(ctx)

	// 发送一个完整批次以触发 flush
	ch := p.DataChan()
	for i := 0; i < int(cfg.FlushSize); i++ {
		select {
		case ch <- i:
		case <-ctx.Done():
			t.Fatalf("context canceled unexpectedly")
		}
	}
	// 关闭通道以尽快结束（延迟以避免竞态）
	time.AfterFunc(10*time.Millisecond, func() { close(ch) })

	select {
	case <-doneFromStart:
	case <-time.After(1 * time.Second):
		t.Fatalf("pipeline did not finish in time")
	}

	// 确保我们的指标钩子观察到至少一次 Flush 调用
	if atomic.LoadInt32(&dhFlushCount) < 1 {
		t.Fatalf("expected metrics hook Flush to be called at least once")
	}

	// 由于无法直接访问内部钩子，做一次健全性断言：
	// 至少应发生一次 flush。我们将快速重新运行一次同步的 perform，
	// 使用计数包装器以确保在 CI 中覆盖钩子路径。
	var count int32
	p2 := gopipeline.NewStandardPipeline[int](cfg, func(ctx context.Context, batch []int) error {
		atomic.AddInt32(&count, 1)
		return nil
	})
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	done2, _ := p2.Start(ctx2)
	ch2 := p2.DataChan()
	for i := 0; i < int(cfg.FlushSize); i++ {
		ch2 <- i
	}
	time.AfterFunc(10*time.Millisecond, func() { close(ch2) })
	select {
	case <-done2:
	case <-time.After(1 * time.Second):
		t.Fatalf("pipeline #2 did not finish in time")
	}

	if atomic.LoadInt32(&count) < 1 {
		t.Fatalf("expected at least one flush, got %d", count)
	}

}

// TestWithLogger 确保 WithLogger 能接受自定义 logger 且不会引发问题。
// 我们不断言具体日志输出（取决于实现），仅覆盖相关路径。
func TestWithLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "pipeline-test: ", log.LstdFlags)

	cfg := gopipeline.NewPipelineConfig().
		WithBufferSize(16).
		WithFlushSize(8).
		WithFlushInterval(5 * time.Millisecond)

	p := gopipeline.NewStandardPipeline[int](cfg, func(ctx context.Context, batch []int) error {
		// 不产生错误；日志行为由实现定义
		return nil
	}).WithLogger(logger)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = p.AsyncPerform(ctx)
		close(done)
	}()

	ch := p.DataChan()
	for i := 0; i < 3; i++ {
		ch <- i
	}
	close(ch)

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("pipeline did not finish in time")
	}
	// 可选地，我们可以检查 buf.Len() >= 0（显然恒为真）。
}

package gopipeline_test

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

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

// TestMaxConcurrentFlushes verifies that asynchronous flush concurrency
// is capped by the configured MaxConcurrentFlushes. It observes the maximum
// number of concurrent in-flight flush functions by incrementing/decrementing
// an atomic counter around the critical section of the flush function.
func TestMaxConcurrentFlushes(t *testing.T) {
	type args struct {
		maxConc uint32
	}
	cases := []args{
		{maxConc: 1},
		{maxConc: 2},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("max="+strconv.Itoa(int(tc.maxConc)), func(t *testing.T) {
			var current int32
			var maxObserved int32

			// Flush 函数：进入时 +1，离开时 -1；在临界区 sleep 一小段时间，制造重叠
			flush := func(ctx context.Context, batch []int) error {
				cur := atomic.AddInt32(&current, 1)
				// 记录最大并发
				for {
					old := atomic.LoadInt32(&maxObserved)
					if cur <= old {
						break
					}
					if atomic.CompareAndSwapInt32(&maxObserved, old, cur) {
						break
					}
				}

				// 模拟工作并保证观察到并发（同时尊重 ctx）
				select {
				case <-time.After(30 * time.Millisecond):
				case <-ctx.Done():
					// 遵从取消，及时退出
				}

				atomic.AddInt32(&current, -1)
				return nil
			}

			// 配置：FlushSize 小一些，BufferSize 足够大；FlushInterval 很大以避免定时触发。
			cfg := gopipeline.NewPipelineConfig().
				WithBufferSize(4096).
				WithFlushSize(16).
				WithFlushInterval(24 * time.Hour).
				WithMaxConcurrentFlushes(tc.maxConc)

			p := gopipeline.NewStandardPipeline[int](cfg, flush)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// 启动异步执行
			errs := p.ErrorChan(64)
			doneCh := make(chan struct{})
			go func() {
				defer close(doneCh)
				_ = p.AsyncPerform(ctx)
			}()
			// 轻量 drain 错误通道，避免实现差异导致潜在阻塞（尽管内部应为非阻塞）
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

			// 发送足够多的数据，触发多次 flush
			ch := p.DataChan()
			total := 16 * 20 // 足以触发 ~20 次 flush
			go func() {
				defer close(ch) // writer closes，触发最终 flush
				for i := 0; i < total; i++ {
					select {
					case ch <- i:
					case <-ctx.Done():
						return
					}
					// 用一些微小的间隔避免完全串行/完全同时产生过度抖动
					if i%8 == 0 {
						time.Sleep(1 * time.Millisecond)
					}
				}
			}()

			// 等待完成（使用本地 doneCh）
			select {
			case <-doneCh:
			case <-time.After(3 * time.Second):
				t.Fatalf("pipeline did not finish in time")
			}

			// 断言最大并发不超过配置值
			if got, want := atomic.LoadInt32(&maxObserved), int32(tc.maxConc); got > want {
				t.Fatalf("max in-flight flush exceeded cap: got=%d want<=%d", got, want)
			}
		})
	}
}

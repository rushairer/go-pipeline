package gopipeline_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

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
		t.Run(("max=" + itoa(tc.maxConc)), func(t *testing.T) {
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

// itoa for small uint32, to avoid importing strconv in tests
func itoa(u uint32) string {
	// fast path for small values used here (1,2,...)
	switch u {
	case 0:
		return "0"
	case 1:
		return "1"
	case 2:
		return "2"
	case 3:
		return "3"
	case 4:
		return "4"
	case 5:
		return "5"
	}
	// fallback (rarely hit in this test)
	n := int(u)
	if n == 0 {
		return "0"
	}
	buf := [10]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + (n % 10))
		n /= 10
	}
	return string(buf[i:])
}

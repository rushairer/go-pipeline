package gopipeline_test

import (
	"context"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

// 验证：通过 DataChan() 传递不同类型时的内存拷贝行为
// 结论预期：
// - 基本类型（int、string）按值拷贝的是头部（string 只拷贝指针+长度），通常不会造成大的内存浪费
// - 数组类型 [N]T 作为通道元素会整体拷贝（值拷贝），因此可能有内存/时间开销
// - 切片类型 []T 作为通道元素仅拷贝切片头（ptr/len/cap），底层数据不拷贝，若生产者复用/修改底层缓冲会被下游看到
// - 指针类型 *T 仅拷贝指针本身，不会重复拷贝指针指向的内存，但存在共享和数据竞争风险
func TestPipeline_ArrayVsSliceMemoryCopy(t *testing.T) {
	// 使用关闭数据通道触发最终 flush 的模式，便于在发送后修改源缓冲再触发 flush
	// FlushSize 设大一点，避免发送时立即触发 flush
	cfg := gopipeline.NewPipelineConfig().
		WithFlushSize(10).
		WithBufferSize(16).
		WithFlushInterval(50 * time.Millisecond).
		WithFinalFlushOnCloseTimeout(200 * time.Millisecond)

	// 1) 切片作为元素类型：验证底层数据共享（不发生字节数据拷贝）
	t.Run("slice element shares underlying bytes", func(t *testing.T) {
		// 结果收集：记录 flush 时看到的内容
		var seen [][]byte
		p := gopipeline.NewStandardPipeline[[]byte](cfg, func(ctx context.Context, batch [][]byte) error {
			// 为避免测试本身因共享导致后续修改影响，这里对观察到的 batch 元素做一次独立复制快照
			for _, b := range batch {
				cp := append([]byte(nil), b...)
				seen = append(seen, cp)
			}
			return nil
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		go func() { _ = p.AsyncPerform(ctx) }()

		ch := p.DataChan()
		// 源缓冲及其切片视图
		src := make([]byte, 8)
		for i := range src {
			src[i] = byte(i)
		}
		view := src[:]

		// 发送切片视图（仅拷贝切片头）
		ch <- view
		// 发送后修改底层缓冲（若共享则下游会观察到修改后的值）
		src[0] = 99
		src[7] = 77
		// 触发最终 flush
		close(ch)

		// 等待一点时间让最终 flush 完成
		select {
		case <-p.Done():
		case <-time.After(500 * time.Millisecond):
		}

		if len(seen) != 1 {
			t.Fatalf("expected 1 item flushed, got %d", len(seen))
		}
		got := seen[0]
		// 由于发送的是切片且底层共享，被修改后的值应当体现在 flush 看到的内容中
		if got[0] != 99 || got[7] != 77 {
			t.Fatalf("slice underlying not shared as expected: got[0]=%d got[7]=%d", got[0], got[7])
		}
	})

	// 2) 数组作为元素类型：验证通道发送发生值拷贝（不受后续源缓冲修改影响）
	t.Run("array element is value-copied", func(t *testing.T) {
		type Arr8 = [8]byte
		var seen []Arr8
		p := gopipeline.NewStandardPipeline[Arr8](cfg, func(ctx context.Context, batch []Arr8) error {
			// 直接记录 batch（数组元素是值）
			seen = append(seen, batch...)
			return nil
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		go func() { _ = p.AsyncPerform(ctx) }()

		ch := p.DataChan()
		// 源缓冲
		src := make([]byte, 8)
		for i := range src {
			src[i] = byte(i)
		}
		// 构造数组值（拷贝一份当前内容）
		var arr Arr8
		copy(arr[:], src)
		// 发送数组值（通过通道再次值拷贝）
		ch <- arr
		// 发送后修改源缓冲（不应影响已发送的数组值）
		src[0] = 123
		src[7] = 231
		// 触发最终 flush
		close(ch)

		// 等待完成
		select {
		case <-p.Done():
		case <-time.After(500 * time.Millisecond):
		}

		if len(seen) != 1 {
			t.Fatalf("expected 1 item flushed, got %d", len(seen))
		}
		got := seen[0]
		if got[0] != byte(0) || got[7] != byte(7) {
			t.Fatalf("array should be independent copy: got[0]=%d got[7]=%d", got[0], got[7])
		}
	})
}

// 验证：指针传递不会产生重复内存拷贝，但会共享指针指向的数据
func TestPipeline_PointerNoDuplicateMemoryButShared(t *testing.T) {
	type Large struct{ data []byte }

	cfg := gopipeline.NewPipelineConfig().
		WithFlushSize(10).
		WithBufferSize(16).
		WithFlushInterval(50 * time.Millisecond).
		WithFinalFlushOnCloseTimeout(200 * time.Millisecond)

	var seen [][]byte
	p := gopipeline.NewStandardPipeline[*Large](cfg, func(ctx context.Context, batch []*Large) error {
		for _, v := range batch {
			// 观察当前指针对象的内容（做快照）
			cp := append([]byte(nil), v.data...)
			seen = append(seen, cp)
		}
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() { _ = p.AsyncPerform(ctx) }()

	ch := p.DataChan()
	// 创建一个较大的对象（这里用小尺寸以加快测试）
	large := &Large{data: make([]byte, 8)}
	for i := range large.data {
		large.data[i] = byte(i)
	}

	// 发送指针（仅拷贝指针值）
	ch <- large
	// 发送后修改指针指向的数据（若共享则下游会看到修改）
	large.data[0] = 42
	large.data[7] = 24
	// 触发最终 flush
	close(ch)

	// 等待完成
	select {
	case <-p.Done():
	case <-time.After(500 * time.Millisecond):
	}

	if len(seen) != 1 {
		t.Fatalf("expected 1 item flushed, got %d", len(seen))
	}
	got := seen[0]
	// 若指针共享，下游看到的是修改后的值
	if got[0] != 42 || got[7] != 24 {
		t.Fatalf("pointer target should be shared (no duplicate copy): got[0]=%d got[7]=%d", got[0], got[7])
	}
}

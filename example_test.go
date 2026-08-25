package gopipeline_test

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

// ExampleProduct 实现 UniqueKeyData 接口，用于去重管道示例。
type ExampleProduct struct {
	ID  string
	Qty int
}

// GetKey 返回用于去重的唯一键。
func (p ExampleProduct) GetKey() string { return p.ID }

// ExampleNewDefaultStandardPipeline 演示标准管道的批量处理用法：
// 投递 3 条数据后关闭通道，最终 flush 一次性输出整批数据。
func ExampleNewDefaultStandardPipeline() {
	p := gopipeline.NewDefaultStandardPipeline(func(ctx context.Context, batch []int) error {
		sum := 0
		for _, v := range batch {
			sum += v
		}
		fmt.Println("flush sum:", sum)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	go func() {
		ch := p.DataChan()
		defer close(ch)
		for i := 0; i < 3; i++ {
			ch <- i
		}
	}()

	if err := p.SyncPerform(ctx); err != nil {
		log.Fatal(err)
	}
	// Output:
	// flush sum: 3
}

// ExampleNewDeduplicationPipeline 演示去重管道的用法：
// 相同 GetKey 的数据只会保留最后一次投递的值。
func ExampleNewDeduplicationPipeline() {
	p := gopipeline.NewDeduplicationPipeline(
		gopipeline.PipelineConfig{
			BufferSize:    16,
			FlushSize:     8,
			FlushInterval: 50 * time.Millisecond,
		}.ValidateOrDefault(),
		func(ctx context.Context, batch map[string]ExampleProduct) error {
			keys := make([]string, 0, len(batch))
			for k := range batch {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("%s:%d ", k, batch[k].Qty)
			}
			fmt.Println()
			return nil
		})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	go func() {
		ch := p.DataChan()
		defer close(ch)
		ch <- ExampleProduct{ID: "A", Qty: 1}
		ch <- ExampleProduct{ID: "B", Qty: 2}
		ch <- ExampleProduct{ID: "A", Qty: 3} // 与第一条 A 相同 key，覆盖
	}()

	if err := p.SyncPerform(ctx); err != nil {
		log.Fatal(err)
	}
	// Output:
	// A:3 B:2
}

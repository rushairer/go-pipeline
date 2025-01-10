# go-pipeline

该库用来实现攒批处理数据的场景：攒到一定数量（flushSize）或一段时间（flushInterval）后运行处理方法（flushFunc）。

# 用例

```golang
package gopipeline_test

import (
	"context"
	"log"
	"math"
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline"
)

func BenchmarkTestPipeline(b *testing.B) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)

	defer cancel()

	pipeline := gopipeline.NewPipeline[int](
		500000,
		time.Second*2,
		func(ctx context.Context, batchData []int) error {
			log.Println("batchData len:", len(batchData))
			return nil
		})

	go func() {
		if err := pipeline.Run(ctx); err != nil {
			log.Println("pipeline.Run error:", err)
		}
	}()

	for i := 0; i < math.MaxInt64; i++ {
		if err := pipeline.Add(ctx, i); err != nil {
			log.Println(err)
			break
		}

	}
}

```

# Run benchmark

```shell

BenchmarkTestPipeline-10               1        10000032667 ns/op       21094192 B/op        222 allocs/op
PASS
ok      github.com/rushairer/go-pipeline        10.567s

```

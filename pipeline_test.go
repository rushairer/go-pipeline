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

	flushSize := 100000

	pipeline := gopipeline.NewPipeline[[]string](
		gopipeline.PipelineConfig{
			FlushSize:     uint32(flushSize),
			FlushInterval: time.Second * 2,
			BufferSize:    uint32(flushSize),
		},
		func(ctx context.Context, batchData [][]string) error {
			log.Println("batchData len:", len(batchData))
			return nil
		})

	go func() {
		if err := pipeline.Run(ctx); err != nil {
			log.Println("pipeline.Run error:", err)
		}
	}()

	for i := 0; i < math.MaxInt64; i++ {
		if err := pipeline.Add(ctx, make([]string, 100)); err != nil {
			log.Println(err)
			break
		}
	}
}

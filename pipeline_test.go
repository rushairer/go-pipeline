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

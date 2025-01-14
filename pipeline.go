package gopipeline

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

// errors

var (
	ErrContextIsClosed = errors.New("context is canceld")
)

// func

type FlushFunc[T any] func(ctx context.Context, batchData []T) error

// pipeline

const (
	defaultFlushSize     = 100000
	defaultBufferSize    = 100000
	defaultFlushInterval = time.Second * 60
)

type Pipeline[T any] struct {
	config    PipelineConfig
	flushFunc FlushFunc[T]

	dataChan chan T
}

type PipelineConfig struct {
	FlushSize     uint32
	BufferSize    uint32
	FlushInterval time.Duration
}

func NewDefaultPipeline[T any](
	flushFunc FlushFunc[T],
) *Pipeline[T] {
	return &Pipeline[T]{
		config: PipelineConfig{
			FlushSize:     defaultFlushSize,
			BufferSize:    defaultBufferSize,
			FlushInterval: defaultFlushInterval,
		},
		flushFunc: flushFunc,
		dataChan:  make(chan T, defaultBufferSize),
	}
}

func NewPipeline[T any](
	config PipelineConfig,
	flushFunc FlushFunc[T],
) *Pipeline[T] {

	return &Pipeline[T]{
		config:    config,
		flushFunc: flushFunc,
		dataChan:  make(chan T, config.BufferSize),
	}
}

func (p *Pipeline[T]) Add(
	ctx context.Context,
	data T,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Add error: %v", r)
		}
	}()

	select {
	case <-ctx.Done():
		return ErrContextIsClosed
	default:
		p.dataChan <- data
	}

	return
}

func (p *Pipeline[T]) Run(
	ctx context.Context,
) (
	err error,
) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()

	defer close(p.dataChan)

	ticker := time.NewTicker(p.config.FlushInterval)
	defer ticker.Stop()

	batchData := []T{}

	for {
		select {
		case event := <-p.dataChan:
			batchData = append(batchData, event)
			if len(batchData) < int(p.config.FlushSize) {
				continue
			}
			go p.flushFunc(ctx, batchData)

			batchData = batchData[:0]
		case <-ticker.C:
			if len(batchData) < 1 {
				continue
			}
			go p.flushFunc(ctx, batchData)

			batchData = batchData[:0]
		case <-ctx.Done():
			return ErrContextIsClosed
		}
	}
}

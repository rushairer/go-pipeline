package gopipeline

import (
	"context"
	"errors"
	"fmt"
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
	defaultFlushSize     = 10000
	defaultFlushInterval = time.Second * 60
)

type Pipeline[T any] struct {
	flushSize     uint32
	flushInterval time.Duration
	flushFunc     FlushFunc[T]

	dataChan chan T
}

func NewDefaultPipeline[T any](
	flushFunc FlushFunc[T],
) *Pipeline[T] {
	return &Pipeline[T]{
		flushSize:     defaultFlushSize,
		flushInterval: defaultFlushInterval,
		flushFunc:     flushFunc,
		dataChan:      make(chan T),
	}
}

func NewPipeline[T any](
	flushSize uint32,
	flushInterval time.Duration,
	flushFunc FlushFunc[T],
) *Pipeline[T] {
	if flushSize == 0 {
		flushSize = defaultFlushSize
	}

	if flushInterval == 0 {
		flushInterval = defaultFlushInterval
	}

	return &Pipeline[T]{
		flushSize:     flushSize,
		flushInterval: flushInterval,
		flushFunc:     flushFunc,
		dataChan:      make(chan T),
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
	defer close(p.dataChan)

	ticker := time.NewTicker(p.flushInterval)

	batchData := []T{}

	for {
		select {
		case event := <-p.dataChan:
			batchData = append(batchData, event)

			if len(batchData) < int(p.flushSize) {
				continue
			}

			p.flushFunc(ctx, batchData)

			batchData = batchData[:0]
		case <-ticker.C:
			if len(batchData) < 1 {
				continue
			}

			p.flushFunc(ctx, batchData)

			batchData = batchData[:0]

		case <-ctx.Done():
			return ErrContextIsClosed
		}
	}
}

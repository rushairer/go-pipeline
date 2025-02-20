package gopipeline

import (
	"context"
	"errors"
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
	defaultBufferSize    = 200000
	defaultFlushInterval = time.Second * 60
)

// Pipeline 实现了基础管道的具体功能
// 该结构体通过组合BasePipelineImpl来实现通用的管道操作
// 并添加了特定的刷新函数来处理批处理数据
type Pipeline[T any] struct {
	*BasePipelineImpl[T]
	flushFunc FlushFunc[T]
}

// 确保Pipeline实现了DataProcessor接口
var _ DataProcessor[any] = (*Pipeline[any])(nil)

// PipelineConfig 定义了管道的配置参数
type PipelineConfig struct {
	// FlushSize 批处理数据的最大容量
	FlushSize uint32
	// BufferSize 缓冲通道的容量
	BufferSize uint32
	// FlushInterval 定时刷新的时间间隔
	FlushInterval time.Duration
}

// NewDefaultPipeline 使用默认配置创建一个新的管道实例
// 参数:
//   - flushFunc: 用于处理批处理数据的刷新函数
//
// 返回值: 返回一个新的Pipeline实例
func NewDefaultPipeline[T any](
	flushFunc FlushFunc[T],
) *Pipeline[T] {
	config := PipelineConfig{
		FlushSize:     defaultFlushSize,
		BufferSize:    defaultBufferSize,
		FlushInterval: defaultFlushInterval,
	}
	p := &Pipeline[T]{
		flushFunc: flushFunc,
	}
	p.BasePipelineImpl = NewBasePipelineImpl[T](config, p)
	return p
}

// NewPipeline 使用自定义配置创建一个新的管道实例
// 参数:
//   - config: 自定义的管道配置
//   - flushFunc: 用于处理批处理数据的刷新函数
//
// 返回值: 返回一个新的Pipeline实例
func NewPipeline[T any](
	config PipelineConfig,
	flushFunc FlushFunc[T],
) *Pipeline[T] {
	p := &Pipeline[T]{
		flushFunc: flushFunc,
	}
	p.BasePipelineImpl = NewBasePipelineImpl[T](config, p)
	return p
}

// initBatchData 初始化一个新的批处理数据切片
// 返回值: 返回一个空的类型T切片
func (p *Pipeline[T]) initBatchData() any {
	return make([]T, 0)
}

// addToBatch 将新数据添加到批处理数据切片中
// 参数:
//   - batchData: 当前的批处理数据切片
//   - data: 需要添加的新数据
//
// 返回值: 返回更新后的批处理数据切片
func (p *Pipeline[T]) addToBatch(batchData any, data T) any {
	return append(batchData.([]T), data)
}

// flush 使用配置的刷新函数处理批处理数据
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//   - batchData: 需要刷新的批处理数据
//
// 返回值: 如果刷新过程中发生错误则返回error
func (p *Pipeline[T]) flush(ctx context.Context, batchData any) error {
	return p.flushFunc(ctx, batchData.([]T))
}

// isBatchFull 检查批处理数据切片是否已达到配置的最大容量
// 参数:
//   - batchData: 要检查的批处理数据切片
//
// 返回值: 如果数据量达到或超过配置的FlushSize则返回true
func (p *Pipeline[T]) isBatchFull(batchData any) bool {
	return len(batchData.([]T)) >= int(p.config.FlushSize)
}

// isBatchEmpty 检查批处理数据切片是否为空
// 参数:
//   - batchData: 要检查的批处理数据切片
//
// 返回值: 如果数据切片长度小于1则返回true
func (p *Pipeline[T]) isBatchEmpty(batchData any) bool {
	return len(batchData.([]T)) < 1
}

// AsyncPerform 异步执行管道操作
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//
// 返回值: 如果执行过程中发生错误则返回error
func (p *Pipeline[T]) AsyncPerform(ctx context.Context) error {
	return p.performLoop(ctx, true)
}

// SyncPerform 同步执行管道操作
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//
// 返回值: 如果执行过程中发生错误则返回error
func (p *Pipeline[T]) SyncPerform(ctx context.Context) error {
	return p.performLoop(ctx, false)
}

// GetErrorChan 返回用于接收异步执行过程中的错误的通道
// 返回值: 返回一个只读的错误通道
func (p *Pipeline[T]) GetErrorChan() <-chan error {
	return p.BasePipelineImpl.errorChan
}

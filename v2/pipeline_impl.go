package gopipeline

import (
	"context"
	"time"
)

// PipelineImpl 实现了通用的管道功能
// 该结构体提供了管道操作的基础实现，包括数据缓冲、批处理和定时刷新等功能
type PipelineImpl[T any] struct {
	// config 存储管道的配置信息
	config PipelineConfig
	// dataChan 用于数据传输的通道
	dataChan chan T
	// processor 用于处理批量数据的处理器
	processor DataProcessor[T]
	// 错误通道，用于捕获和报告异步执行过程中的错误
	errorChan chan error
}

// 确保 StandardPipeline 实现了 Performer 接口
var _ Performer[any] = (*PipelineImpl[any])(nil)

// 确保 StandardPipeline 实现了 PipelineChannel 接口
var _ PipelineChannel[any] = (*PipelineImpl[any])(nil)

// NewPipelineImpl 创建一个新的基础管道实现实例
// 参数:
//   - config: 管道配置信息
//   - processor: 数据处理器实现
//
// 返回值: 返回一个新的PipelineImpl实例
func NewPipelineImpl[T any](
	config PipelineConfig,
	processor DataProcessor[T],
) *PipelineImpl[T] {
	return &PipelineImpl[T]{
		config:    config,
		dataChan:  make(chan T, config.BufferSize),
		processor: processor,
		errorChan: nil,
	}
}

func (p *PipelineImpl[T]) DataChan() chan<- T {
	return p.dataChan
}

// AsyncPerform 异步执行管道操作
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//
// 返回值: 如果执行过程中发生错误则返回error
func (p *PipelineImpl[T]) AsyncPerform(ctx context.Context) error {
	err := p.performLoop(ctx, true)
	return err
}

// SyncPerform 同步执行管道操作
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//
// 返回值: 如果执行过程中发生错误则返回error
func (p *PipelineImpl[T]) SyncPerform(ctx context.Context) error {
	err := p.performLoop(ctx, false)
	return err
}

// performLoop 实现了通用的执行循环逻辑
// 该方法维护了一个事件循环，负责从通道中读取数据并进行批处理
// 支持同步和异步两种处理模式，并具有定时刷新机制
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//   - async: 是否使用异步模式处理数据
func (p *PipelineImpl[T]) performLoop(
	ctx context.Context,
	async bool,
) error {
	ticker := time.NewTicker(p.config.FlushInterval)
	defer ticker.Stop()

	batchData := p.processor.initBatchData()

	for {
		select {
		case newData, ok := <-p.dataChan:
			if ok {
				batchData = p.processor.addToBatch(batchData, newData)
				if !p.processor.isBatchFull(batchData) {
					continue
				}
				p.doFlush(ctx, async, batchData)
				batchData = p.processor.initBatchData()
			}
		case <-ticker.C:
			if p.processor.isBatchEmpty(batchData) {
				continue
			}
			p.doFlush(ctx, async, batchData)
			batchData = p.processor.initBatchData()
		case <-ctx.Done():
			return ErrContextIsClosed
		}
	}
}

// doFlush 执行数据刷新操作
// 该方法根据异步标志位判断是否异步执行刷新操作
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//   - async: 是否使用异步模式刷新数据
//   - batchData: 待刷新的数据批次
//
// 注意: 该方法会根据async参数判断是否异步执行刷新操作
//
//	如果是异步模式，会在新的goroutine中执行刷新操作，
//	并将刷新结果发送到错误通道中
//	如果是同步模式，会直接执行刷新操作，
//	如果刷新过程中发生错误，会将错误发送到错误通道中
func (p *PipelineImpl[T]) doFlush(
	ctx context.Context,
	async bool,
	batchData any,
) {
	if async {
		go p.flushWithErrorChan(ctx, batchData)
	} else {
		p.flushWithErrorChan(ctx, batchData)
	}
}

// flushWithErrorChan 执行数据刷新操作，并将刷新结果发送到错误通道中
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//   - batchData: 待刷新的数据批次
func (p *PipelineImpl[T]) flushWithErrorChan(ctx context.Context, batchData any) {
	defer func() {
		if r := recover(); r != nil {
		}
	}()

	if err := p.processor.flush(ctx, batchData); err != nil {
		// 安全地发送错误到错误通道
		p.safeErrorSend(err)
	}
}

// safeErrorSend 安全地发送错误到错误通道
// 如果错误通道未初始化（用户未调用ErrorChan），则跳过错误发送
func (p *PipelineImpl[T]) safeErrorSend(err error) {
	if p.errorChan == nil {
		// 用户未调用 ErrorChan() 方法，跳过错误处理
		return
	}

	// 使用非阻塞发送，避免阻塞管道处理
	go func() {
		select {
		case p.errorChan <- err:
			// 错误发送成功
		default:
			// 错误通道已满，跳过此错误
			// 可以在这里添加日志记录
		}
	}()
}

// 让调用方直接监听内部 errorChan
func (p *PipelineImpl[T]) ErrorChan(size int) <-chan error {
	if p.errorChan != nil {
		return p.errorChan
	}
	if size == 0 {
		p.errorChan = make(chan error, int((p.config.FlushSize+p.config.BufferSize-1)/p.config.BufferSize))
	} else {
		p.errorChan = make(chan error, size)
	}
	return p.errorChan
}

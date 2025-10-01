package gopipeline

import (
	"context"
	"log"
	"sync"
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
	// errOnce 确保错误通道只初始化一次（用于 ErrorChan 的懒加载）
	errOnce sync.Once
}

// 确保 PipelineImpl 实现了 Performer 接口
var _ Performer[any] = (*PipelineImpl[any])(nil)

// 确保 PipelineImpl 实现了 PipelineChannel 接口
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
// 运行时产生的错误将通过 ErrorChan 下发（需显式调用 ErrorChan 接收）
// 返回值: 如果执行过程中发生错误则返回error
func (p *PipelineImpl[T]) AsyncPerform(ctx context.Context) error {
	err := p.performLoop(ctx, true)
	return err
}

// SyncPerform 同步执行管道操作
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//
// 运行时产生的错误将通过 ErrorChan 下发（需显式调用 ErrorChan 接收）
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
			log.Println("panic recovered in pipeline: ", r)
		}
	}()

	if err := p.processor.flush(ctx, batchData); err != nil {
		// 安全地发送错误到错误通道
		p.safeErrorSend(err)
	}
}

// 计算默认错误通道缓冲区大小
func (p *PipelineImpl[T]) defaultErrBufSize() int {
	return int((p.config.FlushSize + p.config.BufferSize - 1) / p.config.BufferSize)
}

// safeErrorSend 安全地发送错误到错误通道
// 行为说明：
// 1) 调用 ErrorChan(0) 确保通道已按“首次调用决定缓冲大小”的规则完成一次性初始化
// 2) 使用非阻塞发送；当缓冲区已满时丢弃该错误，避免阻塞处理主循环
// 3) 错误通道的生命周期由管道控制，通常不应在外部关闭
func (p *PipelineImpl[T]) safeErrorSend(err error) {
	if err == nil {
		return
	}
	_ = p.ErrorChan(0) // 确保已初始化，并获取同一实例的快照

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

// ErrorChan 返回一个只读的错误通道，用于接收管道处理过程中的错误
// 线程安全、幂等：通过 sync.Once 懒初始化；“首次调用决定缓冲大小”，后续调用忽略 size
// 参数:
//   - size: 错误通道的缓冲区大小；<=0 时使用默认值（根据 FlushSize 和 BufferSize 自动计算）
//
// 返回值:
//   - <-chan error: 错误只读通道
//
// 用法示例:
//
//	ch := p.ErrorChan(128) // 在启动前显式指定错误缓冲区大小
//	go func() {
//	    for err := range ch {
//	        log.Println("pipeline error:", err)
//	    }
//	}()
//	// 如果不关心自定义容量，可在执行前或读取前调用 p.ErrorChan(0)
//
// 异常处理说明:
//   - 刷新过程中的错误通过 safeErrorSend 非阻塞写入本通道，缓冲满时会丢弃该错误以避免阻塞
//   - 通道由管道内部创建且仅初始化一次，不建议外部关闭；收尾由 context/WaitGroup 协调
func (p *PipelineImpl[T]) ErrorChan(size int) <-chan error {
	p.errOnce.Do(func() {
		n := size
		if n <= 0 {
			n = p.defaultErrBufSize()
		}
		p.errorChan = make(chan error, n)
	})
	return p.errorChan
}

package gopipeline

import (
	"context"
	"fmt"
	"log"
	"time"
)

// DataProcessor 定义了批处理数据的接口
// 该接口定义了一组用于处理批量数据的核心方法，包括初始化、添加、刷新和状态检查
type DataProcessor[T any] interface {
	// initBatchData 初始化一个新的批处理数据容器
	// 返回值: 返回一个可以存储批处理数据的容器实例
	initBatchData() any

	// addToBatch 将新数据添加到批处理数据容器中
	// 参数:
	//   - batchData: 当前的批处理数据容器
	//   - data: 需要添加的新数据
	// 返回值: 返回更新后的批处理数据容器
	addToBatch(batchData any, data T) any

	// flush 将批处理数据刷新到目标存储
	// 参数:
	//   - ctx: 上下文对象，用于控制操作的生命周期
	//   - batchData: 需要刷新的批处理数据
	// 返回值: 如果刷新过程中发生错误则返回error
	flush(ctx context.Context, batchData any) error

	// isBatchFull 检查批处理数据容器是否已满
	// 参数:
	//   - batchData: 要检查的批处理数据容器
	// 返回值: 如果批处理数据已满则返回true
	isBatchFull(batchData any) bool

	// isBatchEmpty 检查批处理数据容器是否为空
	// 参数:
	//   - batchData: 要检查的批处理数据容器
	// 返回值: 如果批处理数据为空则返回true
	isBatchEmpty(batchData any) bool
}

// DataAdder 定义了向管道添加数据的接口
type DataAdder[T any] interface {
	// Add 将数据添加到管道中
	// 参数:
	//   - ctx: 上下文对象，用于控制操作的生命周期
	//   - data: 要添加到管道的数据
	// 返回值: 如果添加过程中发生错误则返回error
	Add(ctx context.Context, data T) error
}

// Performer 定义了执行管道操作的接口
type Performer interface {
	// AsyncPerform 异步执行管道操作
	// 参数:
	//   - ctx: 上下文对象，用于控制操作的生命周期
	// 返回值: 如果执行过程中发生错误则返回error
	AsyncPerform(ctx context.Context) error

	// SyncPerform 同步执行管道操作
	// 参数:
	//   - ctx: 上下文对象，用于控制操作的生命周期
	// 返回值: 如果执行过程中发生错误则返回error
	SyncPerform(ctx context.Context) error
}

// BasePipeline 通过嵌入其他接口定义了所有管道类型的通用接口
// 这个接口组合了数据添加、执行操作和数据处理的能力
type BasePipeline[T any] interface {
	DataAdder[T]
	Performer
	DataProcessor[T]
}

// BasePipelineImpl 实现了通用的管道功能
// 该结构体提供了管道操作的基础实现，包括数据缓冲、批处理和定时刷新等功能
type BasePipelineImpl[T any] struct {
	// config 存储管道的配置信息
	config PipelineConfig
	// dataChan 用于数据传输的通道
	dataChan chan T
	// processor 用于处理批量数据的处理器
	processor DataProcessor[T]
	// 错误通道，用于捕获和报告异步执行过程中的错误
	errorChan chan error
}

// NewBasePipelineImpl 创建一个新的基础管道实现实例
// 参数:
//   - config: 管道配置信息
//   - processor: 数据处理器实现
//
// 返回值: 返回一个新的BasePipelineImpl实例
func NewBasePipelineImpl[T any](
	config PipelineConfig,
	processor DataProcessor[T],
) *BasePipelineImpl[T] {
	return &BasePipelineImpl[T]{
		config:    config,
		dataChan:  make(chan T, config.BufferSize),
		processor: processor,
		errorChan: make(chan error, 1), // 初始化错误通道
	}
}

// Add 实现了所有管道的通用Add方法
// 该方法将数据添加到管道的缓冲通道中，支持并发安全的操作
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//   - data: 要添加到管道的数据
//
// 返回值: 如果添加过程中发生错误则返回error
func (p *BasePipelineImpl[T]) Add(
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

// performLoop 实现了通用的执行循环逻辑
// 该方法维护了一个事件循环，负责从通道中读取数据并进行批处理
// 支持同步和异步两种处理模式，并具有定时刷新机制
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//   - async: 是否使用异步模式处理数据
//
// 返回值: 如果执行过程中发生错误则返回error
func (p *BasePipelineImpl[T]) performLoop(
	ctx context.Context,
	async bool,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()

	defer close(p.dataChan)

	ticker := time.NewTicker(p.config.FlushInterval)
	defer ticker.Stop()

	batchData := p.processor.initBatchData()

	for {
		select {
		case event := <-p.dataChan:
			batchData = p.processor.addToBatch(batchData, event)
			if !p.processor.isBatchFull(batchData) {
				continue
			}
			if async {
				go func(data any) {
					if err := p.processor.flush(ctx, data); err != nil {
						select {
						case p.errorChan <- err:
						default:
							// 如果通道已满，记录日志或其他处理
							log.Printf("Error channel full, dropping error: %v", err)
						}
					}
				}(batchData)
			} else {
				p.processor.flush(ctx, batchData)
			}
			batchData = p.processor.initBatchData()
		case <-ticker.C:
			if p.processor.isBatchEmpty(batchData) {
				continue
			}
			if async {
				go func(data any) {
					if err := p.processor.flush(ctx, data); err != nil {
						select {
						case p.errorChan <- err:
						default:
							// 如果通道已满，记录日志或其他处理
							log.Printf("Error channel full, dropping error: %v", err)
						}
					}
				}(batchData)
			} else {
				p.processor.flush(ctx, batchData)
			}
			batchData = p.processor.initBatchData()
		case <-ctx.Done():
			return ErrContextIsClosed
		}
	}
}

// package go-pipeline 实现了一个通用的数据处理管道框架，支持批量处理和去重功能
package gopipeline

import (
	"context"
)

// models

// MapData 定义了可用于去重处理的数据接口
// 实现此接口的类型必须能够提供一个唯一的键值用于去重判断
type MapData interface {
	// GetKey 返回用于去重的唯一键值
	// 返回值将作为map的键，相同键值的数据将被去重
	GetKey() string
}

// errors

// func

// FlushDeduplicationFunc 定义了批量数据刷新的函数类型
// T 必须实现 MapData 接口
// ctx 用于控制操作的上下文
// batchData 包含已去重的批量数据，键为数据的唯一标识，值为对应的数据对象
// 返回处理过程中的错误，如果没有错误则返回 nil
type FlushDeduplicationFunc[T MapData] func(ctx context.Context, batchData map[string]T) error

// pipeline_deduplication

// PipelineDeduplication 实现了一个支持数据去重的处理管道
// T 必须实现 MapData 接口以提供去重所需的键值
type PipelineDeduplication[T MapData] struct {
	// 继承基础管道实现
	*BasePipelineImpl[T]
	// 用于处理已去重的批量数据的函数
	flushFunc FlushDeduplicationFunc[T]
}

// 通过类型断言确保 PipelineDeduplication 实现了 DataProcessor 接口
var _ DataProcessor[MapData] = (*PipelineDeduplication[MapData])(nil)

// NewDefaultPipelineDeduplication 使用默认配置创建一个新的去重管道实例
// flushFunc 用于处理已去重的批量数据
// 返回配置好的去重管道实例
func NewDefaultPipelineDeduplication[T MapData](
	flushFunc FlushDeduplicationFunc[T],
) *PipelineDeduplication[T] {
	config := PipelineConfig{
		FlushSize:     defaultFlushSize,
		BufferSize:    defaultBufferSize,
		FlushInterval: defaultFlushInterval,
	}
	p := &PipelineDeduplication[T]{
		flushFunc: flushFunc,
	}
	p.BasePipelineImpl = NewBasePipelineImpl[T](config, p)
	return p
}

// NewPipelineDeduplication 使用自定义配置创建一个新的去重管道实例
// config 包含管道的配置参数，如批量大小、缓冲区大小和刷新间隔等
// flushFunc 用于处理已去重的批量数据
// 返回配置好的去重管道实例
func NewPipelineDeduplication[T MapData](
	config PipelineConfig,
	flushFunc FlushDeduplicationFunc[T],
) *PipelineDeduplication[T] {
	p := &PipelineDeduplication[T]{
		flushFunc: flushFunc,
	}
	p.BasePipelineImpl = NewBasePipelineImpl[T](config, p)
	return p
}

// initBatchData 初始化用于存储批量数据的容器
// 返回一个空的map，用于存储去重后的数据
func (p *PipelineDeduplication[T]) initBatchData() any {
	return make(map[string]T)
}

// addToBatch 将新数据添加到批处理容器中
// batchData 当前的批处理数据容器
// data 要添加的新数据
// 返回更新后的批处理数据容器
// 如果新数据的键已存在，则会覆盖原有数据，实现去重效果
func (p *PipelineDeduplication[T]) addToBatch(batchData any, data T) any {
	bd := batchData.(map[string]T)
	bd[data.GetKey()] = data
	return bd
}

// flush 执行批量数据的处理
// ctx 用于控制操作的上下文
// batchData 要处理的批量数据
// 返回处理过程中的错误，如果没有错误则返回 nil
func (p *PipelineDeduplication[T]) flush(ctx context.Context, batchData any) error {
	return p.flushFunc(ctx, batchData.(map[string]T))
}

// isBatchFull 检查批处理数据是否已达到配置的最大容量
// batchData 当前的批处理数据容器
// 返回 true 表示已达到最大容量，需要执行刷新操作
func (p *PipelineDeduplication[T]) isBatchFull(batchData any) bool {
	return len(batchData.(map[string]T)) >= int(p.config.FlushSize)
}

// isBatchEmpty 检查批处理数据是否为空
// batchData 当前的批处理数据容器
// 返回 true 表示批处理数据为空
func (p *PipelineDeduplication[T]) isBatchEmpty(batchData any) bool {
	return len(batchData.(map[string]T)) < 1
}

// AsyncPerform 异步执行数据处理
// ctx 用于控制操作的上下文
// 返回处理过程中的错误，如果没有错误则返回 nil
func (p *PipelineDeduplication[T]) AsyncPerform(ctx context.Context) error {
	return p.performLoop(ctx, true)
}

// SyncPerform 同步执行数据处理
// ctx 用于控制操作的上下文
// 返回处理过程中的错误，如果没有错误则返回 nil
func (p *PipelineDeduplication[T]) SyncPerform(ctx context.Context) error {
	return p.performLoop(ctx, false)
}

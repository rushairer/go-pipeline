package gopipeline

import "context"

type FlushStandardFunc[T any] func(ctx context.Context, batchData []T) error

// StandardPipeline 实现了基础管道的具体功能
// 该结构体通过组合 PipelineImpl 来实现通用的管道操作
// 并添加了特定的刷新函数来处理批处理数据
type StandardPipeline[T any] struct {
	*PipelineImpl[T]
	flushFunc FlushStandardFunc[T]
}

// 确保 StandardPipeline 实现了 DataProcessor 接口
var _ DataProcessor[any] = (*StandardPipeline[any])(nil)

// NewDefaultStandardPipeline 使用默认配置创建一个新的管道实例
// 参数:
//   - flushFunc: 用于处理批处理数据的刷新函数
//
// 返回值: 返回一个新的 StandardPipeline 实例
func NewDefaultStandardPipeline[T any](
	flushFunc FlushStandardFunc[T],
) *StandardPipeline[T] {
	config := PipelineConfig{
		FlushSize:     defaultFlushSize,
		BufferSize:    defaultBufferSize,
		FlushInterval: defaultFlushInterval,
	}
	p := &StandardPipeline[T]{
		flushFunc: flushFunc,
	}
	p.PipelineImpl = NewPipelineImpl[T](config, p)
	return p
}

// NewStandardPipeline 使用自定义配置创建一个新的管道实例
// 参数:
//   - config: 自定义的管道配置
//   - flushFunc: 用于处理批处理数据的刷新函数
//
// 返回值: 返回一个新的 StandardPipeline 实例
func NewStandardPipeline[T any](
	config PipelineConfig,
	flushFunc FlushStandardFunc[T],
) *StandardPipeline[T] {
	p := &StandardPipeline[T]{
		flushFunc: flushFunc,
	}
	p.PipelineImpl = NewPipelineImpl[T](config, p)
	return p
}

// initBatchData 初始化一个新的批处理数据切片
// 返回值: 返回一个空的类型T切片
func (p *StandardPipeline[T]) initBatchData() any {
	return make([]T, 0)
}

// addToBatch 将新数据添加到批处理数据切片中
// 参数:
//   - batchData: 当前的批处理数据切片
//   - data: 需要添加的新数据
//
// 返回值: 返回更新后的批处理数据切片
func (p *StandardPipeline[T]) addToBatch(batchData any, data T) any {
	return append(batchData.([]T), data)
}

// flush 使用配置的刷新函数处理批处理数据
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//   - batchData: 需要刷新的批处理数据
//
// 返回值: 如果刷新过程中发生错误则返回error
func (p *StandardPipeline[T]) flush(ctx context.Context, batchData any) error {
	return p.flushFunc(ctx, batchData.([]T))
}

// isBatchFull 检查批处理数据切片是否已达到配置的最大容量
// 参数:
//   - batchData: 要检查的批处理数据切片
//
// 返回值: 如果数据量达到或超过配置的FlushSize则返回true
func (p *StandardPipeline[T]) isBatchFull(batchData any) bool {
	return len(batchData.([]T)) >= int(p.config.FlushSize)
}

// isBatchEmpty 检查批处理数据切片是否为空
// 参数:
//   - batchData: 要检查的批处理数据切片
//
// 返回值: 如果数据切片长度小于1则返回true
func (p *StandardPipeline[T]) isBatchEmpty(batchData any) bool {
	return len(batchData.([]T)) < 1
}

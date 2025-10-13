package gopipeline

import "context"

// UniqueKeyData 定义了可用于去重处理的数据接口
// 实现此接口的类型必须能够提供一个唯一的键值用于去重判断
type UniqueKeyData interface {
	// GetKey 返回用于去重的唯一键值
	// 返回值将作为map的键，相同键值的数据将被去重
	GetKey() string
}

type FlushDeduplicationFunc[T UniqueKeyData] func(ctx context.Context, batchData map[string]T) error

// DeduplicationPipeline 实现了基础管道的具体功能
// 该结构体通过组合 PipelineImpl 来实现通用的管道操作
// 并添加了特定的刷新函数来处理批处理数据
// T 必须实现 UniqueKeyData 接口
type DeduplicationPipeline[T UniqueKeyData] struct {
	*PipelineImpl[T]
	flushFunc FlushDeduplicationFunc[T]
}

// 确保 DeduplicationPipeline 实现了 DataProcessor 接口
var _ DataProcessor[UniqueKeyData] = (*DeduplicationPipeline[UniqueKeyData])(nil)

// NewDefaultDeduplicationPipeline 使用默认配置创建一个新的管道实例
// 参数:
//   - flushFunc: 用于处理批处理数据的刷新函数
//
// 返回值: 返回一个新的 DeduplicationPipeline 实例
func NewDefaultDeduplicationPipeline[T UniqueKeyData](
	flushFunc FlushDeduplicationFunc[T],
) *DeduplicationPipeline[T] {
	config := PipelineConfig{
		FlushSize:     defaultFlushSize,
		BufferSize:    defaultBufferSize,
		FlushInterval: defaultFlushInterval,
	}
	p := &DeduplicationPipeline[T]{
		flushFunc: flushFunc,
	}
	p.PipelineImpl = NewPipelineImpl[T](config, p)
	return p
}

// NewDeduplicationPipeline 使用自定义配置创建一个新的管道实例
// 参数:
//   - config: 自定义的管道配置
//   - flushFunc: 用于处理批处理数据的刷新函数
//
// 返回值: 返回一个新的 DeduplicationPipeline 实例
func NewDeduplicationPipeline[T UniqueKeyData](
	config PipelineConfig,
	flushFunc FlushDeduplicationFunc[T],
) *DeduplicationPipeline[T] {
	p := &DeduplicationPipeline[T]{
		flushFunc: flushFunc,
	}
	p.PipelineImpl = NewPipelineImpl[T](config, p)
	return p
}

// initBatchData 初始化一个新的批处理数据切片
// 返回值: 返回一个空的类型T切片
func (p *DeduplicationPipeline[T]) initBatchData() any {
	// 预分配容量，减少哈希表扩容/rehash（读取当前可调的 FlushSize）
	return make(map[string]T, int(p.CurrentFlushSize()))
}

// addToBatch 将新数据添加到批处理容器中
// 参数:
//   - batchData: 当前的批处理数据容器
//   - data: 需要添加的新数据
//
// 返回值: 返回更新后的批处理数据容器
// 说明:
//   - 该方法将新数据添加到批处理容器中，键为数据的唯一标识，值为对应的数据对象
//   - 如果新数据的键已存在，则会覆盖原有数据，实现去重效果
//   - 注意：该方法在单消费者事件循环内是安全的；并非可在多协程并发写 map 的线程安全结构
func (p *DeduplicationPipeline[T]) addToBatch(batchData any, data T) any {
	bd := batchData.(map[string]T)
	bd[data.GetKey()] = data
	return bd
}

// flush 使用配置的刷新函数处理批处理数据
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//   - batchData: 需要刷新的批处理数据
//
// 返回值: 如果刷新过程中发生错误则返回error
func (p *DeduplicationPipeline[T]) flush(ctx context.Context, batchData any) error {
	return p.flushFunc(ctx, batchData.(map[string]T))
}

// isBatchFull 检查批处理数据切片是否已达到配置的最大容量
// 参数:
//   - batchData: 要检查的批处理数据切片
//
// 返回值: 如果数据量达到或超过配置的FlushSize则返回true
func (p *DeduplicationPipeline[T]) isBatchFull(batchData any) bool {
	return len(batchData.(map[string]T)) >= int(p.CurrentFlushSize())
}

// isBatchEmpty 检查批处理数据切片是否为空
// 参数:
//   - batchData: 要检查的批处理数据切片
//
// 返回值: 如果数据切片长度小于1则返回true
func (p *DeduplicationPipeline[T]) isBatchEmpty(batchData any) bool {
	return len(batchData.(map[string]T)) < 1
}

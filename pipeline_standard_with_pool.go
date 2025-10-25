package gopipeline

import (
	"context"
	"runtime"
	"sync"
	"unsafe"
)

// AutoRecycleBatch 支持自动回收的批次容器
type AutoRecycleBatch[T any] struct {
	Data []T
	pool *sync.Pool
	id   uintptr // 唯一标识
}

// autoRecycle 自动回收方法，由 runtime.SetFinalizer 调用
func (arb *AutoRecycleBatch[T]) autoRecycle() {
	// 从活跃追踪中移除
	activeBatches.Delete(arb.id)
	
	// 清理终结器
	runtime.SetFinalizer(arb, nil)
	
	// 归还到池中
	if arb.pool != nil {
		arb.Data = arb.Data[:0] // 重置长度，保留容量
		arb.pool.Put(arb)
	}
}

// 全局追踪正在使用的批次，防止过早回收
var activeBatches = sync.Map{} // map[uintptr]*AutoRecycleBatch

// StandardPipelineWithPool 实现了带对象池优化的标准管道
// 使用弱引用 + 终结器实现自动内存回收和复用
type StandardPipelineWithPool[T any] struct {
	*PipelineImpl[T]
	flushFunc FlushStandardFunc[T]
	batchPool sync.Pool // 批次对象池
}

// 确保 StandardPipelineWithPool 实现了 DataProcessor 接口
var _ DataProcessor[any] = (*StandardPipelineWithPool[any])(nil)

// NewDefaultStandardPipelineWithPool 使用默认配置创建一个新的带对象池的管道实例
// 参数:
//   - flushFunc: 用于处理批处理数据的刷新函数
//
// 返回值: 返回一个新的 StandardPipelineWithPool 实例
func NewDefaultStandardPipelineWithPool[T any](
	flushFunc FlushStandardFunc[T],
) *StandardPipelineWithPool[T] {
	config := PipelineConfig{
		FlushSize:     defaultFlushSize,
		BufferSize:    defaultBufferSize,
		FlushInterval: defaultFlushInterval,
	}
	p := &StandardPipelineWithPool[T]{
		flushFunc: flushFunc,
	}
	p.PipelineImpl = NewPipelineImpl[T](config, p)
	return p
}

// NewStandardPipelineWithPool 使用自定义配置创建一个新的带对象池的管道实例
// 参数:
//   - config: 自定义的管道配置
//   - flushFunc: 用于处理批处理数据的刷新函数
//
// 返回值: 返回一个新的 StandardPipelineWithPool 实例
func NewStandardPipelineWithPool[T any](
	config PipelineConfig,
	flushFunc FlushStandardFunc[T],
) *StandardPipelineWithPool[T] {
	p := &StandardPipelineWithPool[T]{
		flushFunc: flushFunc,
	}
	p.PipelineImpl = NewPipelineImpl[T](config, p)
	return p
}

// initBatchData 初始化一个新的批处理数据切片，优先从对象池获取
// 使用弱引用 + 终结器实现自动回收
// 返回值: 返回一个空的类型T切片
func (p *StandardPipelineWithPool[T]) initBatchData() any {
	var batch *AutoRecycleBatch[T]
	
	// 尝试从池中获取已有的批次容器
	if pooled := p.batchPool.Get(); pooled != nil {
		batch = pooled.(*AutoRecycleBatch[T])
		batch.Data = batch.Data[:0] // 重置长度，保留容量
	} else {
		// 创建新的批次容器
		size := p.CurrentFlushSize()
		batch = &AutoRecycleBatch[T]{
			Data: make([]T, 0, size),
			pool: &p.batchPool,
		}
	}
	
	// 生成唯一ID并注册到活跃批次追踪
	batch.id = uintptr(unsafe.Pointer(batch))
	activeBatches.Store(batch.id, batch)
	
	// 设置终结器：当没有任何引用时自动回收
	runtime.SetFinalizer(batch, (*AutoRecycleBatch[T]).autoRecycle)
	
	return batch.Data // 返回切片供正常使用
}

// addToBatch 将新数据添加到批处理数据切片中
// 参数:
//   - batchData: 当前的批处理数据切片
//   - data: 需要添加的新数据
//
// 返回值: 返回更新后的批处理数据切片
func (p *StandardPipelineWithPool[T]) addToBatch(batchData any, data T) any {
	return append(batchData.([]T), data)
}

// flush 使用配置的刷新函数处理批处理数据
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//   - batchData: 需要刷新的批处理数据
//
// 返回值: 如果刷新过程中发生错误则返回error
func (p *StandardPipelineWithPool[T]) flush(ctx context.Context, batchData any) error {
	return p.flushFunc(ctx, batchData.([]T))
}

// isBatchFull 检查批处理数据切片是否已达到配置的最大容量
// 参数:
//   - batchData: 要检查的批处理数据切片
//
// 返回值: 如果数据量达到或超过配置的FlushSize则返回true
func (p *StandardPipelineWithPool[T]) isBatchFull(batchData any) bool {
	return len(batchData.([]T)) >= int(p.CurrentFlushSize())
}

// isBatchEmpty 检查批处理数据切片是否为空
// 参数:
//   - batchData: 要检查的批处理数据切片
//
// 返回值: 如果数据切片长度小于1则返回true
func (p *StandardPipelineWithPool[T]) isBatchEmpty(batchData any) bool {
	return len(batchData.([]T)) < 1
}

// GetPoolStats 获取对象池的统计信息（用于性能分析）
func (p *StandardPipelineWithPool[T]) GetPoolStats() (activeCount int, poolSize int) {
	// 统计活跃批次数量
	activeBatches.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})
	
	// 尝试估算池中对象数量（通过临时获取和放回）
	var poolObjects []*AutoRecycleBatch[T]
	for i := 0; i < 100; i++ { // 最多检查100个
		if obj := p.batchPool.Get(); obj != nil {
			poolObjects = append(poolObjects, obj.(*AutoRecycleBatch[T]))
		} else {
			break
		}
	}
	poolSize = len(poolObjects)
	
	// 放回所有临时获取的对象
	for _, obj := range poolObjects {
		p.batchPool.Put(obj)
	}
	
	return activeCount, poolSize
}

// ForceGC 强制触发垃圾回收（用于测试终结器效果）
func (p *StandardPipelineWithPool[T]) ForceGC() {
	runtime.GC()
	runtime.GC() // 调用两次确保终结器运行
}
package gopipeline

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
)

// 优化版本1: 简化对象池，去掉终结器
type StandardPipelineOptimized1[T any] struct {
	*PipelineImpl[T]
	flushFunc FlushStandardFunc[T]
	batchPool sync.Pool // 简化的批次对象池
}

var _ DataProcessor[any] = (*StandardPipelineOptimized1[any])(nil)

func NewDefaultStandardPipelineOptimized1[T any](
	flushFunc FlushStandardFunc[T],
) *StandardPipelineOptimized1[T] {
	config := PipelineConfig{
		FlushSize:     defaultFlushSize,
		BufferSize:    defaultBufferSize,
		FlushInterval: defaultFlushInterval,
	}
	p := &StandardPipelineOptimized1[T]{
		flushFunc: flushFunc,
	}
	p.PipelineImpl = NewPipelineImpl[T](config, p)
	return p
}

func NewStandardPipelineOptimized1[T any](
	config PipelineConfig,
	flushFunc FlushStandardFunc[T],
) *StandardPipelineOptimized1[T] {
	p := &StandardPipelineOptimized1[T]{
		flushFunc: flushFunc,
	}
	p.PipelineImpl = NewPipelineImpl[T](config, p)
	return p
}

// 简化的 initBatchData，直接使用 sync.Pool
func (p *StandardPipelineOptimized1[T]) initBatchData() any {
	if pooled := p.batchPool.Get(); pooled != nil {
		slice := pooled.([]T)
		return slice[:0] // 重置长度，保留容量
	}
	
	// 创建新切片
	size := p.CurrentFlushSize()
	return make([]T, 0, size)
}

func (p *StandardPipelineOptimized1[T]) addToBatch(batchData any, data T) any {
	return append(batchData.([]T), data)
}

func (p *StandardPipelineOptimized1[T]) flush(ctx context.Context, batchData any) error {
	slice := batchData.([]T)
	
	// 在异步模式下，复制数据以避免竞争
	if p.IsAsyncMode() {
		// 创建副本
		copied := make([]T, len(slice))
		copy(copied, slice)
		
		// 归还原始切片到池中
		p.batchPool.Put(slice[:0])
		
		return p.flushFunc(ctx, copied)
	}
	
	// 同步模式，直接使用并在完成后归还
	err := p.flushFunc(ctx, slice)
	p.batchPool.Put(slice[:0])
	return err
}

func (p *StandardPipelineOptimized1[T]) isBatchFull(batchData any) bool {
	return len(batchData.([]T)) >= int(p.CurrentFlushSize())
}

func (p *StandardPipelineOptimized1[T]) isBatchEmpty(batchData any) bool {
	return len(batchData.([]T)) < 1
}

// 检查是否为异步模式的辅助方法
func (p *StandardPipelineOptimized1[T]) IsAsyncMode() bool {
	// 这里需要根据实际的异步检测逻辑来实现
	// 暂时返回 true，表示总是使用安全的复制策略
	return true
}

// 优化版本2: 使用原子计数器替代 sync.Map
type StandardPipelineOptimized2[T any] struct {
	*PipelineImpl[T]
	flushFunc    FlushStandardFunc[T]
	batchPool    sync.Pool
	activeCount  int64 // 原子计数器
}

var _ DataProcessor[any] = (*StandardPipelineOptimized2[any])(nil)

func NewDefaultStandardPipelineOptimized2[T any](
	flushFunc FlushStandardFunc[T],
) *StandardPipelineOptimized2[T] {
	config := PipelineConfig{
		FlushSize:     defaultFlushSize,
		BufferSize:    defaultBufferSize,
		FlushInterval: defaultFlushInterval,
	}
	p := &StandardPipelineOptimized2[T]{
		flushFunc: flushFunc,
	}
	p.PipelineImpl = NewPipelineImpl[T](config, p)
	return p
}

func NewStandardPipelineOptimized2[T any](
	config PipelineConfig,
	flushFunc FlushStandardFunc[T],
) *StandardPipelineOptimized2[T] {
	p := &StandardPipelineOptimized2[T]{
		flushFunc: flushFunc,
	}
	p.PipelineImpl = NewPipelineImpl[T](config, p)
	return p
}

// 轻量级批次包装器
type LightBatch[T any] struct {
	Data []T
	pool *sync.Pool
	counter *int64
}

func (lb *LightBatch[T]) recycle() {
	if lb.pool != nil && lb.counter != nil {
		atomic.AddInt64(lb.counter, -1)
		lb.Data = lb.Data[:0]
		lb.pool.Put(lb.Data)
	}
}

func (p *StandardPipelineOptimized2[T]) initBatchData() any {
	atomic.AddInt64(&p.activeCount, 1)
	
	if pooled := p.batchPool.Get(); pooled != nil {
		slice := pooled.([]T)
		return slice[:0]
	}
	
	size := p.CurrentFlushSize()
	return make([]T, 0, size)
}

func (p *StandardPipelineOptimized2[T]) addToBatch(batchData any, data T) any {
	return append(batchData.([]T), data)
}

func (p *StandardPipelineOptimized2[T]) flush(ctx context.Context, batchData any) error {
	slice := batchData.([]T)
	
	// 创建轻量级包装器
	batch := &LightBatch[T]{
		Data: slice,
		pool: &p.batchPool,
		counter: &p.activeCount,
	}
	
	// 设置终结器（更轻量级）
	runtime.SetFinalizer(batch, (*LightBatch[T]).recycle)
	
	return p.flushFunc(ctx, slice)
}

func (p *StandardPipelineOptimized2[T]) isBatchFull(batchData any) bool {
	return len(batchData.([]T)) >= int(p.CurrentFlushSize())
}

func (p *StandardPipelineOptimized2[T]) isBatchEmpty(batchData any) bool {
	return len(batchData.([]T)) < 1
}

func (p *StandardPipelineOptimized2[T]) GetActiveCount() int64 {
	return atomic.LoadInt64(&p.activeCount)
}

// 优化版本3: 预分配池，完全避免终结器
type StandardPipelineOptimized3[T any] struct {
	*PipelineImpl[T]
	flushFunc FlushStandardFunc[T]
	batchPool sync.Pool
	copyPool  sync.Pool // 专门用于异步复制的池
}

var _ DataProcessor[any] = (*StandardPipelineOptimized3[any])(nil)

func NewDefaultStandardPipelineOptimized3[T any](
	flushFunc FlushStandardFunc[T],
) *StandardPipelineOptimized3[T] {
	config := PipelineConfig{
		FlushSize:     defaultFlushSize,
		BufferSize:    defaultBufferSize,
		FlushInterval: defaultFlushInterval,
	}
	p := &StandardPipelineOptimized3[T]{
		flushFunc: flushFunc,
	}
	
	// 预热对象池
	p.batchPool.New = func() interface{} {
		return make([]T, 0, config.FlushSize)
	}
	p.copyPool.New = func() interface{} {
		return make([]T, 0, config.FlushSize)
	}
	
	p.PipelineImpl = NewPipelineImpl[T](config, p)
	return p
}

func NewStandardPipelineOptimized3[T any](
	config PipelineConfig,
	flushFunc FlushStandardFunc[T],
) *StandardPipelineOptimized3[T] {
	p := &StandardPipelineOptimized3[T]{
		flushFunc: flushFunc,
	}
	
	// 预热对象池
	p.batchPool.New = func() interface{} {
		return make([]T, 0, config.FlushSize)
	}
	p.copyPool.New = func() interface{} {
		return make([]T, 0, config.FlushSize)
	}
	
	p.PipelineImpl = NewPipelineImpl[T](config, p)
	return p
}

func (p *StandardPipelineOptimized3[T]) initBatchData() any {
	slice := p.batchPool.Get().([]T)
	return slice[:0]
}

func (p *StandardPipelineOptimized3[T]) addToBatch(batchData any, data T) any {
	return append(batchData.([]T), data)
}

func (p *StandardPipelineOptimized3[T]) flush(ctx context.Context, batchData any) error {
	slice := batchData.([]T)
	
	// 获取复制用的切片
	copySlice := p.copyPool.Get().([]T)
	
	// 确保容量足够
	if cap(copySlice) < len(slice) {
		copySlice = make([]T, len(slice))
	} else {
		copySlice = copySlice[:len(slice)]
	}
	
	// 复制数据
	copy(copySlice, slice)
	
	// 立即归还原始切片
	p.batchPool.Put(slice[:0])
	
	// 处理复制的数据
	err := p.flushFunc(ctx, copySlice)
	
	// 归还复制切片
	p.copyPool.Put(copySlice[:0])
	
	return err
}

func (p *StandardPipelineOptimized3[T]) isBatchFull(batchData any) bool {
	return len(batchData.([]T)) >= int(p.CurrentFlushSize())
}

func (p *StandardPipelineOptimized3[T]) isBatchEmpty(batchData any) bool {
	return len(batchData.([]T)) < 1
}
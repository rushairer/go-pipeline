package gopipeline

import "context"

// DataProcessor 定义了批处理数据的接口。
//
// DataProcessor 只负责批次构建与批次级 flush。flush 返回的 error 表示该批次处理
// 发生错误；如果业务需要逐条失败明细、持久化失败记录、重试队列或 exactly-once
// 语义，应由 processor 或更上层系统负责，而不是依赖 Pipeline 的错误观测通道。
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

	// flush 将批处理数据刷新到目标存储。
	//
	// 返回的 error 是 batch/flush 级错误。Pipeline 会将其交给错误观测机制，
	// 但不会把 ErrorChan 当作可靠的失败数据存储。
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

// PipelineChannel 定义了管道的通道接口。
type PipelineChannel[T any] interface {

	// DataChan 返回一个可写的通道，用于将数据添加到管道中。
	//
	// 输入通道和 MaxConcurrentFlushes 共同构成背压边界；当下游处理能力不足时，
	// 生产者可能被有意阻塞，以避免无限制堆积。
	DataChan() chan<- T

	// ErrorChan 返回一个只读的错误观测通道。
	//
	// 标准实现采用 non-blocking / best-effort 语义：错误通道缓冲区已满时，
	// 新错误可以被丢弃，从而避免错误消费者反向阻塞高吞吐数据处理主链。
	// 因此 ErrorChan 适合日志、指标与告警，不应作为 durable failure queue 或
	// 完整失败清单的唯一来源。需要完整失败记录时，应由 processor 或上层系统持久化。
	//
	// 参数:
	//   - size: 错误通道缓冲区容量；首次调用决定容量，<=0 时由实现选择默认值
	// 返回值: 返回一个只读错误通道
	ErrorChan(size int) <-chan error
}

// Performer 定义了执行管道操作的接口。
type Performer[T any] interface {
	// AsyncPerform 运行采用并发 flush 的批处理循环。
	//
	// 注意：AsyncPerform 本身不会自动把调用放入新的 goroutine；它会占用调用它的
	// goroutine，直到当前 run loop 结束。这里的 "Async" 指批次 flush 可以并发执行，
	// 从而让一个批次的处理不必串行等待前一个批次完成。
	//
	// 需要非阻塞启动时，优先使用具体 Pipeline 类型提供的 Start(ctx) 辅助方法。
	// flush 过程中的业务错误通过 ErrorChan 进行 best-effort 观测；本方法主要返回
	// run loop 生命周期错误，例如 context 取消或并发重复启动。
	// 参数:
	//   - ctx: 上下文对象，用于控制操作的生命周期
	// 返回值: run loop 生命周期错误
	AsyncPerform(ctx context.Context) error

	// SyncPerform 同步执行管道操作。
	//
	// 与 AsyncPerform 不同，SyncPerform 的 batch flush 在当前执行路径串行完成。
	// 参数:
	//   - ctx: 上下文对象，用于控制操作的生命周期
	// 返回值: run loop 生命周期错误
	SyncPerform(ctx context.Context) error
}

// Pipeline 通过嵌入其他接口定义了所有管道类型的通用接口。
// 这个接口组合了数据添加、执行操作和数据处理的能力。
type Pipeline[T any] interface {
	PipelineChannel[T]
	Performer[T]
	DataProcessor[T]
}

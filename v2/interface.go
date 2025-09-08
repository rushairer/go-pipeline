package gopipeline

import "context"

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
type Performer[T any] interface {
	// AsyncPerform 异步执行管道操作
	// 参数:
	//   - ctx: 上下文对象，用于控制操作的生命周期
	// 返回值: 如果ctx被取消则返回error
	AsyncPerform(ctx context.Context) error

	// SyncPerform 同步执行管道操作
	// 参数:
	//   - ctx: 上下文对象，用于控制操作的生命周期
	// 返回值: 如果ctx被取消则返回error
	SyncPerform(ctx context.Context) error
}

// Pipeline 通过嵌入其他接口定义了所有管道类型的通用接口
// 这个接口组合了数据添加、执行操作和数据处理的能力
type Pipeline[T any] interface {
	DataAdder[T]
	Performer[T]
	DataProcessor[T]

	Close(context.Context)
	ErrorChan() <-chan error
}

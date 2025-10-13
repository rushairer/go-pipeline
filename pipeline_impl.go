package gopipeline

import (
	"context"
	"errors"
	"log"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// PipelineImpl 实现了通用的管道功能
// 该结构体提供了管道操作的基础实现，包括数据缓冲、批处理和定时刷新等功能
type MetricsHook interface {
	// Flush 在一次 flush 完成后被调用
	// items: 本次批次大小；duration: 执行耗时
	Flush(items int, duration time.Duration)
	// Error 在错误成功写入错误通道时调用
	Error(err error)
	// ErrorDropped 在错误通道满导致错误被丢弃时调用
	ErrorDropped()
}

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

	// 运行状态与并发控制
	running  int32         // 0=未运行, 1=运行中（并发启动保护）
	flushSem chan struct{} // 异步 flush 并发上限（nil 表示不限制）

	// 动态可调参数（运行时）
	currFlushSize     atomic.Uint32 // 当前 FlushSize
	currFlushInterval atomic.Int64  // 当前 FlushInterval（ns）
	nudge             chan struct{} // 轻推信号：用于立即重置计时器

	// 可选注入：日志与指标
	logger  *log.Logger
	metrics MetricsHook

	// 最近一次运行的完成信号（Done）
	runMu   sync.Mutex
	runDone chan struct{}
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
	// 规范化配置（与 ValidateOrDefault 一致）
	config = config.ValidateOrDefault()
	p := &PipelineImpl[T]{
		config:    config,
		dataChan:  make(chan T, config.BufferSize),
		processor: processor,
		errorChan: nil,
		nudge:     make(chan struct{}, 1),
	}
	// 初始化动态参数
	p.currFlushSize.Store(config.FlushSize)
	p.currFlushInterval.Store(int64(config.FlushInterval))

	// 初始化固定容量的并发信号量（0 表示不限制）
	if config.MaxConcurrentFlushes > 0 {
		p.flushSem = make(chan struct{}, int(config.MaxConcurrentFlushes))
	}

	return p
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

/*
performLoop 实现了通用的执行循环逻辑
- 异步 vs 同步的批容器语义：
  - AsyncPerform（并发 flush）：在将当前批次交给 doFlush 运行的 goroutine 后，应“分离”当前容器并为后续累计创建新容器（initBatchData 或等价策略）。
    这样可避免与异步 goroutine 共享同一底层存储（slice/map），否则使用 ResetBatchData（如 slice[:0]）清空时会与 flush 重叠，导致数据错乱或丢失。
  - SyncPerform（串行 flush）：flush 在当前 goroutine 完成，使用 ResetBatchData 复用容器是安全的。
  - 实践建议：异步路径优先采用“偷换容器（steal-and-replace）”，可结合对象池降低分配；同步路径可用 ResetBatchData 以减少分配。
*/
// 参数:
//   - ctx: 上下文对象，用于控制操作的生命周期
//   - async: 是否使用异步模式处理数据
func (p *PipelineImpl[T]) performLoop(
	ctx context.Context,
	async bool,
) error {
	// 防并发启动：同一实例不允许并发运行
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		return ErrAlreadyRunning
	}
	// 设置本次运行的 Done 通道（捕获本次专属通道）
	p.runMu.Lock()
	myDone := p.runDone
	if myDone == nil {
		myDone = make(chan struct{})
		p.runDone = myDone
	}
	p.runMu.Unlock()
	defer func() {
		// 运行结束：恢复运行状态并发出完成信号
		atomic.StoreInt32(&p.running, 0)

		p.runMu.Lock()
		// 仅关闭本次运行捕获的通道，避免重复关闭历史通道
		close(myDone)
		if p.runDone == myDone {
			p.runDone = nil
		}
		p.runMu.Unlock()
	}()

	// 使用可重置的 timer，使 FlushInterval 的动态更新在下一次触发时生效
	timer := time.NewTimer(p.CurrentFlushInterval())
	defer timer.Stop()

	batchData := p.processor.initBatchData()

	for {
		select {
		case newData, ok := <-p.dataChan:
			if !ok {
				// 数据通道已关闭：最终刷新未满批次后退出
				if !p.processor.isBatchEmpty(batchData) {
					// 使用 FinalFlushOnCloseTimeout 限时最终 flush（0 表示不限时，保持 Background）
					ctxClose := context.Background()
					if p.config.FinalFlushOnCloseTimeout > 0 {
						var cancel context.CancelFunc
						ctxClose, cancel = context.WithTimeout(context.Background(), p.config.FinalFlushOnCloseTimeout)
						p.doFlush(ctxClose, false, batchData)
						cancel()
					} else {
						p.doFlush(ctxClose, false, batchData)
					}
				}
				return nil
			}
			batchData = p.processor.addToBatch(batchData, newData)
			if !p.processor.isBatchFull(batchData) {
				continue
			}
			p.doFlush(ctx, async, batchData)
			batchData = p.processor.initBatchData()
		case <-timer.C:
			// 定时触发：空批则跳过，但仍需重置定时器
			if !p.processor.isBatchEmpty(batchData) {
				p.doFlush(ctx, async, batchData)
				batchData = p.processor.initBatchData()
			}
			// 重置下一次触发时间，读取当前可调的 FlushInterval
			next := p.CurrentFlushInterval()
			if next <= 0 {
				next = time.Millisecond * 50
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(next)
		case <-p.nudge:
			// 轻推：仅重置计时器到当前 FlushInterval，不触发 flush
			next := p.CurrentFlushInterval()
			if next <= 0 {
				next = time.Millisecond * 50
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(next)
		case <-ctx.Done():
			// 取消退出语义：
			// - DrainOnCancel=false：不做最终 flush，返回 ErrContextIsClosed（可用 errors.Is(err, ErrContextIsClosed) 判断）
			// - DrainOnCancel=true：尽力将当前通道缓冲中的数据吸入批并在独立 drainCtx 下同步 flush；
			//   返回 errors.Join(ErrContextIsClosed, ErrContextDrained)：
			//     * errors.Is(err, ErrContextIsClosed) == true → 因取消退出
			//     * errors.Is(err, ErrContextDrained)  == true → 已执行限时收尾
			if p.config.DrainOnCancel {
				// 1) 独立的收尾上下文，避免被原 ctx 立即打断
				grace := p.config.DrainGracePeriod
				if grace <= 0 {
					grace = 100 * time.Millisecond
				}
				drainCtx, cancel := context.WithTimeout(context.Background(), grace)

				// 2) 非阻塞地抽干当前通道缓冲中的数据，尽量纳入批（避免阻塞/无限等待）
				// 注意：仅在取消瞬间把“已缓冲”的项尽力带走；不会主动长期拉取新生产的数据。
				for {
					select {
					case v, ok := <-p.dataChan:
						if !ok {
							// 通道已关闭，关闭路径已有最终 flush 保障，这里直接跳出
							goto DRAIN_DONE
						}
						batchData = p.processor.addToBatch(batchData, v)
						if p.processor.isBatchFull(batchData) {
							// 批满则立即同步 flush，以免超过 grace 时间
							p.doFlush(drainCtx, false, batchData)
							batchData = p.processor.initBatchData()
						}
					default:
						// 通道当前没有更多缓冲项（非阻塞抽干结束）
						goto DRAIN_DONE
					}
				}
			DRAIN_DONE:
				// 3) 执行最后一次同步 flush（若批非空）
				if !p.processor.isBatchEmpty(batchData) {
					p.doFlush(drainCtx, false, batchData)
				}
				cancel()
				// 4) 返回“取消且已收尾”的组合错误
				return errors.Join(ErrContextIsClosed, ErrContextDrained)
			}
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
		// 若设置了并发上限，则使用信号量限制在飞 flush goroutine 数
		if p.flushSem != nil {
			p.flushSem <- struct{}{}
			go func() {
				defer func() { <-p.flushSem }()
				p.flushWithErrorChan(ctx, batchData)
			}()
		} else {
			go p.flushWithErrorChan(ctx, batchData)
		}
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
			if p.logger != nil {
				p.logger.Println("panic recovered in pipeline: ", r)
			} else {
				log.Println("panic recovered in pipeline: ", r)
			}
		}
	}()

	start := time.Now()
	err := p.processor.flush(ctx, batchData)
	dur := time.Since(start)

	// metrics: flush
	if p.metrics != nil {
		p.metrics.Flush(batchLen(batchData), dur)
	}

	if err != nil {
		// 安全地发送错误到错误通道
		p.safeErrorSend(err)
		// metrics: error
		if p.metrics != nil {
			p.metrics.Error(err)
		}
	}
}

// 计算默认错误通道缓冲区大小
func (p *PipelineImpl[T]) defaultErrBufSize() int {
	// Keep original proportional behavior to preserve backward compatibility and tests
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
	select {
	case p.errorChan <- err:
		// sent
	default:
		// buffer full, drop
		if p.metrics != nil {
			p.metrics.ErrorDropped()
		}
	}
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

// WithLogger 注入日志器（可选）
func (p *PipelineImpl[T]) WithLogger(l *log.Logger) *PipelineImpl[T] {
	p.logger = l
	return p
}

// WithMetrics 注入指标钩子（可选）
func (p *PipelineImpl[T]) WithMetrics(h MetricsHook) *PipelineImpl[T] {
	p.metrics = h
	return p
}

// Done 返回最近一次调用 Perform（Sync/Async）的完成信号通道
// 注意：该通道在每次 Perform 启动时都会被替换；并发多次启动时语义未定义
func (p *PipelineImpl[T]) Done() <-chan struct{} {
	p.runMu.Lock()
	defer p.runMu.Unlock()
	return p.runDone
}

// 计算批次长度（通过反射支持 slice/map）
func batchLen(batch any) int {
	if batch == nil {
		return 0
	}
	v := reflect.ValueOf(batch)
	switch v.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
		return v.Len()
	default:
		return 0
	}
}

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

// Start 启动异步执行，返回本次运行的完成信号（done）和错误通道（errs）。
// 行为与约定：
//   - 若管道“已在运行”：直接复用并返回“当前正在运行”的 done，不新建/不覆盖；同时异步触发一次 AsyncPerform，
//     仅用于将 ErrAlreadyRunning 上报到 errs（不会影响现有运行）。
//   - 若管道“未在运行”：仅当 runDone 为空时创建新的 done，否则复用现有 done；随后启动 AsyncPerform。
//   - 提示：保持测试覆盖，尤其是“并发二次启动（ErrAlreadyRunning）”与“Done 关闭时序”的断言，确保语义稳定。
func (p *PipelineImpl[T]) Start(ctx context.Context) (<-chan struct{}, <-chan error) {
	errs := p.ErrorChan(0)

	p.runMu.Lock()
	if atomic.LoadInt32(&p.running) == 1 && p.runDone != nil {
		// 已在运行：复用当前 done
		done := p.runDone
		p.runMu.Unlock()
		// 触发一次执行，仅用于将 ErrAlreadyRunning 上报告知调用方
		go func() {
			if err := p.AsyncPerform(ctx); err != nil {
				p.safeErrorSend(err)
			}
		}()
		return done, errs
	}
	// 未在运行：如无现有通道则创建；否则复用已有通道，避免并发覆盖
	if p.runDone == nil {
		p.runDone = make(chan struct{})
	}
	done := p.runDone
	p.runMu.Unlock()

	go func() {
		if err := p.AsyncPerform(ctx); err != nil {
			p.safeErrorSend(err)
		}
	}()
	return done, errs
}

// Run 同步运行至结束，同时允许指定错误通道容量（便于在调用前设置容量）。
// 注意：Run 不消费错误通道，仅负责初始化容量并同步执行，错误由调用方按需读取。
func (p *PipelineImpl[T]) Run(ctx context.Context, errBuf int) error {
	_ = p.ErrorChan(errBuf)
	return p.SyncPerform(ctx)
}

// 动态并发限流：获取当前 channel 引用；nil 表示不限流
func (p *PipelineImpl[T]) acquireFlushSlot() chan struct{} {
	// 已恢复为固定容量的并发信号量实现，此方法不再使用
	return p.flushSem
}

// 动态参数：FlushSize
func (p *PipelineImpl[T]) CurrentFlushSize() uint32 {
	return p.currFlushSize.Load()
}

func (p *PipelineImpl[T]) UpdateFlushSize(n uint32) {
	if n == 0 {
		n = 1
	}
	p.currFlushSize.Store(n)
}

// 动态参数：FlushInterval
func (p *PipelineImpl[T]) CurrentFlushInterval() time.Duration {
	return time.Duration(p.currFlushInterval.Load())
}

func (p *PipelineImpl[T]) UpdateFlushInterval(d time.Duration) {
	if d <= 0 {
		d = time.Millisecond * 1
	}
	p.currFlushInterval.Store(int64(d))
	// 轻推主循环，确保新的间隔尽快生效
	select {
	case p.nudge <- struct{}{}:
	default:
	}
}

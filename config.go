package gopipeline

import "time"

// PipelineConfig 定义了管道的配置参数
type PipelineConfig struct {
	// BufferSize 缓冲通道的容量
	BufferSize uint32
	// FlushSize 批处理数据的最大容量
	FlushSize uint32
	// FlushInterval 定时刷新的时间间隔
	FlushInterval time.Duration
	// DrainOnCancel 当 ctx.Done() 触发时，是否在退出前尽力刷新当前未满批次
	// 默认 false：收到取消立即退出；true：使用 DrainGracePeriod 限时进行一次 flush
	DrainOnCancel bool
	// DrainGracePeriod 启用 DrainOnCancel 时用于收尾刷新的最长时间窗口
	// 若未设置或 <=0，将在使用时采用一个保守的短时默认值
	DrainGracePeriod time.Duration
}

const (
	// 基于性能测试优化的默认值
	defaultBufferSize       = 100                   // 缓冲区大小，应该 >= FlushSize * 2 以避免阻塞
	defaultFlushSize        = 50                    // 批处理大小，性能测试显示 50 左右为最优
	defaultFlushInterval    = time.Millisecond * 50 // 刷新间隔，平衡延迟和吞吐量
	defaultDrainOnCancel    = false                 // 默认取消时不进行收尾刷新
	defaultDrainGracePeriod = 0                     // 由使用方或实现侧在启用时选择保守默认
)

// NewPipelineConfig 创建一个带默认值的配置
func NewPipelineConfig() PipelineConfig {
	return PipelineConfig{
		BufferSize:       defaultBufferSize,
		FlushSize:        defaultFlushSize,
		FlushInterval:    defaultFlushInterval,
		DrainOnCancel:    defaultDrainOnCancel,
		DrainGracePeriod: defaultDrainGracePeriod,
	}
}

// WithBufferSize 设置缓冲区大小
func (c PipelineConfig) WithBufferSize(size uint32) PipelineConfig {
	c.BufferSize = size
	return c
}

// WithFlushSize 设置批处理大小
func (c PipelineConfig) WithFlushSize(size uint32) PipelineConfig {
	c.FlushSize = size
	return c
}

// WithFlushInterval 设置刷新间隔
func (c PipelineConfig) WithFlushInterval(interval time.Duration) PipelineConfig {
	c.FlushInterval = interval
	return c
}

// WithDrainOnCancel 设置在 ctx.Done() 时是否进行限时收尾刷新
func (c PipelineConfig) WithDrainOnCancel(enabled bool) PipelineConfig {
	c.DrainOnCancel = enabled
	return c
}

// WithDrainGracePeriod 设置收尾刷新的最长时间窗口（仅在 DrainOnCancel 启用时生效）
func (c PipelineConfig) WithDrainGracePeriod(d time.Duration) PipelineConfig {
	c.DrainGracePeriod = d
	return c
}

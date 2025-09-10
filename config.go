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
}

const (
	// 基于性能测试优化的默认值
	defaultBufferSize    = 100                   // 缓冲区大小，应该 >= FlushSize * 2 以避免阻塞
	defaultFlushSize     = 50                    // 批处理大小，性能测试显示 50 左右为最优
	defaultFlushInterval = time.Millisecond * 50 // 刷新间隔，平衡延迟和吞吐量
)

// NewPipelineConfig 创建一个带默认值的配置
func NewPipelineConfig() PipelineConfig {
	return PipelineConfig{
		BufferSize:    defaultBufferSize,
		FlushSize:     defaultFlushSize,
		FlushInterval: defaultFlushInterval,
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

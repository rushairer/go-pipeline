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
	defaultBufferSize    = 32
	defaultFlushSize     = 64
	defaultFlushInterval = time.Millisecond * 200
)

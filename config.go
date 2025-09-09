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

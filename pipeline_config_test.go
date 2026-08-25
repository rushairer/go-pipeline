package gopipeline_test

import (
	"testing"
	"time"

	gopipeline "github.com/rushairer/go-pipeline/v2"
)

// TestPipelineConfig_ValidateOrDefault_Defaults 验证零值配置回退到默认值
func TestPipelineConfig_ValidateOrDefault_Defaults(t *testing.T) {
	cfg := gopipeline.PipelineConfig{}.ValidateOrDefault()

	if cfg.BufferSize != 100 {
		t.Errorf("default BufferSize = %d, want 100", cfg.BufferSize)
	}
	if cfg.FlushSize != 50 {
		t.Errorf("default FlushSize = %d, want 50", cfg.FlushSize)
	}
	if cfg.FlushInterval != 50*time.Millisecond {
		t.Errorf("default FlushInterval = %v, want 50ms", cfg.FlushInterval)
	}
}

// TestPipelineConfig_ValidateOrDefault_PreservesValues 验证已设置的值不被 ValidateOrDefault 覆盖
func TestPipelineConfig_ValidateOrDefault_PreservesValues(t *testing.T) {
	cfg := gopipeline.PipelineConfig{
		BufferSize:    200,
		FlushSize:     80,
		FlushInterval: 10 * time.Millisecond,
	}.ValidateOrDefault()

	if cfg.BufferSize != 200 || cfg.FlushSize != 80 || cfg.FlushInterval != 10*time.Millisecond {
		t.Errorf("existing values were overwritten: %+v", cfg)
	}
}

// TestPipelineConfig_WithBuilders 验证链式 builder 方法逐一生效
func TestPipelineConfig_WithBuilders(t *testing.T) {
	cfg := gopipeline.NewPipelineConfig().
		WithBufferSize(300).
		WithFlushSize(120).
		WithFlushInterval(25 * time.Millisecond).
		WithDrainOnCancel(true).
		WithDrainGracePeriod(3 * time.Second).
		WithMaxConcurrentFlushes(4).
		WithFinalFlushOnCloseTimeout(2 * time.Second)

	if cfg.BufferSize != 300 {
		t.Errorf("BufferSize = %d, want 300", cfg.BufferSize)
	}
	if cfg.FlushSize != 120 {
		t.Errorf("FlushSize = %d, want 120", cfg.FlushSize)
	}
	if cfg.FlushInterval != 25*time.Millisecond {
		t.Errorf("FlushInterval = %v, want 25ms", cfg.FlushInterval)
	}
	if !cfg.DrainOnCancel {
		t.Error("DrainOnCancel = false, want true")
	}
	if cfg.DrainGracePeriod != 3*time.Second {
		t.Errorf("DrainGracePeriod = %v, want 3s", cfg.DrainGracePeriod)
	}
	if cfg.MaxConcurrentFlushes != 4 {
		t.Errorf("MaxConcurrentFlushes = %d, want 4", cfg.MaxConcurrentFlushes)
	}
	if cfg.FinalFlushOnCloseTimeout != 2*time.Second {
		t.Errorf("FinalFlushOnCloseTimeout = %v, want 2s", cfg.FinalFlushOnCloseTimeout)
	}
}

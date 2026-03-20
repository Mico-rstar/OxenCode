package config

import "time"

// CompressorConfig 压缩器配置
type CompressorConfig struct {
	Model       string  `mapstructure:"compressor_model"`
	MaxTokens   int64   `mapstructure:"compressor_max_tokens"`
	Temperature float64 `mapstructure:"compressor_temperature"`
	MaxRetries  int     `mapstructure:"compressor_max_retries"`
}

// CompressWorkerConfig 压缩工作器配置
type CompressWorkerConfig struct {
	WorkerCount int           `mapstructure:"compress_worker_count"`
	QueueSize   int           `mapstructure:"compress_queue_size"`
	Timeout     time.Duration // 由 CompressTimeout 计算得出
}

// CompressionStrategy 压缩策略
type CompressionStrategy struct {
	Timeout             time.Duration
	MaxToolOutputLength int
	MaxAssistantLength  int
}

// GetCompressionStrategy 获取指定层级的压缩策略
func (c *CompressorConfig) GetCompressionStrategy(compressTimeout time.Duration, maxToolOutput, maxAssistant int) *CompressionStrategy {
	return &CompressionStrategy{
		Timeout:             compressTimeout,
		MaxToolOutputLength: maxToolOutput,
		MaxAssistantLength:  maxAssistant,
	}
}

// Package asr 提供语音识别（ASR）抽象接口和工厂方法
package asr

import (
	"context"
	"net/http"
	"time"
)

// Result ASR 识别结果
type Result struct {
	Text     string  `json:"text"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
}

// Provider ASR 服务接口
type Provider interface {
	// Transcribe 发送音频数据进行语音识别
	// audioData: 16bit 16kHz mono PCM 数据
	// lang: 语言代码 (auto/zh/en/es/ja)
	Transcribe(ctx context.Context, audioData []byte, lang string) (*Result, error)
}

// NewProvider 根据 baseURL 和 apiKey 创建 ASR Provider
func NewProvider(baseURL, apiKey, model string) Provider {
	return &whisperProvider{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

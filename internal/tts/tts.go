// Package tts 提供 Text-to-Speech 语音合成抽象接口和工厂方法
package tts

import (
	"context"
	"fmt"
)

// Provider TTS 服务接口
type Provider interface {
	// Synthesize 将文本合成为语音，返回音频二进制数据（MP3 格式）
	Synthesize(ctx context.Context, text, lang string) ([]byte, error)
}

// NewProvider 创建 TTS Provider
// edge 不需要 apiKey，传空即可
func NewProvider(provider, baseURL, apiKey, model, voice string) (Provider, error) {
	switch provider {
	case "siliconflow", "openai":
		if baseURL == "" || apiKey == "" {
			return nil, fmt.Errorf("TTS provider %s 需要 baseURL 和 apiKey", provider)
		}
		return &openaiProvider{
			baseURL: baseURL,
			apiKey:  apiKey,
			model:   model,
			voice:   voice,
		}, nil
	default:
		return nil, fmt.Errorf("不支持的 TTS provider: %s（可选: siliconflow, openai）", provider)
	}
}

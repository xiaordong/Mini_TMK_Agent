// Package translate 提供翻译抽象接口和工厂方法
package translate

import "context"

// Provider 翻译服务接口
type Provider interface {
	// Translate 流式翻译文本
	// text: 原文
	// srcLang/tgtLang: 源/目标语言代码
	// onChunk: 每收到一段译文就回调
	Translate(ctx context.Context, text, srcLang, tgtLang string, onChunk func(string)) error
}

// NewProvider 创建翻译 Provider
func NewProvider(baseURL, apiKey, model string) Provider {
	return &llmProvider{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
	}
}

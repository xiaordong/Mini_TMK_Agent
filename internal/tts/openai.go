package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// openaiProvider 兼容 OpenAI /v1/audio/speech 接口的 TTS 实现
// 适用于 OpenAI、SiliconFlow 等兼容服务商
type openaiProvider struct {
	baseURL     string
	apiKey      string
	model       string
	voice       string
	httpClient  *http.Client
}

// ttsRequest OpenAI TTS API 请求体
type ttsRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	Voice          string `json:"voice"`
	ResponseFormat string `json:"response_format"`
}

// newOpenAIProvider 创建 OpenAI 兼容 TTS 提供者
func newOpenAIProvider(baseURL, apiKey, model, voice string) *openaiProvider {
	return &openaiProvider{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		voice:      voice,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Synthesize 调用 OpenAI 兼容 TTS API 合成语音
func (p *openaiProvider) Synthesize(ctx context.Context, text, lang string) ([]byte, error) {
	reqBody := ttsRequest{
		Model:          p.model,
		Input:          text,
		Voice:          p.voice,
		ResponseFormat: "mp3",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化 TTS 请求失败: %w", err)
	}

	url := p.baseURL + "/audio/speech" // baseURL 已在构造时 TrimRight
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 TTS 请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TTS 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS API 返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 TTS 响应失败: %w", err)
	}

	return data, nil
}

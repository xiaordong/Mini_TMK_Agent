package translate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// llmProvider 通过 LLM SSE 流式 API 实现翻译
type llmProvider struct {
	baseURL string
	apiKey  string
	model   string
}

// chatRequest OpenAI chat completions 请求格式
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse 非流式响应格式
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// sseChunk SSE 流式数据块
type sseChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// 语言名称映射，用于 prompt
var langNames = map[string]string{
	"zh":   "中文",
	"en":   "英文",
	"es":   "西班牙文",
	"ja":   "日文",
	"auto": "自动检测语言",
}

func (p *llmProvider) Translate(ctx context.Context, text, srcLang, tgtLang string, onChunk func(string)) error {
	srcName := langNames[srcLang]
	tgtName := langNames[tgtLang]
	if srcName == "" {
		srcName = srcLang
	}
	if tgtName == "" {
		tgtName = tgtLang
	}

	systemPrompt := fmt.Sprintf(
		"你是专业同声传译员。将以下%s文本翻译为%s。\n要求：准确、流畅、符合口语习惯。只输出翻译结果，不要解释。",
		srcName, tgtName,
	)

	reqBody := chatRequest{
		Model: p.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		},
		Stream: true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	url := p.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("翻译请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("翻译请求失败 (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// 解析 SSE 流
	return p.parseSSE(resp.Body, onChunk)
}

// TranslateSync 非流式翻译，返回完整结果（文件模式使用）
func (p *llmProvider) TranslateSync(ctx context.Context, text, srcLang, tgtLang string) (string, error) {
	var result strings.Builder
	err := p.Translate(ctx, text, srcLang, tgtLang, func(chunk string) {
		result.WriteString(chunk)
	})
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

func (p *llmProvider) parseSSE(reader io.Reader, onChunk func(string)) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// SSE 数据行以 "data: " 开头
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// 流结束标记
		if data == "[DONE]" {
			break
		}

		var chunk sseChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // 跳过无法解析的行
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		content := chunk.Choices[0].Delta.Content
		if content != "" {
			onChunk(content)
		}
	}

	return scanner.Err()
}

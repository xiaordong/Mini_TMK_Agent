package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// whisperProvider 兼容 OpenAI Whisper API 的 HTTP 实现
type whisperProvider struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// whisperResponse OpenAI transcription API 响应格式
type whisperResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

func (p *whisperProvider) Transcribe(ctx context.Context, audioData []byte, lang string) (*Result, error) {
	// 网络请求最多重试 3 次
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		result, err := p.doTranscribe(ctx, audioData, lang)
		if err == nil {
			return result, nil
		}
		lastErr = err
		// 只有网络/超时类错误才重试
		if !isRetryable(err) {
			break
		}
	}
	return nil, lastErr
}

// isRetryable 判断是否为可重试的网络错误
func isRetryable(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "wsarecv") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "refused") ||
		strings.Contains(msg, "HTTP 429") ||
		strings.Contains(msg, "HTTP 5")
}

func (p *whisperProvider) doTranscribe(ctx context.Context, audioData []byte, lang string) (*Result, error) {
	wavData := pcmToWav(audioData)

	// 构建 multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 写入音频文件字段
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return nil, fmt.Errorf("创建 form file 失败: %w", err)
	}
	if _, err := part.Write(wavData); err != nil {
		return nil, fmt.Errorf("写入音频数据失败: %w", err)
	}

	// 写入 model 字段
	if err := writer.WriteField("model", p.model); err != nil {
		return nil, fmt.Errorf("写入 model 字段失败: %w", err)
	}

	// 写入 language 字段（非 auto 时）
	if lang != "" && lang != "auto" {
		if err := writer.WriteField("language", lang); err != nil {
			return nil, fmt.Errorf("写入 language 字段失败: %w", err)
		}
	}

	// 写入 response_format
	if err := writer.WriteField("response_format", "verbose_json"); err != nil {
		return nil, fmt.Errorf("写入 response_format 字段失败: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭 multipart writer 失败: %w", err)
	}

	// 构建请求
	url := strings.TrimRight(p.baseURL, "/") + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	// 发送请求
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送 ASR 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ASR 请求失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var whisperResp whisperResponse
	if err := json.Unmarshal(body, &whisperResp); err != nil {
		return nil, fmt.Errorf("解析 ASR 响应失败: %w", err)
	}

	return &Result{
		Text:     whisperResp.Text,
		Language: whisperResp.Language,
		Duration: whisperResp.Duration,
	}, nil
}

// pcmToWav 将 16bit 16kHz mono PCM 数据包装为 WAV 格式
func pcmToWav(pcmData []byte) []byte {
	dataSize := uint32(len(pcmData))
	sampleRate := uint32(16000)
	bitsPerSample := uint16(16)
	numChannels := uint16(1)
	byteRate := sampleRate * uint32(bitsPerSample) * uint32(numChannels) / 8
	blockAlign := uint16(bitsPerSample) * numChannels / 8

	// 44 字节 WAV header + PCM 数据
	wav := make([]byte, 44+len(pcmData))

	// RIFF header
	copy(wav[0:4], []byte("RIFF"))
	writeUint32(wav[4:8], 36+dataSize)       // ChunkSize
	copy(wav[8:12], []byte("WAVE"))

	// fmt 子块
	copy(wav[12:16], []byte("fmt "))
	writeUint32(wav[16:20], 16)              // Subchunk1Size (PCM)
	writeUint16(wav[20:22], 1)               // AudioFormat (PCM)
	writeUint16(wav[22:24], numChannels)
	writeUint32(wav[24:28], sampleRate)
	writeUint32(wav[28:32], byteRate)
	writeUint16(wav[32:34], blockAlign)
	writeUint16(wav[34:36], bitsPerSample)

	// data 子块
	copy(wav[36:40], []byte("data"))
	writeUint32(wav[40:44], dataSize)
	copy(wav[44:], pcmData)

	return wav
}

func writeUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func writeUint16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

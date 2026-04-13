package asr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPCMToWav(t *testing.T) {
	// 生成 1000 字节的 PCM 数据
	pcm := make([]byte, 1000)
	for i := range pcm {
		pcm[i] = byte(i % 256)
	}

	wav := pcmToWav(pcm)

	// 验证 header 长度
	if len(wav) != 1044 {
		t.Errorf("WAV 长度 = %d, 期望 1044", len(wav))
	}

	// 验证 RIFF 标识
	if string(wav[0:4]) != "RIFF" {
		t.Errorf("RIFF 标识不正确: %s", wav[0:4])
	}
	if string(wav[8:12]) != "WAVE" {
		t.Errorf("WAVE 标识不正确: %s", wav[8:12])
	}

	// 验证 fmt 子块
	if string(wav[12:16]) != "fmt " {
		t.Errorf("fmt 标识不正确: %s", wav[12:16])
	}

	// 验证 data 子块
	if string(wav[36:40]) != "data" {
		t.Errorf("data 标识不正确: %s", wav[36:40])
	}

	// 验证数据大小
	dataSize := uint32(wav[40]) | uint32(wav[41])<<8 | uint32(wav[42])<<16 | uint32(wav[43])<<24
	if dataSize != 1000 {
		t.Errorf("data 大小 = %d, 期望 1000", dataSize)
	}
}

func TestWhisperProvider_Transcribe(t *testing.T) {
	// 创建 mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法
		if r.Method != http.MethodPost {
			t.Errorf("请求方法 = %s, 期望 POST", r.Method)
		}

		// 验证路径
		if !strings.HasSuffix(r.URL.Path, "/audio/transcriptions") {
			t.Errorf("请求路径 = %s, 不匹配", r.URL.Path)
		}

		// 验证 Authorization header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Authorization header 不正确: %s", auth)
		}

		// 验证 Content-Type 是 multipart
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("Content-Type 不正确: %s", ct)
		}

		// 返回 mock 响应
		resp := whisperResponse{
			Text:     "你好世界",
			Language: "zh",
			Duration: 3.5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := &whisperProvider{
		baseURL: server.URL,
		apiKey:  "fake-key",
		model:   "whisper-large-v3-turbo",
	}

	result, err := provider.Transcribe(context.Background(), make([]byte, 32000), "zh")
	if err != nil {
		t.Fatalf("Transcribe 出错: %v", err)
	}

	if result.Text != "你好世界" {
		t.Errorf("Text = %q, 期望 %q", result.Text, "你好世界")
	}
	if result.Language != "zh" {
		t.Errorf("Language = %q, 期望 %q", result.Language, "zh")
	}
	if result.Duration != 3.5 {
		t.Errorf("Duration = %f, 期望 3.5", result.Duration)
	}
}

func TestWhisperProvider_Transcribe_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"Invalid API key"}}`)
	}))
	defer server.Close()

	provider := &whisperProvider{
		baseURL: server.URL,
		apiKey:  "bad-key",
		model:   "test",
	}

	_, err := provider.Transcribe(context.Background(), make([]byte, 100), "en")
	if err == nil {
		t.Fatal("期望返回错误，但没有")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("错误信息应包含 401: %v", err)
	}
}

func TestNewProvider(t *testing.T) {
	p := NewProvider("http://localhost", "key", "model")
	if p == nil {
		t.Fatal("NewProvider 返回 nil")
	}
}

package tts

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenaiProvider_Synthesize(t *testing.T) {
	// mock 音频数据
	fakeAudio := []byte("fake-mp3-data")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法
		if r.Method != http.MethodPost {
			t.Errorf("请求方法 = %s, 期望 POST", r.Method)
		}

		// 验证路径
		if !strings.HasSuffix(r.URL.Path, "/audio/speech") {
			t.Errorf("请求路径 = %s, 期望包含 /audio/speech", r.URL.Path)
		}

		// 验证 Authorization header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Authorization header 不正确: %s", auth)
		}

		// 验证请求体
		body, _ := io.ReadAll(r.Body)
		var req ttsRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}
		if req.Input != "你好世界" {
			t.Errorf("Input = %q, 期望 %q", req.Input, "你好世界")
		}
		if req.Voice != "alloy" {
			t.Errorf("Voice = %q, 期望 %q", req.Voice, "alloy")
		}
		if req.ResponseFormat != "mp3" {
			t.Errorf("ResponseFormat = %q, 期望 mp3", req.ResponseFormat)
		}

		w.Write(fakeAudio)
	}))
	defer server.Close()

	provider := &openaiProvider{
		baseURL:    server.URL,
		apiKey:     "fake-key",
		model:      "tts-1",
		voice:      "alloy",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	data, err := provider.Synthesize(context.Background(), "你好世界", "zh")
	if err != nil {
		t.Fatalf("Synthesize 出错: %v", err)
	}

	if string(data) != string(fakeAudio) {
		t.Errorf("返回数据不匹配")
	}
}

func TestOpenaiProvider_Synthesize_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"Invalid API key"}}`)
	}))
	defer server.Close()

	provider := &openaiProvider{
		baseURL:    server.URL,
		apiKey:     "bad-key",
		model:      "test",
		voice:      "alloy",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	_, err := provider.Synthesize(context.Background(), "测试", "zh")
	if err == nil {
		t.Fatal("期望返回错误，但没有")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("错误信息应包含 401: %v", err)
	}
}

func TestNewProvider_Valid(t *testing.T) {
	p, err := NewProvider("openai", "https://api.openai.com/v1", "key", "tts-1", "alloy")
	if err != nil {
		t.Fatalf("NewProvider 出错: %v", err)
	}
	if p == nil {
		t.Fatal("NewProvider 返回 nil")
	}
}

func TestNewProvider_SiliconFlow(t *testing.T) {
	p, err := NewProvider("siliconflow", "https://api.siliconflow.cn/v1", "key", "model", "voice")
	if err != nil {
		t.Fatalf("NewProvider 出错: %v", err)
	}
	if p == nil {
		t.Fatal("NewProvider 返回 nil")
	}
}

func TestNewProvider_MissingCredentials(t *testing.T) {
	_, err := NewProvider("openai", "", "", "tts-1", "alloy")
	if err == nil {
		t.Fatal("期望返回错误，但没有")
	}
	if !strings.Contains(err.Error(), "需要 baseURL 和 apiKey") {
		t.Errorf("错误信息不正确: %v", err)
	}
}

func TestNewProvider_Unsupported(t *testing.T) {
	_, err := NewProvider("unknown", "", "", "", "")
	if err == nil {
		t.Fatal("期望返回错误，但没有")
	}
	if !strings.Contains(err.Error(), "不支持的 TTS provider") {
		t.Errorf("错误信息不正确: %v", err)
	}
}

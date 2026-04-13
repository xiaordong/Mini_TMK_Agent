package translate

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLLMProvider_Translate(t *testing.T) {
	// mock SSE server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求
		if r.Method != http.MethodPost {
			t.Errorf("请求方法 = %s, 期望 POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("路径不正确: %s", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Authorization 不正确: %s", auth)
		}

		// 返回 SSE 流
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("不支持 Flusher")
		}

		chunks := []string{
			`data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`data: {"choices":[{"delta":{"content":" World"},"finish_reason":null}]}`,
			`data: {"choices":[{"delta":{"content":""},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			io.WriteString(w, chunk+"\n\n")
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider := &llmProvider{
		baseURL:    server.URL,
		apiKey:     "fake-key",
		model:      "test-model",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	var collected strings.Builder
	err := provider.Translate(context.Background(), "你好世界", "zh", "en", func(chunk string) {
		collected.WriteString(chunk)
	})
	if err != nil {
		t.Fatalf("Translate 出错: %v", err)
	}

	result := collected.String()
	if result != "Hello World" {
		t.Errorf("翻译结果 = %q, 期望 %q", result, "Hello World")
	}
}

func TestLLMProvider_TranslateSync(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		chunks := []string{
			`data: {"choices":[{"delta":{"content":"Bonjour"}}]}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			io_StringWriter, _ := w.(io.StringWriter)
			io_StringWriter.WriteString(chunk + "\n\n")
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider := &llmProvider{
		baseURL:    server.URL,
		apiKey:     "fake-key",
		model:      "test",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	result, err := provider.TranslateSync(context.Background(), "你好", "zh", "fr")
	if err != nil {
		t.Fatalf("TranslateSync 出错: %v", err)
	}
	if result != "Bonjour" {
		t.Errorf("结果 = %q, 期望 %q", result, "Bonjour")
	}
}

func TestLLMProvider_Translate_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"message":"Rate limit exceeded"}}`)
	}))
	defer server.Close()

	provider := &llmProvider{
		baseURL:    server.URL,
		apiKey:     "fake-key",
		model:      "test",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	err := provider.Translate(context.Background(), "test", "en", "zh", func(string) {})
	if err == nil {
		t.Fatal("期望返回错误，但没有")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("错误应包含 429: %v", err)
	}
}

func TestLangNames(t *testing.T) {
	tests := []struct {
		code string
		name string
	}{
		{"zh", "中文"},
		{"en", "英文"},
		{"es", "西班牙文"},
		{"ja", "日文"},
	}
	for _, tt := range tests {
		if langNames[tt.code] != tt.name {
			t.Errorf("langNames[%q] = %q, 期望 %q", tt.code, langNames[tt.code], tt.name)
		}
	}
}

func TestNewProvider(t *testing.T) {
	p := NewProvider("http://localhost", "key", "model")
	if p == nil {
		t.Fatal("NewProvider 返回 nil")
	}
}

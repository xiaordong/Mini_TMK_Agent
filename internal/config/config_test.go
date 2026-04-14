package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultProvider(t *testing.T) {
	os.Setenv("TMK_ASR_API_KEY", "fake-asr-key")
	os.Setenv("TMK_TRANS_API_KEY", "fake-trans-key")
	defer func() {
		os.Unsetenv("TMK_ASR_API_KEY")
		os.Unsetenv("TMK_TRANS_API_KEY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}

	if cfg.ASRProvider != "groq" {
		t.Errorf("ASRProvider = %q, 期望 %q", cfg.ASRProvider, "groq")
	}
	if cfg.ASRBaseURL != "https://api.groq.com/openai/v1" {
		t.Errorf("ASRBaseURL = %q, 不匹配", cfg.ASRBaseURL)
	}
	if cfg.ASRModel != "whisper-large-v3-turbo" {
		t.Errorf("ASRModel = %q, 不匹配", cfg.ASRModel)
	}
}

func TestLoad_MissingAPIKey(t *testing.T) {
	os.Unsetenv("TMK_ASR_API_KEY")
	os.Unsetenv("TMK_TRANS_API_KEY")
	os.Unsetenv("TMK_API_KEY")

	_, err := Load()
	if err == nil {
		t.Fatal("期望返回错误，但没有")
	}
}

func TestLoad_CustomProvider(t *testing.T) {
	os.Setenv("TMK_ASR_API_KEY", "fake-key")
	os.Setenv("TMK_TRANS_API_KEY", "fake-key")
	os.Setenv("TMK_ASR_PROVIDER", "openai")
	defer func() {
		os.Unsetenv("TMK_ASR_API_KEY")
		os.Unsetenv("TMK_TRANS_API_KEY")
		os.Unsetenv("TMK_ASR_PROVIDER")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}

	if cfg.ASRProvider != "openai" {
		t.Errorf("ASRProvider = %q, 期望 %q", cfg.ASRProvider, "openai")
	}
	if cfg.ASRBaseURL != "https://api.openai.com/v1" {
		t.Errorf("ASRBaseURL = %q, 不匹配", cfg.ASRBaseURL)
	}
}

func TestLoad_UnifiedProvider(t *testing.T) {
	os.Setenv("TMK_PROVIDER", "siliconflow")
	os.Setenv("TMK_API_KEY", "fake-unified-key")
	defer func() {
		os.Unsetenv("TMK_PROVIDER")
		os.Unsetenv("TMK_API_KEY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}

	// 统一 provider 应同时设置 ASR 和 Trans
	if cfg.ASRProvider != "siliconflow" {
		t.Errorf("ASRProvider = %q, 期望 %q", cfg.ASRProvider, "siliconflow")
	}
	if cfg.TransProvider != "siliconflow" {
		t.Errorf("TransProvider = %q, 期望 %q", cfg.TransProvider, "siliconflow")
	}
	// 统一 Key 回退
	if cfg.ASRAPIKey != "fake-unified-key" {
		t.Errorf("ASRAPIKey = %q, 不匹配", cfg.ASRAPIKey)
	}
	if cfg.TransAPIKey != "fake-unified-key" {
		t.Errorf("TransAPIKey = %q, 不匹配", cfg.TransAPIKey)
	}
	// 验证预设值
	if cfg.ASRBaseURL != "https://api.siliconflow.cn/v1" {
		t.Errorf("ASRBaseURL = %q, 不匹配", cfg.ASRBaseURL)
	}
	if cfg.TransModel != "Qwen/Qwen3-8B" {
		t.Errorf("TransModel = %q, 不匹配", cfg.TransModel)
	}
}

func TestLoad_UnifiedKeyFallback(t *testing.T) {
	os.Setenv("TMK_API_KEY", "unified-key")
	os.Unsetenv("TMK_ASR_API_KEY")
	os.Unsetenv("TMK_TRANS_API_KEY")
	defer func() {
		os.Unsetenv("TMK_API_KEY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}

	if cfg.ASRAPIKey != "unified-key" {
		t.Errorf("ASRAPIKey = %q, 期望统一 Key 回退", cfg.ASRAPIKey)
	}
	if cfg.TransAPIKey != "unified-key" {
		t.Errorf("TransAPIKey = %q, 期望统一 Key 回退", cfg.TransAPIKey)
	}
}

func TestLoad_DedicatedKeyOverridesUnified(t *testing.T) {
	os.Setenv("TMK_API_KEY", "unified-key")
	os.Setenv("TMK_ASR_API_KEY", "dedicated-asr-key")
	defer func() {
		os.Unsetenv("TMK_API_KEY")
		os.Unsetenv("TMK_ASR_API_KEY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}

	// 专用 Key 应优先
	if cfg.ASRAPIKey != "dedicated-asr-key" {
		t.Errorf("ASRAPIKey = %q, 期望专用 Key", cfg.ASRAPIKey)
	}
	// Trans 应回退到统一 Key
	if cfg.TransAPIKey != "unified-key" {
		t.Errorf("TransAPIKey = %q, 期望统一 Key 回退", cfg.TransAPIKey)
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sk-1234567890abcdef", "sk-1****cdef"},
		{"short", "****"},
		{"", "****"},
	}
	for _, tt := range tests {
		got := MaskKey(tt.input)
		if got != tt.expected {
			t.Errorf("MaskKey(%q) = %q, 期望 %q", tt.input, got, tt.expected)
		}
	}
}

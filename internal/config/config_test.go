package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultProvider(t *testing.T) {
	// 设置必要的环境变量
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

	// 验证默认值
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
	// 清除环境变量
	os.Unsetenv("TMK_ASR_API_KEY")
	os.Unsetenv("TMK_TRANS_API_KEY")

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

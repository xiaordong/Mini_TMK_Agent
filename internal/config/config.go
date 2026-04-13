// Package config 提供全局配置加载和管理
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// ProviderPreset provider 预设配置
type ProviderPreset struct {
	BaseURL string
	Model   string
}

// Config 全局配置
type Config struct {
	// ASR 配置
	ASRProvider string `mapstructure:"asr_provider"`
	ASRBaseURL  string `mapstructure:"asr_base_url"`
	ASRAPIKey   string `mapstructure:"asr_api_key"`
	ASRModel    string `mapstructure:"asr_model"`

	// 翻译配置
	TransProvider string `mapstructure:"trans_provider"`
	TransBaseURL  string `mapstructure:"trans_base_url"`
	TransAPIKey   string `mapstructure:"trans_api_key"`
	TransModel    string `mapstructure:"trans_model"`

	// 运行时参数
	SourceLang string
	TargetLang string
	Verbose    bool
}

// ProviderDefaults ASR provider 预设配置
var ProviderDefaults = map[string]ProviderPreset{
	"groq": {
		BaseURL: "https://api.groq.com/openai/v1",
		Model:   "whisper-large-v3-turbo",
	},
	"siliconflow": {
		BaseURL: "https://api.siliconflow.cn/v1",
		Model:   "FunAudioLLM/SenseVoiceSmall",
	},
	"openai": {
		BaseURL: "https://api.openai.com/v1",
		Model:   "whisper-1",
	},
}

// TransDefaults 翻译 provider 预设配置
var TransDefaults = map[string]ProviderPreset{
	"groq": {
		BaseURL: "https://api.groq.com/openai/v1",
		Model:   "llama-3.3-70b-versatile",
	},
	"siliconflow": {
		BaseURL: "https://api.siliconflow.cn/v1",
		Model:   "Qwen/Qwen3-8B",
	},
	"openai": {
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o-mini",
	},
}

// Load 加载配置，优先级：flags > 环境变量 > .env 文件 > 配置文件 > 默认值
func Load() (*Config, error) {
	// 先加载 .env 文件到环境变量（不覆盖已有的）
	loadEnvFile(".env")

	home, _ := os.UserHomeDir()

	viper.SetConfigName(".mini-tmk-agent")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(home)
	viper.AddConfigPath(".")

	// 环境变量绑定
	viper.SetEnvPrefix("TMK")
	viper.AutomaticEnv()

	// 尝试读取配置文件（不存在也不报错）
	_ = viper.ReadInConfig()

	cfg := &Config{
		ASRProvider:   strings.ToLower(viper.GetString("asr_provider")),
		ASRBaseURL:    viper.GetString("asr_base_url"),
		ASRAPIKey:     viper.GetString("asr_api_key"),
		ASRModel:      viper.GetString("asr_model"),
		TransProvider: strings.ToLower(viper.GetString("trans_provider")),
		TransBaseURL:  viper.GetString("trans_base_url"),
		TransAPIKey:   viper.GetString("trans_api_key"),
		TransModel:    viper.GetString("trans_model"),
	}

	// 应用 ASR provider 预设
	if cfg.ASRProvider == "" {
		cfg.ASRProvider = "groq"
	}
	if cfg.ASRBaseURL == "" || cfg.ASRModel == "" {
		if p, ok := ProviderDefaults[cfg.ASRProvider]; ok {
			if cfg.ASRBaseURL == "" {
				cfg.ASRBaseURL = p.BaseURL
			}
			if cfg.ASRModel == "" {
				cfg.ASRModel = p.Model
			}
		}
	}

	// 应用翻译 provider 预设
	if cfg.TransProvider == "" {
		cfg.TransProvider = "groq"
	}
	if cfg.TransBaseURL == "" || cfg.TransModel == "" {
		if p, ok := TransDefaults[cfg.TransProvider]; ok {
			if cfg.TransBaseURL == "" {
				cfg.TransBaseURL = p.BaseURL
			}
			if cfg.TransModel == "" {
				cfg.TransModel = p.Model
			}
		}
	}

	// 校验必要字段
	if cfg.ASRAPIKey == "" {
		return nil, fmt.Errorf("ASR API Key 未设置，请设置环境变量 TMK_ASR_API_KEY 或在配置文件中配置")
	}
	if cfg.TransAPIKey == "" {
		return nil, fmt.Errorf("翻译 API Key 未设置，请设置环境变量 TMK_TRANS_API_KEY 或在配置文件中配置")
	}

	return cfg, nil
}

// ConfigFilePath 返回配置文件路径
func ConfigFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mini-tmk-agent.yaml")
}

// loadEnvFile 读取 .env 文件，将 KEY=VALUE 设置到环境变量（不覆盖已有值）
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// 不覆盖已有环境变量
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

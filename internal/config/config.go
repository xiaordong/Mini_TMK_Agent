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

// Profile 统一服务商配置，一个 provider 同时设置 ASR + Trans + TTS
type Profile struct {
	ASR   ProviderPreset
	Trans ProviderPreset
	TTS   *ProviderPreset // nil 表示该服务商不支持 TTS
	Voice string          // TTS 默认发音人
}

// Config 全局配置
type Config struct {
	// 统一配置
	Provider string `mapstructure:"provider"`
	APIKey   string `mapstructure:"api_key"`

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

	// TTS 配置
	TTSEnabled  bool   `mapstructure:"tts_enabled"`
	TTSProvider string `mapstructure:"tts_provider"`
	TTSBaseURL  string `mapstructure:"tts_base_url"`
	TTSAPIKey   string `mapstructure:"tts_api_key"`
	TTSModel    string `mapstructure:"tts_model"`
	TTSVoice    string `mapstructure:"tts_voice"`

	// 运行时参数
	SourceLang string
	TargetLang string
	Verbose    bool
}

// Profiles 统一服务商预设
var Profiles = map[string]Profile{
	"groq": {
		ASR:   ProviderPreset{BaseURL: "https://api.groq.com/openai/v1", Model: "whisper-large-v3-turbo"},
		Trans: ProviderPreset{BaseURL: "https://api.groq.com/openai/v1", Model: "llama-3.3-70b-versatile"},
	},
	"siliconflow": {
		ASR:   ProviderPreset{BaseURL: "https://api.siliconflow.cn/v1", Model: "FunAudioLLM/SenseVoiceSmall"},
		Trans: ProviderPreset{BaseURL: "https://api.siliconflow.cn/v1", Model: "Qwen/Qwen3-8B"},
		TTS:   &ProviderPreset{BaseURL: "https://api.siliconflow.cn/v1", Model: "FunAudioLLM/CosyVoice2-0.5B"},
		Voice: "FunAudioLLM/CosyVoice2-0.5B:alex",
	},
	"openai": {
		ASR:   ProviderPreset{BaseURL: "https://api.openai.com/v1", Model: "whisper-1"},
		Trans: ProviderPreset{BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini"},
		TTS:   &ProviderPreset{BaseURL: "https://api.openai.com/v1", Model: "tts-1"},
		Voice: "alloy",
	},
}

// 保留旧预设，向后兼容 hidden flags
var ProviderDefaults = map[string]ProviderPreset{
	"groq":        {BaseURL: "https://api.groq.com/openai/v1", Model: "whisper-large-v3-turbo"},
	"siliconflow": {BaseURL: "https://api.siliconflow.cn/v1", Model: "FunAudioLLM/SenseVoiceSmall"},
	"openai":      {BaseURL: "https://api.openai.com/v1", Model: "whisper-1"},
}

var TransDefaults = map[string]ProviderPreset{
	"groq":        {BaseURL: "https://api.groq.com/openai/v1", Model: "llama-3.3-70b-versatile"},
	"siliconflow": {BaseURL: "https://api.siliconflow.cn/v1", Model: "Qwen/Qwen3-8B"},
	"openai":      {BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini"},
}

var TTSDefaults = map[string]ProviderPreset{
	"siliconflow": {BaseURL: "https://api.siliconflow.cn/v1", Model: "FunAudioLLM/CosyVoice2-0.5B"},
	"openai":      {BaseURL: "https://api.openai.com/v1", Model: "tts-1"},
}

var TTSVoiceDefaults = map[string]string{
	"siliconflow": "FunAudioLLM/CosyVoice2-0.5B:alex",
	"openai":      "alloy",
}

// Load 加载配置，优先级：flags > 环境变量 > .env 文件 > 配置文件 > 默认值
func Load() (*Config, error) {
	loadEnvFile(".env")

	home, _ := os.UserHomeDir()

	viper.SetConfigName(".mini-tmk-agent")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(home)
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("TMK")
	viper.AutomaticEnv()

	_ = viper.ReadInConfig()

	cfg := &Config{
		Provider:     strings.ToLower(viper.GetString("provider")),
		APIKey:       viper.GetString("api_key"),
		ASRProvider:  strings.ToLower(viper.GetString("asr_provider")),
		ASRBaseURL:   viper.GetString("asr_base_url"),
		ASRAPIKey:    viper.GetString("asr_api_key"),
		ASRModel:     viper.GetString("asr_model"),
		TransProvider: strings.ToLower(viper.GetString("trans_provider")),
		TransBaseURL:  viper.GetString("trans_base_url"),
		TransAPIKey:   viper.GetString("trans_api_key"),
		TransModel:    viper.GetString("trans_model"),
		TTSEnabled:    viper.GetBool("tts_enabled"),
		TTSProvider:   strings.ToLower(viper.GetString("tts_provider")),
		TTSBaseURL:    viper.GetString("tts_base_url"),
		TTSAPIKey:     viper.GetString("tts_api_key"),
		TTSModel:      viper.GetString("tts_model"),
		TTSVoice:      viper.GetString("tts_voice"),
	}

	// 应用统一 Provider Profile
	if cfg.Provider != "" {
		profile, ok := Profiles[cfg.Provider]
		if !ok {
			return nil, fmt.Errorf("未知服务商: %s（可选: groq, siliconflow, openai）", cfg.Provider)
		}
		// 仅在用户未单独指定时才使用 Profile 填充
		if cfg.ASRProvider == "" {
			cfg.ASRProvider = cfg.Provider
		}
		if cfg.ASRBaseURL == "" {
			cfg.ASRBaseURL = profile.ASR.BaseURL
		}
		if cfg.ASRModel == "" {
			cfg.ASRModel = profile.ASR.Model
		}
		if cfg.TransProvider == "" {
			cfg.TransProvider = cfg.Provider
		}
		if cfg.TransBaseURL == "" {
			cfg.TransBaseURL = profile.Trans.BaseURL
		}
		if cfg.TransModel == "" {
			cfg.TransModel = profile.Trans.Model
		}
		if cfg.TTSEnabled && profile.TTS != nil {
			if cfg.TTSProvider == "" {
				cfg.TTSProvider = cfg.Provider
			}
			if cfg.TTSBaseURL == "" {
				cfg.TTSBaseURL = profile.TTS.BaseURL
			}
			if cfg.TTSModel == "" {
				cfg.TTSModel = profile.TTS.Model
			}
			if cfg.TTSVoice == "" {
				cfg.TTSVoice = profile.Voice
			}
		}
	}

	// 未设置统一 Provider 时，回退到独立 provider 逻辑
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
	if cfg.TransProvider == "" {
		cfg.TransProvider = cfg.ASRProvider
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

	// TTS 独立 provider 预设
	if cfg.TTSEnabled {
		if cfg.TTSProvider == "" {
			cfg.TTSProvider = "siliconflow"
		}
		if cfg.TTSBaseURL == "" || cfg.TTSModel == "" {
			if p, ok := TTSDefaults[cfg.TTSProvider]; ok {
				if cfg.TTSBaseURL == "" {
					cfg.TTSBaseURL = p.BaseURL
				}
				if cfg.TTSModel == "" {
					cfg.TTSModel = p.Model
				}
			}
		}
		if cfg.TTSVoice == "" {
			cfg.TTSVoice = TTSVoiceDefaults[cfg.TTSProvider]
		}
	}

	// 统一 API Key 回退：专用 Key > 统一 Key
	if cfg.ASRAPIKey == "" {
		cfg.ASRAPIKey = cfg.APIKey
	}
	if cfg.TransAPIKey == "" {
		cfg.TransAPIKey = cfg.APIKey
	}
	if cfg.TTSEnabled && cfg.TTSAPIKey == "" {
		cfg.TTSAPIKey = cfg.APIKey
	}

	// 校验必要字段
	if cfg.ASRAPIKey == "" {
		return nil, fmt.Errorf("ASR API Key 未设置，请运行: mini-tmk-agent config set api-key <key>")
	}
	if cfg.TransAPIKey == "" {
		return nil, fmt.Errorf("翻译 API Key 未设置，请运行: mini-tmk-agent config set api-key <key>")
	}

	return cfg, nil
}

// ConfigFilePath 返回配置文件路径
func ConfigFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mini-tmk-agent.yaml")
}

// Save 保存指定 key-value 到配置文件
func Save(key, value string) error {
	viper.Set(key, value)
	return viper.WriteConfigAs(ConfigFilePath())
}

// MaskKey 将 API Key 脱敏显示
func MaskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
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
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

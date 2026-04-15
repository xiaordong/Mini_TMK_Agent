// Mini_TMK_Agent - 同声传译 Agent CLI
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"Mini_TMK_Agent/internal/asr"
	"Mini_TMK_Agent/internal/audio"
	"Mini_TMK_Agent/internal/config"
	"Mini_TMK_Agent/internal/output"
	"Mini_TMK_Agent/internal/pipeline"
	"Mini_TMK_Agent/internal/translate"
	"Mini_TMK_Agent/internal/tts"
	"Mini_TMK_Agent/internal/web"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	Version = "dev"

	// 全局 flags
	providerFlag string
	sourceLang   string
	targetLang   string
	verbose      bool

	// hidden flags（向后兼容）
	asrProviderLegacy   string
	transProviderLegacy string
	ttsProviderLegacy   string
	ttsVoiceLegacy      string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mini-tmk-agent",
		Short: "同声传译 Agent - 实时麦克风翻译和音频文件转录",
	}

	// 全局 flags（4 个）
	rootCmd.PersistentFlags().StringVar(&providerFlag, "provider", "", "统一服务商 (groq|siliconflow|openai)")
	rootCmd.PersistentFlags().StringVarP(&sourceLang, "source", "s", "auto", "源语言 (auto|zh|en|es|ja)")
	rootCmd.PersistentFlags().StringVarP(&targetLang, "target", "t", "zh", "目标语言 (zh|en|es|ja)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "显示原文")

	// Hidden flags（向后兼容）
	rootCmd.PersistentFlags().StringVar(&asrProviderLegacy, "asr-provider", "", "")
	rootCmd.PersistentFlags().StringVar(&transProviderLegacy, "trans-provider", "", "")
	rootCmd.PersistentFlags().StringVar(&ttsProviderLegacy, "tts-provider", "", "")
	rootCmd.PersistentFlags().StringVar(&ttsVoiceLegacy, "tts-voice", "", "")
	rootCmd.PersistentFlags().MarkHidden("asr-provider")
	rootCmd.PersistentFlags().MarkHidden("trans-provider")
	rootCmd.PersistentFlags().MarkHidden("tts-provider")
	rootCmd.PersistentFlags().MarkHidden("tts-voice")

	// 绑定到 viper
	viper.BindPFlag("provider", rootCmd.PersistentFlags().Lookup("provider"))
	viper.BindPFlag("asr_provider", rootCmd.PersistentFlags().Lookup("asr-provider"))
	viper.BindPFlag("trans_provider", rootCmd.PersistentFlags().Lookup("trans-provider"))
	viper.BindPFlag("tts_provider", rootCmd.PersistentFlags().Lookup("tts-provider"))
	viper.BindPFlag("tts_voice", rootCmd.PersistentFlags().Lookup("tts-voice"))

	// stream 命令
	streamCmd := &cobra.Command{
		Use:   "stream",
		Short: "实时麦克风同传",
		Example: "  mini-tmk-agent stream -s zh -t en\n" +
			"  mini-tmk-agent stream --provider groq",
		RunE: runStream,
	}

	// transcript 命令（位置参数）
	transcriptCmd := &cobra.Command{
		Use:   "transcript <file> [output]",
		Short: "音频文件转录",
		Example: "  mini-tmk-agent transcript input.wav\n" +
			"  mini-tmk-agent transcript audio.mp3 output.txt -s en -t zh",
		Args: cobra.RangeArgs(1, 2),
		RunE: runTranscript,
	}

	// web 命令
	webCmd := &cobra.Command{
		Use:   "web",
		Short: "启动 Web UI 服务",
		Example: "  mini-tmk-agent web\n" +
			"  mini-tmk-agent web -p 3000",
		RunE: runWeb,
	}
	webCmd.Flags().IntP("port", "p", 8080, "Web 服务端口")

	// version 命令
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("mini-tmk-agent %s\n", Version)
		},
	}

	// config 子命令
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "管理配置",
	}

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "显示当前配置",
		RunE:  runConfigShow,
	}

	configSetCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "设置配置项",
		Example: "  mini-tmk-agent config set provider siliconflow\n" +
			"  mini-tmk-agent config set api-key sk-xxx\n" +
			"  mini-tmk-agent config set trans-model deepseek-chat\n" +
			"  mini-tmk-agent config set tts true",
		Args: cobra.ExactArgs(2),
		RunE: runConfigSet,
	}

	configCheckCmd := &cobra.Command{
		Use:   "check",
		Short: "检查配置和运行环境",
		RunE:  runConfigCheck,
	}

	configCmd.AddCommand(configShowCmd, configSetCmd, configCheckCmd)
	rootCmd.AddCommand(streamCmd, transcriptCmd, webCmd, configCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// hidden flags 向后兼容覆盖
	if asrProviderLegacy != "" {
		cfg.ASRProvider = strings.ToLower(asrProviderLegacy)
		if p, ok := config.ProviderDefaults[cfg.ASRProvider]; ok {
			cfg.ASRBaseURL = p.BaseURL
			cfg.ASRModel = p.Model
		}
	}
	if transProviderLegacy != "" {
		cfg.TransProvider = strings.ToLower(transProviderLegacy)
		if p, ok := config.TransDefaults[cfg.TransProvider]; ok {
			cfg.TransBaseURL = p.BaseURL
			cfg.TransModel = p.Model
		}
	}
	if ttsProviderLegacy != "" {
		cfg.TTSProvider = strings.ToLower(ttsProviderLegacy)
		if p, ok := config.TTSDefaults[cfg.TTSProvider]; ok {
			cfg.TTSBaseURL = p.BaseURL
			cfg.TTSModel = p.Model
		}
	}
	if ttsVoiceLegacy != "" {
		cfg.TTSVoice = ttsVoiceLegacy
	}

	cfg.Verbose = verbose
	return cfg, nil
}

// wrapTTSOutput 如果启用了 TTS，将 ConsoleOutput 包装为 TTSOutput
func wrapTTSOutput(cfg *config.Config, consoleOut *output.ConsoleOutput, outputPath string) (output.Output, *output.TTSOutput, error) {
	if !cfg.TTSEnabled {
		return consoleOut, nil, nil
	}

	ttsProv, err := tts.NewProvider(cfg.TTSProvider, cfg.TTSBaseURL, cfg.TTSAPIKey, cfg.TTSModel, cfg.TTSVoice)
	if err != nil {
		return nil, nil, err
	}

	ttsOut := output.NewTTSOutput(consoleOut, output.TTSOutputConfig{
		Provider:   ttsProv,
		TargetLang: cfg.TargetLang,
		OutputPath: outputPath,
	})
	return ttsOut, ttsOut, nil
}

func runStream(cmd *cobra.Command, args []string) error {
	// 检查麦克风采集能力
	if !audio.IsCaptureAvailable() {
		return fmt.Errorf("麦克风采集需要 CGO 支持，请安装 GCC 并使用 make build-full 重新编译")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.SourceLang = sourceLang
	cfg.TargetLang = targetLang

	consoleOut := output.NewConsoleOutput(cfg.Verbose)
	out, ttsOut, err := wrapTTSOutput(cfg, consoleOut, "")
	if err != nil {
		return err
	}

	asrProv := asr.NewProvider(cfg.ASRBaseURL, cfg.ASRAPIKey, cfg.ASRModel)
	transProv := translate.NewProvider(cfg.TransBaseURL, cfg.TransAPIKey, cfg.TransModel)
	capturer := audio.NewMalgoCapturer(16000)

	pipe := pipeline.NewStreamPipeline(pipeline.StreamPipelineConfig{
		ASRProvider:   asrProv,
		TransProvider: transProv,
		Output:        out,
		Capturer:      capturer,
		SourceLang:    sourceLang,
		TargetLang:    targetLang,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		out.OnInfo("正在停止...")
		cancel()
	}()

	err = pipe.Run(ctx)
	if ttsOut != nil {
		if flushErr := ttsOut.Flush(); flushErr != nil {
			out.OnError(fmt.Sprintf("TTS 输出失败: %v", flushErr))
		}
	}
	return err
}

func runTranscript(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.SourceLang = sourceLang
	cfg.TargetLang = targetLang

	filePath := args[0]
	var outputPath string
	if len(args) > 1 {
		outputPath = args[1]
	}

	consoleOut := output.NewConsoleOutput(cfg.Verbose)
	out, ttsOut, err := wrapTTSOutput(cfg, consoleOut, outputPath)
	if err != nil {
		return err
	}

	asrProv := asr.NewProvider(cfg.ASRBaseURL, cfg.ASRAPIKey, cfg.ASRModel)
	transProv := translate.NewProvider(cfg.TransBaseURL, cfg.TransAPIKey, cfg.TransModel)

	pipe := pipeline.NewFilePipeline(pipeline.FilePipelineConfig{
		ASRProvider:   asrProv,
		TransProvider: transProv,
		Output:        out,
		SourceLang:    sourceLang,
		TargetLang:    targetLang,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		out.OnInfo("正在停止...")
		cancel()
	}()

	err = pipe.Run(ctx, filePath, outputPath)
	if ttsOut != nil {
		if flushErr := ttsOut.Flush(); flushErr != nil {
			out.OnError(fmt.Sprintf("TTS 输出失败: %v", flushErr))
		}
	}
	return err
}

func runWeb(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	srv := web.NewServer(cfg)
	addr := fmt.Sprintf(":%d", port)
	return srv.Start(addr)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		// 配置加载失败时仍显示部分信息
		fmt.Printf("配置文件: %s\n", config.ConfigFilePath())
		fmt.Printf("错误: %v\n", err)
		return nil
	}

	fmt.Printf("配置文件: %s\n", config.ConfigFilePath())
	fmt.Println()
	fmt.Printf("服务商:     %s\n", cfg.Provider)
	fmt.Printf("API Key:    %s\n", config.MaskKey(cfg.APIKey))
	fmt.Println()
	fmt.Printf("[ASR]\n")
	fmt.Printf("  Provider: %s\n", cfg.ASRProvider)
	fmt.Printf("  BaseURL:  %s\n", cfg.ASRBaseURL)
	fmt.Printf("  Model:    %s\n", cfg.ASRModel)
	fmt.Printf("  API Key:  %s\n", config.MaskKey(cfg.ASRAPIKey))
	fmt.Println()
	fmt.Printf("[翻译]\n")
	fmt.Printf("  Provider: %s\n", cfg.TransProvider)
	fmt.Printf("  BaseURL:  %s\n", cfg.TransBaseURL)
	fmt.Printf("  Model:    %s\n", cfg.TransModel)
	fmt.Printf("  API Key:  %s\n", config.MaskKey(cfg.TransAPIKey))
	fmt.Println()
	fmt.Printf("[TTS]\n")
	fmt.Printf("  启用:     %v\n", cfg.TTSEnabled)
	if cfg.TTSEnabled {
		fmt.Printf("  Provider: %s\n", cfg.TTSProvider)
		fmt.Printf("  BaseURL:  %s\n", cfg.TTSBaseURL)
		fmt.Printf("  Model:    %s\n", cfg.TTSModel)
		fmt.Printf("  Voice:    %s\n", cfg.TTSVoice)
		fmt.Printf("  API Key:  %s\n", config.MaskKey(cfg.TTSAPIKey))
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// 将用户友好的 key 映射到 viper key
	keyMap := map[string]string{
		"provider":        "provider",
		"api-key":         "api_key",
		"tts":             "tts_enabled",
		"tts-voice":       "tts_voice",
		"asr-base-url":    "asr_base_url",
		"asr-model":       "asr_model",
		"trans-base-url":  "trans_base_url",
		"trans-model":     "trans_model",
		"tts-base-url":    "tts_base_url",
		"tts-model":       "tts_model",
	}

	viperKey, ok := keyMap[key]
	if !ok {
		// 也支持直接使用 viper key
		viperKey = key
	}

	// 特殊处理布尔值
	if key == "tts" {
		viper.Set(viperKey, value == "true")
	} else {
		viper.Set(viperKey, value)
	}

	if err := viper.WriteConfigAs(config.ConfigFilePath()); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	fmt.Printf("已设置 %s = %s\n", key, value)
	fmt.Printf("配置文件: %s\n", config.ConfigFilePath())
	return nil
}

func runConfigCheck(cmd *cobra.Command, args []string) error {
	fmt.Println("检查配置和运行环境...")
	fmt.Println()

	// 检查配置文件
	configPath := config.ConfigFilePath()
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("[✓] 配置文件: %s\n", configPath)
	} else {
		fmt.Printf("[!] 配置文件不存在: %s（使用 config set 创建）\n", configPath)
	}

	// 检查环境变量
	apiKey := os.Getenv("TMK_API_KEY")
	asrKey := os.Getenv("TMK_ASR_API_KEY")
	transKey := os.Getenv("TMK_TRANS_API_KEY")

	if apiKey != "" {
		fmt.Printf("[✓] TMK_API_KEY 已设置 (%s)\n", config.MaskKey(apiKey))
	} else {
		fmt.Println("[ ] TMK_API_KEY 未设置")
	}
	if asrKey != "" {
		fmt.Printf("[✓] TMK_ASR_API_KEY 已设置 (%s)\n", config.MaskKey(asrKey))
	}
	if transKey != "" {
		fmt.Printf("[✓] TMK_TRANS_API_KEY 已设置 (%s)\n", config.MaskKey(transKey))
	}

	fmt.Println()

	// 尝试加载完整配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("[✗] 配置加载失败: %v\n", err)
		fmt.Println()
		fmt.Println("提示:")
		fmt.Println("  mini-tmk-agent config set provider siliconflow")
		fmt.Println("  mini-tmk-agent config set api-key sk-xxx")
		return nil
	}

	fmt.Printf("[✓] 服务商: %s\n", cfg.Provider)
	fmt.Printf("[✓] ASR:    %s (%s)\n", cfg.ASRProvider, cfg.ASRModel)
	fmt.Printf("[✓] 翻译:   %s (%s)\n", cfg.TransProvider, cfg.TransModel)
	if cfg.TTSEnabled {
		fmt.Printf("[✓] TTS:    %s (%s, voice=%s)\n", cfg.TTSProvider, cfg.TTSModel, cfg.TTSVoice)
	} else {
		fmt.Println("[ ] TTS:    未启用")
	}

	fmt.Println()

	// 检查麦克风
	if audio.IsCaptureAvailable() {
		fmt.Println("[✓] 麦克风采集: 可用（CGO 已启用）")
	} else {
		fmt.Println("[ ] 麦克风采集: 不可用（CGO 未启用，stream 命令不可用）")
	}

	fmt.Println()
	fmt.Println("所有检查完成。")
	return nil
}

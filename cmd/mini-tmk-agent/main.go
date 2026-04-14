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

	sourceLang    string
	targetLang    string
	asrProvider   string
	transProvider string
	ttsEnabled    bool
	ttsProvider   string
	ttsVoice      string
	verbose       bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mini-tmk-agent",
		Short: "同声传译 Agent - 实时麦克风翻译和音频文件转录",
		Long:  "Mini TMK Agent 是一个同声传译 CLI 工具，支持实时麦克风流式翻译和音频文件转录。\n支持中文、英文、西班牙文、日文四种语言的互译。",
	}

	rootCmd.PersistentFlags().StringVar(&asrProvider, "asr-provider", "", "ASR 服务商 (groq|siliconflow|openai)")
	rootCmd.PersistentFlags().StringVar(&transProvider, "trans-provider", "", "翻译服务商 (groq|siliconflow|openai)")
	rootCmd.PersistentFlags().BoolVar(&ttsEnabled, "tts", false, "启用 TTS 语音合成输出")
	rootCmd.PersistentFlags().StringVar(&ttsProvider, "tts-provider", "", "TTS 服务商 (siliconflow|openai)")
	rootCmd.PersistentFlags().StringVar(&ttsVoice, "tts-voice", "", "TTS 发音人")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "显示原文和详细信息")

	// 将 flags 绑定到 viper
	viper.BindPFlag("asr_provider", rootCmd.PersistentFlags().Lookup("asr-provider"))
	viper.BindPFlag("trans_provider", rootCmd.PersistentFlags().Lookup("trans-provider"))
	viper.BindPFlag("tts_enabled", rootCmd.PersistentFlags().Lookup("tts"))
	viper.BindPFlag("tts_provider", rootCmd.PersistentFlags().Lookup("tts-provider"))
	viper.BindPFlag("tts_voice", rootCmd.PersistentFlags().Lookup("tts-voice"))

	// stream 命令
	streamCmd := &cobra.Command{
		Use:   "stream",
		Short: "启动流式同传模式",
		Long:  "通过麦克风实时采集音频，进行语音识别和翻译。",
		Example: "  mini-tmk-agent stream --source-lang zh --target-lang en\n" +
			"  mini-tmk-agent stream -s en -t zh --asr-provider groq --trans-provider siliconflow",
		RunE: runStream,
	}
	streamCmd.Flags().StringVarP(&sourceLang, "source-lang", "s", "auto", "源语言 (auto|zh|en|es|ja)")
	streamCmd.Flags().StringVarP(&targetLang, "target-lang", "t", "zh", "目标语言 (zh|en|es|ja)")

	// transcript 命令
	transcriptCmd := &cobra.Command{
		Use:   "transcript",
		Short: "音频文件转录",
		Long:  "读取音频文件，进行语音识别和翻译，输出到文件。",
		Example: "  mini-tmk-agent transcript --file input.mp3 --output output.txt\n" +
			"  mini-tmk-agent transcript --file audio.wav -s en -t zh",
		RunE: runTranscript,
	}
	transcriptCmd.Flags().StringVarP(&sourceLang, "source-lang", "s", "auto", "源语言 (auto|zh|en|es|ja)")
	transcriptCmd.Flags().StringVarP(&targetLang, "target-lang", "t", "zh", "目标语言 (zh|en|es|ja)")
	transcriptCmd.Flags().String("file", "", "输入音频文件路径 (wav/mp3/pcm)")
	transcriptCmd.Flags().String("output", "", "输出文件路径")

	_ = transcriptCmd.MarkFlagRequired("file")

	// version 命令
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("mini-tmk-agent %s\n", Version)
		},
	}

	rootCmd.AddCommand(streamCmd, transcriptCmd, versionCmd)

	// web 命令
	webCmd := &cobra.Command{
		Use:   "web",
		Short: "启动 Web UI 服务",
		Long:  "启动 Web 界面，支持文件转录和实时同传。",
		Example: "  mini-tmk-agent web\n" +
			"  mini-tmk-agent web -p 3000",
		RunE: runWeb,
	}
	webCmd.Flags().IntP("port", "p", 8080, "Web 服务端口")

	rootCmd.AddCommand(webCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// 命令行 flags 覆盖配置
	if asrProvider != "" {
		cfg.ASRProvider = strings.ToLower(asrProvider)
		// 重新应用预设
		if p, ok := config.ProviderDefaults[cfg.ASRProvider]; ok {
			cfg.ASRBaseURL = p.BaseURL
			cfg.ASRModel = p.Model
		}
	}
	if transProvider != "" {
		cfg.TransProvider = strings.ToLower(transProvider)
		if p, ok := config.TransDefaults[cfg.TransProvider]; ok {
			cfg.TransBaseURL = p.BaseURL
			cfg.TransModel = p.Model
		}
	}
	if ttsEnabled {
		cfg.TTSEnabled = true
	}
	if ttsProvider != "" {
		cfg.TTSProvider = strings.ToLower(ttsProvider)
		if p, ok := config.TTSDefaults[cfg.TTSProvider]; ok {
			cfg.TTSBaseURL = p.BaseURL
			cfg.TTSModel = p.Model
		}
	}
	if ttsVoice != "" {
		cfg.TTSVoice = ttsVoice
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

	// 优雅退出
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
	// 流模式结束后保存 TTS 音频
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

	filePath, _ := cmd.Flags().GetString("file")
	outputPath, _ := cmd.Flags().GetString("output")

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

	// 信号处理
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
	// 文件转录结束后保存 TTS 音频
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

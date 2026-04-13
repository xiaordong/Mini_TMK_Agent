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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	Version = "dev"

	sourceLang    string
	targetLang    string
	asrProvider   string
	transProvider string
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
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "显示原文和详细信息")

	// 将 flags 绑定到 viper
	viper.BindPFlag("asr_provider", rootCmd.PersistentFlags().Lookup("asr-provider"))
	viper.BindPFlag("trans_provider", rootCmd.PersistentFlags().Lookup("trans-provider"))

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
	cfg.Verbose = verbose

	return cfg, nil
}

func runStream(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	out := output.NewConsoleOutput(cfg.Verbose)
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

	return pipe.Run(ctx)
}

func runTranscript(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	filePath, _ := cmd.Flags().GetString("file")
	outputPath, _ := cmd.Flags().GetString("output")

	out := output.NewConsoleOutput(cfg.Verbose)
	asrProv := asr.NewProvider(cfg.ASRBaseURL, cfg.ASRAPIKey, cfg.ASRModel)
	transProv := translate.NewProvider(cfg.TransBaseURL, cfg.TransAPIKey, cfg.TransModel)

	pipe := pipeline.NewFilePipeline(pipeline.FilePipelineConfig{
		ASRProvider:   asrProv,
		TransProvider: transProv,
		Output:        out,
		SourceLang:    sourceLang,
		TargetLang:    targetLang,
	})

	return pipe.Run(context.Background(), filePath, outputPath)
}

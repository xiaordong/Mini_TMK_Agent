// Package pipeline 编排音频处理、ASR 和翻译的数据管道
package pipeline

import (
	"context"
	"fmt"
	"os"
	"strings"

	"Mini_TMK_Agent/internal/asr"
	"Mini_TMK_Agent/internal/audio"
	"Mini_TMK_Agent/internal/output"
	"Mini_TMK_Agent/internal/translate"
)

// FilePipeline 文件转录管道
type FilePipeline struct {
	asrProvider   asr.Provider
	transProvider translate.Provider
	out           output.Output
	reader        audio.Reader
	sourceLang    string
	targetLang    string
}

// FilePipelineConfig 文件管道配置
type FilePipelineConfig struct {
	ASRProvider   asr.Provider
	TransProvider translate.Provider
	Output        output.Output
	SourceLang    string
	TargetLang    string
}

// NewFilePipeline 创建文件转录管道
func NewFilePipeline(cfg FilePipelineConfig) *FilePipeline {
	return &FilePipeline{
		asrProvider:   cfg.ASRProvider,
		transProvider: cfg.TransProvider,
		out:           cfg.Output,
		reader:        audio.NewFileReader(),
		sourceLang:    cfg.SourceLang,
		targetLang:    cfg.TargetLang,
	}
}

// Run 执行文件转录
func (p *FilePipeline) Run(ctx context.Context, filePath, outputPath string) error {
	p.out.OnInfo(fmt.Sprintf("读取文件: %s", filePath))

	// 读取音频文件
	pcmData, info, err := p.reader.Read(filePath)
	if err != nil {
		return fmt.Errorf("读取音频文件失败: %w", err)
	}

	p.out.OnInfo(fmt.Sprintf("音频时长: %.1f秒, 采样率: %dHz", info.Duration, info.SampleRate))

	// 用 VAD 按静音边界分段
	segments := p.segmentByVAD(pcmData)
	p.out.OnInfo(fmt.Sprintf("检测到 %d 个语音段", len(segments)))

	var fileContent strings.Builder

	for i, segment := range segments {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if len(segment) < 3200 { // 至少 100ms 数据
			continue
		}

		// ASR
		p.out.OnInfo(fmt.Sprintf("转录第 %d/%d 段...", i+1, len(segments)))
		result, err := p.asrProvider.Transcribe(ctx, segment, p.sourceLang)
		if err != nil {
			p.out.OnError(fmt.Sprintf("第 %d 段转录失败: %v", i+1, err))
			continue
		}

		if result.Text == "" {
			continue
		}

		fileContent.WriteString("[原文] " + result.Text + "\n")
		p.out.OnSourceText(result.Text)

		// 翻译
		p.out.OnInfo(fmt.Sprintf("翻译第 %d/%d 段...", i+1, len(segments)))
		var transText strings.Builder
		err = p.transProvider.Translate(ctx, result.Text, result.Language, p.targetLang, func(chunk string) {
			transText.WriteString(chunk)
			p.out.OnTranslatedText(chunk)
		})
		if err != nil {
			p.out.OnError(fmt.Sprintf("第 %d 段翻译失败: %v", i+1, err))
			continue
		}
		p.out.OnTranslationEnd()

		fileContent.WriteString("[译文] " + transText.String() + "\n\n")
	}

	// 写入输出文件
	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(fileContent.String()), 0644); err != nil {
			return fmt.Errorf("写入输出文件失败: %w", err)
		}
		p.out.OnInfo(fmt.Sprintf("结果已写入: %s", outputPath))
	}

	return nil
}

// segmentByVAD 用 VAD 按静音边界将 PCM 数据分段
func (p *FilePipeline) segmentByVAD(pcmData []byte) [][]byte {
	vad := audio.NewVAD(200, 800) // 200ms 帧，800ms 静音判定句子结束
	vad.SetThreshold(300)         // 文件模式用固定阈值，跳过校准
	frameSize := vad.FrameSize()

	var segments [][]byte

	for offset := 0; offset+frameSize <= len(pcmData); offset += frameSize {
		frame := pcmData[offset : offset+frameSize]
		sentence, complete := vad.Process(frame)
		if complete && len(sentence) > 0 {
			segments = append(segments, sentence)
		}
	}

	// 取出缓冲区中剩余的语音数据
	if remaining := vad.Flush(); len(remaining) > 0 {
		segments = append(segments, remaining)
	}

	return segments
}

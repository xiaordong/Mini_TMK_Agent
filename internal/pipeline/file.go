// Package pipeline 编排音频处理、ASR 和翻译的数据管道
package pipeline

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"Mini_TMK_Agent/internal/asr"
	"Mini_TMK_Agent/internal/audio"
	"Mini_TMK_Agent/internal/output"
	"Mini_TMK_Agent/internal/translate"
)

// segmentResult 单段处理结果
type segmentResult struct {
	index   int
	text    string
	lang    string
	trans   string
	asrErr  error
	transErr error
}

// FilePipeline 文件转录管道
type FilePipeline struct {
	asrProvider   asr.Provider
	transProvider translate.Provider
	out           output.Output
	reader        audio.Reader
	sourceLang    string
	targetLang    string
	concurrency   int // 并发数
}

// FilePipelineConfig 文件管道配置
type FilePipelineConfig struct {
	ASRProvider   asr.Provider
	TransProvider translate.Provider
	Output        output.Output
	SourceLang    string
	TargetLang    string
	Concurrency   int // 并发数，默认 4
}

// NewFilePipeline 创建文件转录管道
func NewFilePipeline(cfg FilePipelineConfig) *FilePipeline {
	cc := cfg.Concurrency
	if cc <= 0 {
		cc = 4
	}
	return &FilePipeline{
		asrProvider:   cfg.ASRProvider,
		transProvider: cfg.TransProvider,
		out:           cfg.Output,
		reader:        audio.NewFileReader(),
		sourceLang:    cfg.SourceLang,
		targetLang:    cfg.TargetLang,
		concurrency:   cc,
	}
}

// Run 执行文件转录（ASR + 翻译并发处理）
func (p *FilePipeline) Run(ctx context.Context, filePath, outputPath string) error {
	p.out.OnInfo(fmt.Sprintf("读取文件: %s", filePath))

	pcmData, info, err := p.reader.Read(filePath)
	if err != nil {
		return fmt.Errorf("读取音频文件失败: %w", err)
	}

	p.out.OnInfo(fmt.Sprintf("音频时长: %.1f秒, 采样率: %dHz", info.Duration, info.SampleRate))

	segments := p.segmentByVAD(pcmData)
	total := len(segments)
	p.out.OnInfo(fmt.Sprintf("检测到 %d 个语音段，并发处理中...", total))

	// 并发处理每段：ASR + 翻译
	results := make([]segmentResult, total)
	sem := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup

	for i, seg := range segments {
		if len(seg) < 3200 {
			results[i] = segmentResult{index: i}
			continue
		}

		wg.Add(1)
		sem <- struct{}{} // 获取信号量
		go func(idx int, data []byte) {
			defer wg.Done()
			defer func() { <-sem }()

			r := segmentResult{index: idx}

			// ASR
			p.out.OnInfo(fmt.Sprintf("转录第 %d/%d 段...", idx+1, total))
			asrResult, err := p.asrProvider.Transcribe(ctx, data, p.sourceLang)
			if err != nil {
				r.asrErr = err
				p.out.OnError(fmt.Sprintf("第 %d 段转录失败: %v", idx+1, err))
				results[idx] = r
				return
			}
			if asrResult.Text == "" {
				results[idx] = r
				return
			}
			r.text = asrResult.Text
			r.lang = asrResult.Language

			// 翻译
			p.out.OnInfo(fmt.Sprintf("翻译第 %d/%d 段...", idx+1, total))
			var transText strings.Builder
			err = p.transProvider.Translate(ctx, asrResult.Text, asrResult.Language, p.targetLang, func(chunk string) {
				transText.WriteString(chunk)
			})
			if err != nil {
				r.transErr = err
				p.out.OnError(fmt.Sprintf("第 %d 段翻译失败: %v", idx+1, err))
				results[idx] = r
				return
			}
			r.trans = transText.String()
			results[idx] = r
		}(i, seg)
	}

	wg.Wait()

	// 按顺序输出结果
	var fileContent strings.Builder
	for _, r := range results {
		if r.text == "" {
			continue
		}

		p.out.OnSourceText(r.text)
		if r.trans != "" {
			p.out.OnTranslatedText(r.trans)
			p.out.OnTranslationEnd()
		}

		fileContent.WriteString("[原文] " + r.text + "\n")
		if r.trans != "" {
			fileContent.WriteString("[译文] " + r.trans + "\n")
		}
		fileContent.WriteString("\n")
	}

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
	vad := audio.NewVAD(200, 800)
	vad.SetThreshold(300)
	frameSize := vad.FrameSize()

	var segments [][]byte

	for offset := 0; offset+frameSize <= len(pcmData); offset += frameSize {
		frame := pcmData[offset : offset+frameSize]
		sentence, complete := vad.Process(frame)
		if complete && len(sentence) > 0 {
			segments = append(segments, sentence)
		}
	}

	if remaining := vad.Flush(); len(remaining) > 0 {
		segments = append(segments, remaining)
	}

	return segments
}

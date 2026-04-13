package output

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"Mini_TMK_Agent/internal/tts"
)

// TTSOutputConfig TTSOutput 配置
type TTSOutputConfig struct {
	Provider   tts.Provider
	TargetLang string
	OutputPath string // 文件模式：输出文件路径（用于推导 mp3 路径）
}

// TTSOutput 装饰器：在 ConsoleOutput 基础上增加 TTS 语音合成
type TTSOutput struct {
	inner      *ConsoleOutput
	provider   tts.Provider
	targetLang string
	mu         sync.Mutex
	buf        strings.Builder
	segments   [][]byte // 收集每段合成结果
	outputPath string   // 文件模式的输出路径
	segIndex   int      // 流模式的段序号
}

// NewTTSOutput 创建 TTS 输出装饰器
func NewTTSOutput(inner *ConsoleOutput, cfg TTSOutputConfig) *TTSOutput {
	return &TTSOutput{
		inner:      inner,
		provider:   cfg.Provider,
		targetLang: cfg.TargetLang,
		outputPath: cfg.OutputPath,
	}
}

func (o *TTSOutput) OnSourceText(text string) {
	o.inner.OnSourceText(text)
}

func (o *TTSOutput) OnTranslatedText(chunk string) {
	o.mu.Lock()
	o.buf.WriteString(chunk)
	o.mu.Unlock()
	o.inner.OnTranslatedText(chunk)
}

func (o *TTSOutput) OnTranslationEnd() {
	o.inner.OnTranslationEnd()

	o.mu.Lock()
	text := o.buf.String()
	o.buf.Reset()
	o.mu.Unlock()

	if text == "" || o.provider == nil {
		return
	}

	// 同步调用 TTS 合成
	o.inner.OnInfo(fmt.Sprintf("TTS 合成中..."))
	data, err := o.provider.Synthesize(context.Background(), text, o.targetLang)
	if err != nil {
		o.inner.OnError(fmt.Sprintf("TTS 合成失败: %v", err))
		return
	}

	o.mu.Lock()
	o.segments = append(o.segments, data)
	o.mu.Unlock()
}

func (o *TTSOutput) OnInfo(msg string) {
	o.inner.OnInfo(msg)
}

func (o *TTSOutput) OnError(msg string) {
	o.inner.OnError(msg)
}

// Flush 合并所有音频段并写入文件
// 文件模式：合并为单个 MP3 文件；流模式：每段独立保存
// 在 Pipeline.Run() 结束后调用
func (o *TTSOutput) Flush() error {
	o.mu.Lock()
	segments := o.segments
	outputPath := o.outputPath
	o.mu.Unlock()

	if len(segments) == 0 {
		return nil
	}

	if outputPath != "" {
		// 文件模式：合并所有段为单个 MP3
		mp3Path := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".mp3"
		var merged []byte
		for _, seg := range segments {
			merged = append(merged, seg...)
		}
		if err := os.WriteFile(mp3Path, merged, 0644); err != nil {
			return fmt.Errorf("写入 TTS 音频文件失败: %w", err)
		}
		o.inner.OnInfo(fmt.Sprintf("TTS 音频已写入: %s", mp3Path))
	} else {
		// 流模式：每段独立保存
		for i, seg := range segments {
			name := fmt.Sprintf("segment_%03d.mp3", i+1)
			if err := os.WriteFile(name, seg, 0644); err != nil {
				return fmt.Errorf("写入 TTS 音频文件失败: %w", err)
			}
			o.inner.OnInfo(fmt.Sprintf("TTS 音频已写入: %s", name))
		}
	}

	return nil
}

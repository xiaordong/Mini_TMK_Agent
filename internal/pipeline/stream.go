package pipeline

import (
	"context"
	"fmt"
	"sync"

	"Mini_TMK_Agent/internal/asr"
	"Mini_TMK_Agent/internal/audio"
	"Mini_TMK_Agent/internal/output"
	"Mini_TMK_Agent/internal/translate"
)

// StreamPipeline 流式同传管道
type StreamPipeline struct {
	asrProvider   asr.Provider
	transProvider translate.Provider
	out           output.Output
	capturer      audio.Capturer
	sourceLang    string
	targetLang    string
}

// StreamPipelineConfig 流式管道配置
type StreamPipelineConfig struct {
	ASRProvider   asr.Provider
	TransProvider translate.Provider
	Output        output.Output
	Capturer      audio.Capturer
	SourceLang    string
	TargetLang    string
}

// NewStreamPipeline 创建流式同传管道
func NewStreamPipeline(cfg StreamPipelineConfig) *StreamPipeline {
	return &StreamPipeline{
		asrProvider:   cfg.ASRProvider,
		transProvider: cfg.TransProvider,
		out:           cfg.Output,
		capturer:      cfg.Capturer,
		sourceLang:    cfg.SourceLang,
		targetLang:    cfg.TargetLang,
	}
}

// Run 启动流式同传
func (p *StreamPipeline) Run(ctx context.Context) error {
	audioChan := make(chan []byte, 100)
	speechChan := make(chan []byte, 10)
	textChan := make(chan *asr.Result, 10)

	var wg sync.WaitGroup
	wg.Add(4)

	// goroutine 1: 麦克风采集 → audioChan
	go func() {
		defer wg.Done()
		p.captureLoop(ctx, audioChan)
	}()

	// goroutine 2: VAD 切句 → speechChan
	go func() {
		defer wg.Done()
		p.vadLoop(ctx, audioChan, speechChan)
	}()

	// goroutine 3: ASR 识别 → textChan
	go func() {
		defer wg.Done()
		p.asrLoop(ctx, speechChan, textChan)
	}()

	// goroutine 4: 翻译 → 输出
	go func() {
		defer wg.Done()
		p.translateLoop(ctx, textChan)
	}()

	p.out.OnInfo("同声传译已启动，请开始说话...")

	// 等待上下文取消
	<-ctx.Done()
	p.capturer.Stop()
	close(audioChan)
	wg.Wait()

	return nil
}

// captureLoop 麦克风采集循环
func (p *StreamPipeline) captureLoop(ctx context.Context, audioChan chan<- []byte) {
	err := p.capturer.Start(func(data []byte) {
		select {
		case audioChan <- data:
		case <-ctx.Done():
		}
	})
	if err != nil {
		p.out.OnError(fmt.Sprintf("麦克风启动失败: %v", err))
		return
	}
}

// vadLoop VAD 切句循环
func (p *StreamPipeline) vadLoop(ctx context.Context, audioChan <-chan []byte, speechChan chan<- []byte) {
	vad := audio.NewVAD(200, 1200) // 200ms 帧，1.2s 静音判定
	defer close(speechChan)

	for frame := range audioChan {
		// VAD 需要固定帧大小
		frameSize := vad.FrameSize()
		if len(frame) < frameSize {
			continue
		}

		// 处理整帧
		sentence, complete := vad.Process(frame[:frameSize])
		if complete && len(sentence) > 0 {
			select {
			case speechChan <- sentence:
			case <-ctx.Done():
				return
			}
		}
	}
}

// asrLoop ASR 识别循环
func (p *StreamPipeline) asrLoop(ctx context.Context, speechChan <-chan []byte, textChan chan<- *asr.Result) {
	defer close(textChan)

	for speech := range speechChan {
		result, err := p.asrProvider.Transcribe(ctx, speech, p.sourceLang)
		if err != nil {
			p.out.OnError(fmt.Sprintf("ASR 失败: %v", err))
			continue
		}
		if result.Text == "" {
			continue
		}

		select {
		case textChan <- result:
		case <-ctx.Done():
			return
		}
	}
}

// translateLoop 翻译输出循环
func (p *StreamPipeline) translateLoop(ctx context.Context, textChan <-chan *asr.Result) {
	for result := range textChan {
		p.out.OnSourceText(result.Text)

		err := p.transProvider.Translate(ctx, result.Text, result.Language, p.targetLang, func(chunk string) {
			p.out.OnTranslatedText(chunk)
		})
		if err != nil {
			p.out.OnError(fmt.Sprintf("翻译失败: %v", err))
			continue
		}
		p.out.OnTranslationEnd()
	}
}

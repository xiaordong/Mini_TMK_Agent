package pipeline

import (
	"context"
	"fmt"
	"log"
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

	go func() {
		defer wg.Done()
		p.captureLoop(ctx, audioChan)
	}()
	go func() {
		defer wg.Done()
		p.vadLoop(ctx, audioChan, speechChan)
	}()
	go func() {
		defer wg.Done()
		p.asrLoop(ctx, speechChan, textChan)
	}()
	go func() {
		defer wg.Done()
		p.translateLoop(ctx, textChan)
	}()

	p.out.OnInfo("同声传译已启动，请开始说话...")

	<-ctx.Done()
	p.out.OnInfo("正在停止...")

	// 先停止采集（停止后回调不会再触发），再关闭 channel
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
			// ctx 取消后不再发送，避免向已关闭 channel 写入
		}
	})
	if err != nil {
		p.out.OnError(fmt.Sprintf("麦克风启动失败: %v", err))
		return
	}
	// Start 返回说明采集已结束，此时安全关闭 channel
	// 但由 Run() 负责关闭，这里不需要关
}

// vadLoop VAD 切句循环
func (p *StreamPipeline) vadLoop(ctx context.Context, audioChan <-chan []byte, speechChan chan<- []byte) {
	vad := audio.NewVAD(200, 1200)
	defer vad.Destroy()
	defer close(speechChan)

	frameSize := vad.FrameSize()
	log.Printf("[VAD] 模式: %s", vad.VADMode())

	// 累积缓冲区：malgo 每次回调长度不固定，需要攒够 frameSize 再处理
	var buf []byte

	for frame := range audioChan {
		buf = append(buf, frame...)

		// 逐帧处理缓冲区数据
		for len(buf) >= frameSize {
			sentence, complete := vad.Process(buf[:frameSize])
			buf = buf[frameSize:]

			if complete && len(sentence) > 0 {
				select {
				case speechChan <- sentence:
				case <-ctx.Done():
					return
				}
			}
		}
	}

	// 处理残余数据
	if len(buf) > 0 {
		padded := make([]byte, frameSize)
		copy(padded, buf)
		if sentence, complete := vad.Process(padded); complete && len(sentence) > 0 {
			select {
			case speechChan <- sentence:
			default:
			}
		}
	}
	if remaining := vad.Flush(); len(remaining) > 0 {
		select {
		case speechChan <- remaining:
		default:
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

		// 优先使用 ASR 检测到的语言，若为空则用用户指定的源语言
		srcLang := result.Language
		if srcLang == "" || srcLang == "auto" {
			srcLang = p.sourceLang
		}

		err := p.transProvider.Translate(ctx, result.Text, srcLang, p.targetLang, func(chunk string) {
			p.out.OnTranslatedText(chunk)
		})
		if err != nil && ctx.Err() == nil {
			p.out.OnError(fmt.Sprintf("翻译失败: %v", err))
			continue
		}
		p.out.OnTranslationEnd()
	}
}

package pipeline

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"Mini_TMK_Agent/internal/asr"
	"Mini_TMK_Agent/internal/audio"
	"Mini_TMK_Agent/internal/output"
	"Mini_TMK_Agent/internal/translate"
)

const (
	defaultTranslateWorkers = 3
	translateTaskTimeout    = 30 * time.Second
	maxTranslateWorkers     = 5
)

type translateTask struct {
	index   int
	text    string
	srcLang string
}

type translateResult struct {
	index int
	text  string
	err   error
}

// StreamPipeline 流式同传管道
type StreamPipeline struct {
	asrProvider   asr.Provider
	transProvider translate.Provider
	out           output.Output
	capturer      audio.Capturer
	sourceLang    string
	targetLang    string
	workers       int
}

// StreamPipelineConfig 流式管道配置
type StreamPipelineConfig struct {
	ASRProvider   asr.Provider
	TransProvider translate.Provider
	Output        output.Output
	Capturer      audio.Capturer
	SourceLang    string
	TargetLang    string
	Workers       int
}

// NewStreamPipeline 创建流式同传管道
func NewStreamPipeline(cfg StreamPipelineConfig) *StreamPipeline {
	workers := cfg.Workers
	if workers <= 0 {
		workers = defaultTranslateWorkers
	}
	if workers > maxTranslateWorkers {
		workers = maxTranslateWorkers
	}
	return &StreamPipeline{
		asrProvider:   cfg.ASRProvider,
		transProvider: cfg.TransProvider,
		out:           cfg.Output,
		capturer:      cfg.Capturer,
		sourceLang:    cfg.SourceLang,
		targetLang:    cfg.TargetLang,
		workers:       workers,
	}
}

// Run 启动流式同传
func (p *StreamPipeline) Run(ctx context.Context) error {
	audioChan := make(chan []byte, 100)
	speechChan := make(chan []byte, 10)
	textChan := make(chan *asr.Result, 10)
	taskChan := make(chan *translateTask, p.workers)
	resultChan := make(chan *translateResult, p.workers)

	var wg sync.WaitGroup
	// captureLoop + vadLoop + asrLoop + translateDispatch + N workers
	wg.Add(4 + p.workers)

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
		p.translateDispatch(ctx, textChan, taskChan)
	}()
	for i := 0; i < p.workers; i++ {
		go func() {
			defer wg.Done()
			p.translateWorker(ctx, taskChan, resultChan)
		}()
	}

	// outputLoop 不进 wg，通过 close(resultChan) 退出
	go p.outputLoop(resultChan)

	p.out.OnInfo("同声传译已启动，请开始说话...")

	<-ctx.Done()
	p.out.OnInfo("正在停止...")

	// 先停止采集（停止后回调不会再触发），再关闭 channel
	p.capturer.Stop()
	close(audioChan)

	// 等待所有上游 goroutine 退出，然后关闭 resultChan 让 outputLoop 退出
	wg.Wait()
	close(resultChan)

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

// translateDispatch 分发翻译任务（单 goroutine，保证 OnSourceText 顺序正确）
func (p *StreamPipeline) translateDispatch(ctx context.Context, textChan <-chan *asr.Result, taskChan chan<- *translateTask) {
	defer close(taskChan)

	index := 0
	for result := range textChan {
		p.out.OnSourceText(result.Text)

		srcLang := result.Language
		if srcLang == "" || srcLang == "auto" {
			srcLang = p.sourceLang
		}

		task := &translateTask{index: index, text: result.Text, srcLang: srcLang}
		index++

		select {
		case taskChan <- task:
		case <-ctx.Done():
			return
		}
	}
}

// translateWorker 翻译 worker（N 个并行）
func (p *StreamPipeline) translateWorker(ctx context.Context, taskChan <-chan *translateTask, resultChan chan<- *translateResult) {
	for task := range taskChan {
		// per-task timeout，防止网络假死
		taskCtx, cancel := context.WithTimeout(ctx, translateTaskTimeout)

		var translated string
		err := p.transProvider.Translate(taskCtx, task.text, task.srcLang, p.targetLang, func(chunk string) {
			translated = chunk
		})
		cancel()

		if err != nil {
			// 主 context 已取消时丢弃结果，加速退出
			if ctx.Err() != nil {
				continue
			}
			resultChan <- &translateResult{index: task.index, err: err}
			continue
		}
		resultChan <- &translateResult{index: task.index, text: translated}
	}
}

// outputLoop 输出循环（单 goroutine，所有 Output 调用串行，无需加锁）
func (p *StreamPipeline) outputLoop(resultChan <-chan *translateResult) {
	for result := range resultChan {
		if result.err != nil {
			p.out.OnError(fmt.Sprintf("翻译失败: %v", result.err))
			continue
		}
		p.out.OnTranslatedText(result.text)
		p.out.OnTranslationEnd()
	}
}

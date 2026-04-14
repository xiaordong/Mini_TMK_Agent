package web

import (
	"context"
	"encoding/base64"
	"net/http"

	"Mini_TMK_Agent/internal/asr"
	"Mini_TMK_Agent/internal/pipeline"
	"Mini_TMK_Agent/internal/translate"
	"Mini_TMK_Agent/internal/tts"
)

// handleStream 处理 WebSocket 实时流式同传
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 从 query 参数读取语言设置
	sourceLang := r.URL.Query().Get("source")
	targetLang := r.URL.Query().Get("target")
	if sourceLang == "" {
		sourceLang = "auto"
	}
	if targetLang == "" {
		targetLang = "zh"
	}

	cfg := s.cfg

	// 创建 WebSocket 采集器和输出
	capturer := NewWebSocketCapturer(conn)
	webOut := NewWebOutput(conn)

	// 创建管道
	asrProv := asr.NewProvider(cfg.ASRBaseURL, cfg.ASRAPIKey, cfg.ASRModel)
	transProv := translate.NewProvider(cfg.TransBaseURL, cfg.TransAPIKey, cfg.TransModel)

	var out interface {
		OnSourceText(string)
		OnTranslatedText(string)
		OnTranslationEnd()
		OnInfo(string)
		OnError(string)
	} = webOut

	// TTS 包装（可选）
	if cfg.TTSEnabled {
		ttsProv, err := tts.NewProvider(cfg.TTSProvider, cfg.TTSBaseURL, cfg.TTSAPIKey, cfg.TTSModel, cfg.TTSVoice)
		if err == nil {
			out = &webTTSOutput{
				inner:      webOut,
				provider:   ttsProv,
				targetLang: targetLang,
			}
		}
	}

	pipe := pipeline.NewStreamPipeline(pipeline.StreamPipelineConfig{
		ASRProvider:   asrProv,
		TransProvider: transProv,
		Output:        out,
		Capturer:      capturer,
		SourceLang:    sourceLang,
		TargetLang:    targetLang,
	})

	// 运行管道（阻塞直到连接关闭）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = pipe.Run(ctx)
}

// webTTSOutput Web 模式的 TTS 装饰器（通过 WebSocket 发送音频）
type webTTSOutput struct {
	inner      *WebOutput
	provider   tts.Provider
	targetLang string
	curText    []byte
}

func (o *webTTSOutput) OnSourceText(text string) {
	o.inner.OnSourceText(text)
}

func (o *webTTSOutput) OnTranslatedText(chunk string) {
	o.curText = append(o.curText, chunk...)
	o.inner.OnTranslatedText(chunk)
}

func (o *webTTSOutput) OnTranslationEnd() {
	o.inner.OnTranslationEnd()

	text := string(o.curText)
	o.curText = o.curText[:0]

	if text == "" || o.provider == nil {
		return
	}

	// 异步合成 TTS 音频并发送
	data, err := o.provider.Synthesize(context.Background(), text, o.targetLang)
	if err != nil {
		o.inner.OnError("TTS 合成失败: " + err.Error())
		return
	}

	o.inner.SendTTSAudio(base64.StdEncoding.EncodeToString(data))
}

func (o *webTTSOutput) OnInfo(msg string)  { o.inner.OnInfo(msg) }
func (o *webTTSOutput) OnError(msg string) { o.inner.OnError(msg) }

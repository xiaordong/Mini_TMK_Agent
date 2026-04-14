package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gorilla/websocket"

	"Mini_TMK_Agent/internal/asr"
	"Mini_TMK_Agent/internal/config"
	"Mini_TMK_Agent/internal/pipeline"
	"Mini_TMK_Agent/internal/translate"
	"Mini_TMK_Agent/internal/tts"
)

// transcriptResponse 文件转录 REST API 响应
type transcriptResponse struct {
	Items []TranscriptItem `json:"items"`
	Error string           `json:"error,omitempty"`
}

// configResponse 配置 API 响应（API Key 脱敏）
type configResponse struct {
	ASRProvider   string `json:"asr_provider"`
	ASRBaseURL    string `json:"asr_base_url"`
	ASRAPIKeySet  bool   `json:"asr_api_key_set"`
	ASRModel      string `json:"asr_model"`

	TransProvider string `json:"trans_provider"`
	TransBaseURL  string `json:"trans_base_url"`
	TransAPIKeySet bool  `json:"trans_api_key_set"`
	TransModel    string `json:"trans_model"`

	TTSEnabled    bool   `json:"tts_enabled"`
	TTSProvider   string `json:"tts_provider"`
	TTSVoice      string `json:"tts_voice"`
	TTSAPIKeySet  bool   `json:"tts_api_key_set"`

	SourceLang    string `json:"source_lang"`
	TargetLang    string `json:"target_lang"`
}

// configUpdateRequest 配置更新请求
type configUpdateRequest struct {
	ASRProvider   *string `json:"asr_provider,omitempty"`
	TransProvider *string `json:"trans_provider,omitempty"`
	TTSEnabled    *bool   `json:"tts_enabled,omitempty"`
	TTSProvider   *string `json:"tts_provider,omitempty"`
	TTSVoice      *string `json:"tts_voice,omitempty"`
	SourceLang    *string `json:"source_lang,omitempty"`
	TargetLang    *string `json:"target_lang,omitempty"`

	// 运行时动态设置 API Key（可选）
	ASRAPIKey   *string `json:"asr_api_key,omitempty"`
	TransAPIKey *string `json:"trans_api_key,omitempty"`
	TTSAPIKey   *string `json:"tts_api_key,omitempty"`
}

// handleWsTranscript 通过 WebSocket 处理文件转录（流式推送进度和结果）
func (s *Server) handleWsTranscript(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 等待客户端发送文件（第一条消息为二进制文件数据）
	msgType, data, err := conn.ReadMessage()
	if err != nil {
		return
	}

	// 第一条消息也可以是 JSON 元数据（文件大小、参数等），简化处理：直接发二进制文件
	_ = msgType

	// 读取 query 参数
	sourceLang := r.URL.Query().Get("source")
	targetLang := r.URL.Query().Get("target")
	fileName := r.URL.Query().Get("filename")
	if sourceLang == "" {
		sourceLang = "auto"
	}
	if targetLang == "" {
		targetLang = "zh"
	}

	// 根据上传文件名确定扩展名，创建带扩展名的临时文件
	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".wav"
	}

	tmpFile, err := os.CreateTemp("", "tmk-ws-*"+ext)
	if err != nil {
		sendWsError(conn, "创建临时文件失败")
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	tmpFile.Write(data)
	tmpFile.Close()

	cfg := s.cfg
	asrProv := asr.NewProvider(cfg.ASRBaseURL, cfg.ASRAPIKey, cfg.ASRModel)
	transProv := translate.NewProvider(cfg.TransBaseURL, cfg.TransAPIKey, cfg.TransModel)

	webOut := &wsTranscriptOutput{conn: conn}

	pipe := pipeline.NewFilePipeline(pipeline.FilePipelineConfig{
		ASRProvider:   asrProv,
		TransProvider: transProv,
		Output:        webOut,
		SourceLang:    sourceLang,
		TargetLang:    targetLang,
		Concurrency:   2,
	})

	if err := pipe.Run(context.Background(), tmpPath, ""); err != nil {
		sendWsError(conn, err.Error())
		return
	}

	// 如果启用 TTS，逐段合成并发送
	if cfg.TTSEnabled {
		ttsProv, err := tts.NewProvider(cfg.TTSProvider, cfg.TTSBaseURL, cfg.TTSAPIKey, cfg.TTSModel, cfg.TTSVoice)
		if err == nil {
			for _, item := range webOut.items {
				if item.Translation == "" {
					continue
				}
				audioData, err := ttsProv.Synthesize(context.Background(), item.Translation, targetLang)
				if err == nil {
					_ = conn.WriteJSON(WebMessage{
						Type:    "tts_audio",
						Content: base64.StdEncoding.EncodeToString(audioData),
						Index:   item.Index,
					})
				}
			}
		}
	}

	// 发送完成信号
	_ = conn.WriteJSON(WebMessage{Type: "done"})
}

// wsTranscriptOutput 文件转录的 WebSocket 输出（每完成一段立即推送）
type wsTranscriptOutput struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	items   []TranscriptItem
	cur     strings.Builder
	index   int
}

func (o *wsTranscriptOutput) send(msg WebMessage) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.conn.WriteJSON(msg)
}

func (o *wsTranscriptOutput) OnSourceText(text string) {
	o.mu.Lock()
	o.index++
	o.cur.Reset()
	o.items = append(o.items, TranscriptItem{Source: text, Index: o.index})
	o.mu.Unlock()

	_ = o.send(WebMessage{Type: "source", Content: text, Index: o.index})
}

func (o *wsTranscriptOutput) OnTranslatedText(chunk string) {
	o.mu.Lock()
	o.cur.WriteString(chunk)
	o.mu.Unlock()

	_ = o.send(WebMessage{Type: "translated", Content: chunk, Index: o.index})
}

func (o *wsTranscriptOutput) OnTranslationEnd() {
	o.mu.Lock()
	var translation string
	if len(o.items) > 0 {
		translation = o.cur.String()
		o.items[len(o.items)-1].Translation = translation
	}
	o.cur.Reset()
	o.mu.Unlock()

	_ = o.send(WebMessage{Type: "end", Index: o.index})
}

func (o *wsTranscriptOutput) OnInfo(msg string) {
	_ = o.send(WebMessage{Type: "progress", Content: msg})
}

func (o *wsTranscriptOutput) OnError(msg string) {
	_ = o.send(WebMessage{Type: "error", Content: msg})
}

// sendWsError 发送错误消息并关闭
func sendWsError(conn *websocket.Conn, msg string) {
	_ = conn.WriteJSON(WebMessage{Type: "error", Content: msg})
}

// handleTranscript REST 文件转录（保留，用于简单场景）
func (s *Server) handleTranscript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, transcriptResponse{Error: fmt.Sprintf("解析表单失败: %v", err)})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, transcriptResponse{Error: "未找到上传文件"})
		return
	}
	defer file.Close()

	tmpFile, err := os.CreateTemp("", "tmk-upload-*"+filepath.Ext(header.Filename))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, transcriptResponse{Error: "创建临时文件失败"})
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, file); err != nil {
		tmpFile.Close()
		writeJSON(w, http.StatusInternalServerError, transcriptResponse{Error: "保存文件失败"})
		return
	}
	tmpFile.Close()

	sourceLang := r.FormValue("source_lang")
	targetLang := r.FormValue("target_lang")
	if sourceLang == "" {
		sourceLang = "auto"
	}
	if targetLang == "" {
		targetLang = "zh"
	}

	cfg := s.cfg
	asrProv := asr.NewProvider(cfg.ASRBaseURL, cfg.ASRAPIKey, cfg.ASRModel)
	transProv := translate.NewProvider(cfg.TransBaseURL, cfg.TransAPIKey, cfg.TransModel)

	collector := NewCollectorOutput()

	pipe := pipeline.NewFilePipeline(pipeline.FilePipelineConfig{
		ASRProvider:   asrProv,
		TransProvider: transProv,
		Output:        collector,
		SourceLang:    sourceLang,
		TargetLang:    targetLang,
	})

	if err := pipe.Run(context.Background(), tmpPath, ""); err != nil {
		writeJSON(w, http.StatusInternalServerError, transcriptResponse{Error: err.Error()})
		return
	}

	items := collector.Results()

	if cfg.TTSEnabled {
		ttsProv, err := tts.NewProvider(cfg.TTSProvider, cfg.TTSBaseURL, cfg.TTSAPIKey, cfg.TTSModel, cfg.TTSVoice)
		if err == nil {
			for i, item := range items {
				if item.Translation == "" {
					continue
				}
				audioData, err := ttsProv.Synthesize(context.Background(), item.Translation, targetLang)
				if err == nil {
					items[i].TTSAudio = base64.StdEncoding.EncodeToString(audioData)
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, transcriptResponse{Items: items})
}

// handleGetConfig 返回配置（API Key 脱敏）
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	cfg := *s.cfg // 值拷贝，避免持有锁期间被修改
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, configResponse{
		ASRProvider:   cfg.ASRProvider,
		ASRBaseURL:    cfg.ASRBaseURL,
		ASRAPIKeySet:  cfg.ASRAPIKey != "",
		ASRModel:      cfg.ASRModel,
		TransProvider: cfg.TransProvider,
		TransBaseURL:  cfg.TransBaseURL,
		TransAPIKeySet: cfg.TransAPIKey != "",
		TransModel:    cfg.TransModel,
		TTSEnabled:    cfg.TTSEnabled,
		TTSProvider:   cfg.TTSProvider,
		TTSVoice:      cfg.TTSVoice,
		TTSAPIKeySet:  cfg.TTSAPIKey != "",
		SourceLang:    cfg.SourceLang,
		TargetLang:    cfg.TargetLang,
	})
}

// handleUpdateConfig 更新运行时配置
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req configUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的 JSON"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.cfg

	if req.ASRProvider != nil {
		p := strings.ToLower(*req.ASRProvider)
		cfg.ASRProvider = p
		if preset, ok := config.ProviderDefaults[p]; ok {
			cfg.ASRBaseURL = preset.BaseURL
			cfg.ASRModel = preset.Model
		}
	}
	if req.TransProvider != nil {
		p := strings.ToLower(*req.TransProvider)
		cfg.TransProvider = p
		if preset, ok := config.TransDefaults[p]; ok {
			cfg.TransBaseURL = preset.BaseURL
			cfg.TransModel = preset.Model
		}
	}
	if req.TTSEnabled != nil {
		cfg.TTSEnabled = *req.TTSEnabled
	}
	if req.TTSProvider != nil {
		p := strings.ToLower(*req.TTSProvider)
		cfg.TTSProvider = p
		if preset, ok := config.TTSDefaults[p]; ok {
			cfg.TTSBaseURL = preset.BaseURL
			cfg.TTSModel = preset.Model
		}
	}
	if req.TTSVoice != nil {
		cfg.TTSVoice = *req.TTSVoice
	}
	if req.SourceLang != nil {
		cfg.SourceLang = *req.SourceLang
	}
	if req.TargetLang != nil {
		cfg.TargetLang = *req.TargetLang
	}
	if req.ASRAPIKey != nil && *req.ASRAPIKey != "" {
		cfg.ASRAPIKey = *req.ASRAPIKey
	}
	if req.TransAPIKey != nil && *req.TransAPIKey != "" {
		cfg.TransAPIKey = *req.TransAPIKey
	}
	if req.TTSAPIKey != nil && *req.TTSAPIKey != "" {
		cfg.TTSAPIKey = *req.TTSAPIKey
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

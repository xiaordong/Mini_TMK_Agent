// Package web 提供 Web UI 的 HTTP/WS 服务端实现
package web

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// WebMessage WebSocket 消息格式
type WebMessage struct {
	Type    string `json:"type"`    // source|translated|end|info|error|tts_audio|progress|done
	Content string `json:"content"`
	Index   int    `json:"index,omitempty"`
	Total   int    `json:"total,omitempty"`   // 用于 progress 消息：总数
}

// WebOutput 通过 WebSocket 发送管道输出的 Output 实现
type WebOutput struct {
	conn  *websocket.Conn
	mu    sync.Mutex
	index int
}

// NewWebOutput 创建 WebSocket 输出
func NewWebOutput(conn *websocket.Conn) *WebOutput {
	return &WebOutput{conn: conn}
}

func (o *WebOutput) send(msg WebMessage) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.conn.WriteJSON(msg)
}

func (o *WebOutput) OnSourceText(text string) {
	o.index++
	_ = o.send(WebMessage{Type: "source", Content: text, Index: o.index})
}

func (o *WebOutput) OnTranslatedText(chunk string) {
	_ = o.send(WebMessage{Type: "translated", Content: chunk, Index: o.index})
}

func (o *WebOutput) OnTranslationEnd() {
	_ = o.send(WebMessage{Type: "end", Index: o.index})
}

func (o *WebOutput) OnInfo(msg string) {
	_ = o.send(WebMessage{Type: "info", Content: msg})
}

func (o *WebOutput) OnError(msg string) {
	_ = o.send(WebMessage{Type: "error", Content: msg})
}

// SendTTSAudio 发送 TTS 音频（base64 编码的 MP3）
func (o *WebOutput) SendTTSAudio(audioBase64 string) {
	_ = o.send(WebMessage{Type: "tts_audio", Content: audioBase64, Index: o.index})
}

// TranscriptItem 单条转录结果
type TranscriptItem struct {
	Source      string `json:"source"`
	Translation string `json:"translation"`
	Index       int    `json:"index"`
	TTSAudio    string `json:"tts_audio,omitempty"`
}

// CollectorOutput 收集文件转录结果到 slice（供 REST API 返回 JSON）
type CollectorOutput struct {
	mu    sync.Mutex
	items []TranscriptItem
	cur   strings.Builder
	index int
}

// NewCollectorOutput 创建收集器输出
func NewCollectorOutput() *CollectorOutput {
	return &CollectorOutput{}
}

func (o *CollectorOutput) OnSourceText(text string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.index++
	o.cur.Reset()
	o.items = append(o.items, TranscriptItem{Source: text, Index: o.index})
}

func (o *CollectorOutput) OnTranslatedText(chunk string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.cur.WriteString(chunk)
}

func (o *CollectorOutput) OnTranslationEnd() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.items) > 0 {
		o.items[len(o.items)-1].Translation = o.cur.String()
	}
	o.cur.Reset()
}

func (o *CollectorOutput) OnInfo(msg string)  {}
func (o *CollectorOutput) OnError(msg string) {}

// SetTTSAudio 设置最后一条结果的 TTS 音频
func (o *CollectorOutput) SetTTSAudio(audioBase64 string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.items) > 0 {
		o.items[len(o.items)-1].TTSAudio = audioBase64
	}
}

// Results 返回收集到的结果
func (o *CollectorOutput) Results() []TranscriptItem {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]TranscriptItem, len(o.items))
	copy(out, o.items)
	return out
}

// MarshalJSON 实现 json.Marshaler（方便直接返回 JSON）
func (o *CollectorOutput) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.Results())
}

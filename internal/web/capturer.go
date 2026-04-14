package web

import (
	"context"
	"sync"

	"github.com/gorilla/websocket"
)

// WebSocketCapturer 通过 WebSocket 接收浏览器 PCM 音频的采集器
// 实现 audio.Capturer 接口，让 StreamPipeline 无需修改即可复用
type WebSocketCapturer struct {
	conn   *websocket.Conn
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// NewWebSocketCapturer 创建 WebSocket 采集器
func NewWebSocketCapturer(conn *websocket.Conn) *WebSocketCapturer {
	return &WebSocketCapturer{
		conn: conn,
		done: make(chan struct{}),
	}
}

// Start 阻塞读取 WebSocket 二进制消息（16bit PCM），调用 onData 回调
// 阻塞直到连接关闭或 Stop() 被调用（与 MalgoCapturer 行为一致）
func (c *WebSocketCapturer) Start(onData func([]byte)) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go func() {
		defer close(c.done)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			msgType, data, err := c.conn.ReadMessage()
			if err != nil {
				// 连接关闭或读取错误，正常退出
				return
			}
			// 只处理二进制消息（PCM 音频数据）
			if msgType == websocket.BinaryMessage && len(data) > 0 {
				onData(data)
			}
		}
	}()

	// 阻塞等待采集结束
	<-c.done
	return nil
}

// Stop 停止采集
func (c *WebSocketCapturer) Stop() error {
	c.once.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		_ = c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	})
	<-c.done
	return nil
}

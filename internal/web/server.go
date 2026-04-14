package web

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"Mini_TMK_Agent/internal/config"

	"github.com/gorilla/websocket"
)

//go:embed static/*
var staticFS embed.FS

// Server Web UI 服务器
type Server struct {
	cfg      *config.Config
	upgrader websocket.Upgrader
	mu       sync.Mutex
}

// NewServer 创建 Web 服务器
func NewServer(cfg *config.Config) *Server {
	return &Server{
		cfg: cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Start 启动 Web 服务器（支持优雅关闭）
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	// 静态文件
	staticContent, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(staticContent)))

	// REST API
	mux.HandleFunc("/api/transcript", s.handleTranscript)
	mux.HandleFunc("/api/config", s.handleConfig)

	// WebSocket
	mux.HandleFunc("/ws/stream", s.handleStream)
	mux.HandleFunc("/ws/transcript", s.handleWsTranscript)

	srv := &http.Server{Addr: addr, Handler: mux}

	// 信号监听
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n正在关闭 Web 服务器...")
		srv.Shutdown(context.Background())
	}()

	fmt.Printf("Web UI 启动在 http://localhost%s\n", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// handleConfig 路由 GET/PUT 到对应 handler
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetConfig(w, r)
	case http.MethodPut:
		s.handleUpdateConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

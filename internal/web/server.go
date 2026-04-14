package web

import (
	"embed"
	"io/fs"
	"net/http"
	"sync"

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

// Start 启动 Web 服务器
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

	return http.ListenAndServe(addr, mux)
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

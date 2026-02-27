package engine

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
)

// Version can be set before starting the server.
var Version = "1.0.0"

//go:embed index.html
var indexHTML []byte

//go:embed apple-touch-icon.png
var appleTouchIcon []byte

// Server wraps a Game instance with an HTTP/WebSocket server.
type Server struct {
	Game       *Game
	httpServer *http.Server
	listener   net.Listener
}

// NewServer creates a new server with the given game configuration.
func NewServer(cfg GameConfig) *Server {
	return &Server{
		Game: NewGame(cfg),
	}
}

func (s *Server) setupMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		HandleWS(s.Game, w, r)
	})

	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		HandleStats(s.Game, w, r)
	})

	mux.HandleFunc("/dashboard", HandleDashboard)

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/apple-touch-icon.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(appleTouchIcon)
	})

	return mux
}

func (s *Server) logStartup(addr string) {
	log.Printf("Schlangen.TV server v%s starting...", Version)
	log.Printf("Listening on http://%s", addr)
	log.Printf("WebSocket: ws://%s/ws", addr)
	log.Printf("Dashboard: http://%s/dashboard", addr)
}

// Start starts the game loop and HTTP server in the background (non-blocking).
func (s *Server) Start(port int) error {
	go s.Game.Run()

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	s.httpServer = &http.Server{Addr: addr, Handler: s.setupMux()}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = ln

	s.logStartup(addr)

	go s.httpServer.Serve(ln)
	return nil
}

// ListenAndServe starts the game loop and HTTP server (blocks until error).
func (s *Server) ListenAndServe(port int) error {
	go s.Game.Run()

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	s.httpServer = &http.Server{Addr: addr, Handler: s.setupMux()}

	s.logStartup(addr)

	return s.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() error {
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

// GetStatsJSON returns the current game stats as a JSON string.
func (s *Server) GetStatsJSON() string {
	snap := s.Game.GetStats()
	b, _ := json.Marshal(snap)
	return string(b)
}

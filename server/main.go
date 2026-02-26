package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	port := flag.Int("port", 8080, "Server port")
	staticDir := flag.String("static", "", "Static files directory (default: auto-detect)")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime)
	log.Println("Snake.io server starting...")

	// Auto-detect static dir: prefer CWD if it has index.html, else try binary's parent dir
	if *staticDir == "" {
		cwd, _ := os.Getwd()
		if _, err := os.Stat(filepath.Join(cwd, "index.html")); err == nil {
			*staticDir = cwd
		} else {
			// Binary is in server/, so parent should have index.html
			exe, _ := os.Executable()
			binDir := filepath.Dir(exe)
			parent := filepath.Dir(binDir)
			if _, err := os.Stat(filepath.Join(parent, "index.html")); err == nil {
				*staticDir = parent
			} else {
				*staticDir = cwd
				log.Printf("WARNING: index.html not found in %s or %s", cwd, parent)
			}
		}
	}

	absStatic, _ := filepath.Abs(*staticDir)

	game := NewGame()
	go game.Run()

	// Serve static files (index.html etc.)
	fs := http.FileServer(http.Dir(absStatic))
	http.Handle("/", fs)

	// WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		HandleWS(game, w, r)
	})

	// Stats API and dashboard
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		HandleStats(game, w, r)
	})
	http.HandleFunc("/dashboard", HandleDashboard)

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("Listening on http://%s", addr)
	log.Printf("Serving static files from %s", absStatic)
	log.Printf("WebSocket: ws://%s/ws", addr)
	log.Printf("Dashboard: http://%s/dashboard", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

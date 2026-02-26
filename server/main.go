package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"
)

//go:embed index.html
var indexHTML []byte

func main() {
	port := flag.Int("port", 8080, "Server port")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime)
	log.Println("Snake.io server starting...")

	game := NewGame()
	go game.Run()

	// Serve embedded index.html
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	// WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		HandleWS(game, w, r)
	})

	// Stats API and dashboard
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		HandleStats(game, w, r)
	})
	http.HandleFunc("/dashboard", HandleDashboard)
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("Listening on http://%s", addr)
	log.Printf("WebSocket: ws://%s/ws", addr)
	log.Printf("Dashboard: http://%s/dashboard", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

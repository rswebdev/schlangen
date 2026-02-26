// Package mobile provides gomobile-compatible bindings for embedding
// the snake.io game server in iOS/tvOS/Android applications.
//
// All exported functions use only primitive types (int, string, error)
// to satisfy gomobile's type restrictions.
package mobile

import (
	"fmt"
	"net"
	"sync"

	"snake.io/engine"
)

var (
	srv  *engine.Server
	mu   sync.Mutex
	port int
)

// Start initializes and starts the snake server on the given port.
// The server runs in the background. Call Stop() to shut it down.
func Start(serverPort int) error {
	mu.Lock()
	defer mu.Unlock()

	if srv != nil {
		return fmt.Errorf("server already running")
	}

	cfg := engine.DefaultConfig()
	srv = engine.NewServer(cfg)
	port = serverPort

	return srv.Start(serverPort)
}

// Stop shuts down the running server.
func Stop() {
	mu.Lock()
	defer mu.Unlock()

	if srv != nil {
		srv.Stop()
		srv = nil
	}
}

// IsRunning returns true if the server is currently running.
func IsRunning() bool {
	mu.Lock()
	defer mu.Unlock()
	return srv != nil
}

// GetStats returns the current game stats as a JSON string.
func GetStats() string {
	mu.Lock()
	s := srv
	mu.Unlock()

	if s == nil {
		return "{}"
	}
	return s.GetStatsJSON()
}

// GetLocalIP returns the device's local network IP address.
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return "unknown"
}

// GetConnectURL returns the URL players should open on their phones.
func GetConnectURL() string {
	mu.Lock()
	p := port
	mu.Unlock()

	ip := GetLocalIP()
	return fmt.Sprintf("http://%s:%d", ip, p)
}

// GetVersion returns the server version string.
func GetVersion() string {
	return engine.Version
}

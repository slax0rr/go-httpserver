package httpserver

import (
	"net"
	"time"
)

// Config struct for the HTTP server.
type Config struct {
	// SockFile defines the location of the socket file for the listener.
	// Defaults to `/tmp/go-httpserver.sock`.
	SockFile string

	// Addr of the listener.
	// Defaults to `:3000`.
	Addr string

	// Timeout controls the shutdown timeout. If the server takes longer to shut
	// down, all open connections will be dropped.
	// Defaults to `5` seconds.
	Timeout time.Duration

	// SignalBufferSize is the size of the buffer for the system signals.
	// Defaults to 1.
	SignalBufferSize int

	ln net.Listener
}

func getConfig(cfg Config) Config {
	if cfg.SockFile == "" {
		cfg.SockFile = "/tmp/go-httpserver.sock"
	}

	if cfg.Addr == "" {
		cfg.Addr = ":3000"
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}

	if cfg.SignalBufferSize == 0 {
		cfg.SignalBufferSize = 1
	}

	return cfg
}

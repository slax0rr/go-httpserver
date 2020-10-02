package httpserver

import (
	"errors"
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
}

func (cfg Config) isValid() error {
	if cfg.SockFile == "" {
		return errors.New("socket file is required")
	}

	if cfg.Addr == "" {
		return errors.New("server address is required")
	}

	if cfg.Timeout == 0 {
		return errors.New("shutdown timeout must be greater than 0")
	}

	if cfg.SignalBufferSize < 1 {
		return errors.New("signal buffer size must be at least 1")
	}

	return nil
}

func defaultConfig() *Config {
	return &Config{
		SockFile:         "/tmp/go-httpserver.sock",
		Addr:             ":3000",
		Timeout:          5 * time.Second,
		SignalBufferSize: 1,
	}
}

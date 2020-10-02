package httpserver

import (
	"net"
	"time"
)

// Config struct for the HTTP server.
type Config struct {
	// SockFile defines the location of the socket file for the listener.
	SockFile string

	// Addr of the listener.
	Addr string

	// Timeout controls the shutdown timeout. If the server takes longer to shut
	// down, all open connections will be dropped.
	Timeout time.Duration

	ln net.Listener
}

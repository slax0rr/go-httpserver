package httpserver

import (
	"net"
	"time"
)

type Config struct {
	SockFile string
	Addr     string
	Timeout  time.Duration

	ln net.Listener
}

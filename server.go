package httpserver

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

func init() {
	defaultServer = NewServer(nil)
}

var defaultServer *server

type server struct {
	cfg     *Config
	srv     *http.Server
	handler http.Handler
	ln      net.Listener
}

// NewServer creates a new Server object.
func NewServer(handler http.Handler) *server {
	return &server{
		cfg:     defaultConfig(),
		handler: handler,
	}
}

// SetConfig overrides the default Config. If an invalid config value is used
// the server will panic.
func (s *server) SetConfig(cfg Config) {
	if err := cfg.isValid(); err != nil {
		panic(err)
	}
	s.cfg = &cfg
}

// SetHandler sets the initial HTTP handler for the server.
func (s *server) SetHandler(handler http.Handler) {
	s.handler = handler
}

// Serve starts a new listener and an HTTP server. It will also start listening
// for system signals.
//
// SIGHUP - gracefully restart the server
// SIGTERM - gracefully shutdown the server with timeout
func (s *server) Serve() error {
	var err error
	s.ln, err = getListener(s.cfg.Addr, s.cfg.SockFile)
	if err != nil {
		log.WithError(err).Panic("Unable to create or import a listener")
	}

	s.start()

	err = s.waitForSignals()
	if err != nil {
		log.WithError(err).Info("Exiting with error")
		return err
	}

	log.Info("Exiting")

	return nil
}

// Stop gracefully stops the server. If 'true' is passed, the server will be
// killed without gracefull shutting down open connections.
func (s *server) Stop(kill bool) error {
	if kill {
		return s.srv.Close()
	}

	return s.shutdown()
}

// Serve starts a new listener and the default HTTP server. It will also start
// listening for system signals.
//
// SIGHUP - gracefully restart the server
// SIGTERM - gracefully shutdown the server with timeout
func Serve(config Config, handler http.Handler) error {
	defaultServer.SetConfig(config)
	defaultServer.SetHandler(handler)
	return defaultServer.Serve()
}

// Stop gracefully stops the default server. If 'true' is passed, the server
// will be killed without gracefull shutting down open connections.
func Stop(kill bool) error {
	return defaultServer.Stop(kill)
}

func (s *server) start() {
	s.srv = &http.Server{
		Addr:    s.cfg.Addr,
		Handler: s.handler,
	}

	go s.srv.Serve(s.ln)
}

func (s *server) shutdown() error {
	log.Debug("Server shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Timeout*time.Second)
	defer cancel()

	err := s.srv.Shutdown(ctx)
	log.WithError(err).Debug("Server shut down")

	return err
}

func (s *server) fork() (*os.Process, error) {
	// get the listener file
	lnFile, err := getListenerFile(s.ln)
	if err != nil {
		return nil, err
	}
	defer lnFile.Close()

	// pass the stdin, stdout, stderr, and the listener files to the child
	files := []*os.File{
		os.Stdin,
		os.Stdout,
		os.Stderr,
		lnFile,
	}

	// get process name and dir
	execName, err := os.Executable()
	if err != nil {
		return nil, err
	}
	execDir := filepath.Dir(execName)

	// spawn a child
	p, err := os.StartProcess(execName, []string{execName}, &os.ProcAttr{
		Dir:   execDir,
		Files: files,
		Sys:   &syscall.SysProcAttr{},
	})
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (s *server) handleHangup() error {
	c := make(chan string)
	defer close(c)
	errChn := make(chan error)
	defer close(errChn)

	go socketListener(s.cfg.Addr, s.cfg.SockFile, s.ln, c, errChn)

	for {
		select {
		case cmd := <-c:
			switch cmd {
			case "socket_opened":
				p, err := s.fork()
				if err != nil {
					log.WithError(err).
						Error("Unable to fork child")
					continue
				}
				log.WithField("PID", p.Pid).Info("Spawned a new child. Waiting for spinup.")

			case "listener_sent":
				log.
					Debug("Sent listener information to fork, shutting down parent")

				return nil
			}

		case err := <-errChn:
			return err
		}
	}
}

func (s *server) waitForSignals() error {
	sig := make(chan os.Signal, s.cfg.SignalBufferSize)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
	for {
		select {
		case recvSig := <-sig:
			switch recvSig {
			case syscall.SIGHUP:
				err := s.handleHangup()
				if err == nil {
					return s.shutdown()
				}

			case syscall.SIGTERM, syscall.SIGINT:
				return s.shutdown()
			}
		}
	}
}

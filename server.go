package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

var cfg *Config

func fork() (*os.Process, error) {
	// get the listener file
	lnFile, err := getListenerFile(cfg.ln)
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

func handleHangup() error {
	c := make(chan string)
	defer close(c)
	errChn := make(chan error)
	defer close(errChn)

	go socketListener(c, errChn)

	for {
		select {
		case cmd := <-c:
			switch cmd {
			case "socket_opened":
				p, err := fork()
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

	return nil
}

func waitForSignals(srv *http.Server) error {
	sig := make(chan os.Signal, 1024)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
	for {
		select {
		case s := <-sig:
			switch s {
			case syscall.SIGHUP:
				err := handleHangup()
				if err == nil {
					return shutdown(srv)
				}

			case syscall.SIGTERM, syscall.SIGINT:
				return shutdown(srv)
			}
		}
	}
}

func start(handler http.Handler) *http.Server {
	http.Handle("/", handler)

	srv := &http.Server{
		Addr: cfg.Addr,
	}

	go srv.Serve(cfg.ln)

	return srv
}

func shutdown(srv *http.Server) error {
	log.Debug("Server shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := srv.Shutdown(ctx)

	log.WithError(err).Debug("Server shut down")

	return err
}

func Serve(config Config, handler http.Handler) error {
	cfg = &config

	var err error
	cfg.ln, err = getListener()
	if err != nil {
		log.WithError(err).Panic("Unable to create or import a listener")
	}

	srv := start(handler)

	err = waitForSignals(srv)
	if err != nil {
		log.WithError(err).Info("Exiting with error")
		return err
	}

	log.Info("Exiting")

	return nil
}

package httpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
)

type listener struct {
	Addr     string `json:"addr"`
	FD       int    `json:"fd"`
	Filename string `json:"filename"`
}

func getListenerFile(ln net.Listener) (*os.File, error) {
	switch t := ln.(type) {
	case *net.TCPListener:
		return t.File()

	case *net.UnixListener:
		return t.File()
	}

	return nil, fmt.Errorf("unsupported listener: %T", ln)
}

func importListener(addr, sockFile string) (net.Listener, error) {
	c, err := net.Dial("unix", sockFile)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	var lnEnv string
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func(r io.Reader) {
		defer wg.Done()

		buf := make([]byte, 1024)
		n, err := r.Read(buf[:])
		if err != nil {
			return
		}

		lnEnv = string(buf[0:n])
	}(c)

	_, err = c.Write([]byte("get_listener"))
	if err != nil {
		return nil, err
	}

	wg.Wait()

	if lnEnv == "" {
		return nil, fmt.Errorf("Listener info not received from socket")
	}

	var l listener
	err = json.Unmarshal([]byte(lnEnv), &l)
	if err != nil {
		return nil, err
	}
	if l.Addr != addr {
		return nil, fmt.Errorf("unable to find listener for %v", addr)
	}

	// the file has already been passed to this process, extract the file
	// descriptor and name from the metadata to rebuild/find the *os.File for
	// the listener.
	lnFile := os.NewFile(uintptr(l.FD), l.Filename)
	if lnFile == nil {
		return nil, fmt.Errorf("unable to create listener file: %v", l.Filename)
	}
	defer lnFile.Close()

	// create a listerer with the *os.File
	ln, err := net.FileListener(lnFile)
	if err != nil {
		return nil, err
	}

	return ln, nil
}

func createListener(addr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return ln, nil
}

func getListener(addr, sockFile string) (net.Listener, error) {
	// try to import a listener if we are a fork
	ln, err := importListener(addr, sockFile)
	if err == nil {
		log.WithField("addr", addr).
			Debug("imported listener file descriptor")
		return ln, nil
	}

	log.WithError(err).Info("listener not imported")

	// couldn't import a listener, let's create one
	ln, err = createListener(addr)
	if err != nil {
		return nil, err
	}

	return ln, err
}

func sendListener(addr string, ln net.Listener, c net.Conn) error {
	lnFile, err := getListenerFile(ln)
	if err != nil {
		return err
	}
	defer lnFile.Close()

	l := listener{
		Addr:     addr,
		FD:       3,
		Filename: lnFile.Name(),
	}

	lnEnv, err := json.Marshal(l)
	if err != nil {
		return err
	}

	_, err = c.Write(lnEnv)
	if err != nil {
		return err
	}

	return nil
}

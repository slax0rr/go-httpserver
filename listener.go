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

func importListener() (net.Listener, error) {
	c, err := net.Dial("unix", cfg.SockFile)
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
	if l.Addr != cfg.Addr {
		return nil, fmt.Errorf("unable to find listener for %v", cfg.Addr)
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

func createListener() (net.Listener, error) {
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, err
	}

	return ln, nil
}

func getListener() (net.Listener, error) {
	// try to import a listener if we are a fork
	ln, err := importListener()
	if err == nil {
		log.WithField("addr", cfg.Addr).
			Debug("imported listener file descriptor")
		return ln, nil
	}

	log.WithError(err).Info("listener not imported")

	// couldn't import a listener, let's create one
	ln, err = createListener()
	if err != nil {
		return nil, err
	}

	return ln, err
}

func sendListener(c net.Conn) error {
	lnFile, err := getListenerFile(cfg.ln)
	if err != nil {
		return err
	}
	defer lnFile.Close()

	l := listener{
		Addr:     cfg.Addr,
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

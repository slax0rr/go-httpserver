package httpserver

import (
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

func acceptConn(l net.Listener) (c net.Conn, err error) {
	chn := make(chan error)
	go func() {
		defer close(chn)
		c, err = l.Accept()
		if err != nil {
			chn <- err
		}
	}()

	select {
	case err = <-chn:
		if err != nil {
			log.WithError(err).Error("Error occurred when accepting socket connection")
		}

	case <-time.After(4 * time.Second):
		err = fmt.Errorf("Timeout occurred waiting for connection from child")
		log.Info(err.Error())
	}

	return
}

func socketListener(addr, sockFile string, ln net.Listener, chn chan<- string, errChn chan<- error) {
	sockLn, err := net.Listen("unix", sockFile)
	if err != nil {
		log.WithError(err).Error("Unable to start unix domain socket")
		errChn <- err
		return
	}
	defer sockLn.Close()

	chn <- "socket_opened"

	c, err := acceptConn(sockLn)
	if err != nil {
		errChn <- err
		return
	}

	buf := make([]byte, 512)
	nr, err := c.Read(buf)
	if err != nil {
		log.WithError(err).
			Error("Unable to read data from socket")
		errChn <- err
		return
	}

	data := buf[0:nr]
	switch string(data) {
	case "get_listener":
		log.Debug("Fork requested listener information")

		err := sendListener(addr, ln, c)
		if err != nil {
			log.WithError(err).
				Error("Unable to send http listener socket over the unix domain socket")
			errChn <- err
			return
		}

		chn <- "listener_sent"
	}
}

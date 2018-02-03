package remotedialer

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

func clientDial(conn *connection, message *message) {
	defer conn.Close()

	var (
		netConn net.Conn
		err     error
	)

	if message.deadline == 0 {
		netConn, err = net.Dial(message.proto, message.address)
	} else {
		netConn, err = net.DialTimeout(message.proto, message.address, time.Duration(message.deadline)*time.Millisecond)
	}

	if err != nil {
		conn.tunnelClose(err)
		return
	}
	defer netConn.Close()

	pipe(conn.connID, conn, netConn)
}

func pipe(connID int64, client *connection, server net.Conn) {
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() error {
		defer wg.Done()
		_, err := io.Copy(server, client)
		if err != nil {
			client.tunnelClose(err)
			server.Close()
		}
		return err
	}()

	_, err := io.Copy(client, server)
	if err != nil {
		client.tunnelClose(err)
		server.Close()
		logrus.WithError(err).Errorf("client connection failed: client %d", connID)
	}

	wg.Wait()
}

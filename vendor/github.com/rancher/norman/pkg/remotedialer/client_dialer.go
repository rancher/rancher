package remotedialer

import (
	"io"
	"net"
	"sync"
	"time"
)

func clientDial(dialer Dialer, conn *connection, message *message) {
	defer conn.Close()

	var (
		netConn net.Conn
		err     error
	)

	if dialer == nil {
		netDialer := &net.Dialer{
			Timeout:   time.Duration(message.deadline) * time.Millisecond,
			KeepAlive: 30 * time.Second,
		}
		netConn, err = netDialer.Dial(message.proto, message.address)
	} else {
		netConn, err = dialer(message.proto, message.address)
	}

	if err != nil {
		conn.tunnelClose(err)
		return
	}
	defer netConn.Close()

	pipe(conn, netConn)
}

func pipe(client *connection, server net.Conn) {
	wg := sync.WaitGroup{}
	wg.Add(1)

	close := func(err error) error {
		if err == nil {
			err = io.EOF
		}
		client.doTunnelClose(err)
		server.Close()
		return err
	}

	go func() {
		defer wg.Done()
		_, err := io.Copy(server, client)
		close(err)
	}()

	_, err := io.Copy(client, server)
	err = close(err)
	wg.Wait()

	// Write tunnel error after no more I/O is happening, just incase messages get out of order
	client.writeErr(err)
}

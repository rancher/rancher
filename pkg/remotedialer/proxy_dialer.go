package remotedialer

import (
	"io"
)

func proxyPipe(client *connection, server *connection) {
	close := func(err error) error {
		if err == nil {
			err = io.EOF
		}
		client.doTunnelClose(err)
		server.doTunnelClose(err)
		return err
	}

	_, err := io.Copy(client, server)
	err = close(err)

	// Write tunnel error after no more I/O is happening, just incase messages get out of order
	client.writeErr(err)
	server.writeErr(err)
}

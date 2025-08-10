package remotedialer

import (
	"context"
	"io"
	"log"
	"net"
	"time"
)

func clientDial(ctx context.Context, dialer Dialer, conn *connection, message *message) {
	log.Printf("REMOTEDIALER DEBUG: clientDial called - proto: %s, address: %s, connID: %d", message.proto, message.address, conn.connID)
	defer conn.Close()

	var (
		netConn net.Conn
		err     error
	)

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Minute))
	log.Printf("REMOTEDIALER DEBUG: Attempting to dial %s://%s", message.proto, message.address)
	if dialer == nil {
		d := net.Dialer{}
		netConn, err = d.DialContext(ctx, message.proto, message.address)
		log.Printf("REMOTEDIALER DEBUG: Default dialer result - success: %t, err: %v", err == nil, err)
	} else {
		netConn, err = dialer(ctx, message.proto, message.address)
		log.Printf("REMOTEDIALER DEBUG: Custom dialer result - success: %t, err: %v", err == nil, err)
	}
	cancel()

	if err != nil {
		log.Printf("REMOTEDIALER DEBUG: Dial failed, closing tunnel connection: %v", err)
		conn.tunnelClose(err)
		return
	}
	defer netConn.Close()

	log.Printf("REMOTEDIALER DEBUG: Dial successful, starting pipe between tunnel and target %s", message.address)
	pipe(conn, netConn)
}

func pipe(client *connection, server net.Conn) {
	log.Printf("REMOTEDIALER DEBUG: pipe() started - connID: %d, server: %s", client.connID, server.RemoteAddr())
	
	// Use a channel to coordinate graceful shutdown
	closeSignal := make(chan error, 1)
	
	closePipe := func(err error) {
		select {
		case closeSignal <- err:
		default:
			// Channel already closed, ignore
		}
	}

	// Client to server data transfer
	go func() {
		log.Printf("REMOTEDIALER DEBUG: Starting io.Copy from client to server - connID: %d", client.connID)
		bytesWritten, err := io.Copy(server, client)
		log.Printf("REMOTEDIALER DEBUG: io.Copy client->server completed - connID: %d, bytes: %d, err: %v", client.connID, bytesWritten, err)
		
		// Only close if there's an actual error, not just EOF (which is normal for persistent connections)
		if err != nil && err != io.EOF {
			log.Printf("REMOTEDIALER DEBUG: Client->server error, closing pipe - connID: %d, err: %v", client.connID, err)
			closePipe(err)
		} else if err == io.EOF {
			log.Printf("REMOTEDIALER DEBUG: Client->server EOF (normal for persistent connections) - connID: %d", client.connID)
		}
	}()

	// Server to client data transfer
	log.Printf("REMOTEDIALER DEBUG: Starting io.Copy from server to client - connID: %d", client.connID)
	bytesWritten, err := io.Copy(client, server)
	log.Printf("REMOTEDIALER DEBUG: io.Copy server->client completed - connID: %d, bytes: %d, err: %v", client.connID, bytesWritten, err)
	
	// Only close if there's an actual error, not just EOF
	if err != nil && err != io.EOF {
		log.Printf("REMOTEDIALER DEBUG: Server->client error, closing pipe - connID: %d, err: %v", client.connID, err)
		closePipe(err)
	} else if err == io.EOF {
		log.Printf("REMOTEDIALER DEBUG: Server->client EOF (normal for persistent connections) - connID: %d", client.connID)
	}

	// Wait for close signal (connection will stay alive until explicitly closed)
	err = <-closeSignal
	if err == nil {
		err = io.EOF
	}
	log.Printf("REMOTEDIALER DEBUG: pipe() closing - connID: %d, err: %v", client.connID, err)
	client.doTunnelClose(err)
	server.Close()

	// Write tunnel error after no more I/O is happening, just in case messages get out of order
	log.Printf("REMOTEDIALER DEBUG: pipe() finished - connID: %d, writing final error: %v", client.connID, err)
	client.writeErr(err)
}

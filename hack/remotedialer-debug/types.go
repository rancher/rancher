package remotedialer

import "time"

const (
	PingWaitDuration  = 60 * time.Second
	PingWriteInterval = 5 * time.Second
	// SyncConnectionsInterval is the time after which the client will send the list of active connection IDs
	SyncConnectionsInterval = 60 * time.Second
	// SyncConnectionsTimeout sets the maximum duration for a SyncConnections operation
	SyncConnectionsTimeout = 60 * time.Second
	MaxRead                = 8192
	HandshakeTimeOut       = 10 * time.Second
	// SendErrorTimeout sets the maximum duration for sending an error message to close a single connection
	SendErrorTimeout = 5 * time.Second
)

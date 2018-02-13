package mesh

import (
	"net"
)

// Overlay yields OverlayConnections.
type Overlay interface {
	// Enhance a features map with overlay-related features.
	AddFeaturesTo(map[string]string)

	// Prepare on overlay connection. The connection should remain
	// passive until it has been Confirm()ed.
	PrepareConnection(OverlayConnectionParams) (OverlayConnection, error)

	// Obtain diagnostic information specific to the overlay.
	Diagnostics() interface{}

	// Stop the overlay.
	Stop()
}

// OverlayConnectionParams are used to set up overlay connections.
type OverlayConnectionParams struct {
	RemotePeer *Peer

	// The local address of the corresponding TCP connection. Used to
	// derive the local IP address for sending. May differ for
	// different overlay connections.
	LocalAddr *net.TCPAddr

	// The remote address of the corresponding TCP connection. Used to
	// determine the address to send to, but only if the TCP
	// connection is outbound. Otherwise the Overlay needs to discover
	// it (e.g. from incoming datagrams).
	RemoteAddr *net.TCPAddr

	// Is the corresponding TCP connection outbound?
	Outbound bool

	// Unique identifier for this connection
	ConnUID uint64

	// Session key, if connection is encrypted; nil otherwise.
	//
	// NB: overlay connections must take care not to use nonces which
	// may collide with those of the main connection. These nonces are
	// 192 bits, with the top most bit unspecified, the next bit set
	// to 1, followed by 126 zero bits, and a message sequence number
	// in the lowest 64 bits.
	SessionKey *[32]byte

	// Function to send a control message to the counterpart
	// overlay connection.
	SendControlMessage func(tag byte, msg []byte) error

	// Features passed at connection initiation
	Features map[string]string
}

// OverlayConnection describes all of the machinery to manage overlay
// connectivity to a particular peer.
type OverlayConnection interface {
	// Confirm that the connection is really wanted, and so the
	// Overlay should begin heartbeats etc. to verify the operation of
	// the overlay connection.
	Confirm()

	// EstablishedChannel returns a channel that will be closed when the
	// overlay connection is established, i.e. its operation has been
	// confirmed.
	EstablishedChannel() <-chan struct{}

	// ErrorChannel returns a channel that forwards errors from the overlay
	// connection. The overlay connection is not expected to be operational
	// after the first error, so the channel only needs to buffer a single
	// error.
	ErrorChannel() <-chan error

	// Stop terminates the connection.
	Stop()

	// ControlMessage handles a message from the remote peer. 'tag' exists for
	// compatibility, and should always be ProtocolOverlayControlMessage for
	// non-sleeve overlays.
	ControlMessage(tag byte, msg []byte)

	// Attrs returns the user-facing overlay name plus any other
	// data that users may wish to check or monitor
	Attrs() map[string]interface{}
}

// NullOverlay implements Overlay and OverlayConnection with no-ops.
type NullOverlay struct{}

// AddFeaturesTo implements Overlay.
func (NullOverlay) AddFeaturesTo(map[string]string) {}

// PrepareConnection implements Overlay.
func (NullOverlay) PrepareConnection(OverlayConnectionParams) (OverlayConnection, error) {
	return NullOverlay{}, nil
}

// Diagnostics implements Overlay.
func (NullOverlay) Diagnostics() interface{} { return nil }

// Confirm implements OverlayConnection.
func (NullOverlay) Confirm() {}

// EstablishedChannel implements OverlayConnection.
func (NullOverlay) EstablishedChannel() <-chan struct{} { return nil }

// ErrorChannel implements OverlayConnection.
func (NullOverlay) ErrorChannel() <-chan error { return nil }

// Stop implements OverlayConnection.
func (NullOverlay) Stop() {}

// ControlMessage implements OverlayConnection.
func (NullOverlay) ControlMessage(byte, []byte) {}

// Attrs implements OverlayConnection.
func (NullOverlay) Attrs() map[string]interface{} { return nil }

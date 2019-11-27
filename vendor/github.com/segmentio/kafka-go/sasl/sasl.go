package sasl

import "context"

// Mechanism implements the SASL state machine for a particular mode of
// authentication.  It is used by the kafka.Dialer to perform the SASL
// handshake.
//
// A Mechanism must be re-usable and safe for concurrent access by multiple
// goroutines.
type Mechanism interface {
	// Name returns the identifier for this SASL mechanism.  This string will be
	// passed to the SASL handshake request and much match one of the mechanisms
	// supported by Kafka.
	Name() string

	// Start begins SASL authentication. It returns an authentication state
	// machine and "initial response" data (if required by the selected
	// mechanism). A non-nil error causes the client to abort the authentication
	// attempt.
	//
	// A nil ir value is different from a zero-length value. The nil value
	// indicates that the selected mechanism does not use an initial response,
	// while a zero-length value indicates an empty initial response, which must
	// be sent to the server.
	Start(ctx context.Context) (sess StateMachine, ir []byte, err error)
}

// StateMachine implements the SASL challenge/response flow for a single SASL
// handshake.  A StateMachine will be created by the Mechanism per connection,
// so it does not need to be safe for concurrent access by multiple goroutines.
//
// Once the StateMachine is created by the Mechanism, the caller loops by
// passing the server's response into Next and then sending Next's returned
// bytes to the server.  Eventually either Next will indicate that the
// authentication has been successfully completed via the done return value, or
// it will indicate that the authentication failed by returning a non-nil error.
type StateMachine interface {
	// Next continues challenge-response authentication. A non-nil error
	// indicates that the client should abort the authentication attempt.  If
	// the client has been successfully authenticated, then the done return
	// value will be true.
	Next(ctx context.Context, challenge []byte) (done bool, response []byte, err error)
}

package sasl

import "context"

// Mechanism implements the SASL state machine.  It is initialized by calling
// Start at which point the initial bytes should be sent to the server. The
// caller then loops by passing the server's response into Next and then sending
// Next's returned bytes to the server.  Eventually either Next will indicate
// that the authentication has been successfully completed or an error will
// cause the state machine to exit prematurely.
//
// A Mechanism must be re-usable, but it does not need to be safe for concurrent
// access by multiple go routines.
type Mechanism interface {
	// Start begins SASL authentication. It returns the authentication mechanism
	// name and "initial response" data (if required by the selected mechanism).
	// A non-nil error causes the client to abort the authentication attempt.
	//
	// A nil ir value is different from a zero-length value. The nil value
	// indicates that the selected mechanism does not use an initial response,
	// while a zero-length value indicates an empty initial response, which must
	// be sent to the server.
	//
	// In order to ensure that the Mechanism is reusable, calling Start must
	// reset any internal state.
	Start(ctx context.Context) (mech string, ir []byte, err error)

	// Next continues challenge-response authentication. A non-nil error causes
	// the client to abort the authentication attempt.
	Next(ctx context.Context, challenge []byte) (done bool, response []byte, err error)
}

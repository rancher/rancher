package kafka

import (
	"bufio"
)

// saslHandshakeRequestV0 implements the format for V0 and V1 SASL
// requests (they are identical)
type saslHandshakeRequestV0 struct {
	// Mechanism holds the SASL Mechanism chosen by the client.
	Mechanism string
}

func (t saslHandshakeRequestV0) size() int32 {
	return sizeofString(t.Mechanism)
}

func (t *saslHandshakeRequestV0) readFrom(r *bufio.Reader, sz int) (remain int, err error) {
	return readString(r, sz, &t.Mechanism)
}

func (t saslHandshakeRequestV0) writeTo(w *bufio.Writer) {
	writeString(w, t.Mechanism)
}

// saslHandshakeResponseV0 implements the format for V0 and V1 SASL
// responses (they are identical)
type saslHandshakeResponseV0 struct {
	// ErrorCode holds response error code
	ErrorCode int16

	// Array of mechanisms enabled in the server
	EnabledMechanisms []string
}

func (t saslHandshakeResponseV0) size() int32 {
	return sizeofInt16(t.ErrorCode) + sizeofStringArray(t.EnabledMechanisms)
}

func (t saslHandshakeResponseV0) writeTo(w *bufio.Writer) {
	writeInt16(w, t.ErrorCode)
	writeStringArray(w, t.EnabledMechanisms)
}

func (t *saslHandshakeResponseV0) readFrom(r *bufio.Reader, sz int) (remain int, err error) {
	if remain, err = readInt16(r, sz, &t.ErrorCode); err != nil {
		return
	}
	if remain, err = readStringArray(r, remain, &t.EnabledMechanisms); err != nil {
		return
	}
	return
}

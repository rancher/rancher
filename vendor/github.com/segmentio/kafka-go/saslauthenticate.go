package kafka

import (
	"bufio"
)

type saslAuthenticateRequestV0 struct {
	// Data holds the SASL payload
	Data []byte
}

func (t saslAuthenticateRequestV0) size() int32 {
	return sizeofBytes(t.Data)
}

func (t *saslAuthenticateRequestV0) readFrom(r *bufio.Reader, sz int) (remain int, err error) {
	return readBytes(r, sz, &t.Data)
}

func (t saslAuthenticateRequestV0) writeTo(w *bufio.Writer) {
	writeBytes(w, t.Data)
}

type saslAuthenticateResponseV0 struct {
	// ErrorCode holds response error code
	ErrorCode int16

	ErrorMessage string

	Data []byte
}

func (t saslAuthenticateResponseV0) size() int32 {
	return sizeofInt16(t.ErrorCode) + sizeofString(t.ErrorMessage) + sizeofBytes(t.Data)
}

func (t saslAuthenticateResponseV0) writeTo(w *bufio.Writer) {
	writeInt16(w, t.ErrorCode)
	writeString(w, t.ErrorMessage)
	writeBytes(w, t.Data)
}

func (t *saslAuthenticateResponseV0) readFrom(r *bufio.Reader, sz int) (remain int, err error) {
	if remain, err = readInt16(r, sz, &t.ErrorCode); err != nil {
		return
	}
	if remain, err = readString(r, remain, &t.ErrorMessage); err != nil {
		return
	}
	if remain, err = readBytes(r, remain, &t.Data); err != nil {
		return
	}
	return
}

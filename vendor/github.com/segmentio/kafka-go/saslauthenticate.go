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

func (t saslAuthenticateRequestV0) writeTo(wb *writeBuffer) {
	wb.writeBytes(t.Data)
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

func (t saslAuthenticateResponseV0) writeTo(wb *writeBuffer) {
	wb.writeInt16(t.ErrorCode)
	wb.writeString(t.ErrorMessage)
	wb.writeBytes(t.Data)
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

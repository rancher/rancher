package kafka

import "bufio"

type heartbeatRequestV0 struct {
	// GroupID holds the unique group identifier
	GroupID string

	// GenerationID holds the generation of the group.
	GenerationID int32

	// MemberID assigned by the group coordinator
	MemberID string
}

func (t heartbeatRequestV0) size() int32 {
	return sizeofString(t.GroupID) +
		sizeofInt32(t.GenerationID) +
		sizeofString(t.MemberID)
}

func (t heartbeatRequestV0) writeTo(wb *writeBuffer) {
	wb.writeString(t.GroupID)
	wb.writeInt32(t.GenerationID)
	wb.writeString(t.MemberID)
}

type heartbeatResponseV0 struct {
	// ErrorCode holds response error code
	ErrorCode int16
}

func (t heartbeatResponseV0) size() int32 {
	return sizeofInt16(t.ErrorCode)
}

func (t heartbeatResponseV0) writeTo(wb *writeBuffer) {
	wb.writeInt16(t.ErrorCode)
}

func (t *heartbeatResponseV0) readFrom(r *bufio.Reader, sz int) (remain int, err error) {
	if remain, err = readInt16(r, sz, &t.ErrorCode); err != nil {
		return
	}
	return
}

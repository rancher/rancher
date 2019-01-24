package kafka

import "bufio"

type leaveGroupRequestV0 struct {
	// GroupID holds the unique group identifier
	GroupID string

	// MemberID assigned by the group coordinator or the zero string if joining
	// for the first time.
	MemberID string
}

func (t leaveGroupRequestV0) size() int32 {
	return sizeofString(t.GroupID) +
		sizeofString(t.MemberID)
}

func (t leaveGroupRequestV0) writeTo(w *bufio.Writer) {
	writeString(w, t.GroupID)
	writeString(w, t.MemberID)
}

type leaveGroupResponseV0 struct {
	// ErrorCode holds response error code
	ErrorCode int16
}

func (t leaveGroupResponseV0) size() int32 {
	return sizeofInt16(t.ErrorCode)
}

func (t leaveGroupResponseV0) writeTo(w *bufio.Writer) {
	writeInt16(w, t.ErrorCode)
}

func (t *leaveGroupResponseV0) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	if remain, err = readInt16(r, size, &t.ErrorCode); err != nil {
		return
	}
	return
}

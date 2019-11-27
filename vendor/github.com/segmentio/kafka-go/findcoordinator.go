package kafka

import (
	"bufio"
)

// FindCoordinatorRequestV0 requests the coordinator for the specified group or transaction
//
// See http://kafka.apache.org/protocol.html#The_Messages_FindCoordinator
type findCoordinatorRequestV0 struct {
	// CoordinatorKey holds id to use for finding the coordinator (for groups, this is
	// the groupId, for transactional producers, this is the transactional id)
	CoordinatorKey string
}

func (t findCoordinatorRequestV0) size() int32 {
	return sizeofString(t.CoordinatorKey)
}

func (t findCoordinatorRequestV0) writeTo(wb *writeBuffer) {
	wb.writeString(t.CoordinatorKey)
}

type findCoordinatorResponseCoordinatorV0 struct {
	// NodeID holds the broker id.
	NodeID int32

	// Host of the broker
	Host string

	// Port on which broker accepts requests
	Port int32
}

func (t findCoordinatorResponseCoordinatorV0) size() int32 {
	return sizeofInt32(t.NodeID) +
		sizeofString(t.Host) +
		sizeofInt32(t.Port)
}

func (t findCoordinatorResponseCoordinatorV0) writeTo(wb *writeBuffer) {
	wb.writeInt32(t.NodeID)
	wb.writeString(t.Host)
	wb.writeInt32(t.Port)
}

func (t *findCoordinatorResponseCoordinatorV0) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	if remain, err = readInt32(r, size, &t.NodeID); err != nil {
		return
	}
	if remain, err = readString(r, remain, &t.Host); err != nil {
		return
	}
	if remain, err = readInt32(r, remain, &t.Port); err != nil {
		return
	}
	return
}

type findCoordinatorResponseV0 struct {
	// ErrorCode holds response error code
	ErrorCode int16

	// Coordinator holds host and port information for the coordinator
	Coordinator findCoordinatorResponseCoordinatorV0
}

func (t findCoordinatorResponseV0) size() int32 {
	return sizeofInt16(t.ErrorCode) +
		t.Coordinator.size()
}

func (t findCoordinatorResponseV0) writeTo(wb *writeBuffer) {
	wb.writeInt16(t.ErrorCode)
	t.Coordinator.writeTo(wb)
}

func (t *findCoordinatorResponseV0) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	if remain, err = readInt16(r, size, &t.ErrorCode); err != nil {
		return
	}
	if remain, err = (&t.Coordinator).readFrom(r, remain); err != nil {
		return
	}
	return
}

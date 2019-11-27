package kafka

import (
	"bufio"
	"bytes"
)

type memberGroupMetadata struct {
	// MemberID assigned by the group coordinator or null if joining for the
	// first time.
	MemberID string
	Metadata groupMetadata
}

type groupMetadata struct {
	Version  int16
	Topics   []string
	UserData []byte
}

func (t groupMetadata) size() int32 {
	return sizeofInt16(t.Version) +
		sizeofStringArray(t.Topics) +
		sizeofBytes(t.UserData)
}

func (t groupMetadata) writeTo(wb *writeBuffer) {
	wb.writeInt16(t.Version)
	wb.writeStringArray(t.Topics)
	wb.writeBytes(t.UserData)
}

func (t groupMetadata) bytes() []byte {
	buf := bytes.NewBuffer(nil)
	t.writeTo(&writeBuffer{w: buf})
	return buf.Bytes()
}

func (t *groupMetadata) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	if remain, err = readInt16(r, size, &t.Version); err != nil {
		return
	}
	if remain, err = readStringArray(r, remain, &t.Topics); err != nil {
		return
	}
	if remain, err = readBytes(r, remain, &t.UserData); err != nil {
		return
	}
	return
}

type joinGroupRequestGroupProtocolV1 struct {
	ProtocolName     string
	ProtocolMetadata []byte
}

func (t joinGroupRequestGroupProtocolV1) size() int32 {
	return sizeofString(t.ProtocolName) +
		sizeofBytes(t.ProtocolMetadata)
}

func (t joinGroupRequestGroupProtocolV1) writeTo(wb *writeBuffer) {
	wb.writeString(t.ProtocolName)
	wb.writeBytes(t.ProtocolMetadata)
}

type joinGroupRequestV1 struct {
	// GroupID holds the unique group identifier
	GroupID string

	// SessionTimeout holds the coordinator considers the consumer dead if it
	// receives no heartbeat after this timeout in ms.
	SessionTimeout int32

	// RebalanceTimeout holds the maximum time that the coordinator will wait
	// for each member to rejoin when rebalancing the group in ms
	RebalanceTimeout int32

	// MemberID assigned by the group coordinator or the zero string if joining
	// for the first time.
	MemberID string

	// ProtocolType holds the unique name for class of protocols implemented by group
	ProtocolType string

	// GroupProtocols holds the list of protocols that the member supports
	GroupProtocols []joinGroupRequestGroupProtocolV1
}

func (t joinGroupRequestV1) size() int32 {
	return sizeofString(t.GroupID) +
		sizeofInt32(t.SessionTimeout) +
		sizeofInt32(t.RebalanceTimeout) +
		sizeofString(t.MemberID) +
		sizeofString(t.ProtocolType) +
		sizeofArray(len(t.GroupProtocols), func(i int) int32 { return t.GroupProtocols[i].size() })
}

func (t joinGroupRequestV1) writeTo(wb *writeBuffer) {
	wb.writeString(t.GroupID)
	wb.writeInt32(t.SessionTimeout)
	wb.writeInt32(t.RebalanceTimeout)
	wb.writeString(t.MemberID)
	wb.writeString(t.ProtocolType)
	wb.writeArray(len(t.GroupProtocols), func(i int) { t.GroupProtocols[i].writeTo(wb) })
}

type joinGroupResponseMemberV1 struct {
	// MemberID assigned by the group coordinator
	MemberID       string
	MemberMetadata []byte
}

func (t joinGroupResponseMemberV1) size() int32 {
	return sizeofString(t.MemberID) +
		sizeofBytes(t.MemberMetadata)
}

func (t joinGroupResponseMemberV1) writeTo(wb *writeBuffer) {
	wb.writeString(t.MemberID)
	wb.writeBytes(t.MemberMetadata)
}

func (t *joinGroupResponseMemberV1) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	if remain, err = readString(r, size, &t.MemberID); err != nil {
		return
	}
	if remain, err = readBytes(r, remain, &t.MemberMetadata); err != nil {
		return
	}
	return
}

type joinGroupResponseV1 struct {
	// ErrorCode holds response error code
	ErrorCode int16

	// GenerationID holds the generation of the group.
	GenerationID int32

	// GroupProtocol holds the group protocol selected by the coordinator
	GroupProtocol string

	// LeaderID holds the leader of the group
	LeaderID string

	// MemberID assigned by the group coordinator
	MemberID string
	Members  []joinGroupResponseMemberV1
}

func (t joinGroupResponseV1) size() int32 {
	return sizeofInt16(t.ErrorCode) +
		sizeofInt32(t.GenerationID) +
		sizeofString(t.GroupProtocol) +
		sizeofString(t.LeaderID) +
		sizeofString(t.MemberID) +
		sizeofArray(len(t.MemberID), func(i int) int32 { return t.Members[i].size() })
}

func (t joinGroupResponseV1) writeTo(wb *writeBuffer) {
	wb.writeInt16(t.ErrorCode)
	wb.writeInt32(t.GenerationID)
	wb.writeString(t.GroupProtocol)
	wb.writeString(t.LeaderID)
	wb.writeString(t.MemberID)
	wb.writeArray(len(t.Members), func(i int) { t.Members[i].writeTo(wb) })
}

func (t *joinGroupResponseV1) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	if remain, err = readInt16(r, size, &t.ErrorCode); err != nil {
		return
	}
	if remain, err = readInt32(r, remain, &t.GenerationID); err != nil {
		return
	}
	if remain, err = readString(r, remain, &t.GroupProtocol); err != nil {
		return
	}
	if remain, err = readString(r, remain, &t.LeaderID); err != nil {
		return
	}
	if remain, err = readString(r, remain, &t.MemberID); err != nil {
		return
	}

	fn := func(r *bufio.Reader, size int) (fnRemain int, fnErr error) {
		var item joinGroupResponseMemberV1
		if fnRemain, fnErr = (&item).readFrom(r, size); fnErr != nil {
			return
		}
		t.Members = append(t.Members, item)
		return
	}
	if remain, err = readArrayWith(r, remain, fn); err != nil {
		return
	}

	return
}

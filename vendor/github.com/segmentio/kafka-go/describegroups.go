package kafka

import "bufio"

// See http://kafka.apache.org/protocol.html#The_Messages_DescribeGroups
type describeGroupsRequestV0 struct {
	// List of groupIds to request metadata for (an empty groupId array
	// will return empty group metadata).
	GroupIDs []string
}

func (t describeGroupsRequestV0) size() int32 {
	return sizeofStringArray(t.GroupIDs)
}

func (t describeGroupsRequestV0) writeTo(w *bufio.Writer) {
	writeStringArray(w, t.GroupIDs)
}

type describeGroupsResponseMemberV0 struct {
	// MemberID assigned by the group coordinator
	MemberID string

	// ClientID used in the member's latest join group request
	ClientID string

	// ClientHost used in the request session corresponding to the member's
	// join group.
	ClientHost string

	// MemberMetadata the metadata corresponding to the current group protocol
	// in use (will only be present if the group is stable).
	MemberMetadata []byte

	// MemberAssignments provided by the group leader (will only be present if
	// the group is stable).
	//
	// See consumer groups section of https://cwiki.apache.org/confluence/display/KAFKA/A+Guide+To+The+Kafka+Protocol
	MemberAssignments []byte
}

func (t describeGroupsResponseMemberV0) size() int32 {
	return sizeofString(t.MemberID) +
		sizeofString(t.ClientID) +
		sizeofString(t.ClientHost) +
		sizeofBytes(t.MemberMetadata) +
		sizeofBytes(t.MemberAssignments)
}

func (t describeGroupsResponseMemberV0) writeTo(w *bufio.Writer) {
	writeString(w, t.MemberID)
	writeString(w, t.ClientID)
	writeString(w, t.ClientHost)
	writeBytes(w, t.MemberMetadata)
	writeBytes(w, t.MemberAssignments)
}

func (t *describeGroupsResponseMemberV0) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	if remain, err = readString(r, size, &t.MemberID); err != nil {
		return
	}
	if remain, err = readString(r, remain, &t.ClientID); err != nil {
		return
	}
	if remain, err = readString(r, remain, &t.ClientHost); err != nil {
		return
	}
	if remain, err = readBytes(r, remain, &t.MemberMetadata); err != nil {
		return
	}
	if remain, err = readBytes(r, remain, &t.MemberAssignments); err != nil {
		return
	}
	return
}

type describeGroupsResponseGroupV0 struct {
	// ErrorCode holds response error code
	ErrorCode int16

	// GroupID holds the unique group identifier
	GroupID string

	// State holds current state of the group (one of: Dead, Stable, AwaitingSync,
	// PreparingRebalance, or empty if there is no active group)
	State string

	// ProtocolType holds the current group protocol type (will be empty if there is
	// no active group)
	ProtocolType string

	// Protocol holds the current group protocol (only provided if the group is Stable)
	Protocol string

	// Members contains the current group members (only provided if the group is not Dead)
	Members []describeGroupsResponseMemberV0
}

func (t describeGroupsResponseGroupV0) size() int32 {
	return sizeofInt16(t.ErrorCode) +
		sizeofString(t.GroupID) +
		sizeofString(t.State) +
		sizeofString(t.ProtocolType) +
		sizeofString(t.Protocol) +
		sizeofArray(len(t.Members), func(i int) int32 { return t.Members[i].size() })
}

func (t describeGroupsResponseGroupV0) writeTo(w *bufio.Writer) {
	writeInt16(w, t.ErrorCode)
	writeString(w, t.GroupID)
	writeString(w, t.State)
	writeString(w, t.ProtocolType)
	writeString(w, t.Protocol)
	writeArray(w, len(t.Members), func(i int) { t.Members[i].writeTo(w) })
}

func (t *describeGroupsResponseGroupV0) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	if remain, err = readInt16(r, size, &t.ErrorCode); err != nil {
		return
	}
	if remain, err = readString(r, remain, &t.GroupID); err != nil {
		return
	}
	if remain, err = readString(r, remain, &t.State); err != nil {
		return
	}
	if remain, err = readString(r, remain, &t.ProtocolType); err != nil {
		return
	}
	if remain, err = readString(r, remain, &t.Protocol); err != nil {
		return
	}

	fn := func(r *bufio.Reader, size int) (fnRemain int, fnErr error) {
		item := describeGroupsResponseMemberV0{}
		if fnRemain, fnErr = (&item).readFrom(r, size); err != nil {
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

type describeGroupsResponseV0 struct {
	// Groups holds selected group information
	Groups []describeGroupsResponseGroupV0
}

func (t describeGroupsResponseV0) size() int32 {
	return sizeofArray(len(t.Groups), func(i int) int32 { return t.Groups[i].size() })
}

func (t describeGroupsResponseV0) writeTo(w *bufio.Writer) {
	writeArray(w, len(t.Groups), func(i int) { t.Groups[i].writeTo(w) })
}

func (t *describeGroupsResponseV0) readFrom(r *bufio.Reader, sz int) (remain int, err error) {
	fn := func(r *bufio.Reader, size int) (fnRemain int, fnErr error) {
		item := describeGroupsResponseGroupV0{}
		if fnRemain, fnErr = (&item).readFrom(r, size); fnErr != nil {
			return
		}
		t.Groups = append(t.Groups, item)
		return
	}
	if remain, err = readArrayWith(r, sz, fn); err != nil {
		return
	}

	return
}

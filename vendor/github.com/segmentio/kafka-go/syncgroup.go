package kafka

import (
	"bufio"
	"bytes"
)

type groupAssignment struct {
	Version  int16
	Topics   map[string][]int32
	UserData []byte
}

func (t groupAssignment) size() int32 {
	sz := sizeofInt16(t.Version) + sizeofInt16(int16(len(t.Topics)))

	for topic, partitions := range t.Topics {
		sz += sizeofString(topic) + sizeofInt32Array(partitions)
	}

	return sz + sizeofBytes(t.UserData)
}

func (t groupAssignment) writeTo(w *bufio.Writer) {
	writeInt16(w, t.Version)
	writeInt32(w, int32(len(t.Topics)))

	for topic, partitions := range t.Topics {
		writeString(w, topic)
		writeInt32Array(w, partitions)
	}

	writeBytes(w, t.UserData)
}

func (t *groupAssignment) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	// I came across this case when testing for compatibility with bsm/sarama-cluster. It
	// appears in some cases, sarama-cluster can send a nil array entry. Admittedly, I
	// didn't look too closely at it.
	if size == 0 {
		t.Topics = map[string][]int32{}
		return 0, nil
	}

	if remain, err = readInt16(r, size, &t.Version); err != nil {
		return
	}
	if remain, err = readMapStringInt32(r, remain, &t.Topics); err != nil {
		return
	}
	if remain, err = readBytes(r, remain, &t.UserData); err != nil {
		return
	}

	return
}

func (t groupAssignment) bytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := bufio.NewWriter(buf)
	t.writeTo(w)
	w.Flush()
	return buf.Bytes()
}

type syncGroupRequestGroupAssignmentV0 struct {
	// MemberID assigned by the group coordinator
	MemberID string

	// MemberAssignments holds client encoded assignments
	//
	// See consumer groups section of https://cwiki.apache.org/confluence/display/KAFKA/A+Guide+To+The+Kafka+Protocol
	MemberAssignments []byte
}

func (t syncGroupRequestGroupAssignmentV0) size() int32 {
	return sizeofString(t.MemberID) +
		sizeofBytes(t.MemberAssignments)
}

func (t syncGroupRequestGroupAssignmentV0) writeTo(w *bufio.Writer) {
	writeString(w, t.MemberID)
	writeBytes(w, t.MemberAssignments)
}

type syncGroupRequestV0 struct {
	// GroupID holds the unique group identifier
	GroupID string

	// GenerationID holds the generation of the group.
	GenerationID int32

	// MemberID assigned by the group coordinator
	MemberID string

	GroupAssignments []syncGroupRequestGroupAssignmentV0
}

func (t syncGroupRequestV0) size() int32 {
	return sizeofString(t.GroupID) +
		sizeofInt32(t.GenerationID) +
		sizeofString(t.MemberID) +
		sizeofArray(len(t.GroupAssignments), func(i int) int32 { return t.GroupAssignments[i].size() })
}

func (t syncGroupRequestV0) writeTo(w *bufio.Writer) {
	writeString(w, t.GroupID)
	writeInt32(w, t.GenerationID)
	writeString(w, t.MemberID)
	writeArray(w, len(t.GroupAssignments), func(i int) { t.GroupAssignments[i].writeTo(w) })
}

type syncGroupResponseV0 struct {
	// ErrorCode holds response error code
	ErrorCode int16

	// MemberAssignments holds client encoded assignments
	//
	// See consumer groups section of https://cwiki.apache.org/confluence/display/KAFKA/A+Guide+To+The+Kafka+Protocol
	MemberAssignments []byte
}

func (t syncGroupResponseV0) size() int32 {
	return sizeofInt16(t.ErrorCode) +
		sizeofBytes(t.MemberAssignments)
}

func (t syncGroupResponseV0) writeTo(w *bufio.Writer) {
	writeInt16(w, t.ErrorCode)
	writeBytes(w, t.MemberAssignments)
}

func (t *syncGroupResponseV0) readFrom(r *bufio.Reader, sz int) (remain int, err error) {
	if remain, err = readInt16(r, sz, &t.ErrorCode); err != nil {
		return
	}
	if remain, err = readBytes(r, remain, &t.MemberAssignments); err != nil {
		return
	}
	return
}

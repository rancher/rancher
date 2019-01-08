package kafka

import "bufio"

type fetchRequestV2 struct {
	ReplicaID   int32
	MaxWaitTime int32
	MinBytes    int32
	Topics      []fetchRequestTopicV2
}

func (r fetchRequestV2) size() int32 {
	return 4 + 4 + 4 + sizeofArray(len(r.Topics), func(i int) int32 { return r.Topics[i].size() })
}

func (r fetchRequestV2) writeTo(w *bufio.Writer) {
	writeInt32(w, r.ReplicaID)
	writeInt32(w, r.MaxWaitTime)
	writeInt32(w, r.MinBytes)
	writeArray(w, len(r.Topics), func(i int) { r.Topics[i].writeTo(w) })
}

type fetchRequestTopicV2 struct {
	TopicName  string
	Partitions []fetchRequestPartitionV2
}

func (t fetchRequestTopicV2) size() int32 {
	return sizeofString(t.TopicName) +
		sizeofArray(len(t.Partitions), func(i int) int32 { return t.Partitions[i].size() })
}

func (t fetchRequestTopicV2) writeTo(w *bufio.Writer) {
	writeString(w, t.TopicName)
	writeArray(w, len(t.Partitions), func(i int) { t.Partitions[i].writeTo(w) })
}

type fetchRequestPartitionV2 struct {
	Partition   int32
	FetchOffset int64
	MaxBytes    int32
}

func (p fetchRequestPartitionV2) size() int32 {
	return 4 + 8 + 4
}

func (p fetchRequestPartitionV2) writeTo(w *bufio.Writer) {
	writeInt32(w, p.Partition)
	writeInt64(w, p.FetchOffset)
	writeInt32(w, p.MaxBytes)
}

type fetchResponseV2 struct {
	ThrottleTime int32
	Topics       []fetchResponseTopicV2
}

func (r fetchResponseV2) size() int32 {
	return 4 + sizeofArray(len(r.Topics), func(i int) int32 { return r.Topics[i].size() })
}

func (r fetchResponseV2) writeTo(w *bufio.Writer) {
	writeInt32(w, r.ThrottleTime)
	writeArray(w, len(r.Topics), func(i int) { r.Topics[i].writeTo(w) })
}

type fetchResponseTopicV2 struct {
	TopicName  string
	Partitions []fetchResponsePartitionV2
}

func (t fetchResponseTopicV2) size() int32 {
	return sizeofString(t.TopicName) +
		sizeofArray(len(t.Partitions), func(i int) int32 { return t.Partitions[i].size() })
}

func (t fetchResponseTopicV2) writeTo(w *bufio.Writer) {
	writeString(w, t.TopicName)
	writeArray(w, len(t.Partitions), func(i int) { t.Partitions[i].writeTo(w) })
}

type fetchResponsePartitionV2 struct {
	Partition           int32
	ErrorCode           int16
	HighwaterMarkOffset int64
	MessageSetSize      int32
	MessageSet          messageSet
}

func (p fetchResponsePartitionV2) size() int32 {
	return 4 + 2 + 8 + 4 + p.MessageSet.size()
}

func (p fetchResponsePartitionV2) writeTo(w *bufio.Writer) {
	writeInt32(w, p.Partition)
	writeInt16(w, p.ErrorCode)
	writeInt64(w, p.HighwaterMarkOffset)
	writeInt32(w, p.MessageSetSize)
	p.MessageSet.writeTo(w)
}

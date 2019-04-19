package kafka

import "bufio"

type produceRequestV2 struct {
	RequiredAcks int16
	Timeout      int32
	Topics       []produceRequestTopicV2
}

func (r produceRequestV2) size() int32 {
	return 2 + 4 + sizeofArray(len(r.Topics), func(i int) int32 { return r.Topics[i].size() })
}

func (r produceRequestV2) writeTo(w *bufio.Writer) {
	writeInt16(w, r.RequiredAcks)
	writeInt32(w, r.Timeout)
	writeArray(w, len(r.Topics), func(i int) { r.Topics[i].writeTo(w) })
}

type produceRequestTopicV2 struct {
	TopicName  string
	Partitions []produceRequestPartitionV2
}

func (t produceRequestTopicV2) size() int32 {
	return sizeofString(t.TopicName) +
		sizeofArray(len(t.Partitions), func(i int) int32 { return t.Partitions[i].size() })
}

func (t produceRequestTopicV2) writeTo(w *bufio.Writer) {
	writeString(w, t.TopicName)
	writeArray(w, len(t.Partitions), func(i int) { t.Partitions[i].writeTo(w) })
}

type produceRequestPartitionV2 struct {
	Partition      int32
	MessageSetSize int32
	MessageSet     messageSet
}

func (p produceRequestPartitionV2) size() int32 {
	return 4 + 4 + p.MessageSet.size()
}

func (p produceRequestPartitionV2) writeTo(w *bufio.Writer) {
	writeInt32(w, p.Partition)
	writeInt32(w, p.MessageSetSize)
	p.MessageSet.writeTo(w)
}

type produceResponseV2 struct {
	ThrottleTime int32
	Topics       []produceResponseTopicV2
}

func (r produceResponseV2) size() int32 {
	return 4 + sizeofArray(len(r.Topics), func(i int) int32 { return r.Topics[i].size() })
}

func (r produceResponseV2) writeTo(w *bufio.Writer) {
	writeInt32(w, r.ThrottleTime)
	writeArray(w, len(r.Topics), func(i int) { r.Topics[i].writeTo(w) })
}

type produceResponseTopicV2 struct {
	TopicName  string
	Partitions []produceResponsePartitionV2
}

func (t produceResponseTopicV2) size() int32 {
	return sizeofString(t.TopicName) +
		sizeofArray(len(t.Partitions), func(i int) int32 { return t.Partitions[i].size() })
}

func (t produceResponseTopicV2) writeTo(w *bufio.Writer) {
	writeString(w, t.TopicName)
	writeArray(w, len(t.Partitions), func(i int) { t.Partitions[i].writeTo(w) })
}

type produceResponsePartitionV2 struct {
	Partition int32
	ErrorCode int16
	Offset    int64
	Timestamp int64
}

func (p produceResponsePartitionV2) size() int32 {
	return 4 + 2 + 8 + 8
}

func (p produceResponsePartitionV2) writeTo(w *bufio.Writer) {
	writeInt32(w, p.Partition)
	writeInt16(w, p.ErrorCode)
	writeInt64(w, p.Offset)
	writeInt64(w, p.Timestamp)
}

func (p *produceResponsePartitionV2) readFrom(r *bufio.Reader, sz int) (remain int, err error) {
	if remain, err = readInt32(r, sz, &p.Partition); err != nil {
		return
	}
	if remain, err = readInt16(r, remain, &p.ErrorCode); err != nil {
		return
	}
	if remain, err = readInt64(r, remain, &p.Offset); err != nil {
		return
	}
	if remain, err = readInt64(r, remain, &p.Timestamp); err != nil {
		return
	}
	return
}

type produceResponsePartitionV7 struct {
	Partition   int32
	ErrorCode   int16
	Offset      int64
	Timestamp   int64
	StartOffset int64
}

func (p produceResponsePartitionV7) size() int32 {
	return 4 + 2 + 8 + 8 + 8
}

func (p produceResponsePartitionV7) writeTo(w *bufio.Writer) {
	writeInt32(w, p.Partition)
	writeInt16(w, p.ErrorCode)
	writeInt64(w, p.Offset)
	writeInt64(w, p.Timestamp)
	writeInt64(w, p.StartOffset)
}

func (p *produceResponsePartitionV7) readFrom(r *bufio.Reader, sz int) (remain int, err error) {
	if remain, err = readInt32(r, sz, &p.Partition); err != nil {
		return
	}
	if remain, err = readInt16(r, remain, &p.ErrorCode); err != nil {
		return
	}
	if remain, err = readInt64(r, remain, &p.Offset); err != nil {
		return
	}
	if remain, err = readInt64(r, remain, &p.Timestamp); err != nil {
		return
	}
	if remain, err = readInt64(r, remain, &p.StartOffset); err != nil {
		return
	}
	return
}

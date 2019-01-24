package kafka

import "bufio"

type topicMetadataRequestV1 []string

func (r topicMetadataRequestV1) size() int32 {
	return sizeofStringArray([]string(r))
}

func (r topicMetadataRequestV1) writeTo(w *bufio.Writer) {
	writeStringArray(w, []string(r))
}

type metadataResponseV1 struct {
	Brokers      []brokerMetadataV1
	ControllerID int32
	Topics       []topicMetadataV1
}

func (r metadataResponseV1) size() int32 {
	n1 := sizeofArray(len(r.Brokers), func(i int) int32 { return r.Brokers[i].size() })
	n2 := sizeofArray(len(r.Topics), func(i int) int32 { return r.Topics[i].size() })
	return 4 + n1 + n2
}

func (r metadataResponseV1) writeTo(w *bufio.Writer) {
	writeArray(w, len(r.Brokers), func(i int) { r.Brokers[i].writeTo(w) })
	writeInt32(w, r.ControllerID)
	writeArray(w, len(r.Topics), func(i int) { r.Topics[i].writeTo(w) })
}

type brokerMetadataV1 struct {
	NodeID int32
	Host   string
	Port   int32
	Rack   string
}

func (b brokerMetadataV1) size() int32 {
	return 4 + 4 + sizeofString(b.Host) + sizeofString(b.Rack)
}

func (b brokerMetadataV1) writeTo(w *bufio.Writer) {
	writeInt32(w, b.NodeID)
	writeString(w, b.Host)
	writeInt32(w, b.Port)
	writeString(w, b.Rack)
}

type topicMetadataV1 struct {
	TopicErrorCode int16
	TopicName      string
	Internal       bool
	Partitions     []partitionMetadataV1
}

func (t topicMetadataV1) size() int32 {
	return 2 + 1 +
		sizeofString(t.TopicName) +
		sizeofArray(len(t.Partitions), func(i int) int32 { return t.Partitions[i].size() })
}

func (t topicMetadataV1) writeTo(w *bufio.Writer) {
	writeInt16(w, t.TopicErrorCode)
	writeString(w, t.TopicName)
	writeBool(w, t.Internal)
	writeArray(w, len(t.Partitions), func(i int) { t.Partitions[i].writeTo(w) })
}

type partitionMetadataV1 struct {
	PartitionErrorCode int16
	PartitionID        int32
	Leader             int32
	Replicas           []int32
	Isr                []int32
}

func (p partitionMetadataV1) size() int32 {
	return 2 + 4 + 4 + sizeofInt32Array(p.Replicas) + sizeofInt32Array(p.Isr)
}

func (p partitionMetadataV1) writeTo(w *bufio.Writer) {
	writeInt16(w, p.PartitionErrorCode)
	writeInt32(w, p.PartitionID)
	writeInt32(w, p.Leader)
	writeInt32Array(w, p.Replicas)
	writeInt32Array(w, p.Isr)
}

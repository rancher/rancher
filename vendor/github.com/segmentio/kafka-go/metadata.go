package kafka

type topicMetadataRequestV1 []string

func (r topicMetadataRequestV1) size() int32 {
	return sizeofStringArray([]string(r))
}

func (r topicMetadataRequestV1) writeTo(wb *writeBuffer) {
	wb.writeStringArray([]string(r))
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

func (r metadataResponseV1) writeTo(wb *writeBuffer) {
	wb.writeArray(len(r.Brokers), func(i int) { r.Brokers[i].writeTo(wb) })
	wb.writeInt32(r.ControllerID)
	wb.writeArray(len(r.Topics), func(i int) { r.Topics[i].writeTo(wb) })
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

func (b brokerMetadataV1) writeTo(wb *writeBuffer) {
	wb.writeInt32(b.NodeID)
	wb.writeString(b.Host)
	wb.writeInt32(b.Port)
	wb.writeString(b.Rack)
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

func (t topicMetadataV1) writeTo(wb *writeBuffer) {
	wb.writeInt16(t.TopicErrorCode)
	wb.writeString(t.TopicName)
	wb.writeBool(t.Internal)
	wb.writeArray(len(t.Partitions), func(i int) { t.Partitions[i].writeTo(wb) })
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

func (p partitionMetadataV1) writeTo(wb *writeBuffer) {
	wb.writeInt16(p.PartitionErrorCode)
	wb.writeInt32(p.PartitionID)
	wb.writeInt32(p.Leader)
	wb.writeInt32Array(p.Replicas)
	wb.writeInt32Array(p.Isr)
}

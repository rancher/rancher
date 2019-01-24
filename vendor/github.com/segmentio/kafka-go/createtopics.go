package kafka

import (
	"bufio"
	"time"
)

type ConfigEntry struct {
	ConfigName  string
	ConfigValue string
}

func (c ConfigEntry) toCreateTopicsRequestV0ConfigEntry() createTopicsRequestV0ConfigEntry {
	return createTopicsRequestV0ConfigEntry{
		ConfigName:  c.ConfigName,
		ConfigValue: c.ConfigValue,
	}
}

type createTopicsRequestV0ConfigEntry struct {
	ConfigName  string
	ConfigValue string
}

func (t createTopicsRequestV0ConfigEntry) size() int32 {
	return sizeofString(t.ConfigName) +
		sizeofString(t.ConfigValue)
}

func (t createTopicsRequestV0ConfigEntry) writeTo(w *bufio.Writer) {
	writeString(w, t.ConfigName)
	writeString(w, t.ConfigValue)
}

type ReplicaAssignment struct {
	Partition int
	Replicas  int
}

func (a ReplicaAssignment) toCreateTopicsRequestV0ReplicaAssignment() createTopicsRequestV0ReplicaAssignment {
	return createTopicsRequestV0ReplicaAssignment{
		Partition: int32(a.Partition),
		Replicas:  int32(a.Replicas),
	}
}

type createTopicsRequestV0ReplicaAssignment struct {
	Partition int32
	Replicas  int32
}

func (t createTopicsRequestV0ReplicaAssignment) size() int32 {
	return sizeofInt32(t.Partition) +
		sizeofInt32(t.Replicas)
}

func (t createTopicsRequestV0ReplicaAssignment) writeTo(w *bufio.Writer) {
	writeInt32(w, t.Partition)
	writeInt32(w, t.Replicas)
}

type TopicConfig struct {
	// Topic name
	Topic string

	// NumPartitions created. -1 indicates unset.
	NumPartitions int

	// ReplicationFactor for the topic. -1 indicates unset.
	ReplicationFactor int

	// ReplicaAssignments among kafka brokers for this topic partitions. If this
	// is set num_partitions and replication_factor must be unset.
	ReplicaAssignments []ReplicaAssignment

	// ConfigEntries holds topic level configuration for topic to be set.
	ConfigEntries []ConfigEntry
}

func (t TopicConfig) toCreateTopicsRequestV0Topic() createTopicsRequestV0Topic {
	var requestV0ReplicaAssignments []createTopicsRequestV0ReplicaAssignment
	for _, a := range t.ReplicaAssignments {
		requestV0ReplicaAssignments = append(
			requestV0ReplicaAssignments,
			a.toCreateTopicsRequestV0ReplicaAssignment())
	}
	var requestV0ConfigEntries []createTopicsRequestV0ConfigEntry
	for _, c := range t.ConfigEntries {
		requestV0ConfigEntries = append(
			requestV0ConfigEntries,
			c.toCreateTopicsRequestV0ConfigEntry())
	}

	return createTopicsRequestV0Topic{
		Topic:              t.Topic,
		NumPartitions:      int32(t.NumPartitions),
		ReplicationFactor:  int16(t.ReplicationFactor),
		ReplicaAssignments: requestV0ReplicaAssignments,
		ConfigEntries:      requestV0ConfigEntries,
	}
}

type createTopicsRequestV0Topic struct {
	// Topic name
	Topic string

	// NumPartitions created. -1 indicates unset.
	NumPartitions int32

	// ReplicationFactor for the topic. -1 indicates unset.
	ReplicationFactor int16

	// ReplicaAssignments among kafka brokers for this topic partitions. If this
	// is set num_partitions and replication_factor must be unset.
	ReplicaAssignments []createTopicsRequestV0ReplicaAssignment

	// ConfigEntries holds topic level configuration for topic to be set.
	ConfigEntries []createTopicsRequestV0ConfigEntry
}

func (t createTopicsRequestV0Topic) size() int32 {
	return sizeofString(t.Topic) +
		sizeofInt32(t.NumPartitions) +
		sizeofInt16(t.ReplicationFactor) +
		sizeofArray(len(t.ReplicaAssignments), func(i int) int32 { return t.ReplicaAssignments[i].size() }) +
		sizeofArray(len(t.ConfigEntries), func(i int) int32 { return t.ConfigEntries[i].size() })
}

func (t createTopicsRequestV0Topic) writeTo(w *bufio.Writer) {
	writeString(w, t.Topic)
	writeInt32(w, t.NumPartitions)
	writeInt16(w, t.ReplicationFactor)
	writeArray(w, len(t.ReplicaAssignments), func(i int) { t.ReplicaAssignments[i].writeTo(w) })
	writeArray(w, len(t.ConfigEntries), func(i int) { t.ConfigEntries[i].writeTo(w) })
}

// See http://kafka.apache.org/protocol.html#The_Messages_CreateTopics
type createTopicsRequestV0 struct {
	// Topics contains n array of single topic creation requests. Can not
	// have multiple entries for the same topic.
	Topics []createTopicsRequestV0Topic

	// Timeout ms to wait for a topic to be completely created on the
	// controller node. Values <= 0 will trigger topic creation and return immediately
	Timeout int32
}

func (t createTopicsRequestV0) size() int32 {
	return sizeofArray(len(t.Topics), func(i int) int32 { return t.Topics[i].size() }) +
		sizeofInt32(t.Timeout)
}

func (t createTopicsRequestV0) writeTo(w *bufio.Writer) {
	writeArray(w, len(t.Topics), func(i int) { t.Topics[i].writeTo(w) })
	writeInt32(w, t.Timeout)
}

type createTopicsResponseV0TopicError struct {
	// Topic name
	Topic string

	// ErrorCode holds response error code
	ErrorCode int16
}

func (t createTopicsResponseV0TopicError) size() int32 {
	return sizeofString(t.Topic) +
		sizeofInt16(t.ErrorCode)
}

func (t createTopicsResponseV0TopicError) writeTo(w *bufio.Writer) {
	writeString(w, t.Topic)
	writeInt16(w, t.ErrorCode)
}

func (t *createTopicsResponseV0TopicError) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	if remain, err = readString(r, size, &t.Topic); err != nil {
		return
	}
	if remain, err = readInt16(r, remain, &t.ErrorCode); err != nil {
		return
	}
	return
}

// See http://kafka.apache.org/protocol.html#The_Messages_CreateTopics
type createTopicsResponseV0 struct {
	TopicErrors []createTopicsResponseV0TopicError
}

func (t createTopicsResponseV0) size() int32 {
	return sizeofArray(len(t.TopicErrors), func(i int) int32 { return t.TopicErrors[i].size() })
}

func (t createTopicsResponseV0) writeTo(w *bufio.Writer) {
	writeArray(w, len(t.TopicErrors), func(i int) { t.TopicErrors[i].writeTo(w) })
}

func (t *createTopicsResponseV0) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	fn := func(r *bufio.Reader, size int) (fnRemain int, fnErr error) {
		var topic createTopicsResponseV0TopicError
		if fnRemain, fnErr = (&topic).readFrom(r, size); err != nil {
			return
		}
		t.TopicErrors = append(t.TopicErrors, topic)
		return
	}
	if remain, err = readArrayWith(r, size, fn); err != nil {
		return
	}

	return
}

func (c *Conn) createTopics(request createTopicsRequestV0) (createTopicsResponseV0, error) {
	var response createTopicsResponseV0

	err := c.writeOperation(
		func(deadline time.Time, id int32) error {
			if request.Timeout == 0 {
				now := time.Now()
				deadline = adjustDeadlineForRTT(deadline, now, defaultRTT)
				request.Timeout = milliseconds(deadlineToTimeout(deadline, now))
			}
			return c.writeRequest(createTopicsRequest, v0, id, request)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(func() (remain int, err error) {
				return (&response).readFrom(&c.rbuf, size)
			}())
		},
	)
	if err != nil {
		return response, err
	}
	for _, tr := range response.TopicErrors {
		if tr.ErrorCode != 0 {
			return response, Error(tr.ErrorCode)
		}
	}

	return response, nil
}

// CreateTopics creates one topic per provided configuration with idempotent
// operational semantics. In other words, if CreateTopics is invoked with a
// configuration for an existing topic, it will have no effect.
func (c *Conn) CreateTopics(topics ...TopicConfig) error {
	var requestV0Topics []createTopicsRequestV0Topic
	for _, t := range topics {
		requestV0Topics = append(
			requestV0Topics,
			t.toCreateTopicsRequestV0Topic())
	}

	_, err := c.createTopics(createTopicsRequestV0{
		Topics: requestV0Topics,
	})

	switch err {
	case TopicAlreadyExists:
		// ok
		return nil
	default:
		return err
	}
}

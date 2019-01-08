package kafka

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	errInvalidWriteTopic     = errors.New("writes must NOT set Topic on kafka.Message")
	errInvalidWritePartition = errors.New("writes must NOT set Partition on kafka.Message")
)

// Broker carries the metadata associated with a kafka broker.
type Broker struct {
	Host string
	Port int
	ID   int
	Rack string
}

// Partition carries the metadata associated with a kafka partition.
type Partition struct {
	Topic    string
	Leader   Broker
	Replicas []Broker
	Isr      []Broker
	ID       int
}

// Conn represents a connection to a kafka broker.
//
// Instances of Conn are safe to use concurrently from multiple goroutines.
type Conn struct {
	// base network connection
	conn net.Conn

	// offset management (synchronized on the mutex field)
	mutex  sync.Mutex
	offset int64

	// read buffer (synchronized on rlock)
	rlock sync.Mutex
	rbuf  bufio.Reader

	// write buffer (synchronized on wlock)
	wlock sync.Mutex
	wbuf  bufio.Writer

	// deadline management
	wdeadline connDeadline
	rdeadline connDeadline

	// immutable values of the connection object
	clientID      string
	topic         string
	partition     int32
	fetchMaxBytes int32
	fetchMinSize  int32

	// correlation ID generator (synchronized on wlock)
	correlationID int32

	// number of replica acks required when publishing to a partition
	requiredAcks int32
}

// ConnConfig is a configuration object used to create new instances of Conn.
type ConnConfig struct {
	ClientID  string
	Topic     string
	Partition int
}

var (
	// DefaultClientID is the default value used as ClientID of kafka
	// connections.
	DefaultClientID string
)

func init() {
	progname := filepath.Base(os.Args[0])
	hostname, _ := os.Hostname()
	DefaultClientID = fmt.Sprintf("%s@%s (github.com/segmentio/kafka-go)", progname, hostname)
}

// NewConn returns a new kafka connection for the given topic and partition.
func NewConn(conn net.Conn, topic string, partition int) *Conn {
	return NewConnWith(conn, ConnConfig{
		Topic:     topic,
		Partition: partition,
	})
}

// NewConnWith returns a new kafka connection configured with config.
// The offset is initialized to FirstOffset.
func NewConnWith(conn net.Conn, config ConnConfig) *Conn {
	if len(config.ClientID) == 0 {
		config.ClientID = DefaultClientID
	}

	if config.Partition < 0 || config.Partition > math.MaxInt32 {
		panic(fmt.Sprintf("invalid partition number: %d", config.Partition))
	}

	c := &Conn{
		conn:         conn,
		rbuf:         *bufio.NewReader(conn),
		wbuf:         *bufio.NewWriter(conn),
		clientID:     config.ClientID,
		topic:        config.Topic,
		partition:    int32(config.Partition),
		offset:       FirstOffset,
		requiredAcks: -1,
	}

	// The fetch request needs to ask for a MaxBytes value that is at least
	// enough to load the control data of the response. To avoid having to
	// recompute it on every read, it is cached here in the Conn value.
	c.fetchMinSize = (fetchResponseV2{
		Topics: []fetchResponseTopicV2{{
			TopicName: config.Topic,
			Partitions: []fetchResponsePartitionV2{{
				Partition:  int32(config.Partition),
				MessageSet: messageSet{{}},
			}},
		}},
	}).size()
	c.fetchMaxBytes = math.MaxInt32 - c.fetchMinSize
	return c
}

// DeleteTopics deletes the specified topics.
func (c *Conn) DeleteTopics(topics ...string) error {
	_, err := c.deleteTopics(deleteTopicsRequestV1{
		Topics: topics,
	})
	return err
}

// describeGroups retrieves the specified groups
//
// See http://kafka.apache.org/protocol.html#The_Messages_DescribeGroups
func (c *Conn) describeGroups(request describeGroupsRequestV1) (describeGroupsResponseV1, error) {
	var response describeGroupsResponseV1

	err := c.readOperation(
		func(deadline time.Time, id int32) error {
			return c.writeRequest(describeGroupsRequest, v1, id, request)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(func() (remain int, err error) {
				return (&response).readFrom(&c.rbuf, size)
			}())
		},
	)
	if err != nil {
		return describeGroupsResponseV1{}, err
	}
	for _, group := range response.Groups {
		if group.ErrorCode != 0 {
			return describeGroupsResponseV1{}, Error(group.ErrorCode)
		}
	}

	return response, nil
}

// findCoordinator finds the coordinator for the specified group or transaction
//
// See http://kafka.apache.org/protocol.html#The_Messages_FindCoordinator
func (c *Conn) findCoordinator(request findCoordinatorRequestV0) (findCoordinatorResponseV0, error) {
	var response findCoordinatorResponseV0

	err := c.readOperation(
		func(deadline time.Time, id int32) error {
			return c.writeRequest(groupCoordinatorRequest, v0, id, request)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(func() (remain int, err error) {
				return (&response).readFrom(&c.rbuf, size)
			}())
		},
	)
	if err != nil {
		return findCoordinatorResponseV0{}, err
	}
	if response.ErrorCode != 0 {
		return findCoordinatorResponseV0{}, Error(response.ErrorCode)
	}

	return response, nil
}

// heartbeat sends a heartbeat message required by consumer groups
//
// See http://kafka.apache.org/protocol.html#The_Messages_Heartbeat
func (c *Conn) heartbeat(request heartbeatRequestV0) (heartbeatResponseV0, error) {
	var response heartbeatResponseV0

	err := c.writeOperation(
		func(deadline time.Time, id int32) error {
			return c.writeRequest(heartbeatRequest, v0, id, request)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(func() (remain int, err error) {
				return (&response).readFrom(&c.rbuf, size)
			}())
		},
	)
	if err != nil {
		return heartbeatResponseV0{}, err
	}
	if response.ErrorCode != 0 {
		return heartbeatResponseV0{}, Error(response.ErrorCode)
	}

	return response, nil
}

// joinGroup attempts to join a consumer group
//
// See http://kafka.apache.org/protocol.html#The_Messages_JoinGroup
func (c *Conn) joinGroup(request joinGroupRequestV1) (joinGroupResponseV1, error) {
	var response joinGroupResponseV1

	err := c.writeOperation(
		func(deadline time.Time, id int32) error {
			return c.writeRequest(joinGroupRequest, v1, id, request)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(func() (remain int, err error) {
				return (&response).readFrom(&c.rbuf, size)
			}())
		},
	)
	if err != nil {
		return joinGroupResponseV1{}, err
	}
	if response.ErrorCode != 0 {
		return joinGroupResponseV1{}, Error(response.ErrorCode)
	}

	return response, nil
}

// leaveGroup leaves the consumer from the consumer group
//
// See http://kafka.apache.org/protocol.html#The_Messages_LeaveGroup
func (c *Conn) leaveGroup(request leaveGroupRequestV0) (leaveGroupResponseV0, error) {
	var response leaveGroupResponseV0

	err := c.writeOperation(
		func(deadline time.Time, id int32) error {
			return c.writeRequest(leaveGroupRequest, v0, id, request)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(func() (remain int, err error) {
				return (&response).readFrom(&c.rbuf, size)
			}())
		},
	)
	if err != nil {
		return leaveGroupResponseV0{}, err
	}
	if response.ErrorCode != 0 {
		return leaveGroupResponseV0{}, Error(response.ErrorCode)
	}

	return response, nil
}

// listGroups lists all the consumer groups
//
// See http://kafka.apache.org/protocol.html#The_Messages_ListGroups
func (c *Conn) listGroups(request listGroupsRequestV1) (listGroupsResponseV1, error) {
	var response listGroupsResponseV1

	err := c.readOperation(
		func(deadline time.Time, id int32) error {
			return c.writeRequest(listGroupsRequest, v1, id, request)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(func() (remain int, err error) {
				return (&response).readFrom(&c.rbuf, size)
			}())
		},
	)
	if err != nil {
		return listGroupsResponseV1{}, err
	}
	if response.ErrorCode != 0 {
		return listGroupsResponseV1{}, Error(response.ErrorCode)
	}

	return response, nil
}

// offsetCommit commits the specified topic partition offsets
//
// See http://kafka.apache.org/protocol.html#The_Messages_OffsetCommit
func (c *Conn) offsetCommit(request offsetCommitRequestV2) (offsetCommitResponseV2, error) {
	var response offsetCommitResponseV2

	err := c.writeOperation(
		func(deadline time.Time, id int32) error {
			return c.writeRequest(offsetCommitRequest, v2, id, request)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(func() (remain int, err error) {
				return (&response).readFrom(&c.rbuf, size)
			}())
		},
	)
	if err != nil {
		return offsetCommitResponseV2{}, err
	}
	for _, r := range response.Responses {
		for _, pr := range r.PartitionResponses {
			if pr.ErrorCode != 0 {
				return offsetCommitResponseV2{}, Error(pr.ErrorCode)
			}
		}
	}

	return response, nil
}

// offsetFetch fetches the offsets for the specified topic partitions.
// -1 indicates that there is no offset saved for the partition.
//
// See http://kafka.apache.org/protocol.html#The_Messages_OffsetFetch
func (c *Conn) offsetFetch(request offsetFetchRequestV1) (offsetFetchResponseV1, error) {
	var response offsetFetchResponseV1

	err := c.readOperation(
		func(deadline time.Time, id int32) error {
			return c.writeRequest(offsetFetchRequest, v1, id, request)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(func() (remain int, err error) {
				return (&response).readFrom(&c.rbuf, size)
			}())
		},
	)
	if err != nil {
		return offsetFetchResponseV1{}, err
	}
	for _, r := range response.Responses {
		for _, pr := range r.PartitionResponses {
			if pr.ErrorCode != 0 {
				return offsetFetchResponseV1{}, Error(pr.ErrorCode)
			}
		}
	}

	return response, nil
}

// syncGroups completes the handshake to join a consumer group
//
// See http://kafka.apache.org/protocol.html#The_Messages_SyncGroup
func (c *Conn) syncGroups(request syncGroupRequestV0) (syncGroupResponseV0, error) {
	var response syncGroupResponseV0

	err := c.readOperation(
		func(deadline time.Time, id int32) error {
			return c.writeRequest(syncGroupRequest, v0, id, request)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(func() (remain int, err error) {
				return (&response).readFrom(&c.rbuf, size)
			}())
		},
	)
	if err != nil {
		return syncGroupResponseV0{}, err
	}
	if response.ErrorCode != 0 {
		return syncGroupResponseV0{}, Error(response.ErrorCode)
	}

	return response, nil
}

// Close closes the kafka connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// LocalAddr returns the local network address.
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// SetDeadline sets the read and write deadlines associated with the connection.
// It is equivalent to calling both SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations fail with a timeout
// (see type Error) instead of blocking. The deadline applies to all future and
// pending I/O, not just the immediately following call to Read or Write. After
// a deadline has been exceeded, the connection may be closed if it was found to
// be in an unrecoverable state.
//
// A zero value for t means I/O operations will not time out.
func (c *Conn) SetDeadline(t time.Time) error {
	c.rdeadline.setDeadline(t)
	c.wdeadline.setDeadline(t)
	return nil
}

// SetReadDeadline sets the deadline for future Read calls and any
// currently-blocked Read call.
// A zero value for t means Read will not time out.
func (c *Conn) SetReadDeadline(t time.Time) error {
	c.rdeadline.setDeadline(t)
	return nil
}

// SetWriteDeadline sets the deadline for future Write calls and any
// currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that some of the
// data was successfully written.
// A zero value for t means Write will not time out.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.wdeadline.setDeadline(t)
	return nil
}

// Offset returns the current offset of the connection as pair of integers,
// where the first one is an offset value and the second one indicates how
// to interpret it.
//
// See Seek for more details about the offset and whence values.
func (c *Conn) Offset() (offset int64, whence int) {
	c.mutex.Lock()
	offset = c.offset
	c.mutex.Unlock()

	switch offset {
	case FirstOffset:
		offset = 0
		whence = SeekStart
	case LastOffset:
		offset = 0
		whence = SeekEnd
	default:
		whence = SeekAbsolute
	}
	return
}

const (
	SeekStart    = 0 // Seek relative to the first offset available in the partition.
	SeekAbsolute = 1 // Seek to an absolute offset.
	SeekEnd      = 2 // Seek relative to the last offset available in the partition.
	SeekCurrent  = 3 // Seek relative to the current offset.
)

// Seek sets the offset for the next read or write operation according to whence, which
// should be one of SeekStart, SeekAbsolute, SeekEnd, or SeekCurrent.
// When seeking relative to the end, the offset is subtracted from the current offset.
// Note that for historical reasons, these do not align with the usual whence constants
// as in lseek(2) or os.Seek.
// The method returns the new absolute offset of the connection.
func (c *Conn) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case SeekStart, SeekAbsolute, SeekEnd, SeekCurrent:
	default:
		return 0, fmt.Errorf("whence must be one of 0, 1, 2, or 3. (whence = %d)", whence)
	}

	if whence == SeekAbsolute {
		c.mutex.Lock()
		unchanged := offset == c.offset
		c.mutex.Unlock()
		if unchanged {
			return offset, nil
		}
	}
	if whence == SeekCurrent {
		c.mutex.Lock()
		offset = c.offset + offset
		c.mutex.Unlock()
	}

	first, last, err := c.ReadOffsets()
	if err != nil {
		return 0, err
	}

	switch whence {
	case SeekStart:
		offset = first + offset
	case SeekEnd:
		offset = last - offset
	}

	if offset < first || offset > last {
		return 0, OffsetOutOfRange
	}

	c.mutex.Lock()
	c.offset = offset
	c.mutex.Unlock()
	return offset, nil
}

// Read reads the message at the current offset from the connection, advancing
// the offset on success so the next call to a read method will produce the next
// message.
// The method returns the number of bytes read, or an error if something went
// wrong.
//
// While it is safe to call Read concurrently from multiple goroutines it may
// be hard for the program to predict the results as the connection offset will
// be read and written by multiple goroutines, they could read duplicates, or
// messages may be seen by only some of the goroutines.
//
// The method fails with io.ErrShortBuffer if the buffer passed as argument is
// too small to hold the message value.
//
// This method is provided to satisfy the net.Conn interface but is much less
// efficient than using the more general purpose ReadBatch method.
func (c *Conn) Read(b []byte) (int, error) {
	batch := c.ReadBatch(1, len(b))
	n, err := batch.Read(b)
	return n, coalesceErrors(silentEOF(err), batch.Close())
}

// ReadMessage reads the message at the current offset from the connection,
// advancing the offset on success so the next call to a read method will
// produce the next message.
//
// Because this method allocate memory buffers for the message key and value
// it is less memory-efficient than Read, but has the advantage of never
// failing with io.ErrShortBuffer.
//
// While it is safe to call Read concurrently from multiple goroutines it may
// be hard for the program to predict the results as the connection offset will
// be read and written by multiple goroutines, they could read duplicates, or
// messages may be seen by only some of the goroutines.
//
// This method is provided for convenience purposes but is much less efficient
// than using the more general purpose ReadBatch method.
func (c *Conn) ReadMessage(maxBytes int) (Message, error) {
	batch := c.ReadBatch(1, maxBytes)
	msg, err := batch.ReadMessage()
	return msg, coalesceErrors(silentEOF(err), batch.Close())
}

// ReadBatch reads a batch of messages from the kafka server. The method always
// returns a non-nil Batch value. If an error occurred, either sending the fetch
// request or reading the response, the error will be made available by the
// returned value of  the batch's Close method.
//
// While it is safe to call ReadBatch concurrently from multiple goroutines it
// may be hard for the program to predict the results as the connection offset
// will be read and written by multiple goroutines, they could read duplicates,
// or messages may be seen by only some of the goroutines.
//
// A program doesn't specify the number of messages in wants from a batch, but
// gives the minimum and maximum number of bytes that it wants to receive from
// the kafka server.
func (c *Conn) ReadBatch(minBytes, maxBytes int) *Batch {
	var adjustedDeadline time.Time
	var maxFetch = int(c.fetchMaxBytes)

	if minBytes < 0 || minBytes > maxFetch {
		return &Batch{err: fmt.Errorf("kafka.(*Conn).ReadBatch: minBytes of %d out of [1,%d] bounds", minBytes, maxFetch)}
	}
	if maxBytes < 0 || maxBytes > maxFetch {
		return &Batch{err: fmt.Errorf("kafka.(*Conn).ReadBatch: maxBytes of %d out of [1,%d] bounds", maxBytes, maxFetch)}
	}
	if minBytes > maxBytes {
		return &Batch{err: fmt.Errorf("kafka.(*Conn).ReadBatch: minBytes (%d) > maxBytes (%d)", minBytes, maxBytes)}
	}

	offset, err := c.Seek(c.Offset())
	if err != nil {
		return &Batch{err: dontExpectEOF(err)}
	}

	id, err := c.doRequest(&c.rdeadline, func(deadline time.Time, id int32) error {
		now := time.Now()
		deadline = adjustDeadlineForRTT(deadline, now, defaultRTT)
		adjustedDeadline = deadline
		return writeFetchRequestV2(
			&c.wbuf,
			id,
			c.clientID,
			c.topic,
			c.partition,
			offset,
			minBytes,
			maxBytes+int(c.fetchMinSize),
			deadlineToTimeout(deadline, now),
		)
	})
	if err != nil {
		return &Batch{err: dontExpectEOF(err)}
	}

	_, size, lock, err := c.waitResponse(&c.rdeadline, id)
	if err != nil {
		return &Batch{err: dontExpectEOF(err)}
	}

	throttle, highWaterMark, remain, err := readFetchResponseHeader(&c.rbuf, size)
	return &Batch{
		conn:          c,
		msgs:          newMessageSetReader(&c.rbuf, remain),
		deadline:      adjustedDeadline,
		throttle:      duration(throttle),
		lock:          lock,
		topic:         c.topic,          // topic is copied to Batch to prevent race with Batch.close
		partition:     int(c.partition), // partition is copied to Batch to prevent race with Batch.close
		offset:        offset,
		highWaterMark: highWaterMark,
		err:           dontExpectEOF(err),
	}
}

// ReadOffset returns the offset of the first message with a timestamp equal or
// greater to t.
func (c *Conn) ReadOffset(t time.Time) (int64, error) {
	return c.readOffset(timestamp(t))
}

// ReadFirstOffset returns the first offset available on the connection.
func (c *Conn) ReadFirstOffset() (int64, error) {
	return c.readOffset(FirstOffset)
}

// ReadLastOffset returns the last offset available on the connection.
func (c *Conn) ReadLastOffset() (int64, error) {
	return c.readOffset(LastOffset)
}

// ReadOffsets returns the absolute first and last offsets of the topic used by
// the connection.
func (c *Conn) ReadOffsets() (first, last int64, err error) {
	// We have to submit two different requests to fetch the first and last
	// offsets because kafka refuses requests that ask for multiple offsets
	// on the same topic and partition.
	if first, err = c.ReadFirstOffset(); err != nil {
		return
	}
	if last, err = c.ReadLastOffset(); err != nil {
		first = 0 // don't leak the value on error
		return
	}
	return
}

func (c *Conn) readOffset(t int64) (offset int64, err error) {
	err = c.readOperation(
		func(deadline time.Time, id int32) error {
			return writeListOffsetRequestV1(&c.wbuf, id, c.clientID, c.topic, c.partition, t)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(readArrayWith(&c.rbuf, size, func(r *bufio.Reader, size int) (int, error) {
				// We skip the topic name because we've made a request for
				// a single topic.
				size, err := discardString(r, size)
				if err != nil {
					return size, err
				}

				// Reading the array of partitions, there will be only one
				// partition which gives the offset we're looking for.
				return readArrayWith(r, size, func(r *bufio.Reader, size int) (int, error) {
					var p partitionOffsetV1
					size, err := p.readFrom(r, size)
					if err != nil {
						return size, err
					}
					if p.ErrorCode != 0 {
						return size, Error(p.ErrorCode)
					}
					offset = p.Offset
					return size, nil
				})
			}))
		},
	)
	return
}

// ReadPartitions returns the list of available partitions for the given list of
// topics.
//
// If the method is called with no topic, it uses the topic configured on the
// connection. If there are none, the method fetches all partitions of the kafka
// cluster.
func (c *Conn) ReadPartitions(topics ...string) (partitions []Partition, err error) {
	defaultTopics := [...]string{c.topic}

	if len(topics) == 0 && len(c.topic) != 0 {
		topics = defaultTopics[:]
	}

	err = c.readOperation(
		func(deadline time.Time, id int32) error {
			return c.writeRequest(metadataRequest, v1, id, topicMetadataRequestV1(topics))
		},
		func(deadline time.Time, size int) error {
			var res metadataResponseV1

			if err := c.readResponse(size, &res); err != nil {
				return err
			}

			brokers := make(map[int32]Broker, len(res.Brokers))
			for _, b := range res.Brokers {
				brokers[b.NodeID] = Broker{
					Host: b.Host,
					Port: int(b.Port),
					ID:   int(b.NodeID),
					Rack: b.Rack,
				}
			}

			makeBrokers := func(ids ...int32) []Broker {
				b := make([]Broker, len(ids))
				for i, id := range ids {
					b[i] = brokers[id]
				}
				return b
			}

			for _, t := range res.Topics {
				if t.TopicErrorCode != 0 && t.TopicName == c.topic {
					// We only report errors if they happened for the topic of
					// the connection, otherwise the topic will simply have no
					// partitions in the result set.
					return Error(t.TopicErrorCode)
				}
				for _, p := range t.Partitions {
					partitions = append(partitions, Partition{
						Topic:    t.TopicName,
						Leader:   brokers[p.Leader],
						Replicas: makeBrokers(p.Replicas...),
						Isr:      makeBrokers(p.Isr...),
						ID:       int(p.PartitionID),
					})
				}
			}
			return nil
		},
	)
	return
}

// Write writes a message to the kafka broker that this connection was
// established to. The method returns the number of bytes written, or an error
// if something went wrong.
//
// The operation either succeeds or fail, it never partially writes the message.
//
// This method is exposed to satisfy the net.Conn interface but is less efficient
// than the more general purpose WriteMessages method.
func (c *Conn) Write(b []byte) (int, error) {
	return c.WriteCompressedMessages(nil, Message{Value: b})
}

// WriteMessages writes a batch of messages to the connection's topic and
// partition, returning the number of bytes written. The write is an atomic
// operation, it either fully succeeds or fails.
func (c *Conn) WriteMessages(msgs ...Message) (int, error) {
	return c.WriteCompressedMessages(nil, msgs...)
}

// WriteCompressedMessages writes a batch of messages to the connection's topic
// and partition, returning the number of bytes written. The write is an atomic
// operation, it either fully succeeds or fails.
//
// If the compression codec is not nil, the messages will be compressed.
func (c *Conn) WriteCompressedMessages(codec CompressionCodec, msgs ...Message) (int, error) {
	if len(msgs) == 0 {
		return 0, nil
	}

	writeTime := time.Now()
	n := 0
	for i, msg := range msgs {
		// users may believe they can set the Topic and/or Partition
		// on the kafka message.
		if msg.Topic != "" && msg.Topic != c.topic {
			return 0, errInvalidWriteTopic
		}
		if msg.Partition != 0 {
			return 0, errInvalidWritePartition
		}

		if msg.Time.IsZero() {
			msgs[i].Time = writeTime
		}

		n += len(msg.Key) + len(msg.Value)
	}

	err := c.writeOperation(
		func(deadline time.Time, id int32) error {
			now := time.Now()
			deadline = adjustDeadlineForRTT(deadline, now, defaultRTT)
			return writeProduceRequestV2(
				&c.wbuf,
				codec,
				id,
				c.clientID,
				c.topic,
				c.partition,
				deadlineToTimeout(deadline, now),
				int16(atomic.LoadInt32(&c.requiredAcks)),
				msgs...,
			)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(readArrayWith(&c.rbuf, size, func(r *bufio.Reader, size int) (int, error) {
				// Skip the topic, we've produced the message to only one topic,
				// no need to waste resources loading it in memory.
				size, err := discardString(r, size)
				if err != nil {
					return size, err
				}

				// Read the list of partitions, there should be only one since
				// we've produced a message to a single partition.
				size, err = readArrayWith(r, size, func(r *bufio.Reader, size int) (int, error) {
					var p produceResponsePartitionV2
					size, err := p.readFrom(r, size)
					if err == nil && p.ErrorCode != 0 {
						err = Error(p.ErrorCode)
					}
					return size, err
				})
				if err != nil {
					return size, err
				}

				// The response is trailed by the throttle time, also skipping
				// since it's not interesting here.
				return discardInt32(r, size)
			}))
		},
	)

	if err != nil {
		n = 0
	}

	return n, err
}

// SetRequiredAcks sets the number of acknowledges from replicas that the
// connection requests when producing messages.
func (c *Conn) SetRequiredAcks(n int) error {
	switch n {
	case -1, 1:
		atomic.StoreInt32(&c.requiredAcks, int32(n))
		return nil
	default:
		return InvalidRequiredAcks
	}
}

func (c *Conn) writeRequestHeader(apiKey apiKey, apiVersion apiVersion, correlationID int32, size int32) {
	hdr := c.requestHeader(apiKey, apiVersion, correlationID)
	hdr.Size = (hdr.size() + size) - 4
	hdr.writeTo(&c.wbuf)
}

func (c *Conn) writeRequest(apiKey apiKey, apiVersion apiVersion, correlationID int32, req request) error {
	hdr := c.requestHeader(apiKey, apiVersion, correlationID)
	hdr.Size = (hdr.size() + req.size()) - 4
	hdr.writeTo(&c.wbuf)
	req.writeTo(&c.wbuf)
	return c.wbuf.Flush()
}

func (c *Conn) readResponse(size int, res interface{}) error {
	size, err := read(&c.rbuf, size, res)
	switch err.(type) {
	case Error:
		var e error
		if size, e = discardN(&c.rbuf, size, size); e != nil {
			err = e
		}
	}
	return expectZeroSize(size, err)
}

func (c *Conn) peekResponseSizeAndID() (int32, int32, error) {
	b, err := c.rbuf.Peek(8)
	if err != nil {
		return 0, 0, err
	}
	size, id := makeInt32(b[:4]), makeInt32(b[4:])
	return size, id, nil
}

func (c *Conn) skipResponseSizeAndID() {
	c.rbuf.Discard(8)
}

func (c *Conn) readDeadline() time.Time {
	return c.rdeadline.deadline()
}

func (c *Conn) writeDeadline() time.Time {
	return c.wdeadline.deadline()
}

func (c *Conn) readOperation(write func(time.Time, int32) error, read func(time.Time, int) error) error {
	return c.do(&c.rdeadline, write, read)
}

func (c *Conn) writeOperation(write func(time.Time, int32) error, read func(time.Time, int) error) error {
	return c.do(&c.wdeadline, write, read)
}

func (c *Conn) do(d *connDeadline, write func(time.Time, int32) error, read func(time.Time, int) error) error {
	id, err := c.doRequest(d, write)
	if err != nil {
		return err
	}

	deadline, size, lock, err := c.waitResponse(d, id)
	if err != nil {
		return err
	}

	if err = read(deadline, size); err != nil {
		switch err.(type) {
		case Error:
		default:
			c.conn.Close()
		}
	}

	d.unsetConnReadDeadline()
	lock.Unlock()
	return err
}

func (c *Conn) doRequest(d *connDeadline, write func(time.Time, int32) error) (id int32, err error) {
	c.wlock.Lock()
	c.correlationID++
	id = c.correlationID
	err = write(d.setConnWriteDeadline(c.conn), id)
	d.unsetConnWriteDeadline()

	if err != nil {
		// When an error occurs there's no way to know if the connection is in a
		// recoverable state so we're better off just giving up at this point to
		// avoid any risk of corrupting the following operations.
		c.conn.Close()
	}

	c.wlock.Unlock()
	return
}

func (c *Conn) waitResponse(d *connDeadline, id int32) (deadline time.Time, size int, lock *sync.Mutex, err error) {
	for {
		var rsz int32
		var rid int32

		c.rlock.Lock()
		deadline = d.setConnReadDeadline(c.conn)

		if rsz, rid, err = c.peekResponseSizeAndID(); err != nil {
			d.unsetConnReadDeadline()
			c.conn.Close()
			c.rlock.Unlock()
			return
		}

		if id == rid {
			c.skipResponseSizeAndID()
			size, lock = int(rsz-4), &c.rlock
			return
		}

		// Optimistically release the read lock if a response has already
		// been received but the current operation is not the target for it.
		c.rlock.Unlock()
		runtime.Gosched()
	}
}

func (c *Conn) requestHeader(apiKey apiKey, apiVersion apiVersion, correlationID int32) requestHeader {
	return requestHeader{
		ApiKey:        int16(apiKey),
		ApiVersion:    int16(apiVersion),
		CorrelationID: correlationID,
		ClientID:      c.clientID,
	}
}

// connDeadline is a helper type to implement read/write deadline management on
// the kafka connection.
type connDeadline struct {
	mutex sync.Mutex
	value time.Time
	rconn net.Conn
	wconn net.Conn
}

func (d *connDeadline) deadline() time.Time {
	d.mutex.Lock()
	t := d.value
	d.mutex.Unlock()
	return t
}

func (d *connDeadline) setDeadline(t time.Time) {
	d.mutex.Lock()
	d.value = t

	if d.rconn != nil {
		d.rconn.SetReadDeadline(t)
	}

	if d.wconn != nil {
		d.wconn.SetWriteDeadline(t)
	}

	d.mutex.Unlock()
}

func (d *connDeadline) setConnReadDeadline(conn net.Conn) time.Time {
	d.mutex.Lock()
	deadline := d.value
	d.rconn = conn
	d.rconn.SetReadDeadline(deadline)
	d.mutex.Unlock()
	return deadline
}

func (d *connDeadline) setConnWriteDeadline(conn net.Conn) time.Time {
	d.mutex.Lock()
	deadline := d.value
	d.wconn = conn
	d.wconn.SetWriteDeadline(deadline)
	d.mutex.Unlock()
	return deadline
}

func (d *connDeadline) unsetConnReadDeadline() {
	d.mutex.Lock()
	d.rconn = nil
	d.mutex.Unlock()
}

func (d *connDeadline) unsetConnWriteDeadline() {
	d.mutex.Lock()
	d.wconn = nil
	d.mutex.Unlock()
}

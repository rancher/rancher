package kafka

import (
	"bufio"
	"time"
)

// See http://kafka.apache.org/protocol.html#The_Messages_DeleteTopics
type deleteTopicsRequestV1 struct {
	// Topics holds the topic names
	Topics []string

	// Timeout holds the time in ms to wait for a topic to be completely deleted
	// on the controller node. Values <= 0 will trigger topic deletion and return
	// immediately.
	Timeout int32
}

func (t deleteTopicsRequestV1) size() int32 {
	return sizeofStringArray(t.Topics) +
		sizeofInt32(t.Timeout)
}

func (t deleteTopicsRequestV1) writeTo(w *bufio.Writer) {
	writeStringArray(w, t.Topics)
	writeInt32(w, t.Timeout)
}

type deleteTopicsResponseV1 struct {
	// ThrottleTimeMS holds the duration in milliseconds for which the request
	// was throttled due to quota violation (Zero if the request did not violate
	// any quota)
	ThrottleTimeMS int32

	// TopicErrorCodes holds per topic error codes
	TopicErrorCodes []deleteTopicsResponseV1TopicErrorCode
}

func (t deleteTopicsResponseV1) size() int32 {
	return sizeofInt32(t.ThrottleTimeMS) +
		sizeofArray(len(t.TopicErrorCodes), func(i int) int32 { return t.TopicErrorCodes[i].size() })
}

func (t *deleteTopicsResponseV1) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	if remain, err = readInt32(r, size, &t.ThrottleTimeMS); err != nil {
		return
	}
	fn := func(withReader *bufio.Reader, withSize int) (fnRemain int, fnErr error) {
		var item deleteTopicsResponseV1TopicErrorCode
		if fnRemain, fnErr = (&item).readFrom(withReader, withSize); err != nil {
			return
		}
		t.TopicErrorCodes = append(t.TopicErrorCodes, item)
		return
	}
	if remain, err = readArrayWith(r, remain, fn); err != nil {
		return
	}
	return
}

func (t deleteTopicsResponseV1) writeTo(w *bufio.Writer) {
	writeInt32(w, t.ThrottleTimeMS)
	writeArray(w, len(t.TopicErrorCodes), func(i int) { t.TopicErrorCodes[i].writeTo(w) })
}

type deleteTopicsResponseV1TopicErrorCode struct {
	// Topic holds the topic name
	Topic string

	// ErrorCode holds the error code
	ErrorCode int16
}

func (t deleteTopicsResponseV1TopicErrorCode) size() int32 {
	return sizeofString(t.Topic) +
		sizeofInt16(t.ErrorCode)
}

func (t *deleteTopicsResponseV1TopicErrorCode) readFrom(r *bufio.Reader, size int) (remain int, err error) {
	if remain, err = readString(r, size, &t.Topic); err != nil {
		return
	}
	if remain, err = readInt16(r, remain, &t.ErrorCode); err != nil {
		return
	}
	return
}

func (t deleteTopicsResponseV1TopicErrorCode) writeTo(w *bufio.Writer) {
	writeString(w, t.Topic)
	writeInt16(w, t.ErrorCode)
}

// deleteTopics deletes the specified topics.
//
// See http://kafka.apache.org/protocol.html#The_Messages_DeleteTopics
func (c *Conn) deleteTopics(request deleteTopicsRequestV1) (deleteTopicsResponseV1, error) {
	var response deleteTopicsResponseV1
	err := c.writeOperation(
		func(deadline time.Time, id int32) error {
			if request.Timeout == 0 {
				now := time.Now()
				deadline = adjustDeadlineForRTT(deadline, now, defaultRTT)
				request.Timeout = milliseconds(deadlineToTimeout(deadline, now))
			}
			return c.writeRequest(deleteTopicsRequest, v1, id, request)
		},
		func(deadline time.Time, size int) error {
			return expectZeroSize(func() (remain int, err error) {
				return (&response).readFrom(&c.rbuf, size)
			}())
		},
	)
	if err != nil {
		return deleteTopicsResponseV1{}, err
	}
	for _, c := range response.TopicErrorCodes {
		if c.ErrorCode != 0 {
			return response, Error(c.ErrorCode)
		}
	}
	return response, nil
}

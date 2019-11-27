package kafka

import (
	"bufio"
	"io"
	"sync"
	"time"
)

// A Batch is an iterator over a sequence of messages fetched from a kafka
// server.
//
// Batches are created by calling (*Conn).ReadBatch. They hold a internal lock
// on the connection, which is released when the batch is closed. Failing to
// call a batch's Close method will likely result in a dead-lock when trying to
// use the connection.
//
// Batches are safe to use concurrently from multiple goroutines.
type Batch struct {
	mutex         sync.Mutex
	conn          *Conn
	lock          *sync.Mutex
	msgs          *messageSetReader
	deadline      time.Time
	throttle      time.Duration
	topic         string
	partition     int
	offset        int64
	highWaterMark int64
	err           error
}

// Throttle gives the throttling duration applied by the kafka server on the
// connection.
func (batch *Batch) Throttle() time.Duration {
	return batch.throttle
}

// Watermark returns the current highest watermark in a partition.
func (batch *Batch) HighWaterMark() int64 {
	return batch.highWaterMark
}

// Offset returns the offset of the next message in the batch.
func (batch *Batch) Offset() int64 {
	batch.mutex.Lock()
	offset := batch.offset
	batch.mutex.Unlock()
	return offset
}

// Close closes the batch, releasing the connection lock and returning an error
// if reading the batch failed for any reason.
func (batch *Batch) Close() error {
	batch.mutex.Lock()
	err := batch.close()
	batch.mutex.Unlock()
	return err
}

func (batch *Batch) close() (err error) {
	conn := batch.conn
	lock := batch.lock

	batch.conn = nil
	batch.lock = nil
	if batch.msgs != nil {
		batch.msgs.discard()
	}

	if err = batch.err; err == io.EOF {
		err = nil
	}

	if conn != nil {
		conn.rdeadline.unsetConnReadDeadline()
		conn.mutex.Lock()
		conn.offset = batch.offset
		conn.mutex.Unlock()

		if err != nil {
			if _, ok := err.(Error); !ok && err != io.ErrShortBuffer {
				conn.Close()
			}
		}
	}

	if lock != nil {
		lock.Unlock()
	}

	return
}

// Err returns a non-nil error if the batch is broken. This is the same error
// that would be returned by Read, ReadMessage or Close (except in the case of
// io.EOF which is never returned by Close).
//
// This method is useful when building retry mechanisms for (*Conn).ReadBatch,
// the program can check whether the batch carried a error before attempting to
// read the first message.
//
// Note that checking errors on a batch is optional, calling Read or ReadMessage
// is always valid and can be used to either read a message or an error in cases
// where that's convenient.
func (batch *Batch) Err() error { return batch.err }

// Read reads the value of the next message from the batch into b, returning the
// number of bytes read, or an error if the next message couldn't be read.
//
// If an error is returned the batch cannot be used anymore and calling Read
// again will keep returning that error. All errors except io.EOF (indicating
// that the program consumed all messages from the batch) are also returned by
// Close.
//
// The method fails with io.ErrShortBuffer if the buffer passed as argument is
// too small to hold the message value.
func (batch *Batch) Read(b []byte) (int, error) {
	n := 0

	batch.mutex.Lock()
	offset := batch.offset

	_, _, _, err := batch.readMessage(
		func(r *bufio.Reader, size int, nbytes int) (int, error) {
			if nbytes < 0 {
				return size, nil
			}
			return discardN(r, size, nbytes)
		},
		func(r *bufio.Reader, size int, nbytes int) (int, error) {
			if nbytes < 0 {
				return size, nil
			}
			// make sure there are enough bytes for the message value.  return
			// errShortRead if the message is truncated.
			if nbytes > size {
				return size, errShortRead
			}
			n = nbytes // return value
			if nbytes > cap(b) {
				nbytes = cap(b)
			}
			if nbytes > len(b) {
				b = b[:nbytes]
			}
			nbytes, err := io.ReadFull(r, b[:nbytes])
			if err != nil {
				return size - nbytes, err
			}
			return discardN(r, size-nbytes, n-nbytes)
		},
	)

	if err == nil && n > len(b) {
		n, err = len(b), io.ErrShortBuffer
		batch.err = io.ErrShortBuffer
		batch.offset = offset // rollback
	}

	batch.mutex.Unlock()
	return n, err
}

// ReadMessage reads and return the next message from the batch.
//
// Because this method allocate memory buffers for the message key and value
// it is less memory-efficient than Read, but has the advantage of never
// failing with io.ErrShortBuffer.
func (batch *Batch) ReadMessage() (Message, error) {
	msg := Message{}
	batch.mutex.Lock()

	var offset, timestamp int64
	var headers []Header
	var err error

	offset, timestamp, headers, err = batch.readMessage(
		func(r *bufio.Reader, size int, nbytes int) (remain int, err error) {
			msg.Key, remain, err = readNewBytes(r, size, nbytes)
			return
		},
		func(r *bufio.Reader, size int, nbytes int) (remain int, err error) {
			msg.Value, remain, err = readNewBytes(r, size, nbytes)
			return
		},
	)
	for batch.conn != nil && offset < batch.conn.offset {
		if err != nil {
			break
		}
		offset, timestamp, headers, err = batch.readMessage(
			func(r *bufio.Reader, size int, nbytes int) (remain int, err error) {
				msg.Key, remain, err = readNewBytes(r, size, nbytes)
				return
			},
			func(r *bufio.Reader, size int, nbytes int) (remain int, err error) {
				msg.Value, remain, err = readNewBytes(r, size, nbytes)
				return
			},
		)
	}

	batch.mutex.Unlock()
	msg.Topic = batch.topic
	msg.Partition = batch.partition
	msg.Offset = offset
	msg.Time = timestampToTime(timestamp)
	msg.Headers = headers

	return msg, err
}

func (batch *Batch) readMessage(
	key func(*bufio.Reader, int, int) (int, error),
	val func(*bufio.Reader, int, int) (int, error),
) (offset int64, timestamp int64, headers []Header, err error) {
	if err = batch.err; err != nil {
		return
	}

	offset, timestamp, headers, err = batch.msgs.readMessage(batch.offset, key, val)
	switch err {
	case nil:
		batch.offset = offset + 1
	case errShortRead:
		// As an "optimization" kafka truncates the returned response after
		// producing MaxBytes, which could then cause the code to return
		// errShortRead.
		err = batch.msgs.discard()
		switch {
		case err != nil:
			// Since io.EOF is used by the batch to indicate that there is are
			// no more messages to consume, it is crucial that any io.EOF errors
			// on the underlying connection are repackaged.  Otherwise, the
			// caller can't tell the difference between a batch that was fully
			// consumed or a batch whose connection is in an error state.
			batch.err = dontExpectEOF(err)
		case batch.msgs.remaining() == 0:
			// Because we use the adjusted deadline we could end up returning
			// before the actual deadline occurred. This is necessary otherwise
			// timing out the connection for real could end up leaving it in an
			// unpredictable state, which would require closing it.
			// This design decision was made to maximize the chances of keeping
			// the connection open, the trade off being to lose precision on the
			// read deadline management.
			err = checkTimeoutErr(batch.deadline)
			batch.err = err
		}
	default:
		// Since io.EOF is used by the batch to indicate that there is are
		// no more messages to consume, it is crucial that any io.EOF errors
		// on the underlying connection are repackaged.  Otherwise, the
		// caller can't tell the difference between a batch that was fully
		// consumed or a batch whose connection is in an error state.
		batch.err = dontExpectEOF(err)
	}

	return
}

func checkTimeoutErr(deadline time.Time) (err error) {
	if !deadline.IsZero() && time.Now().After(deadline) {
		err = RequestTimedOut
	} else {
		err = io.EOF
	}
	return
}

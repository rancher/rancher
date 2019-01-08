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

	_, _, err := batch.readMessage(
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
			n = nbytes // return value
			if nbytes > len(b) {
				nbytes = len(b)
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

	offset, timestamp, err := batch.readMessage(
		func(r *bufio.Reader, size int, nbytes int) (remain int, err error) {
			msg.Key, remain, err = readNewBytes(r, size, nbytes)
			return
		},
		func(r *bufio.Reader, size int, nbytes int) (remain int, err error) {
			msg.Value, remain, err = readNewBytes(r, size, nbytes)
			return
		},
	)

	batch.mutex.Unlock()
	msg.Topic = batch.topic
	msg.Partition = batch.partition
	msg.Offset = offset
	msg.Time = timestampToTime(timestamp)

	return msg, err
}

func (batch *Batch) readMessage(
	key func(*bufio.Reader, int, int) (int, error),
	val func(*bufio.Reader, int, int) (int, error),
) (offset int64, timestamp int64, err error) {
	if err = batch.err; err != nil {
		return
	}

	offset, timestamp, err = batch.msgs.readMessage(batch.offset, key, val)
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
			batch.err = err
		case batch.msgs.remaining() == 0:
			// Because we use the adjusted deadline we could end up returning
			// before the actual deadline occurred. This is necessary otherwise
			// timing out the connection for real could end up leaving it in an
			// unpredictable state, which would require closing it.
			// This design decision was made to maximize the chances of keeping
			// the connection open, the trade off being to lose precision on the
			// read deadline management.
			if !batch.deadline.IsZero() && time.Now().After(batch.deadline) {
				err = RequestTimedOut
			} else {
				err = io.EOF
			}
			batch.err = err
		}
	default:
		batch.err = err
	}

	return
}

package kafka

import (
	"bufio"
	"bytes"
	"time"
)

// Message is a data structure representing kafka messages.
type Message struct {
	// Topic is reads only and MUST NOT be set when writing messages
	Topic string

	// Partition is reads only and MUST NOT be set when writing messages
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte

	// If not set at the creation, Time will be automatically set when
	// writing the message.
	Time time.Time
}

func (msg Message) item() messageSetItem {
	item := messageSetItem{
		Offset:  msg.Offset,
		Message: msg.message(),
	}
	item.MessageSize = item.Message.size()
	return item
}

func (msg Message) message() message {
	m := message{
		MagicByte: 1,
		Key:       msg.Key,
		Value:     msg.Value,
		Timestamp: timestamp(msg.Time),
	}
	m.CRC = m.crc32()
	return m
}

type message struct {
	CRC        int32
	MagicByte  int8
	Attributes int8
	Timestamp  int64
	Key        []byte
	Value      []byte
}

func (m message) crc32() int32 {
	return int32(crc32OfMessage(m.MagicByte, m.Attributes, m.Timestamp, m.Key, m.Value))
}

func (m message) size() int32 {
	size := 4 + 1 + 1 + sizeofBytes(m.Key) + sizeofBytes(m.Value)
	if m.MagicByte != 0 {
		size += 8 // Timestamp
	}
	return size
}

func (m message) writeTo(w *bufio.Writer) {
	writeInt32(w, m.CRC)
	writeInt8(w, m.MagicByte)
	writeInt8(w, m.Attributes)
	if m.MagicByte != 0 {
		writeInt64(w, m.Timestamp)
	}
	writeBytes(w, m.Key)
	writeBytes(w, m.Value)
}

type messageSetItem struct {
	Offset      int64
	MessageSize int32
	Message     message
}

func (m messageSetItem) size() int32 {
	return 8 + 4 + m.Message.size()
}

func (m messageSetItem) writeTo(w *bufio.Writer) {
	writeInt64(w, m.Offset)
	writeInt32(w, m.MessageSize)
	m.Message.writeTo(w)
}

type messageSet []messageSetItem

func (s messageSet) size() (size int32) {
	for _, m := range s {
		size += m.size()
	}
	return
}

func (s messageSet) writeTo(w *bufio.Writer) {
	for _, m := range s {
		m.writeTo(w)
	}
}

type messageSetReader struct {
	*readerStack
}

type readerStack struct {
	reader *bufio.Reader
	remain int
	base   int64
	parent *readerStack
}

func newMessageSetReader(reader *bufio.Reader, remain int) *messageSetReader {
	return &messageSetReader{&readerStack{
		reader: reader,
		remain: remain,
	}}
}

func (r *messageSetReader) readMessage(min int64,
	key func(*bufio.Reader, int, int) (int, error),
	val func(*bufio.Reader, int, int) (int, error),
) (offset int64, timestamp int64, err error) {
	for r.readerStack != nil {
		if r.remain == 0 {
			r.readerStack = r.parent
			continue
		}

		var attributes int8
		if offset, attributes, timestamp, r.remain, err = readMessageHeader(r.reader, r.remain); err != nil {
			return
		}

		// if the message is compressed, decompress it and push a new reader
		// onto the stack.
		code := attributes & compressionCodecMask
		if code != 0 {
			var codec CompressionCodec
			if codec, err = resolveCodec(attributes); err != nil {
				return
			}

			// discard next four bytes...will be -1 to indicate null key
			if r.remain, err = discardN(r.reader, r.remain, 4); err != nil {
				return
			}

			// read and decompress the contained message set.
			var decompressed []byte
			if r.remain, err = readBytesWith(r.reader, r.remain, func(r *bufio.Reader, sz, n int) (remain int, err error) {
				var value []byte
				if value, remain, err = readNewBytes(r, sz, n); err != nil {
					return
				}
				decompressed, err = codec.Decode(value)
				return
			}); err != nil {
				return
			}

			// the compressed message's offset will be equal to the offset of
			// the last message in the set.  within the compressed set, the
			// offsets will be relative, so we have to scan through them to
			// get the base offset.  for example, if there are four compressed
			// messages at offsets 10-13, then the container message will have
			// offset 13 and the contained messages will be 0,1,2,3.  the base
			// offset for the container, then is 13-3=10.
			if offset, err = extractOffset(offset, decompressed); err != nil {
				return
			}

			r.readerStack = &readerStack{
				reader: bufio.NewReader(bytes.NewReader(decompressed)),
				remain: len(decompressed),
				base:   offset,
				parent: r.readerStack,
			}
			continue
		}

		// adjust the offset in case we're reading compressed messages.  the
		// base will be zero otherwise.
		offset += r.base

		// When the messages are compressed kafka may return messages at an
		// earlier offset than the one that was requested, it's the client's
		// responsibility to ignore those.
		if offset < min {
			if r.remain, err = discardBytes(r.reader, r.remain); err != nil {
				return
			}
			if r.remain, err = discardBytes(r.reader, r.remain); err != nil {
				return
			}
			continue
		}

		if r.remain, err = readBytesWith(r.reader, r.remain, key); err != nil {
			return
		}
		r.remain, err = readBytesWith(r.reader, r.remain, val)
		return
	}

	err = errShortRead
	return
}

func (r *messageSetReader) remaining() (remain int) {
	for s := r.readerStack; s != nil; s = s.parent {
		remain += s.remain
	}
	return
}

func (r *messageSetReader) discard() (err error) {
	if r.readerStack == nil {
		return
	}
	// rewind up to the top-most reader b/c it's the only one that's doing
	// actual i/o.  the rest are byte buffers that have been pushed on the stack
	// while reading compressed message sets.
	for r.parent != nil {
		r.readerStack = r.parent
	}
	r.remain, err = discardN(r.reader, r.remain, r.remain)
	return
}

func extractOffset(base int64, msgSet []byte) (offset int64, err error) {
	r, remain := bufio.NewReader(bytes.NewReader(msgSet)), len(msgSet)
	for remain > 0 {
		if remain, err = readInt64(r, remain, &offset); err != nil {
			return
		}
		var sz int32
		if remain, err = readInt32(r, remain, &sz); err != nil {
			return
		}
		if remain, err = discardN(r, remain, int(sz)); err != nil {
			return
		}
	}
	offset = base - offset
	return
}

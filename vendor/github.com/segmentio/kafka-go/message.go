package kafka

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
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
	Headers   []Header

	// If not set at the creation, Time will be automatically set when
	// writing the message.
	Time time.Time
}

func (msg Message) message(cw *crc32Writer) message {
	m := message{
		MagicByte: 1,
		Key:       msg.Key,
		Value:     msg.Value,
		Timestamp: timestamp(msg.Time),
	}
	if cw != nil {
		m.CRC = m.crc32(cw)
	}
	return m
}

const timestampSize = 8

func (msg Message) size() int32 {
	return 4 + 1 + 1 + sizeofBytes(msg.Key) + sizeofBytes(msg.Value) + timestampSize
}

type message struct {
	CRC        int32
	MagicByte  int8
	Attributes int8
	Timestamp  int64
	Key        []byte
	Value      []byte
}

func (m message) crc32(cw *crc32Writer) int32 {
	cw.crc32 = 0
	cw.writeInt8(m.MagicByte)
	cw.writeInt8(m.Attributes)
	if m.MagicByte != 0 {
		cw.writeInt64(m.Timestamp)
	}
	cw.writeBytes(m.Key)
	cw.writeBytes(m.Value)
	return int32(cw.crc32)
}

func (m message) size() int32 {
	size := 4 + 1 + 1 + sizeofBytes(m.Key) + sizeofBytes(m.Value)
	if m.MagicByte != 0 {
		size += timestampSize
	}
	return size
}

func (m message) writeTo(wb *writeBuffer) {
	wb.writeInt32(m.CRC)
	wb.writeInt8(m.MagicByte)
	wb.writeInt8(m.Attributes)
	if m.MagicByte != 0 {
		wb.writeInt64(m.Timestamp)
	}
	wb.writeBytes(m.Key)
	wb.writeBytes(m.Value)
}

type messageSetItem struct {
	Offset      int64
	MessageSize int32
	Message     message
}

func (m messageSetItem) size() int32 {
	return 8 + 4 + m.Message.size()
}

func (m messageSetItem) writeTo(wb *writeBuffer) {
	wb.writeInt64(m.Offset)
	wb.writeInt32(m.MessageSize)
	m.Message.writeTo(wb)
}

type messageSet []messageSetItem

func (s messageSet) size() (size int32) {
	for _, m := range s {
		size += m.size()
	}
	return
}

func (s messageSet) writeTo(wb *writeBuffer) {
	for _, m := range s {
		m.writeTo(wb)
	}
}

type messageSetReader struct {
	empty   bool
	version int
	v1      messageSetReaderV1
	v2      messageSetReaderV2
}

func (r *messageSetReader) readMessage(min int64,
	key func(*bufio.Reader, int, int) (int, error),
	val func(*bufio.Reader, int, int) (int, error),
) (offset int64, timestamp int64, headers []Header, err error) {
	if r.empty {
		return 0, 0, nil, RequestTimedOut
	}
	switch r.version {
	case 1:
		return r.v1.readMessage(min, key, val)
	case 2:
		return r.v2.readMessage(min, key, val)
	default:
		panic("Invalid messageSetReader - unknown message reader version")
	}
}

func (r *messageSetReader) remaining() (remain int) {
	if r.empty {
		return 0
	}
	switch r.version {
	case 1:
		return r.v1.remaining()
	case 2:
		return r.v2.remaining()
	default:
		panic("Invalid messageSetReader - unknown message reader version")
	}
}

func (r *messageSetReader) discard() (err error) {
	if r.empty {
		return nil
	}
	switch r.version {
	case 1:
		return r.v1.discard()
	case 2:
		return r.v2.discard()
	default:
		panic("Invalid messageSetReader - unknown message reader version")
	}
}

type messageSetReaderV1 struct {
	*readerStack
}

type readerStack struct {
	reader *bufio.Reader
	remain int
	base   int64
	parent *readerStack
}

func newMessageSetReader(reader *bufio.Reader, remain int) (*messageSetReader, error) {
	headerLength := 8 + 4 + 4 + 1 // offset + messageSize + crc + magicByte

	if headerLength > remain {
		return nil, errShortRead
	}

	b, err := reader.Peek(headerLength)
	if err != nil {
		return nil, err
	}
	var version int8 = int8(b[headerLength-1])

	switch version {
	case 0, 1:
		return &messageSetReader{
			version: 1,
			v1: messageSetReaderV1{&readerStack{
				reader: reader,
				remain: remain,
			}}}, nil
	case 2:
		mr := &messageSetReader{
			version: 2,
			v2: messageSetReaderV2{
				readerStack: &readerStack{
					reader: reader,
					remain: remain,
				},
				messageCount: 0,
			}}
		return mr, nil
	default:
		return nil, fmt.Errorf("unsupported message version %d found in fetch response", version)
	}
}

func (r *messageSetReaderV1) readMessage(min int64,
	key func(*bufio.Reader, int, int) (int, error),
	val func(*bufio.Reader, int, int) (int, error),
) (offset int64, timestamp int64, headers []Header, err error) {
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
			if codec, err = resolveCodec(code); err != nil {
				return
			}

			// discard next four bytes...will be -1 to indicate null key
			if r.remain, err = discardN(r.reader, r.remain, 4); err != nil {
				return
			}

			// read and decompress the contained message set.
			var decompressed bytes.Buffer

			if r.remain, err = readBytesWith(r.reader, r.remain, func(r *bufio.Reader, sz, n int) (remain int, err error) {
				// x4 as a guess that the average compression ratio is near 75%
				decompressed.Grow(4 * n)

				l := io.LimitedReader{R: r, N: int64(n)}
				d := codec.NewReader(&l)

				_, err = decompressed.ReadFrom(d)
				remain = sz - (n - int(l.N))

				d.Close()
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
			if offset, err = extractOffset(offset, decompressed.Bytes()); err != nil {
				return
			}

			r.readerStack = &readerStack{
				// Allocate a buffer of size 0, which gets capped at 16 bytes
				// by the bufio package. We are already reading buffered data
				// here, no need to reserve another 4KB buffer.
				reader: bufio.NewReaderSize(&decompressed, 0),
				remain: decompressed.Len(),
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

func (r *messageSetReaderV1) remaining() (remain int) {
	for s := r.readerStack; s != nil; s = s.parent {
		remain += s.remain
	}
	return
}

func (r *messageSetReaderV1) discard() (err error) {
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

type Header struct {
	Key   string
	Value []byte
}

type messageSetHeaderV2 struct {
	firstOffset          int64
	length               int32
	partitionLeaderEpoch int32
	magic                int8
	crc                  int32
	batchAttributes      int16
	lastOffsetDelta      int32
	firstTimestamp       int64
	maxTimestamp         int64
	producerId           int64
	producerEpoch        int16
	firstSequence        int32
}

type timestampType int8

const (
	createTime    timestampType = 0
	logAppendTime timestampType = 1
)

type transactionType int8

const (
	nonTransactional transactionType = 0
	transactional    transactionType = 1
)

type controlType int8

const (
	nonControlMessage controlType = 0
	controlMessage    controlType = 1
)

func (h *messageSetHeaderV2) compression() int8 {
	return int8(h.batchAttributes & 7)
}

func (h *messageSetHeaderV2) timestampType() timestampType {
	return timestampType((h.batchAttributes & (1 << 3)) >> 3)
}

func (h *messageSetHeaderV2) transactionType() transactionType {
	return transactionType((h.batchAttributes & (1 << 4)) >> 4)
}

func (h *messageSetHeaderV2) controlType() controlType {
	return controlType((h.batchAttributes & (1 << 5)) >> 5)
}

type messageSetReaderV2 struct {
	*readerStack
	messageCount int

	header messageSetHeaderV2
}

func (r *messageSetReaderV2) readHeader() (err error) {
	h := &r.header
	if r.remain, err = readInt64(r.reader, r.remain, &h.firstOffset); err != nil {
		return
	}
	if r.remain, err = readInt32(r.reader, r.remain, &h.length); err != nil {
		return
	}
	if r.remain, err = readInt32(r.reader, r.remain, &h.partitionLeaderEpoch); err != nil {
		return
	}
	if r.remain, err = readInt8(r.reader, r.remain, &h.magic); err != nil {
		return
	}
	if r.remain, err = readInt32(r.reader, r.remain, &h.crc); err != nil {
		return
	}
	if r.remain, err = readInt16(r.reader, r.remain, &h.batchAttributes); err != nil {
		return
	}
	if r.remain, err = readInt32(r.reader, r.remain, &h.lastOffsetDelta); err != nil {
		return
	}
	if r.remain, err = readInt64(r.reader, r.remain, &h.firstTimestamp); err != nil {
		return
	}
	if r.remain, err = readInt64(r.reader, r.remain, &h.maxTimestamp); err != nil {
		return
	}
	if r.remain, err = readInt64(r.reader, r.remain, &h.producerId); err != nil {
		return
	}
	if r.remain, err = readInt16(r.reader, r.remain, &h.producerEpoch); err != nil {
		return
	}
	if r.remain, err = readInt32(r.reader, r.remain, &h.firstSequence); err != nil {
		return
	}
	var messageCount int32
	if r.remain, err = readInt32(r.reader, r.remain, &messageCount); err != nil {
		return
	}
	r.messageCount = int(messageCount)

	return nil
}

func (r *messageSetReaderV2) readMessage(min int64,
	key func(*bufio.Reader, int, int) (int, error),
	val func(*bufio.Reader, int, int) (int, error),
) (offset int64, timestamp int64, headers []Header, err error) {

	if r.messageCount == 0 {
		if r.remain == 0 {
			if r.parent != nil {
				r.readerStack = r.parent
			}
		}

		if err = r.readHeader(); err != nil {
			return
		}

		if code := r.header.compression(); code != 0 {
			var codec CompressionCodec
			if codec, err = resolveCodec(code); err != nil {
				return
			}

			var batchRemain = int(r.header.length - 49)
			if batchRemain > r.remain {
				err = errShortRead
				return
			}

			var decompressed bytes.Buffer
			decompressed.Grow(4 * batchRemain)

			l := io.LimitedReader{R: r.reader, N: int64(batchRemain)}
			d := codec.NewReader(&l)

			_, err = decompressed.ReadFrom(d)
			r.remain = r.remain - (batchRemain - int(l.N))
			d.Close()

			if err != nil {
				return
			}

			r.readerStack = &readerStack{
				reader: bufio.NewReaderSize(&decompressed, 0),
				remain: decompressed.Len(),
				base:   -1, // base is unused here
				parent: r.readerStack,
			}
		}
	}

	var length int64
	if r.remain, err = readVarInt(r.reader, r.remain, &length); err != nil {
		return
	}

	var attrs int8
	if r.remain, err = readInt8(r.reader, r.remain, &attrs); err != nil {
		return
	}
	var timestampDelta int64
	if r.remain, err = readVarInt(r.reader, r.remain, &timestampDelta); err != nil {
		return
	}
	var offsetDelta int64
	if r.remain, err = readVarInt(r.reader, r.remain, &offsetDelta); err != nil {
		return
	}
	var keyLen int64
	if r.remain, err = readVarInt(r.reader, r.remain, &keyLen); err != nil {
		return
	}

	if r.remain, err = key(r.reader, r.remain, int(keyLen)); err != nil {
		return
	}
	var valueLen int64
	if r.remain, err = readVarInt(r.reader, r.remain, &valueLen); err != nil {
		return
	}

	if r.remain, err = val(r.reader, r.remain, int(valueLen)); err != nil {
		return
	}

	var headerCount int64
	if r.remain, err = readVarInt(r.reader, r.remain, &headerCount); err != nil {
		return
	}

	headers = make([]Header, headerCount)

	for i := 0; i < int(headerCount); i++ {
		if err = r.readMessageHeader(&headers[i]); err != nil {
			return
		}
	}
	r.messageCount--
	return r.header.firstOffset + offsetDelta, r.header.firstTimestamp + timestampDelta, headers, nil
}

func (r *messageSetReaderV2) readMessageHeader(header *Header) (err error) {
	var keyLen int64
	if r.remain, err = readVarInt(r.reader, r.remain, &keyLen); err != nil {
		return
	}
	if header.Key, r.remain, err = readNewString(r.reader, r.remain, int(keyLen)); err != nil {
		return
	}
	var valLen int64
	if r.remain, err = readVarInt(r.reader, r.remain, &valLen); err != nil {
		return
	}
	if header.Value, r.remain, err = readNewBytes(r.reader, r.remain, int(valLen)); err != nil {
		return
	}
	return nil
}

func (r *messageSetReaderV2) remaining() (remain int) {
	return r.remain
}

func (r *messageSetReaderV2) discard() (err error) {
	r.remain, err = discardN(r.reader, r.remain, r.remain)
	return
}

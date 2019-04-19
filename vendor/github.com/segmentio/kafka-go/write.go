package kafka

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"time"
)

type writable interface {
	writeTo(*bufio.Writer)
}

func writeInt8(w *bufio.Writer, i int8) {
	w.WriteByte(byte(i))
}

func writeInt16(w *bufio.Writer, i int16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(i))
	w.WriteByte(b[0])
	w.WriteByte(b[1])
}

func writeInt32(w *bufio.Writer, i int32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(i))
	w.WriteByte(b[0])
	w.WriteByte(b[1])
	w.WriteByte(b[2])
	w.WriteByte(b[3])
}

func writeInt64(w *bufio.Writer, i int64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(i))
	w.WriteByte(b[0])
	w.WriteByte(b[1])
	w.WriteByte(b[2])
	w.WriteByte(b[3])
	w.WriteByte(b[4])
	w.WriteByte(b[5])
	w.WriteByte(b[6])
	w.WriteByte(b[7])
}

func writeVarInt(w *bufio.Writer, i int64) {
	i = i<<1 ^ i>>63
	for i&0x7f != i {
		w.WriteByte(byte(i&0x7f | 0x80))
		i >>= 7
	}
	w.WriteByte(byte(i))
}

func varIntLen(i int64) (l int) {
	i = i<<1 ^ i>>63
	for i&0x7f != i {
		l++
		i >>= 7
	}
	l++
	return l
}

func writeString(w *bufio.Writer, s string) {
	writeInt16(w, int16(len(s)))
	w.WriteString(s)
}

func writeNullableString(w *bufio.Writer, s *string) {
	if s == nil {
		writeInt16(w, -1)
	} else {
		writeString(w, *s)
	}
}

func writeBytes(w *bufio.Writer, b []byte) {
	n := len(b)
	if b == nil {
		n = -1
	}
	writeInt32(w, int32(n))
	w.Write(b)
}

func writeBool(w *bufio.Writer, b bool) {
	v := int8(0)
	if b {
		v = 1
	}
	writeInt8(w, v)
}

func writeArrayLen(w *bufio.Writer, n int) {
	writeInt32(w, int32(n))
}

func writeArray(w *bufio.Writer, n int, f func(int)) {
	writeArrayLen(w, n)
	for i := 0; i != n; i++ {
		f(i)
	}
}

func writeStringArray(w *bufio.Writer, a []string) {
	writeArray(w, len(a), func(i int) { writeString(w, a[i]) })
}

func writeInt32Array(w *bufio.Writer, a []int32) {
	writeArray(w, len(a), func(i int) { writeInt32(w, a[i]) })
}

func write(w *bufio.Writer, a interface{}) {
	switch v := a.(type) {
	case int8:
		writeInt8(w, v)
	case int16:
		writeInt16(w, v)
	case int32:
		writeInt32(w, v)
	case int64:
		writeInt64(w, v)
	case string:
		writeString(w, v)
	case []byte:
		writeBytes(w, v)
	case bool:
		writeBool(w, v)
	case writable:
		v.writeTo(w)
	default:
		panic(fmt.Sprintf("unsupported type: %T", a))
	}
}

// The functions bellow are used as optimizations to avoid dynamic memory
// allocations that occur when building the data structures representing the
// kafka protocol requests.

func writeFetchRequestV2(w *bufio.Writer, correlationID int32, clientID, topic string, partition int32, offset int64, minBytes, maxBytes int, maxWait time.Duration) error {
	h := requestHeader{
		ApiKey:        int16(fetchRequest),
		ApiVersion:    int16(v2),
		CorrelationID: correlationID,
		ClientID:      clientID,
	}
	h.Size = (h.size() - 4) +
		4 + // replica ID
		4 + // max wait time
		4 + // min bytes
		4 + // topic array length
		sizeofString(topic) +
		4 + // partition array length
		4 + // partition
		8 + // offset
		4 // max bytes

	h.writeTo(w)
	writeInt32(w, -1) // replica ID
	writeInt32(w, milliseconds(maxWait))
	writeInt32(w, int32(minBytes))

	// topic array
	writeArrayLen(w, 1)
	writeString(w, topic)

	// partition array
	writeArrayLen(w, 1)
	writeInt32(w, partition)
	writeInt64(w, offset)
	writeInt32(w, int32(maxBytes))

	return w.Flush()
}

func writeFetchRequestV5(w *bufio.Writer, correlationID int32, clientID, topic string, partition int32, offset int64, minBytes, maxBytes int, maxWait time.Duration, isolationLevel int8) error {
	h := requestHeader{
		ApiKey:        int16(fetchRequest),
		ApiVersion:    int16(v5),
		CorrelationID: correlationID,
		ClientID:      clientID,
	}
	h.Size = (h.size() - 4) +
		4 + // replica ID
		4 + // max wait time
		4 + // min bytes
		4 + // max bytes
		1 + // isolation level
		4 + // topic array length
		sizeofString(topic) +
		4 + // partition array length
		4 + // partition
		8 + // offset
		8 + // log start offset
		4 // max bytes

	h.writeTo(w)
	writeInt32(w, -1) // replica ID
	writeInt32(w, milliseconds(maxWait))
	writeInt32(w, int32(minBytes))
	writeInt32(w, int32(maxBytes))
	writeInt8(w, isolationLevel) // isolation level 0 - read uncommitted

	// topic array
	writeArrayLen(w, 1)
	writeString(w, topic)

	// partition array
	writeArrayLen(w, 1)
	writeInt32(w, partition)
	writeInt64(w, offset)
	writeInt64(w, int64(0)) // log start offset only used when is sent by follower
	writeInt32(w, int32(maxBytes))

	return w.Flush()
}

func writeFetchRequestV10(w *bufio.Writer, correlationID int32, clientID, topic string, partition int32, offset int64, minBytes, maxBytes int, maxWait time.Duration, isolationLevel int8) error {
	h := requestHeader{
		ApiKey:        int16(fetchRequest),
		ApiVersion:    int16(v10),
		CorrelationID: correlationID,
		ClientID:      clientID,
	}
	h.Size = (h.size() - 4) +
		4 + // replica ID
		4 + // max wait time
		4 + // min bytes
		4 + // max bytes
		1 + // isolation level
		4 + // session ID
		4 + // session epoch
		4 + // topic array length
		sizeofString(topic) +
		4 + // partition array length
		4 + // partition
		4 + // current leader epoch
		8 + // fetch offset
		8 + // log start offset
		4 + // partition max bytes
		4 // forgotten topics data

	h.writeTo(w)
	writeInt32(w, -1) // replica ID
	writeInt32(w, milliseconds(maxWait))
	writeInt32(w, int32(minBytes))
	writeInt32(w, int32(maxBytes))
	writeInt8(w, isolationLevel) // isolation level 0 - read uncommitted
	writeInt32(w, 0)             //FIXME
	writeInt32(w, -1)            //FIXME

	// topic array
	writeArrayLen(w, 1)
	writeString(w, topic)

	// partition array
	writeArrayLen(w, 1)
	writeInt32(w, partition)
	writeInt32(w, -1) //FIXME
	writeInt64(w, offset)
	writeInt64(w, int64(0)) // log start offset only used when is sent by follower
	writeInt32(w, int32(maxBytes))

	// forgotten topics array
	writeArrayLen(w, 0) // forgotten topics not supported yet

	return w.Flush()
}

func writeListOffsetRequestV1(w *bufio.Writer, correlationID int32, clientID, topic string, partition int32, time int64) error {
	h := requestHeader{
		ApiKey:        int16(listOffsetRequest),
		ApiVersion:    int16(v1),
		CorrelationID: correlationID,
		ClientID:      clientID,
	}
	h.Size = (h.size() - 4) +
		4 + // replica ID
		4 + // topic array length
		sizeofString(topic) + // topic
		4 + // partition array length
		4 + // partition
		8 // time

	h.writeTo(w)
	writeInt32(w, -1) // replica ID

	// topic array
	writeArrayLen(w, 1)
	writeString(w, topic)

	// partition array
	writeArrayLen(w, 1)
	writeInt32(w, partition)
	writeInt64(w, time)

	return w.Flush()
}

func writeProduceRequestV2(w *bufio.Writer, codec CompressionCodec, correlationID int32, clientID, topic string, partition int32, timeout time.Duration, requiredAcks int16, msgs ...Message) (err error) {

	attributes := int8(CompressionNoneCode)
	if codec != nil {
		if msgs, err = compress(codec, msgs...); err != nil {
			return err
		}
		attributes = codec.Code()
	}
	size := messageSetSize(msgs...)

	h := requestHeader{
		ApiKey:        int16(produceRequest),
		ApiVersion:    int16(v2),
		CorrelationID: correlationID,
		ClientID:      clientID,
	}
	h.Size = (h.size() - 4) +
		2 + // required acks
		4 + // timeout
		4 + // topic array length
		sizeofString(topic) + // topic
		4 + // partition array length
		4 + // partition
		4 + // message set size
		size

	h.writeTo(w)
	writeInt16(w, requiredAcks) // required acks
	writeInt32(w, milliseconds(timeout))

	// topic array
	writeArrayLen(w, 1)
	writeString(w, topic)

	// partition array
	writeArrayLen(w, 1)
	writeInt32(w, partition)

	writeInt32(w, size)
	for _, msg := range msgs {
		writeMessage(w, msg.Offset, attributes, msg.Time, msg.Key, msg.Value)
	}
	return w.Flush()
}

func writeProduceRequestV3(w *bufio.Writer, codec CompressionCodec, correlationID int32, clientID, topic string, partition int32, timeout time.Duration, requiredAcks int16, transactionalID *string, msgs ...Message) (err error) {

	var size int32
	var compressed []byte
	var attributes int16
	if codec != nil {
		attributes = int16(codec.Code())
		recordBuf := &bytes.Buffer{}
		recordBuf.Grow(int(recordBatchSize(msgs...)))
		compressedWriter := bufio.NewWriter(recordBuf)
		for i, msg := range msgs {
			writeRecord(compressedWriter, 0, msgs[0].Time, int64(i), msg)
		}
		compressedWriter.Flush()

		compressed, err = codec.Encode(recordBuf.Bytes())
		if err != nil {
			return
		}
		attributes = int16(codec.Code())
		size = recordBatchHeaderSize() + int32(len(compressed))
	} else {
		size = recordBatchSize(msgs...)
	}

	h := requestHeader{
		ApiKey:        int16(produceRequest),
		ApiVersion:    int16(v3),
		CorrelationID: correlationID,
		ClientID:      clientID,
	}
	h.Size = (h.size() - 4) +
		sizeofNullableString(transactionalID) +
		2 + // required acks
		4 + // timeout
		4 + // topic array length
		sizeofString(topic) + // topic
		4 + // partition array length
		4 + // partition
		4 + // message set size
		size

	h.writeTo(w)
	writeNullableString(w, transactionalID)
	writeInt16(w, requiredAcks) // required acks
	writeInt32(w, milliseconds(timeout))

	// topic array
	writeArrayLen(w, 1)
	writeString(w, topic)

	// partition array
	writeArrayLen(w, 1)
	writeInt32(w, partition)

	writeInt32(w, size)
	if codec != nil {
		err = writeRecordBatch(w, attributes, size, func(w *bufio.Writer) {
			w.Write(compressed)
		}, msgs...)
	} else {
		err = writeRecordBatch(w, attributes, size, func(w *bufio.Writer) {
			for i, msg := range msgs {
				writeRecord(w, 0, msgs[0].Time, int64(i), msg)
			}
		}, msgs...)
	}
	if err != nil {
		return
	}

	return w.Flush()
}

func writeProduceRequestV7(w *bufio.Writer, codec CompressionCodec, correlationID int32, clientID, topic string, partition int32, timeout time.Duration, requiredAcks int16, transactionalID *string, msgs ...Message) (err error) {

	var size int32
	var compressed []byte
	var attributes int16
	if codec != nil {
		attributes = int16(codec.Code())
		recordBuf := &bytes.Buffer{}
		recordBuf.Grow(int(recordBatchSize(msgs...)))
		compressedWriter := bufio.NewWriter(recordBuf)
		for i, msg := range msgs {
			writeRecord(compressedWriter, 0, msgs[0].Time, int64(i), msg)
		}
		compressedWriter.Flush()

		compressed, err = codec.Encode(recordBuf.Bytes())
		if err != nil {
			return
		}
		attributes = int16(codec.Code())
		size = recordBatchHeaderSize() + int32(len(compressed))
	} else {
		size = recordBatchSize(msgs...)
	}

	h := requestHeader{
		ApiKey:        int16(produceRequest),
		ApiVersion:    int16(v7),
		CorrelationID: correlationID,
		ClientID:      clientID,
	}
	h.Size = (h.size() - 4) +
		sizeofNullableString(transactionalID) +
		2 + // required acks
		4 + // timeout
		4 + // topic array length
		sizeofString(topic) + // topic
		4 + // partition array length
		4 + // partition
		4 + // message set size
		size

	h.writeTo(w)
	writeNullableString(w, transactionalID)
	writeInt16(w, requiredAcks) // required acks
	writeInt32(w, milliseconds(timeout))

	// topic array
	writeArrayLen(w, 1)
	writeString(w, topic)

	// partition array
	writeArrayLen(w, 1)
	writeInt32(w, partition)

	writeInt32(w, size)
	if codec != nil {
		err = writeRecordBatch(w, attributes, size, func(w *bufio.Writer) {
			w.Write(compressed)
		}, msgs...)
	} else {
		err = writeRecordBatch(w, attributes, size, func(w *bufio.Writer) {
			for i, msg := range msgs {
				writeRecord(w, 0, msgs[0].Time, int64(i), msg)
			}
		}, msgs...)
	}
	if err != nil {
		return
	}

	return w.Flush()
}

func messageSetSize(msgs ...Message) (size int32) {
	for _, msg := range msgs {
		size += 8 + // offset
			4 + // message size
			4 + // crc
			1 + // magic byte
			1 + // attributes
			8 + // timestamp
			sizeofBytes(msg.Key) +
			sizeofBytes(msg.Value)
	}
	return
}

func recordBatchHeaderSize() int32 {
	return 8 + // base offset
		4 + // batch length
		4 + // partition leader epoch
		1 + // magic
		4 + // crc
		2 + // attributes
		4 + // last offset delta
		8 + // first timestamp
		8 + // max timestamp
		8 + // producer id
		2 + // producer epoch
		4 + // base sequence
		4 // msg count
}

func recordBatchSize(msgs ...Message) (size int32) {
	size = recordBatchHeaderSize()

	baseTime := msgs[0].Time

	for i, msg := range msgs {

		sz := recordSize(&msg, msg.Time.Sub(baseTime), int64(i))

		size += int32(sz + varIntLen(int64(sz)))
	}
	return
}

func writeRecordBatch(w *bufio.Writer, attributes int16, size int32, write func(*bufio.Writer), msgs ...Message) error {

	baseTime := msgs[0].Time

	writeInt64(w, int64(0))

	writeInt32(w, int32(size-12)) // 12 = batch length + base offset sizes

	writeInt32(w, -1) // partition leader epoch
	writeInt8(w, 2)   // magic byte

	crcBuf := &bytes.Buffer{}
	crcBuf.Grow(int(size - 12)) // 12 = batch length + base offset sizes
	crcWriter := bufio.NewWriter(crcBuf)

	writeInt16(crcWriter, attributes)         // attributes, timestamp type 0 - create time, not part of a transaction, no control messages
	writeInt32(crcWriter, int32(len(msgs)-1)) // max offset
	writeInt64(crcWriter, timestamp(baseTime))
	lastTime := timestamp(msgs[len(msgs)-1].Time)
	writeInt64(crcWriter, int64(lastTime))
	writeInt64(crcWriter, -1)               // default producer id for now
	writeInt16(crcWriter, -1)               // default producer epoch for now
	writeInt32(crcWriter, -1)               // default base sequence
	writeInt32(crcWriter, int32(len(msgs))) // record count

	write(crcWriter)
	if err := crcWriter.Flush(); err != nil {
		return err
	}

	crcTable := crc32.MakeTable(crc32.Castagnoli)
	crcChecksum := crc32.Checksum(crcBuf.Bytes(), crcTable)

	writeInt32(w, int32(crcChecksum))
	if _, err := w.Write(crcBuf.Bytes()); err != nil {
		return err
	}

	return nil
}

var maxDate = time.Date(5000, time.January, 0, 0, 0, 0, 0, time.UTC)

func recordSize(msg *Message, timestampDelta time.Duration, offsetDelta int64) (size int) {
	size += 1 + // attributes
		varIntLen(int64(timestampDelta)) +
		varIntLen(offsetDelta) +
		varIntLen(int64(len(msg.Key))) +
		len(msg.Key) +
		varIntLen(int64(len(msg.Value))) +
		len(msg.Value) +
		varIntLen(int64(len(msg.Headers)))
	for _, h := range msg.Headers {
		size += varIntLen(int64(len([]byte(h.Key)))) +
			len([]byte(h.Key)) +
			varIntLen(int64(len(h.Value))) +
			len(h.Value)
	}
	return
}

func compress(codec CompressionCodec, msgs ...Message) ([]Message, error) {
	estimatedLen := 0
	for _, msg := range msgs {
		estimatedLen += int(msgSize(msg.Key, msg.Value))
	}
	buf := &bytes.Buffer{}
	buf.Grow(estimatedLen)
	bufWriter := bufio.NewWriter(buf)
	for offset, msg := range msgs {
		writeMessage(bufWriter, int64(offset), CompressionNoneCode, msg.Time, msg.Key, msg.Value)
	}
	bufWriter.Flush()

	compressed, err := codec.Encode(buf.Bytes())
	if err != nil {
		return nil, err
	}

	return []Message{{Value: compressed}}, nil
}

const magicByte = 1 // compatible with kafka 0.10.0.0+

func writeMessage(w *bufio.Writer, offset int64, attributes int8, time time.Time, key, value []byte) {
	timestamp := timestamp(time)
	crc32 := crc32OfMessage(magicByte, attributes, timestamp, key, value)
	size := msgSize(key, value)

	writeInt64(w, offset)
	writeInt32(w, size)
	writeInt32(w, int32(crc32))
	writeInt8(w, magicByte)
	writeInt8(w, attributes)
	writeInt64(w, timestamp)
	writeBytes(w, key)
	writeBytes(w, value)
}

func msgSize(key, value []byte) int32 {
	return 4 + // crc
		1 + // magic byte
		1 + // attributes
		8 + // timestamp
		sizeofBytes(key) +
		sizeofBytes(value)
}

// Messages with magic >2 are called records. This method writes messages using message format 2.
func writeRecord(w *bufio.Writer, attributes int8, baseTime time.Time, offset int64, msg Message) {

	timestampDelta := msg.Time.Sub(baseTime)
	offsetDelta := int64(offset)

	writeVarInt(w, int64(recordSize(&msg, timestampDelta, offsetDelta)))

	writeInt8(w, attributes)
	writeVarInt(w, int64(timestampDelta))
	writeVarInt(w, offsetDelta)

	writeVarInt(w, int64(len(msg.Key)))
	w.Write(msg.Key)
	writeVarInt(w, int64(len(msg.Value)))
	w.Write(msg.Value)
	writeVarInt(w, int64(len(msg.Headers)))

	for _, h := range msg.Headers {
		writeVarInt(w, int64(len(h.Key)))
		w.Write([]byte(h.Key))
		writeVarInt(w, int64(len(h.Value)))
		w.Write(h.Value)
	}
}

package kafka

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
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

func writeString(w *bufio.Writer, s string) {
	writeInt16(w, int16(len(s)))
	w.WriteString(s)
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

func writeProduceRequestV2(w *bufio.Writer, codec CompressionCodec, correlationID int32, clientID, topic string, partition int32, timeout time.Duration, requiredAcks int16, msgs ...Message) error {
	var size int32
	attributes := int8(CompressionNoneCode)

	// if compressing, replace the slice of messages with a single compressed
	// message set.
	if codec != nil {
		var err error
		if msgs, err = compress(codec, msgs...); err != nil {
			return err
		}
		attributes = codec.Code()
	}

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

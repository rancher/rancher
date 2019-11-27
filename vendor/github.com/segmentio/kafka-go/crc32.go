package kafka

import (
	"encoding/binary"
	"hash/crc32"
)

type crc32Writer struct {
	table  *crc32.Table
	buffer [8]byte
	crc32  uint32
}

func (w *crc32Writer) update(b []byte) {
	w.crc32 = crc32.Update(w.crc32, w.table, b)
}

func (w *crc32Writer) writeInt8(i int8) {
	w.buffer[0] = byte(i)
	w.update(w.buffer[:1])
}

func (w *crc32Writer) writeInt16(i int16) {
	binary.BigEndian.PutUint16(w.buffer[:2], uint16(i))
	w.update(w.buffer[:2])
}

func (w *crc32Writer) writeInt32(i int32) {
	binary.BigEndian.PutUint32(w.buffer[:4], uint32(i))
	w.update(w.buffer[:4])
}

func (w *crc32Writer) writeInt64(i int64) {
	binary.BigEndian.PutUint64(w.buffer[:8], uint64(i))
	w.update(w.buffer[:8])
}

func (w *crc32Writer) writeBytes(b []byte) {
	n := len(b)
	if b == nil {
		n = -1
	}
	w.writeInt32(int32(n))
	w.update(b)
}

func (w *crc32Writer) Write(b []byte) (int, error) {
	w.update(b)
	return len(b), nil
}

func (w *crc32Writer) WriteString(s string) (int, error) {
	w.update([]byte(s))
	return len(s), nil
}

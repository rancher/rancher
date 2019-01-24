package kafka

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"sync"
)

func crc32OfMessage(magicByte int8, attributes int8, timestamp int64, key []byte, value []byte) uint32 {
	b := acquireCrc32Buffer()
	b.writeInt8(magicByte)
	b.writeInt8(attributes)
	if magicByte != 0 {
		b.writeInt64(timestamp)
	}
	b.writeBytes(key)
	b.writeBytes(value)
	sum := b.sum
	releaseCrc32Buffer(b)
	return sum
}

type crc32Buffer struct {
	sum uint32
	buf bytes.Buffer
}

func (c *crc32Buffer) writeInt8(i int8) {
	c.buf.Truncate(0)
	c.buf.WriteByte(byte(i))
	c.update()
}

func (c *crc32Buffer) writeInt32(i int32) {
	a := [4]byte{}
	binary.BigEndian.PutUint32(a[:], uint32(i))
	c.buf.Truncate(0)
	c.buf.Write(a[:])
	c.update()
}

func (c *crc32Buffer) writeInt64(i int64) {
	a := [8]byte{}
	binary.BigEndian.PutUint64(a[:], uint64(i))
	c.buf.Truncate(0)
	c.buf.Write(a[:])
	c.update()
}

func (c *crc32Buffer) writeBytes(b []byte) {
	if b == nil {
		c.writeInt32(-1)
	} else {
		c.writeInt32(int32(len(b)))
	}
	c.sum = crc32Update(c.sum, b)
}

func (c *crc32Buffer) update() {
	c.sum = crc32Update(c.sum, c.buf.Bytes())
}

func crc32Update(sum uint32, b []byte) uint32 {
	return crc32.Update(sum, crc32.IEEETable, b)
}

var crc32BufferPool = sync.Pool{
	New: func() interface{} { return &crc32Buffer{} },
}

func acquireCrc32Buffer() *crc32Buffer {
	c := crc32BufferPool.Get().(*crc32Buffer)
	c.sum = 0
	return c
}

func releaseCrc32Buffer(b *crc32Buffer) {
	crc32BufferPool.Put(b)
}

package kafka

import (
	"bytes"
	"sync"
)

var bufferPool = sync.Pool{
	New: func() interface{} { return newBuffer() },
}

func newBuffer() *bytes.Buffer {
	b := new(bytes.Buffer)
	b.Grow(65536)
	return b
}

func acquireBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

func releaseBuffer(b *bytes.Buffer) {
	if b != nil {
		b.Reset()
		bufferPool.Put(b)
	}
}

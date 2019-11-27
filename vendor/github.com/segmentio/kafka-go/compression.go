package kafka

import (
	"errors"
	"io"
	"sync"
)

const (
	compressionCodecMask = 0x07
)

var (
	errUnknownCodec = errors.New("the compression code is invalid or its codec has not been imported")

	codecs      = make(map[int8]CompressionCodec)
	codecsMutex sync.RWMutex
)

// RegisterCompressionCodec registers a compression codec so it can be used by a Writer.
func RegisterCompressionCodec(codec CompressionCodec) {
	code := codec.Code()
	codecsMutex.Lock()
	codecs[code] = codec
	codecsMutex.Unlock()
}

// resolveCodec looks up a codec by Code()
func resolveCodec(code int8) (codec CompressionCodec, err error) {
	codecsMutex.RLock()
	codec = codecs[code]
	codecsMutex.RUnlock()

	if codec == nil {
		err = errUnknownCodec
	}
	return
}

// CompressionCodec represents a compression codec to encode and decode
// the messages.
// See : https://cwiki.apache.org/confluence/display/KAFKA/Compression
//
// A CompressionCodec must be safe for concurrent access by multiple go
// routines.
type CompressionCodec interface {
	// Code returns the compression codec code
	Code() int8

	// Human-readable name for the codec.
	Name() string

	// Constructs a new reader which decompresses data from r.
	NewReader(r io.Reader) io.ReadCloser

	// Constructs a new writer which writes compressed data to w.
	NewWriter(w io.Writer) io.WriteCloser
}

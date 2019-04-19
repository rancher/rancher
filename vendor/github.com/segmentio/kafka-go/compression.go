package kafka

import (
	"errors"
	"sync"
)

var errUnknownCodec = errors.New("the compression code is invalid or its codec has not been imported")

var codecs = make(map[int8]CompressionCodec)
var codecsMutex sync.RWMutex

// RegisterCompressionCodec registers a compression codec so it can be used by a Writer.
func RegisterCompressionCodec(codec func() CompressionCodec) {
	c := codec()
	codecsMutex.Lock()
	codecs[c.Code()] = c
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

	// Encode encodes the src data
	Encode(src []byte) ([]byte, error)

	// Decode decodes the src data
	Decode(src []byte) ([]byte, error)
}

const compressionCodecMask int8 = 0x07
const CompressionNoneCode = 0

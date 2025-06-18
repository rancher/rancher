package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/rancher/rancher/pkg/scc/util/log"
	"math/rand"
	"time"
)

func utilContextLogger() log.StructuredLogger {
	return log.NewLog().WithField("subcomponent", "util")
}

func JSONToBase64(data interface{}) ([]byte, error) {
	var jsonData []byte
	var err error

	if b, ok := data.([]byte); ok {
		jsonData = b
	} else {
		jsonData, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON data: %w", err)
		}
	}

	encodedLen := base64.StdEncoding.EncodedLen(len(jsonData))
	output := make([]byte, encodedLen)

	// Base64 encode the JSON byte slice
	base64.StdEncoding.Encode(output, jsonData)

	return output, nil
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type SaltGen struct {
	randSrc     *rand.Rand
	saltCharset string
	charsetLen  int
}

func NewSaltGen(timeIn *time.Time, charsetIn *string) *SaltGen {
	if timeIn == nil {
		now := time.Now()
		timeIn = &now
	}
	randSrc := rand.New(rand.NewSource(timeIn.UnixNano()))

	setCharset := charset
	if charsetIn != nil {
		setCharset = *charsetIn
	}

	return &SaltGen{randSrc: randSrc, saltCharset: setCharset, charsetLen: len(setCharset)}
}

func (s *SaltGen) GenerateCharacter() uint8 {
	randIndex := s.randSrc.Intn(s.charsetLen)
	return s.saltCharset[randIndex]
}

func (s *SaltGen) GenerateSalt() string {
	salt := make([]byte, 8)
	for i := range salt {
		salt[i] = s.GenerateCharacter()
	}

	return string(salt)
}

// Define constants for clarity and reusability
const (
	KB  = 1024
	MiB = 1024 * KB // 1 MiB = 1,048,576 bytes
)

func BytesToMiBRounded(bytes int) int {
	// Handle zero or negative bytes gracefully to avoid issues with (bytes + MiB - 1)
	if bytes <= 0 {
		return 0
	}
	return (bytes + MiB - 1) / MiB
}

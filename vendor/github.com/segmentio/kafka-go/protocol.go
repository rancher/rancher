package kafka

import (
	"bufio"
	"encoding/binary"
	"fmt"
)

type apiKey int16

const (
	produceRequest          apiKey = 0
	fetchRequest            apiKey = 1
	listOffsetRequest       apiKey = 2
	metadataRequest         apiKey = 3
	offsetCommitRequest     apiKey = 8
	offsetFetchRequest      apiKey = 9
	groupCoordinatorRequest apiKey = 10
	joinGroupRequest        apiKey = 11
	heartbeatRequest        apiKey = 12
	leaveGroupRequest       apiKey = 13
	syncGroupRequest        apiKey = 14
	describeGroupsRequest   apiKey = 15
	listGroupsRequest       apiKey = 16
	saslHandshakeRequest    apiKey = 17
	apiVersionsRequest      apiKey = 18
	createTopicsRequest     apiKey = 19
	deleteTopicsRequest     apiKey = 20
	saslAuthenticateRequest apiKey = 36
)

type apiVersion int16

const (
	v0  apiVersion = 0
	v1  apiVersion = 1
	v2  apiVersion = 2
	v3  apiVersion = 3
	v5  apiVersion = 5
	v7  apiVersion = 7
	v10 apiVersion = 10
)

type requestHeader struct {
	Size          int32
	ApiKey        int16
	ApiVersion    int16
	CorrelationID int32
	ClientID      string
}

func (h requestHeader) size() int32 {
	return 4 + 2 + 2 + 4 + sizeofString(h.ClientID)
}

func (h requestHeader) writeTo(w *bufio.Writer) {
	writeInt32(w, h.Size)
	writeInt16(w, h.ApiKey)
	writeInt16(w, h.ApiVersion)
	writeInt32(w, h.CorrelationID)
	writeString(w, h.ClientID)
}

type request interface {
	size() int32
	writeTo(*bufio.Writer)
}

func makeInt8(b []byte) int8 {
	return int8(b[0])
}

func makeInt16(b []byte) int16 {
	return int16(binary.BigEndian.Uint16(b))
}

func makeInt32(b []byte) int32 {
	return int32(binary.BigEndian.Uint32(b))
}

func makeInt64(b []byte) int64 {
	return int64(binary.BigEndian.Uint64(b))
}

func expectZeroSize(sz int, err error) error {
	if err == nil && sz != 0 {
		err = fmt.Errorf("reading a response left %d unread bytes", sz)
	}
	return err
}

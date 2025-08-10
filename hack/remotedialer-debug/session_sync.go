package remotedialer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

var errCloseSyncConnections = errors.New("sync from client")

// encodeConnectionIDs serializes a slice of connection IDs
func encodeConnectionIDs(ids []int64) []byte {
	payload := make([]byte, 0, 8*len(ids))
	for _, id := range ids {
		payload = binary.LittleEndian.AppendUint64(payload, uint64(id))
	}
	return payload
}

// decodeConnectionIDs deserializes a slice of connection IDs
func decodeConnectionIDs(payload []byte) ([]int64, error) {
	if len(payload)%8 != 0 {
		return nil, fmt.Errorf("incorrect data format")
	}
	result := make([]int64, 0, len(payload)/8)
	for x := 0; x < len(payload); x += 8 {
		id := binary.LittleEndian.Uint64(payload[x : x+8])
		result = append(result, int64(id))
	}
	return result, nil
}

func newSyncConnectionsMessage(connectionIDs []int64) *message {
	return &message{
		id:          nextid(),
		messageType: SyncConnections,
		bytes:       encodeConnectionIDs(connectionIDs),
	}
}

// sendSyncConnections sends a binary message of type SyncConnections, whose payload is a list of the active connection IDs for this session
func (s *Session) sendSyncConnections() error {
	_, err := s.writeMessage(time.Now().Add(SyncConnectionsTimeout), newSyncConnectionsMessage(s.activeConnectionIDs()))
	return err
}

// compareAndCloseStaleConnections compares the Session's activeConnectionIDs with the provided list from the client, then closing every connection not present in it
func (s *Session) compareAndCloseStaleConnections(clientIDs []int64) {
	serverIDs := s.activeConnectionIDs()
	toClose := diffSortedSetsGetRemoved(serverIDs, clientIDs)
	if len(toClose) == 0 {
		return
	}

	s.Lock()
	defer s.Unlock()
	for _, id := range toClose {
		// Connection no longer active in the client, close it server-side
		conn := s.removeConnectionLocked(id)
		if conn != nil {
			// Using doTunnelClose directly instead of tunnelClose, omitting unnecessarily sending an Error message
			conn.doTunnelClose(errCloseSyncConnections)
		}
	}
}

// diffSortedSetsGetRemoved compares two sorted slices and returns those items present in a that are not present in b
// similar to coreutil's "comm -23"
func diffSortedSetsGetRemoved(a, b []int64) []int64 {
	var res []int64
	var i, j int
	for i < len(a) && j < len(b) {
		if a[i] < b[j] { // present in "a", not in "b"
			res = append(res, a[i])
			i++
		} else if a[i] > b[j] { // present in "b", not in "a"
			j++
		} else { // present in both
			i++
			j++
		}
	}
	res = append(res, a[i:]...) // any remainders in "a" are also removed from "b"
	return res
}

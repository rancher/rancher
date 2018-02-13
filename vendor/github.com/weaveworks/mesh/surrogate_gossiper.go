package mesh

import (
	"bytes"
	"hash/fnv"
	"sync"
	"time"
)

// surrogateGossiper ignores unicasts and relays broadcasts and gossips.
type surrogateGossiper struct {
	sync.Mutex
	prevUpdates []prevUpdate
}

type prevUpdate struct {
	update []byte
	hash   uint64
	t      time.Time
}

var _ Gossiper = &surrogateGossiper{}

// Hook to mock time for testing
var now = func() time.Time { return time.Now() }

// OnGossipUnicast implements Gossiper.
func (*surrogateGossiper) OnGossipUnicast(sender PeerName, msg []byte) error {
	return nil
}

// OnGossipBroadcast implements Gossiper.
func (*surrogateGossiper) OnGossipBroadcast(_ PeerName, update []byte) (GossipData, error) {
	return newSurrogateGossipData(update), nil
}

// Gossip implements Gossiper.
func (*surrogateGossiper) Gossip() GossipData {
	return nil
}

// OnGossip should return "everything new I've just learnt".
// surrogateGossiper doesn't understand the content of messages, but it can eliminate simple duplicates
func (s *surrogateGossiper) OnGossip(update []byte) (GossipData, error) {
	hash := fnv.New64a()
	_, _ = hash.Write(update)
	updateHash := hash.Sum64()
	s.Lock()
	defer s.Unlock()
	for _, p := range s.prevUpdates {
		if updateHash == p.hash && bytes.Equal(update, p.update) {
			return nil, nil
		}
	}
	// Delete anything that's older than the gossip interval, so we don't grow forever
	// (this time limit is arbitrary; surrogateGossiper should pass on new gossip immediately
	// so there should be no reason for a duplicate to show up after a long time)
	updateTime := now()
	deleteBefore := updateTime.Add(-gossipInterval)
	keepFrom := len(s.prevUpdates)
	for i, p := range s.prevUpdates {
		if p.t.After(deleteBefore) {
			keepFrom = i
			break
		}
	}
	s.prevUpdates = append(s.prevUpdates[keepFrom:], prevUpdate{update, updateHash, updateTime})
	return newSurrogateGossipData(update), nil
}

// surrogateGossipData is a simple in-memory GossipData.
type surrogateGossipData struct {
	messages [][]byte
}

var _ GossipData = &surrogateGossipData{}

func newSurrogateGossipData(msg []byte) *surrogateGossipData {
	return &surrogateGossipData{messages: [][]byte{msg}}
}

// Encode implements GossipData.
func (d *surrogateGossipData) Encode() [][]byte {
	return d.messages
}

// Merge implements GossipData.
func (d *surrogateGossipData) Merge(other GossipData) GossipData {
	o := other.(*surrogateGossipData)
	messages := make([][]byte, 0, len(d.messages)+len(o.messages))
	messages = append(messages, d.messages...)
	messages = append(messages, o.messages...)
	return &surrogateGossipData{messages: messages}
}

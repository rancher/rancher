package mesh

import "sync"

// Gossip is the sending interface.
//
// TODO(pb): rename to e.g. Sender
type Gossip interface {
	// GossipUnicast emits a single message to a peer in the mesh.
	//
	// TODO(pb): rename to Unicast?
	//
	// Unicast takes []byte instead of GossipData because "to date there has
	// been no compelling reason [in practice] to do merging on unicast."
	// But there may be some motivation to have unicast Mergeable; see
	// https://github.com/weaveworks/weave/issues/1764
	//
	// TODO(pb): for uniformity of interface, rather take GossipData?
	GossipUnicast(dst PeerName, msg []byte) error

	// GossipBroadcast emits a message to all peers in the mesh.
	//
	// TODO(pb): rename to Broadcast?
	GossipBroadcast(update GossipData)
}

// Gossiper is the receiving interface.
//
// TODO(pb): rename to e.g. Receiver
type Gossiper interface {
	// OnGossipUnicast merges received data into state.
	//
	// TODO(pb): rename to e.g. OnUnicast
	OnGossipUnicast(src PeerName, msg []byte) error

	// OnGossipBroadcast merges received data into state and returns a
	// representation of the received data (typically a delta) for further
	// propagation.
	//
	// TODO(pb): rename to e.g. OnBroadcast
	OnGossipBroadcast(src PeerName, update []byte) (received GossipData, err error)

	// Gossip returns the state of everything we know; gets called periodically.
	Gossip() (complete GossipData)

	// OnGossip merges received data into state and returns "everything new
	// I've just learnt", or nil if nothing in the received data was new.
	OnGossip(msg []byte) (delta GossipData, err error)
}

// GossipData is a merge-able dataset.
// Think: log-structured data.
type GossipData interface {
	// Encode encodes the data into multiple byte-slices.
	Encode() [][]byte

	// Merge combines another GossipData into this one and returns the result.
	//
	// TODO(pb): does it need to be leave the original unmodified?
	Merge(GossipData) GossipData
}

// GossipSender accumulates GossipData that needs to be sent to one
// destination, and sends it when possible. GossipSender is one-to-one with a
// channel.
type gossipSender struct {
	sync.Mutex
	makeMsg          func(msg []byte) protocolMsg
	makeBroadcastMsg func(srcName PeerName, msg []byte) protocolMsg
	sender           protocolSender
	gossip           GossipData
	broadcasts       map[PeerName]GossipData
	more             chan<- struct{}
	flush            chan<- chan<- bool // for testing
}

// NewGossipSender constructs a usable GossipSender.
func newGossipSender(
	makeMsg func(msg []byte) protocolMsg,
	makeBroadcastMsg func(srcName PeerName, msg []byte) protocolMsg,
	sender protocolSender,
	stop <-chan struct{},
) *gossipSender {
	more := make(chan struct{}, 1)
	flush := make(chan chan<- bool)
	s := &gossipSender{
		makeMsg:          makeMsg,
		makeBroadcastMsg: makeBroadcastMsg,
		sender:           sender,
		broadcasts:       make(map[PeerName]GossipData),
		more:             more,
		flush:            flush,
	}
	go s.run(stop, more, flush)
	return s
}

func (s *gossipSender) run(stop <-chan struct{}, more <-chan struct{}, flush <-chan chan<- bool) {
	sent := false
	for {
		select {
		case <-stop:
			return
		case <-more:
			sentSomething, err := s.deliver(stop)
			if err != nil {
				return
			}
			sent = sent || sentSomething
		case ch := <-flush: // for testing
			// send anything pending, then reply back whether we sent
			// anything since previous flush
			select {
			case <-more:
				sentSomething, err := s.deliver(stop)
				if err != nil {
					return
				}
				sent = sent || sentSomething
			default:
			}
			ch <- sent
			sent = false
		}
	}
}

func (s *gossipSender) deliver(stop <-chan struct{}) (bool, error) {
	sent := false
	// We must not hold our lock when sending, since that would block
	// the callers of Send/Broadcast while we are stuck waiting for
	// network congestion to clear. So we pick and send one piece of
	// data at a time, only holding the lock during the picking.
	for {
		select {
		case <-stop:
			return sent, nil
		default:
		}
		data, makeProtocolMsg := s.pick()
		if data == nil {
			return sent, nil
		}
		for _, msg := range data.Encode() {
			if err := s.sender.SendProtocolMsg(makeProtocolMsg(msg)); err != nil {
				return sent, err
			}
		}
		sent = true
	}
}

func (s *gossipSender) pick() (data GossipData, makeProtocolMsg func(msg []byte) protocolMsg) {
	s.Lock()
	defer s.Unlock()
	switch {
	case s.gossip != nil: // usually more important than broadcasts
		data = s.gossip
		makeProtocolMsg = s.makeMsg
		s.gossip = nil
	case len(s.broadcasts) > 0:
		for srcName, d := range s.broadcasts {
			data = d
			makeProtocolMsg = func(msg []byte) protocolMsg { return s.makeBroadcastMsg(srcName, msg) }
			delete(s.broadcasts, srcName)
			break
		}
	}
	return
}

// Send accumulates the GossipData and will send it eventually.
// Send and Broadcast accumulate into different buckets.
func (s *gossipSender) Send(data GossipData) {
	s.Lock()
	defer s.Unlock()
	if s.empty() {
		defer s.prod()
	}
	if s.gossip == nil {
		s.gossip = data
	} else {
		s.gossip = s.gossip.Merge(data)
	}
}

// Broadcast accumulates the GossipData under the given srcName and will send
// it eventually. Send and Broadcast accumulate into different buckets.
func (s *gossipSender) Broadcast(srcName PeerName, data GossipData) {
	s.Lock()
	defer s.Unlock()
	if s.empty() {
		defer s.prod()
	}
	d, found := s.broadcasts[srcName]
	if !found {
		s.broadcasts[srcName] = data
	} else {
		s.broadcasts[srcName] = d.Merge(data)
	}
}

func (s *gossipSender) empty() bool { return s.gossip == nil && len(s.broadcasts) == 0 }

func (s *gossipSender) prod() {
	select {
	case s.more <- struct{}{}:
	default:
	}
}

// Flush sends all pending data, and returns true if anything was sent since
// the previous flush. For testing.
func (s *gossipSender) Flush() bool {
	ch := make(chan bool)
	s.flush <- ch
	return <-ch
}

// gossipSenders wraps a ProtocolSender (e.g. a LocalConnection) and yields
// per-channel GossipSenders.
// TODO(pb): may be able to remove this and use makeGossipSender directly
type gossipSenders struct {
	sync.Mutex
	sender  protocolSender
	stop    <-chan struct{}
	senders map[string]*gossipSender
}

// NewGossipSenders returns a usable GossipSenders leveraging the ProtocolSender.
// TODO(pb): is stop chan the best way to do that?
func newGossipSenders(sender protocolSender, stop <-chan struct{}) *gossipSenders {
	return &gossipSenders{
		sender:  sender,
		stop:    stop,
		senders: make(map[string]*gossipSender),
	}
}

// Sender yields the GossipSender for the named channel.
// It will use the factory function if no sender yet exists.
func (gs *gossipSenders) Sender(channelName string, makeGossipSender func(sender protocolSender, stop <-chan struct{}) *gossipSender) *gossipSender {
	gs.Lock()
	defer gs.Unlock()
	s, found := gs.senders[channelName]
	if !found {
		s = makeGossipSender(gs.sender, gs.stop)
		gs.senders[channelName] = s
	}
	return s
}

// Flush flushes all managed senders. Used for testing.
func (gs *gossipSenders) Flush() bool {
	sent := false
	gs.Lock()
	defer gs.Unlock()
	for _, sender := range gs.senders {
		sent = sender.Flush() || sent
	}
	return sent
}

// GossipChannels is an index of channel name to gossip channel.
type gossipChannels map[string]*gossipChannel

type gossipConnection interface {
	gossipSenders() *gossipSenders
}

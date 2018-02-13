package mesh

import (
	"fmt"
	"math/rand"
	"net"
	"time"
	"unicode"
)

const (
	initialInterval = 2 * time.Second
	maxInterval     = 6 * time.Minute
	resetAfter      = 1 * time.Minute
)

type peerAddrs map[string]*net.TCPAddr

// ConnectionMaker initiates and manages connections to peers.
type connectionMaker struct {
	ourself          *localPeer
	peers            *Peers
	localAddr        string
	port             int
	discovery        bool
	targets          map[string]*target
	connections      map[Connection]struct{}
	directPeers      peerAddrs
	terminationCount int
	actionChan       chan<- connectionMakerAction
	logger           Logger
}

// TargetState describes the connection state of a remote target.
type targetState int

const (
	targetWaiting targetState = iota
	targetAttempting
	targetConnected
	targetSuspended
)

// Information about an address where we may find a peer.
type target struct {
	state       targetState
	lastError   error         // reason for disconnection last time
	tryAfter    time.Time     // next time to try this address
	tryInterval time.Duration // retry delay on next failure
}

// The actor closure used by ConnectionMaker. If an action returns true, the
// ConnectionMaker will check the state of its targets, and reconnect to
// relevant candidates.
type connectionMakerAction func() bool

// newConnectionMaker returns a usable ConnectionMaker, seeded with
// peers, making outbound connections from localAddr, and listening on
// port. If discovery is true, ConnectionMaker will attempt to
// initiate new connections with peers it's not directly connected to.
func newConnectionMaker(ourself *localPeer, peers *Peers, localAddr string, port int, discovery bool, logger Logger) *connectionMaker {
	actionChan := make(chan connectionMakerAction, ChannelSize)
	cm := &connectionMaker{
		ourself:     ourself,
		peers:       peers,
		localAddr:   localAddr,
		port:        port,
		discovery:   discovery,
		directPeers: peerAddrs{},
		targets:     make(map[string]*target),
		connections: make(map[Connection]struct{}),
		actionChan:  actionChan,
		logger:      logger,
	}
	go cm.queryLoop(actionChan)
	return cm
}

// InitiateConnections creates new connections to the provided peers,
// specified in host:port format. If replace is true, any existing direct
// peers are forgotten.
//
// TODO(pb): Weave Net invokes router.ConnectionMaker.InitiateConnections;
// it may be better to provide that on Router directly.
func (cm *connectionMaker) InitiateConnections(peers []string, replace bool) []error {
	errors := []error{}
	addrs := peerAddrs{}
	for _, peer := range peers {
		host, port, err := net.SplitHostPort(peer)
		if err != nil {
			host = peer
			port = "0" // we use that as an indication that "no port was supplied"
		}
		if host == "" || !isAlnum(port) {
			errors = append(errors, fmt.Errorf("invalid peer name %q, should be host[:port]", peer))
		} else if addr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%s", host, port)); err != nil {
			errors = append(errors, err)
		} else {
			addrs[peer] = addr
		}
	}
	cm.actionChan <- func() bool {
		if replace {
			cm.directPeers = peerAddrs{}
		}
		for peer, addr := range addrs {
			cm.directPeers[peer] = addr
			// curtail any existing reconnect interval
			if target, found := cm.targets[cm.completeAddr(*addr)]; found {
				target.nextTryNow()
			}
		}
		return true
	}
	return errors
}

func isAlnum(s string) bool {
	for _, c := range s {
		if !unicode.In(c, unicode.Letter, unicode.Digit) {
			return false
		}
	}
	return true
}

// ForgetConnections removes direct connections to the provided peers,
// specified in host:port format.
//
// TODO(pb): Weave Net invokes router.ConnectionMaker.ForgetConnections;
// it may be better to provide that on Router directly.
func (cm *connectionMaker) ForgetConnections(peers []string) {
	cm.actionChan <- func() bool {
		for _, peer := range peers {
			delete(cm.directPeers, peer)
		}
		return true
	}
}

// Targets takes a snapshot of the targets (direct peers),
// either just the ones we are still trying, or all of them.
// Note these are the same things that InitiateConnections and ForgetConnections talks about,
// but a method to retrieve 'Connections' would obviously return the current connections.
func (cm *connectionMaker) Targets(activeOnly bool) []string {
	resultChan := make(chan []string)
	cm.actionChan <- func() bool {
		var slice []string
		for peer, addr := range cm.directPeers {
			if activeOnly {
				if target, ok := cm.targets[cm.completeAddr(*addr)]; ok && target.tryAfter.IsZero() {
					continue
				}
			}
			slice = append(slice, peer)
		}
		resultChan <- slice
		return false
	}
	return <-resultChan
}

// connectionAborted marks the target identified by address as broken, and
// puts it in the TargetWaiting state.
func (cm *connectionMaker) connectionAborted(address string, err error) {
	cm.actionChan <- func() bool {
		target := cm.targets[address]
		target.state = targetWaiting
		target.lastError = err
		target.nextTryLater()
		return true
	}
}

// connectionCreated registers the passed connection, and marks the target
// identified by conn.RemoteTCPAddr() as established, and puts it in the
// TargetConnected state.
func (cm *connectionMaker) connectionCreated(conn Connection) {
	cm.actionChan <- func() bool {
		cm.connections[conn] = struct{}{}
		if conn.isOutbound() {
			target := cm.targets[conn.remoteTCPAddress()]
			target.state = targetConnected
		}
		return false
	}
}

// connectionTerminated unregisters the passed connection, and marks the
// target identified by conn.RemoteTCPAddr() as Waiting.
func (cm *connectionMaker) connectionTerminated(conn Connection, err error) {
	cm.actionChan <- func() bool {
		if err != errConnectToSelf {
			cm.terminationCount++
		}
		delete(cm.connections, conn)
		if conn.isOutbound() {
			target := cm.targets[conn.remoteTCPAddress()]
			target.state = targetWaiting
			target.lastError = err
			_, peerNameCollision := err.(*peerNameCollisionError)
			switch {
			case peerNameCollision || err == errConnectToSelf:
				target.nextTryNever()
			case time.Now().After(target.tryAfter.Add(resetAfter)):
				target.nextTryNow()
			default:
				target.nextTryLater()
			}
		}
		return true
	}
}

// refresh sends a no-op action into the ConnectionMaker, purely so that the
// ConnectionMaker will check the state of its targets and reconnect to
// relevant candidates.
func (cm *connectionMaker) refresh() {
	cm.actionChan <- func() bool { return true }
}

func (cm *connectionMaker) queryLoop(actionChan <-chan connectionMakerAction) {
	timer := time.NewTimer(maxDuration)
	run := func() { timer.Reset(cm.checkStateAndAttemptConnections()) }
	for {
		select {
		case action := <-actionChan:
			if action() {
				run()
			}
		case <-timer.C:
			run()
		}
	}
}

func (cm *connectionMaker) completeAddr(addr net.TCPAddr) string {
	if addr.Port == 0 {
		addr.Port = cm.port
	}
	return addr.String()
}

func (cm *connectionMaker) checkStateAndAttemptConnections() time.Duration {
	var (
		validTarget  = make(map[string]struct{})
		directTarget = make(map[string]struct{})
	)
	ourConnectedPeers, ourConnectedTargets, ourInboundIPs := cm.ourConnections()

	addTarget := func(address string) {
		if _, connected := ourConnectedTargets[address]; connected {
			return
		}
		validTarget[address] = struct{}{}
		if _, found := cm.targets[address]; found {
			return
		}
		tgt := &target{state: targetWaiting}
		tgt.nextTryNow()
		cm.targets[address] = tgt
	}

	// Add direct targets that are not connected
	for _, addr := range cm.directPeers {
		attempt := true
		if addr.Port == 0 {
			// If a peer was specified w/o a port, then we do not
			// attempt to connect to it if we have any inbound
			// connections from that IP.
			if _, connected := ourInboundIPs[addr.IP.String()]; connected {
				attempt = false
			}
		}
		address := cm.completeAddr(*addr)
		directTarget[address] = struct{}{}
		if attempt {
			addTarget(address)
		}
	}

	// Add targets for peers that someone else is connected to, but we
	// aren't
	if cm.discovery {
		cm.addPeerTargets(ourConnectedPeers, addTarget)
	}

	return cm.connectToTargets(validTarget, directTarget)
}

func (cm *connectionMaker) ourConnections() (peerNameSet, map[string]struct{}, map[string]struct{}) {
	var (
		ourConnectedPeers   = make(peerNameSet)
		ourConnectedTargets = make(map[string]struct{})
		ourInboundIPs       = make(map[string]struct{})
	)
	for conn := range cm.connections {
		address := conn.remoteTCPAddress()
		ourConnectedPeers[conn.Remote().Name] = struct{}{}
		ourConnectedTargets[address] = struct{}{}
		if conn.isOutbound() {
			continue
		}
		if ip, _, err := net.SplitHostPort(address); err == nil { // should always succeed
			ourInboundIPs[ip] = struct{}{}
		}
	}
	return ourConnectedPeers, ourConnectedTargets, ourInboundIPs
}

func (cm *connectionMaker) addPeerTargets(ourConnectedPeers peerNameSet, addTarget func(string)) {
	cm.peers.forEach(func(peer *Peer) {
		if peer == cm.ourself.Peer {
			return
		}
		// Modifying peer.connections requires a write lock on Peers,
		// and since we are holding a read lock (due to the ForEach),
		// access without locking the peer is safe.
		for otherPeer, conn := range peer.connections {
			if otherPeer == cm.ourself.Name {
				continue
			}
			if _, connected := ourConnectedPeers[otherPeer]; connected {
				continue
			}
			address := conn.remoteTCPAddress()
			if conn.isOutbound() {
				addTarget(address)
			} else if ip, _, err := net.SplitHostPort(address); err == nil {
				// There is no point connecting to the (likely
				// ephemeral) remote port of an inbound connection
				// that some peer has. Let's try to connect on the
				// weave port instead.
				addTarget(fmt.Sprintf("%s:%d", ip, cm.port))
			}
		}
	})
}

func (cm *connectionMaker) connectToTargets(validTarget map[string]struct{}, directTarget map[string]struct{}) time.Duration {
	now := time.Now() // make sure we catch items just added
	after := maxDuration
	for address, target := range cm.targets {
		if target.state != targetWaiting && target.state != targetSuspended {
			continue
		}
		if _, valid := validTarget[address]; !valid {
			// Not valid: suspend reconnects if direct peer,
			// otherwise forget this target entirely
			if _, direct := directTarget[address]; direct {
				target.state = targetSuspended
			} else {
				delete(cm.targets, address)
			}
			continue
		}
		if target.tryAfter.IsZero() {
			continue
		}
		target.state = targetWaiting
		switch duration := target.tryAfter.Sub(now); {
		case duration <= 0:
			target.state = targetAttempting
			_, isCmdLineTarget := directTarget[address]
			go cm.attemptConnection(address, isCmdLineTarget)
		case duration < after:
			after = duration
		}
	}
	return after
}

func (cm *connectionMaker) attemptConnection(address string, acceptNewPeer bool) {
	cm.logger.Printf("->[%s] attempting connection", address)
	if err := cm.ourself.createConnection(cm.localAddr, address, acceptNewPeer, cm.logger); err != nil {
		cm.logger.Printf("->[%s] error during connection attempt: %v", address, err)
		cm.connectionAborted(address, err)
	}
}

func (t *target) nextTryNever() {
	t.tryAfter = time.Time{}
	t.tryInterval = maxInterval
}

func (t *target) nextTryNow() {
	t.tryAfter = time.Now()
	t.tryInterval = initialInterval
}

// The delay at the nth retry is a random value in the range
// [i-i/2,i+i/2], where i = InitialInterval * 1.5^(n-1).
func (t *target) nextTryLater() {
	t.tryAfter = time.Now().Add(t.tryInterval/2 + time.Duration(rand.Int63n(int64(t.tryInterval))))
	t.tryInterval = t.tryInterval * 3 / 2
	if t.tryInterval > maxInterval {
		t.tryInterval = maxInterval
	}
}

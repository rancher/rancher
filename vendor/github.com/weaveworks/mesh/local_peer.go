package mesh

import (
	"encoding/gob"
	"fmt"
	"net"
	"sync"
	"time"
)

// localPeer is the only "active" peer in the mesh. It extends Peer with
// additional behaviors, mostly to retrieve and manage connection state.
type localPeer struct {
	sync.RWMutex
	*Peer
	router     *Router
	actionChan chan<- localPeerAction
}

// The actor closure used by localPeer.
type localPeerAction func()

// newLocalPeer returns a usable LocalPeer.
func newLocalPeer(name PeerName, nickName string, router *Router) *localPeer {
	actionChan := make(chan localPeerAction, ChannelSize)
	peer := &localPeer{
		Peer:       newPeer(name, nickName, randomPeerUID(), 0, randomPeerShortID()),
		router:     router,
		actionChan: actionChan,
	}
	go peer.actorLoop(actionChan)
	return peer
}

// Connections returns all the connections that the local peer is aware of.
func (peer *localPeer) getConnections() connectionSet {
	connections := make(connectionSet)
	peer.RLock()
	defer peer.RUnlock()
	for _, conn := range peer.connections {
		connections[conn] = struct{}{}
	}
	return connections
}

// ConnectionTo returns the connection to the named peer, if any.
//
// TODO(pb): Weave Net invokes router.Ourself.ConnectionTo;
// it may be better to provide that on Router directly.
func (peer *localPeer) ConnectionTo(name PeerName) (Connection, bool) {
	peer.RLock()
	defer peer.RUnlock()
	conn, found := peer.connections[name]
	return conn, found // yes, you really can't inline that. FFS.
}

// ConnectionsTo returns all known connections to the named peers.
//
// TODO(pb): Weave Net invokes router.Ourself.ConnectionsTo;
// it may be better to provide that on Router directly.
func (peer *localPeer) ConnectionsTo(names []PeerName) []Connection {
	if len(names) == 0 {
		return nil
	}
	conns := make([]Connection, 0, len(names))
	peer.RLock()
	defer peer.RUnlock()
	for _, name := range names {
		conn, found := peer.connections[name]
		// Again, !found could just be due to a race.
		if found {
			conns = append(conns, conn)
		}
	}
	return conns
}

// createConnection creates a new connection, originating from
// localAddr, to peerAddr. If acceptNewPeer is false, peerAddr must
// already be a member of the mesh.
func (peer *localPeer) createConnection(localAddr string, peerAddr string, acceptNewPeer bool, logger Logger) error {
	if err := peer.checkConnectionLimit(); err != nil {
		return err
	}
	localTCPAddr, err := net.ResolveTCPAddr("tcp4", localAddr)
	if err != nil {
		return err
	}
	remoteTCPAddr, err := net.ResolveTCPAddr("tcp4", peerAddr)
	if err != nil {
		return err
	}
	tcpConn, err := net.DialTCP("tcp4", localTCPAddr, remoteTCPAddr)
	if err != nil {
		return err
	}
	connRemote := newRemoteConnection(peer.Peer, nil, peerAddr, true, false)
	startLocalConnection(connRemote, tcpConn, peer.router, acceptNewPeer, logger)
	return nil
}

// ACTOR client API

// Synchronous.
func (peer *localPeer) doAddConnection(conn ourConnection, isRestartedPeer bool) error {
	resultChan := make(chan error)
	peer.actionChan <- func() {
		resultChan <- peer.handleAddConnection(conn, isRestartedPeer)
	}
	return <-resultChan
}

// Asynchronous.
func (peer *localPeer) doConnectionEstablished(conn ourConnection) {
	peer.actionChan <- func() {
		peer.handleConnectionEstablished(conn)
	}
}

// Synchronous.
func (peer *localPeer) doDeleteConnection(conn ourConnection) {
	resultChan := make(chan interface{})
	peer.actionChan <- func() {
		peer.handleDeleteConnection(conn)
		resultChan <- nil
	}
	<-resultChan
}

func (peer *localPeer) encode(enc *gob.Encoder) {
	peer.RLock()
	defer peer.RUnlock()
	peer.Peer.encode(enc)
}

// ACTOR server

func (peer *localPeer) actorLoop(actionChan <-chan localPeerAction) {
	gossipTimer := time.Tick(gossipInterval)
	for {
		select {
		case action := <-actionChan:
			action()
		case <-gossipTimer:
			peer.router.sendAllGossip()
		}
	}
}

func (peer *localPeer) handleAddConnection(conn ourConnection, isRestartedPeer bool) error {
	if peer.Peer != conn.getLocal() {
		panic("Attempt made to add connection to peer where peer is not the source of connection")
	}
	if conn.Remote() == nil {
		panic("Attempt made to add connection to peer with unknown remote peer")
	}
	toName := conn.Remote().Name
	dupErr := fmt.Errorf("Multiple connections to %s added to %s", conn.Remote(), peer.String())
	// deliberately non symmetrical
	if dupConn, found := peer.connections[toName]; found {
		if dupConn == conn {
			return nil
		}
		dupOurConn := dupConn.(ourConnection)
		switch conn.breakTie(dupOurConn) {
		case tieBreakWon:
			dupOurConn.shutdown(dupErr)
			peer.handleDeleteConnection(dupOurConn)
		case tieBreakLost:
			return dupErr
		case tieBreakTied:
			// oh good grief. Sod it, just kill both of them.
			dupOurConn.shutdown(dupErr)
			peer.handleDeleteConnection(dupOurConn)
			return dupErr
		}
	}
	if err := peer.checkConnectionLimit(); err != nil {
		return err
	}
	_, isConnectedPeer := peer.router.Routes.Unicast(toName)
	peer.addConnection(conn)
	switch {
	case isRestartedPeer:
		conn.logf("connection added (restarted peer)")
		peer.router.sendAllGossipDown(conn)
	case isConnectedPeer:
		conn.logf("connection added")
	default:
		conn.logf("connection added (new peer)")
		peer.router.sendAllGossipDown(conn)
	}

	peer.router.Routes.recalculate()
	peer.broadcastPeerUpdate(conn.Remote())

	return nil
}

func (peer *localPeer) handleConnectionEstablished(conn ourConnection) {
	if peer.Peer != conn.getLocal() {
		panic("Peer informed of active connection where peer is not the source of connection")
	}
	if dupConn, found := peer.connections[conn.Remote().Name]; !found || conn != dupConn {
		conn.shutdown(fmt.Errorf("Cannot set unknown connection active"))
		return
	}
	peer.connectionEstablished(conn)
	conn.logf("connection fully established")

	peer.router.Routes.recalculate()
	peer.broadcastPeerUpdate()
}

func (peer *localPeer) handleDeleteConnection(conn ourConnection) {
	if peer.Peer != conn.getLocal() {
		panic("Attempt made to delete connection from peer where peer is not the source of connection")
	}
	if conn.Remote() == nil {
		panic("Attempt made to delete connection to peer with unknown remote peer")
	}
	toName := conn.Remote().Name
	if connFound, found := peer.connections[toName]; !found || connFound != conn {
		return
	}
	peer.deleteConnection(conn)
	conn.logf("connection deleted")
	// Must do garbage collection first to ensure we don't send out an
	// update with unreachable peers (can cause looping)
	peer.router.Peers.GarbageCollect()
	peer.router.Routes.recalculate()
	peer.broadcastPeerUpdate()
}

// helpers

func (peer *localPeer) broadcastPeerUpdate(peers ...*Peer) {
	// Some tests run without a router.  This should be fixed so
	// that the relevant part of Router can be easily run in the
	// context of a test, but that will involve significant
	// reworking of tests.
	if peer.router != nil {
		peer.router.broadcastTopologyUpdate(append(peers, peer.Peer))
	}
}

func (peer *localPeer) checkConnectionLimit() error {
	limit := peer.router.ConnLimit
	if 0 != limit && peer.connectionCount() >= limit {
		return fmt.Errorf("Connection limit reached (%v)", limit)
	}
	return nil
}

func (peer *localPeer) addConnection(conn Connection) {
	peer.Lock()
	defer peer.Unlock()
	peer.connections[conn.Remote().Name] = conn
	peer.Version++
}

func (peer *localPeer) deleteConnection(conn Connection) {
	peer.Lock()
	defer peer.Unlock()
	delete(peer.connections, conn.Remote().Name)
	peer.Version++
}

func (peer *localPeer) connectionEstablished(conn Connection) {
	peer.Lock()
	defer peer.Unlock()
	peer.Version++
}

func (peer *localPeer) connectionCount() int {
	peer.RLock()
	defer peer.RUnlock()
	return len(peer.connections)
}

func (peer *localPeer) setShortID(shortID PeerShortID) {
	peer.Lock()
	defer peer.Unlock()
	peer.ShortID = shortID
	peer.Version++
}

func (peer *localPeer) setVersionBeyond(version uint64) bool {
	peer.Lock()
	defer peer.Unlock()
	if version >= peer.Version {
		peer.Version = version + 1
		return true
	}
	return false
}

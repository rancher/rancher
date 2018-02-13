package mesh

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

// Connection describes a link between peers.
// It may be in any state, not necessarily established.
type Connection interface {
	Remote() *Peer

	getLocal() *Peer
	remoteTCPAddress() string
	isOutbound() bool
	isEstablished() bool
}

type ourConnection interface {
	Connection

	breakTie(ourConnection) connectionTieBreak
	shutdown(error)
	logf(format string, args ...interface{})
}

// A local representation of the remote side of a connection.
// Limited capabilities compared to LocalConnection.
type remoteConnection struct {
	local         *Peer
	remote        *Peer
	remoteTCPAddr string
	outbound      bool
	established   bool
}

func newRemoteConnection(from, to *Peer, tcpAddr string, outbound bool, established bool) *remoteConnection {
	return &remoteConnection{
		local:         from,
		remote:        to,
		remoteTCPAddr: tcpAddr,
		outbound:      outbound,
		established:   established,
	}
}

func (conn *remoteConnection) Remote() *Peer { return conn.remote }

func (conn *remoteConnection) getLocal() *Peer { return conn.local }

func (conn *remoteConnection) remoteTCPAddress() string { return conn.remoteTCPAddr }

func (conn *remoteConnection) isOutbound() bool { return conn.outbound }

func (conn *remoteConnection) isEstablished() bool { return conn.established }

// LocalConnection is the local (our) side of a connection.
// It implements ProtocolSender, and manages per-channel GossipSenders.
type LocalConnection struct {
	OverlayConn OverlayConnection

	remoteConnection
	tcpConn         *net.TCPConn
	trustRemote     bool // is remote on a trusted subnet?
	trustedByRemote bool // does remote trust us?
	version         byte
	tcpSender       tcpSender
	sessionKey      *[32]byte
	heartbeatTCP    *time.Ticker
	router          *Router
	uid             uint64
	actionChan      chan<- connectionAction
	errorChan       chan<- error
	finished        <-chan struct{} // closed to signal that actorLoop has finished
	senders         *gossipSenders
	logger          Logger
}

// If the connection is successful, it will end up in the local peer's
// connections map.
func startLocalConnection(connRemote *remoteConnection, tcpConn *net.TCPConn, router *Router, acceptNewPeer bool, logger Logger) {
	if connRemote.local != router.Ourself.Peer {
		panic("attempt to create local connection from a peer which is not ourself")
	}
	actionChan := make(chan connectionAction, ChannelSize)
	errorChan := make(chan error, 1)
	finished := make(chan struct{})
	conn := &LocalConnection{
		remoteConnection: *connRemote, // NB, we're taking a copy of connRemote here.
		router:           router,
		tcpConn:          tcpConn,
		trustRemote:      router.trusts(connRemote),
		uid:              randUint64(),
		actionChan:       actionChan,
		errorChan:        errorChan,
		finished:         finished,
		logger:           logger,
	}
	conn.senders = newGossipSenders(conn, finished)
	go conn.run(actionChan, errorChan, finished, acceptNewPeer)
}

func (conn *LocalConnection) logf(format string, args ...interface{}) {
	format = "->[" + conn.remoteTCPAddr + "|" + conn.remote.String() + "]: " + format
	conn.logger.Printf(format, args...)
}

func (conn *LocalConnection) breakTie(dupConn ourConnection) connectionTieBreak {
	dupConnLocal := dupConn.(*LocalConnection)
	// conn.uid is used as the tie breaker here, in the knowledge that
	// both sides will make the same decision.
	if conn.uid < dupConnLocal.uid {
		return tieBreakWon
	} else if dupConnLocal.uid < conn.uid {
		return tieBreakLost
	}
	return tieBreakTied
}

// Established returns true if the connection is established.
// TODO(pb): data race?
func (conn *LocalConnection) isEstablished() bool {
	return conn.established
}

// SendProtocolMsg implements ProtocolSender.
func (conn *LocalConnection) SendProtocolMsg(m protocolMsg) error {
	if err := conn.sendProtocolMsg(m); err != nil {
		conn.shutdown(err)
		return err
	}
	return nil
}

func (conn *LocalConnection) gossipSenders() *gossipSenders {
	return conn.senders
}

// ACTOR methods

// NB: The conn.* fields are only written by the connection actor
// process, which is the caller of the ConnectionAction funs. Hence we
// do not need locks for reading, and only need write locks for fields
// read by other processes.

// Non-blocking.
func (conn *LocalConnection) shutdown(err error) {
	// err should always be a real error, even if only io.EOF
	if err == nil {
		panic("nil error")
	}

	select {
	case conn.errorChan <- err:
	default:
	}
}

// Send an actor request to the actorLoop, but don't block if actorLoop has
// exited. See http://blog.golang.org/pipelines for pattern.
func (conn *LocalConnection) sendAction(action connectionAction) {
	select {
	case conn.actionChan <- action:
	case <-conn.finished:
	}
}

// ACTOR server

func (conn *LocalConnection) run(actionChan <-chan connectionAction, errorChan <-chan error, finished chan<- struct{}, acceptNewPeer bool) {
	var err error // important to use this var and not create another one with 'err :='
	defer func() { conn.teardown(err) }()
	defer close(finished)

	if err = conn.tcpConn.SetLinger(0); err != nil {
		return
	}

	intro, err := protocolIntroParams{
		MinVersion: conn.router.ProtocolMinVersion,
		MaxVersion: ProtocolMaxVersion,
		Features:   conn.makeFeatures(),
		Conn:       conn.tcpConn,
		Password:   conn.router.Password,
		Outbound:   conn.outbound,
	}.doIntro()
	if err != nil {
		return
	}

	conn.sessionKey = intro.SessionKey
	conn.tcpSender = intro.Sender
	conn.version = intro.Version

	remote, err := conn.parseFeatures(intro.Features)
	if err != nil {
		return
	}

	if err = conn.registerRemote(remote, acceptNewPeer); err != nil {
		return
	}
	isRestartedPeer := conn.Remote().UID != remote.UID

	conn.logf("connection ready; using protocol version %v", conn.version)

	// only use negotiated session key for untrusted connections
	var sessionKey *[32]byte
	if conn.untrusted() {
		sessionKey = conn.sessionKey
	}

	params := OverlayConnectionParams{
		RemotePeer:         conn.remote,
		LocalAddr:          conn.tcpConn.LocalAddr().(*net.TCPAddr),
		RemoteAddr:         conn.tcpConn.RemoteAddr().(*net.TCPAddr),
		Outbound:           conn.outbound,
		ConnUID:            conn.uid,
		SessionKey:         sessionKey,
		SendControlMessage: conn.sendOverlayControlMessage,
		Features:           intro.Features,
	}
	if conn.OverlayConn, err = conn.router.Overlay.PrepareConnection(params); err != nil {
		return
	}

	// As soon as we do AddConnection, the new connection becomes
	// visible to the packet routing logic.  So AddConnection must
	// come after PrepareConnection
	if err = conn.router.Ourself.doAddConnection(conn, isRestartedPeer); err != nil {
		return
	}
	conn.router.ConnectionMaker.connectionCreated(conn)

	// OverlayConnection confirmation comes after AddConnection,
	// because only after that completes do we know the connection is
	// valid: in particular that it is not a duplicate connection to
	// the same peer. Overlay communication on a duplicate connection
	// can cause problems such as tripping up overlay crypto at the
	// other end due to data being decoded by the other connection. It
	// is also generally wasteful to engage in any interaction with
	// the remote on a connection that turns out to be invalid.
	conn.OverlayConn.Confirm()

	// receiveTCP must follow also AddConnection. In the absence
	// of any indirect connectivity to the remote peer, the first
	// we hear about it (and any peers reachable from it) is
	// through topology gossip it sends us on the connection. We
	// must ensure that the connection has been added to Ourself
	// prior to processing any such gossip, otherwise we risk
	// immediately gc'ing part of that newly received portion of
	// the topology (though not the remote peer itself, since that
	// will have a positive ref count), leaving behind dangling
	// references to peers. Hence we must invoke AddConnection,
	// which is *synchronous*, first.
	conn.heartbeatTCP = time.NewTicker(tcpHeartbeat)
	go conn.receiveTCP(intro.Receiver)

	// AddConnection must precede actorLoop. More precisely, it
	// must precede shutdown, since that invokes DeleteConnection
	// and is invoked on termination of this entire
	// function. Essentially this boils down to a prohibition on
	// running AddConnection in a separate goroutine, at least not
	// without some synchronisation. Which in turn requires the
	// launching of the receiveTCP goroutine to precede actorLoop.
	err = conn.actorLoop(actionChan, errorChan)
}

func (conn *LocalConnection) makeFeatures() map[string]string {
	features := map[string]string{
		"PeerNameFlavour": PeerNameFlavour,
		"Name":            conn.local.Name.String(),
		"NickName":        conn.local.NickName,
		"ShortID":         fmt.Sprint(conn.local.ShortID),
		"UID":             fmt.Sprint(conn.local.UID),
		"ConnID":          fmt.Sprint(conn.uid),
		"Trusted":         fmt.Sprint(conn.trustRemote),
	}
	conn.router.Overlay.AddFeaturesTo(features)
	return features
}

func (conn *LocalConnection) parseFeatures(features map[string]string) (*Peer, error) {
	if err := mustHave(features, []string{"PeerNameFlavour", "Name", "NickName", "UID", "ConnID"}); err != nil {
		return nil, err
	}

	remotePeerNameFlavour := features["PeerNameFlavour"]
	if remotePeerNameFlavour != PeerNameFlavour {
		return nil, fmt.Errorf("Peer name flavour mismatch (ours: '%s', theirs: '%s')", PeerNameFlavour, remotePeerNameFlavour)
	}

	name, err := PeerNameFromString(features["Name"])
	if err != nil {
		return nil, err
	}

	nickName := features["NickName"]

	var shortID uint64
	var hasShortID bool
	if shortIDStr, ok := features["ShortID"]; ok {
		hasShortID = true
		shortID, err = strconv.ParseUint(shortIDStr, 10, peerShortIDBits)
		if err != nil {
			return nil, err
		}
	}

	var trusted bool
	if trustedStr, ok := features["Trusted"]; ok {
		trusted, err = strconv.ParseBool(trustedStr)
		if err != nil {
			return nil, err
		}
	}
	conn.trustedByRemote = trusted

	uid, err := parsePeerUID(features["UID"])
	if err != nil {
		return nil, err
	}

	remoteConnID, err := strconv.ParseUint(features["ConnID"], 10, 64)
	if err != nil {
		return nil, err
	}

	conn.uid ^= remoteConnID
	peer := newPeer(name, nickName, uid, 0, PeerShortID(shortID))
	peer.HasShortID = hasShortID
	return peer, nil
}

func (conn *LocalConnection) registerRemote(remote *Peer, acceptNewPeer bool) error {
	if acceptNewPeer {
		conn.remote = conn.router.Peers.fetchWithDefault(remote)
	} else {
		conn.remote = conn.router.Peers.fetchAndAddRef(remote.Name)
		if conn.remote == nil {
			return fmt.Errorf("Found unknown remote name: %s at %s", remote.Name, conn.remoteTCPAddr)
		}
	}

	if remote.Name == conn.local.Name && remote.UID != conn.local.UID {
		return &peerNameCollisionError{conn.local, remote}
	}
	if conn.remote == conn.local {
		return errConnectToSelf
	}

	return nil
}

func (conn *LocalConnection) actorLoop(actionChan <-chan connectionAction, errorChan <-chan error) (err error) {
	fwdErrorChan := conn.OverlayConn.ErrorChannel()
	fwdEstablishedChan := conn.OverlayConn.EstablishedChannel()

	for err == nil {
		select {
		case err = <-errorChan:
		case err = <-fwdErrorChan:
		default:
			select {
			case action := <-actionChan:
				err = action()
			case <-conn.heartbeatTCP.C:
				err = conn.sendSimpleProtocolMsg(ProtocolHeartbeat)
			case <-fwdEstablishedChan:
				conn.established = true
				fwdEstablishedChan = nil
				conn.router.Ourself.doConnectionEstablished(conn)
			case err = <-errorChan:
			case err = <-fwdErrorChan:
			}
		}
	}

	return
}

func (conn *LocalConnection) teardown(err error) {
	if conn.remote == nil {
		conn.logger.Printf("->[%s] connection shutting down due to error during handshake: %v", conn.remoteTCPAddr, err)
	} else {
		conn.logf("connection shutting down due to error: %v", err)
	}

	if conn.tcpConn != nil {
		if closeErr := conn.tcpConn.Close(); closeErr != nil {
			conn.logger.Printf("warning: %v", closeErr)
		}
	}

	if conn.remote != nil {
		conn.router.Peers.dereference(conn.remote)
		conn.router.Ourself.doDeleteConnection(conn)
	}

	if conn.heartbeatTCP != nil {
		conn.heartbeatTCP.Stop()
	}

	if conn.OverlayConn != nil {
		conn.OverlayConn.Stop()
	}

	conn.router.ConnectionMaker.connectionTerminated(conn, err)
}

func (conn *LocalConnection) sendOverlayControlMessage(tag byte, msg []byte) error {
	return conn.sendProtocolMsg(protocolMsg{protocolTag(tag), msg})
}

// Helpers

func (conn *LocalConnection) sendSimpleProtocolMsg(tag protocolTag) error {
	return conn.sendProtocolMsg(protocolMsg{tag: tag})
}

func (conn *LocalConnection) sendProtocolMsg(m protocolMsg) error {
	return conn.tcpSender.Send(append([]byte{byte(m.tag)}, m.msg...))
}

func (conn *LocalConnection) receiveTCP(receiver tcpReceiver) {
	var err error
	for {
		if err = conn.extendReadDeadline(); err != nil {
			break
		}
		var msg []byte
		if msg, err = receiver.Receive(); err != nil {
			break
		}
		if len(msg) < 1 {
			conn.logf("ignoring blank msg")
			continue
		}
		if err = conn.handleProtocolMsg(protocolTag(msg[0]), msg[1:]); err != nil {
			break
		}
	}
	conn.shutdown(err)
}

func (conn *LocalConnection) handleProtocolMsg(tag protocolTag, payload []byte) error {
	switch tag {
	case ProtocolHeartbeat:
	case ProtocolReserved1, ProtocolReserved2, ProtocolReserved3, ProtocolOverlayControlMsg:
		conn.OverlayConn.ControlMessage(byte(tag), payload)
	case ProtocolGossipUnicast, ProtocolGossipBroadcast, ProtocolGossip:
		return conn.router.handleGossip(tag, payload)
	default:
		conn.logf("ignoring unknown protocol tag: %v", tag)
	}
	return nil
}

func (conn *LocalConnection) extendReadDeadline() error {
	return conn.tcpConn.SetReadDeadline(time.Now().Add(tcpHeartbeat * 2))
}

// Untrusted returns true if either we don't trust our remote, or are not
// trusted by our remote.
func (conn *LocalConnection) untrusted() bool {
	return !conn.trustRemote || !conn.trustedByRemote
}

type connectionTieBreak int

const (
	tieBreakWon connectionTieBreak = iota
	tieBreakLost
	tieBreakTied
)

var errConnectToSelf = fmt.Errorf("cannot connect to ourself")

type peerNameCollisionError struct {
	local, remote *Peer
}

func (err *peerNameCollisionError) Error() string {
	return fmt.Sprintf("local %q and remote %q peer names collision", err.local, err.remote)
}

// The actor closure used by LocalConnection. If an action returns an error,
// it will terminate the actor loop, which terminates the connection in turn.
type connectionAction func() error

func mustHave(features map[string]string, keys []string) error {
	for _, key := range keys {
		if _, ok := features[key]; !ok {
			return fmt.Errorf("field %s is missing", key)
		}
	}
	return nil
}

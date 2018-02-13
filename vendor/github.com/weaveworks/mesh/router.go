package mesh

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math"
	"net"
	"sync"
	"time"
)

var (
	// Port is the port used for all mesh communication.
	Port = 6783

	// ChannelSize is the buffer size used by so-called actor goroutines
	// throughout mesh.
	ChannelSize = 16
)

const (
	tcpHeartbeat     = 30 * time.Second
	gossipInterval   = 30 * time.Second
	maxDuration      = time.Duration(math.MaxInt64)
	acceptMaxTokens  = 100
	acceptTokenDelay = 100 * time.Millisecond // [2]
)

// Config defines dimensions of configuration for the router.
// TODO(pb): provide usable defaults in NewRouter
type Config struct {
	Host               string
	Port               int
	ProtocolMinVersion byte
	Password           []byte
	ConnLimit          int
	PeerDiscovery      bool
	TrustedSubnets     []*net.IPNet
}

// Router manages communication between this peer and the rest of the mesh.
// Router implements Gossiper.
type Router struct {
	Config
	Overlay         Overlay
	Ourself         *localPeer
	Peers           *Peers
	Routes          *routes
	ConnectionMaker *connectionMaker
	gossipLock      sync.RWMutex
	gossipChannels  gossipChannels
	topologyGossip  Gossip
	acceptLimiter   *tokenBucket
	logger          Logger
}

// NewRouter returns a new router. It must be started.
func NewRouter(config Config, name PeerName, nickName string, overlay Overlay, logger Logger) (*Router, error) {
	router := &Router{Config: config, gossipChannels: make(gossipChannels)}

	if overlay == nil {
		overlay = NullOverlay{}
	}

	router.Overlay = overlay
	router.Ourself = newLocalPeer(name, nickName, router)
	router.Peers = newPeers(router.Ourself)
	router.Peers.OnGC(func(peer *Peer) {
		logger.Printf("Removed unreachable peer %s", peer)
	})
	router.Routes = newRoutes(router.Ourself, router.Peers)
	router.ConnectionMaker = newConnectionMaker(router.Ourself, router.Peers, net.JoinHostPort(router.Host, "0"), router.Port, router.PeerDiscovery, logger)
	router.logger = logger
	gossip, err := router.NewGossip("topology", router)
	if err != nil {
		return nil, err
	}
	router.topologyGossip = gossip
	router.acceptLimiter = newTokenBucket(acceptMaxTokens, acceptTokenDelay)
	return router, nil
}

// Start listening for TCP connections. This is separate from NewRouter so
// that gossipers can register before we start forming connections.
func (router *Router) Start() {
	router.listenTCP()
}

// Stop shuts down the router.
func (router *Router) Stop() error {
	router.Overlay.Stop()
	// TODO: perform more graceful shutdown...
	return nil
}

func (router *Router) usingPassword() bool {
	return router.Password != nil
}

func (router *Router) listenTCP() {
	localAddr, err := net.ResolveTCPAddr("tcp4", net.JoinHostPort(router.Host, fmt.Sprint(router.Port)))
	if err != nil {
		panic(err)
	}
	ln, err := net.ListenTCP("tcp4", localAddr)
	if err != nil {
		panic(err)
	}
	go func() {
		defer ln.Close()
		for {
			tcpConn, err := ln.AcceptTCP()
			if err != nil {
				router.logger.Printf("%v", err)
				continue
			}
			router.acceptTCP(tcpConn)
			router.acceptLimiter.wait()
		}
	}()
}

func (router *Router) acceptTCP(tcpConn *net.TCPConn) {
	remoteAddrStr := tcpConn.RemoteAddr().String()
	router.logger.Printf("->[%s] connection accepted", remoteAddrStr)
	connRemote := newRemoteConnection(router.Ourself.Peer, nil, remoteAddrStr, false, false)
	startLocalConnection(connRemote, tcpConn, router, true, router.logger)
}

// NewGossip returns a usable GossipChannel from the router.
//
// TODO(pb): rename?
func (router *Router) NewGossip(channelName string, g Gossiper) (Gossip, error) {
	channel := newGossipChannel(channelName, router.Ourself, router.Routes, g, router.logger)
	router.gossipLock.Lock()
	defer router.gossipLock.Unlock()
	if _, found := router.gossipChannels[channelName]; found {
		return nil, fmt.Errorf("[gossip] duplicate channel %s", channelName)
	}
	router.gossipChannels[channelName] = channel
	return channel, nil
}

func (router *Router) gossipChannel(channelName string) *gossipChannel {
	router.gossipLock.RLock()
	channel, found := router.gossipChannels[channelName]
	router.gossipLock.RUnlock()
	if found {
		return channel
	}
	router.gossipLock.Lock()
	defer router.gossipLock.Unlock()
	if channel, found = router.gossipChannels[channelName]; found {
		return channel
	}
	channel = newGossipChannel(channelName, router.Ourself, router.Routes, &surrogateGossiper{}, router.logger)
	channel.logf("created surrogate channel")
	router.gossipChannels[channelName] = channel
	return channel
}

func (router *Router) gossipChannelSet() map[*gossipChannel]struct{} {
	channels := make(map[*gossipChannel]struct{})
	router.gossipLock.RLock()
	defer router.gossipLock.RUnlock()
	for _, channel := range router.gossipChannels {
		channels[channel] = struct{}{}
	}
	return channels
}

func (router *Router) handleGossip(tag protocolTag, payload []byte) error {
	decoder := gob.NewDecoder(bytes.NewReader(payload))
	var channelName string
	if err := decoder.Decode(&channelName); err != nil {
		return err
	}
	channel := router.gossipChannel(channelName)
	var srcName PeerName
	if err := decoder.Decode(&srcName); err != nil {
		return err
	}
	switch tag {
	case ProtocolGossipUnicast:
		return channel.deliverUnicast(srcName, payload, decoder)
	case ProtocolGossipBroadcast:
		return channel.deliverBroadcast(srcName, payload, decoder)
	case ProtocolGossip:
		return channel.deliver(srcName, payload, decoder)
	}
	return nil
}

// Relay all pending gossip data for each channel via random neighbours.
func (router *Router) sendAllGossip() {
	for channel := range router.gossipChannelSet() {
		if gossip := channel.gossiper.Gossip(); gossip != nil {
			channel.Send(gossip)
		}
	}
}

// Relay all pending gossip data for each channel via conn.
func (router *Router) sendAllGossipDown(conn Connection) {
	for channel := range router.gossipChannelSet() {
		if gossip := channel.gossiper.Gossip(); gossip != nil {
			channel.SendDown(conn, gossip)
		}
	}
}

// for testing
func (router *Router) sendPendingGossip() bool {
	sentSomething := false
	for conn := range router.Ourself.getConnections() {
		sentSomething = conn.(gossipConnection).gossipSenders().Flush() || sentSomething
	}
	return sentSomething
}

// BroadcastTopologyUpdate is invoked whenever there is a change to the mesh
// topology, and broadcasts the new set of peers to the mesh.
func (router *Router) broadcastTopologyUpdate(update []*Peer) {
	names := make(peerNameSet)
	for _, p := range update {
		names[p.Name] = struct{}{}
	}
	router.topologyGossip.GossipBroadcast(&topologyGossipData{peers: router.Peers, update: names})
}

// OnGossipUnicast implements Gossiper, but always returns an error, as a
// router should only receive gossip broadcasts of TopologyGossipData.
func (router *Router) OnGossipUnicast(sender PeerName, msg []byte) error {
	return fmt.Errorf("unexpected topology gossip unicast: %v", msg)
}

// OnGossipBroadcast receives broadcasts of TopologyGossipData.
// It returns the received update unchanged.
func (router *Router) OnGossipBroadcast(_ PeerName, update []byte) (GossipData, error) {
	origUpdate, _, err := router.applyTopologyUpdate(update)
	if err != nil || len(origUpdate) == 0 {
		return nil, err
	}
	return &topologyGossipData{peers: router.Peers, update: origUpdate}, nil
}

// Gossip yields the current topology as GossipData.
func (router *Router) Gossip() GossipData {
	return &topologyGossipData{peers: router.Peers, update: router.Peers.names()}
}

// OnGossip receives broadcasts of TopologyGossipData.
// It returns an "improved" version of the received update.
// See peers.ApplyUpdate.
func (router *Router) OnGossip(update []byte) (GossipData, error) {
	_, newUpdate, err := router.applyTopologyUpdate(update)
	if err != nil || len(newUpdate) == 0 {
		return nil, err
	}
	return &topologyGossipData{peers: router.Peers, update: newUpdate}, nil
}

func (router *Router) applyTopologyUpdate(update []byte) (peerNameSet, peerNameSet, error) {
	origUpdate, newUpdate, err := router.Peers.applyUpdate(update)
	if err != nil {
		return nil, nil, err
	}
	if len(newUpdate) > 0 {
		router.ConnectionMaker.refresh()
		router.Routes.recalculate()
	}
	return origUpdate, newUpdate, nil
}

func (router *Router) trusts(remote *remoteConnection) bool {
	if tcpAddr, err := net.ResolveTCPAddr("tcp4", remote.remoteTCPAddr); err == nil {
		for _, trustedSubnet := range router.TrustedSubnets {
			if trustedSubnet.Contains(tcpAddr.IP) {
				return true
			}
		}
	} else {
		// Should not happen as remoteTCPAddr was obtained from TCPConn
		router.logger.Printf("Unable to parse remote TCP addr: %s", err)
	}
	return false
}

// The set of peers in the mesh network.
// Gossiped just like anything else.
type topologyGossipData struct {
	peers  *Peers
	update peerNameSet
}

// Merge implements GossipData.
func (d *topologyGossipData) Merge(other GossipData) GossipData {
	names := make(peerNameSet)
	for name := range d.update {
		names[name] = struct{}{}
	}
	for name := range other.(*topologyGossipData).update {
		names[name] = struct{}{}
	}
	return &topologyGossipData{peers: d.peers, update: names}
}

// Encode implements GossipData.
func (d *topologyGossipData) Encode() [][]byte {
	return [][]byte{d.peers.encodePeers(d.update)}
}

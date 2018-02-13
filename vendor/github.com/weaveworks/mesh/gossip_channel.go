package mesh

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

// gossipChannel is a logical communication channel within a physical mesh.
type gossipChannel struct {
	name     string
	ourself  *localPeer
	routes   *routes
	gossiper Gossiper
	logger   Logger
}

// newGossipChannel returns a named, usable channel.
// It delegates receiving duties to the passed Gossiper.
func newGossipChannel(channelName string, ourself *localPeer, r *routes, g Gossiper, logger Logger) *gossipChannel {
	return &gossipChannel{
		name:     channelName,
		ourself:  ourself,
		routes:   r,
		gossiper: g,
		logger:   logger,
	}
}

func (c *gossipChannel) deliverUnicast(srcName PeerName, origPayload []byte, dec *gob.Decoder) error {
	var destName PeerName
	if err := dec.Decode(&destName); err != nil {
		return err
	}
	if c.ourself.Name == destName {
		var payload []byte
		if err := dec.Decode(&payload); err != nil {
			return err
		}
		return c.gossiper.OnGossipUnicast(srcName, payload)
	}
	if err := c.relayUnicast(destName, origPayload); err != nil {
		c.logf("%v", err)
	}
	return nil
}

func (c *gossipChannel) deliverBroadcast(srcName PeerName, _ []byte, dec *gob.Decoder) error {
	var payload []byte
	if err := dec.Decode(&payload); err != nil {
		return err
	}
	data, err := c.gossiper.OnGossipBroadcast(srcName, payload)
	if err != nil || data == nil {
		return err
	}
	c.relayBroadcast(srcName, data)
	return nil
}

func (c *gossipChannel) deliver(srcName PeerName, _ []byte, dec *gob.Decoder) error {
	var payload []byte
	if err := dec.Decode(&payload); err != nil {
		return err
	}
	update, err := c.gossiper.OnGossip(payload)
	if err != nil || update == nil {
		return err
	}
	c.relay(srcName, update)
	return nil
}

// GossipUnicast implements Gossip, relaying msg to dst, which must be a
// member of the channel.
func (c *gossipChannel) GossipUnicast(dstPeerName PeerName, msg []byte) error {
	return c.relayUnicast(dstPeerName, gobEncode(c.name, c.ourself.Name, dstPeerName, msg))
}

// GossipBroadcast implements Gossip, relaying update to all members of the
// channel.
func (c *gossipChannel) GossipBroadcast(update GossipData) {
	c.relayBroadcast(c.ourself.Name, update)
}

// Send relays data into the channel topology via random neighbours.
func (c *gossipChannel) Send(data GossipData) {
	c.relay(c.ourself.Name, data)
}

// SendDown relays data into the channel topology via conn.
func (c *gossipChannel) SendDown(conn Connection, data GossipData) {
	c.senderFor(conn).Send(data)
}

func (c *gossipChannel) relayUnicast(dstPeerName PeerName, buf []byte) (err error) {
	if relayPeerName, found := c.routes.UnicastAll(dstPeerName); !found {
		err = fmt.Errorf("unknown relay destination: %s", dstPeerName)
	} else if conn, found := c.ourself.ConnectionTo(relayPeerName); !found {
		err = fmt.Errorf("unable to find connection to relay peer %s", relayPeerName)
	} else {
		err = conn.(protocolSender).SendProtocolMsg(protocolMsg{ProtocolGossipUnicast, buf})
	}
	return err
}

func (c *gossipChannel) relayBroadcast(srcName PeerName, update GossipData) {
	c.routes.ensureRecalculated()
	for _, conn := range c.ourself.ConnectionsTo(c.routes.BroadcastAll(srcName)) {
		c.senderFor(conn).Broadcast(srcName, update)
	}
}

func (c *gossipChannel) relay(srcName PeerName, data GossipData) {
	c.routes.ensureRecalculated()
	for _, conn := range c.ourself.ConnectionsTo(c.routes.randomNeighbours(srcName)) {
		c.senderFor(conn).Send(data)
	}
}

func (c *gossipChannel) senderFor(conn Connection) *gossipSender {
	return conn.(gossipConnection).gossipSenders().Sender(c.name, c.makeGossipSender)
}

func (c *gossipChannel) makeGossipSender(sender protocolSender, stop <-chan struct{}) *gossipSender {
	return newGossipSender(c.makeMsg, c.makeBroadcastMsg, sender, stop)
}

func (c *gossipChannel) makeMsg(msg []byte) protocolMsg {
	return protocolMsg{ProtocolGossip, gobEncode(c.name, c.ourself.Name, msg)}
}

func (c *gossipChannel) makeBroadcastMsg(srcName PeerName, msg []byte) protocolMsg {
	return protocolMsg{ProtocolGossipBroadcast, gobEncode(c.name, srcName, msg)}
}

func (c *gossipChannel) logf(format string, args ...interface{}) {
	format = "[gossip " + c.name + "]: " + format
	c.logger.Printf(format, args...)
}

// GobEncode gob-encodes each item and returns the resulting byte slice.
func gobEncode(items ...interface{}) []byte {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	for _, i := range items {
		if err := enc.Encode(i); err != nil {
			panic(err)
		}
	}
	return buf.Bytes()
}

package mesh

import (
	"bytes"
	"encoding/gob"
	"io"
	"math/rand"
	"sync"
)

// Peers collects all of the known peers in the mesh, including ourself.
type Peers struct {
	sync.RWMutex
	ourself   *localPeer
	byName    map[PeerName]*Peer
	byShortID map[PeerShortID]shortIDPeers
	onGC      []func(*Peer)

	// Called when the mapping from short IDs to peers changes
	onInvalidateShortIDs []func()
}

type shortIDPeers struct {
	// If we know about a single peer with the short ID, this is
	// that peer. If there is a collision, this is the peer with
	// the lowest Name.
	peer *Peer

	// In case of a collision, this holds the other peers.
	others []*Peer
}

type peerNameSet map[PeerName]struct{}

type connectionSummary struct {
	NameByte      []byte
	RemoteTCPAddr string
	Outbound      bool
	Established   bool
}

// Due to changes to Peers that need to be sent out
// once the Peers is unlocked.
type peersPendingNotifications struct {
	// Peers that have been GCed
	removed []*Peer

	// The mapping from short IDs to peers changed
	invalidateShortIDs bool

	// The local short ID needs reassigning due to a collision
	reassignLocalShortID bool

	// The local peer was modified
	localPeerModified bool
}

func newPeers(ourself *localPeer) *Peers {
	peers := &Peers{
		ourself:   ourself,
		byName:    make(map[PeerName]*Peer),
		byShortID: make(map[PeerShortID]shortIDPeers),
	}
	peers.fetchWithDefault(ourself.Peer)
	return peers
}

// Descriptions returns descriptions for all known peers.
func (peers *Peers) Descriptions() []PeerDescription {
	peers.RLock()
	defer peers.RUnlock()
	descriptions := make([]PeerDescription, 0, len(peers.byName))
	for _, peer := range peers.byName {
		descriptions = append(descriptions, PeerDescription{
			Name:           peer.Name,
			NickName:       peer.peerSummary.NickName,
			UID:            peer.UID,
			Self:           peer.Name == peers.ourself.Name,
			NumConnections: len(peer.connections),
		})
	}
	return descriptions
}

// OnGC adds a new function to be set of functions that will be executed on
// all subsequent GC runs, receiving the GC'd peer.
func (peers *Peers) OnGC(callback func(*Peer)) {
	peers.Lock()
	defer peers.Unlock()

	// Although the array underlying peers.onGC might be accessed
	// without holding the lock in unlockAndNotify, we don't
	// support removing callbacks, so a simple append here is
	// safe.
	peers.onGC = append(peers.onGC, callback)
}

// OnInvalidateShortIDs adds a new function to a set of functions that will be
// executed on all subsequent GC runs, when the mapping from short IDs to
// peers has changed.
func (peers *Peers) OnInvalidateShortIDs(callback func()) {
	peers.Lock()
	defer peers.Unlock()

	// Safe, as in OnGC
	peers.onInvalidateShortIDs = append(peers.onInvalidateShortIDs, callback)
}

func (peers *Peers) unlockAndNotify(pending *peersPendingNotifications) {
	broadcastLocalPeer := (pending.reassignLocalShortID && peers.reassignLocalShortID(pending)) || pending.localPeerModified
	onGC := peers.onGC
	onInvalidateShortIDs := peers.onInvalidateShortIDs
	peers.Unlock()

	if pending.removed != nil {
		for _, callback := range onGC {
			for _, peer := range pending.removed {
				callback(peer)
			}
		}
	}

	if pending.invalidateShortIDs {
		for _, callback := range onInvalidateShortIDs {
			callback()
		}
	}

	if broadcastLocalPeer {
		peers.ourself.broadcastPeerUpdate()
	}
}

func (peers *Peers) addByShortID(peer *Peer, pending *peersPendingNotifications) {
	if !peer.HasShortID {
		return
	}

	entry, ok := peers.byShortID[peer.ShortID]
	if !ok {
		entry = shortIDPeers{peer: peer}
	} else if entry.peer == nil {
		// This short ID is free, but was used in the past.
		// Because we are reusing it, it's an invalidation
		// event.
		entry.peer = peer
		pending.invalidateShortIDs = true
	} else if peer.Name < entry.peer.Name {
		// Short ID collision, this peer becomes the principal
		// peer for the short ID, bumping the previous one
		// into others.

		if entry.peer == peers.ourself.Peer {
			// The bumped peer is peers.ourself, so we
			// need to look for a new short ID.
			pending.reassignLocalShortID = true
		}

		entry.others = append(entry.others, entry.peer)
		entry.peer = peer
		pending.invalidateShortIDs = true
	} else {
		// Short ID collision, this peer is secondary
		entry.others = append(entry.others, peer)
	}

	peers.byShortID[peer.ShortID] = entry
}

func (peers *Peers) deleteByShortID(peer *Peer, pending *peersPendingNotifications) {
	if !peer.HasShortID {
		return
	}

	entry := peers.byShortID[peer.ShortID]
	var otherIndex int

	if peer != entry.peer {
		// peer is secondary, find its index in others
		otherIndex = -1

		for i, other := range entry.others {
			if peer == other {
				otherIndex = i
				break
			}
		}

		if otherIndex < 0 {
			return
		}
	} else if len(entry.others) != 0 {
		// need to find the peer with the lowest name to
		// become the new principal one
		otherIndex = 0
		minName := entry.others[0].Name

		for i := 1; i < len(entry.others); i++ {
			otherName := entry.others[i].Name
			if otherName < minName {
				minName = otherName
				otherIndex = i
			}
		}

		entry.peer = entry.others[otherIndex]
		pending.invalidateShortIDs = true
	} else {
		// This is the last peer with the short ID. We clear
		// the entry, don't delete it, in order to detect when
		// it gets re-used.
		peers.byShortID[peer.ShortID] = shortIDPeers{}
		return
	}

	entry.others[otherIndex] = entry.others[len(entry.others)-1]
	entry.others = entry.others[:len(entry.others)-1]
	peers.byShortID[peer.ShortID] = entry
}

func (peers *Peers) reassignLocalShortID(pending *peersPendingNotifications) bool {
	newShortID, ok := peers.chooseShortID()
	if ok {
		peers.setLocalShortID(newShortID, pending)
		return true
	}

	// Otherwise we'll try again later on in garbageColleect
	return false
}

func (peers *Peers) setLocalShortID(newShortID PeerShortID, pending *peersPendingNotifications) {
	peers.deleteByShortID(peers.ourself.Peer, pending)
	peers.ourself.setShortID(newShortID)
	peers.addByShortID(peers.ourself.Peer, pending)
}

// Choose an available short ID at random.
func (peers *Peers) chooseShortID() (PeerShortID, bool) {
	rng := rand.New(rand.NewSource(int64(randUint64())))

	// First, just try picking some short IDs at random, and
	// seeing if they are available:
	for i := 0; i < 10; i++ {
		shortID := PeerShortID(rng.Intn(1 << peerShortIDBits))
		if peers.byShortID[shortID].peer == nil {
			return shortID, true
		}
	}

	// Looks like most short IDs are used. So count the number of
	// unused ones, and pick one at random.
	available := int(1 << peerShortIDBits)
	for _, entry := range peers.byShortID {
		if entry.peer != nil {
			available--
		}
	}

	if available == 0 {
		// All short IDs are used.
		return 0, false
	}

	n := rng.Intn(available)
	var i PeerShortID
	for {
		if peers.byShortID[i].peer == nil {
			if n == 0 {
				return i, true
			}

			n--
		}

		i++
	}
}

// fetchWithDefault will use reference fields of the passed peer object to
// look up and return an existing, matching peer. If no matching peer is
// found, the passed peer is saved and returned.
func (peers *Peers) fetchWithDefault(peer *Peer) *Peer {
	peers.Lock()
	var pending peersPendingNotifications
	defer peers.unlockAndNotify(&pending)

	if existingPeer, found := peers.byName[peer.Name]; found {
		existingPeer.localRefCount++
		return existingPeer
	}

	peers.byName[peer.Name] = peer
	peers.addByShortID(peer, &pending)
	peer.localRefCount++
	return peer
}

// Fetch returns a peer matching the passed name, without incrementing its
// refcount. If no matching peer is found, Fetch returns nil.
func (peers *Peers) Fetch(name PeerName) *Peer {
	peers.RLock()
	defer peers.RUnlock()
	return peers.byName[name]
}

// Like fetch, but increments local refcount.
func (peers *Peers) fetchAndAddRef(name PeerName) *Peer {
	peers.Lock()
	defer peers.Unlock()
	peer := peers.byName[name]
	if peer != nil {
		peer.localRefCount++
	}
	return peer
}

// FetchByShortID returns a peer matching the passed short ID.
// If no matching peer is found, FetchByShortID returns nil.
func (peers *Peers) FetchByShortID(shortID PeerShortID) *Peer {
	peers.RLock()
	defer peers.RUnlock()
	return peers.byShortID[shortID].peer
}

// Dereference decrements the refcount of the matching peer.
// TODO(pb): this is an awkward way to use the mutex; consider refactoring
func (peers *Peers) dereference(peer *Peer) {
	peers.Lock()
	defer peers.Unlock()
	peer.localRefCount--
}

func (peers *Peers) forEach(fun func(*Peer)) {
	peers.RLock()
	defer peers.RUnlock()
	for _, peer := range peers.byName {
		fun(peer)
	}
}

// Merge an incoming update with our own topology.
//
// We add peers hitherto unknown to us, and update peers for which the
// update contains a more recent version than known to us. The return
// value is a) a representation of the received update, and b) an
// "improved" update containing just these new/updated elements.
func (peers *Peers) applyUpdate(update []byte) (peerNameSet, peerNameSet, error) {
	peers.Lock()
	var pending peersPendingNotifications
	defer peers.unlockAndNotify(&pending)

	newPeers, decodedUpdate, decodedConns, err := peers.decodeUpdate(update)
	if err != nil {
		return nil, nil, err
	}

	// Add new peers
	for name, newPeer := range newPeers {
		peers.byName[name] = newPeer
		peers.addByShortID(newPeer, &pending)
	}

	// Now apply the updates
	newUpdate := peers.applyDecodedUpdate(decodedUpdate, decodedConns, &pending)
	peers.garbageCollect(&pending)
	for _, peerRemoved := range pending.removed {
		delete(newUpdate, peerRemoved.Name)
	}

	updateNames := make(peerNameSet)
	for _, peer := range decodedUpdate {
		updateNames[peer.Name] = struct{}{}
	}

	return updateNames, newUpdate, nil
}

func (peers *Peers) names() peerNameSet {
	peers.RLock()
	defer peers.RUnlock()

	names := make(peerNameSet)
	for name := range peers.byName {
		names[name] = struct{}{}
	}
	return names
}

func (peers *Peers) encodePeers(names peerNameSet) []byte {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	peers.RLock()
	defer peers.RUnlock()
	for name := range names {
		if peer, found := peers.byName[name]; found {
			if peer == peers.ourself.Peer {
				peers.ourself.encode(enc)
			} else {
				peer.encode(enc)
			}
		}
	}
	return buf.Bytes()
}

// GarbageCollect takes a lock, triggers a GC, and invokes the accumulated GC
// callbacks.
func (peers *Peers) GarbageCollect() {
	peers.Lock()
	var pending peersPendingNotifications
	defer peers.unlockAndNotify(&pending)

	peers.garbageCollect(&pending)
}

func (peers *Peers) garbageCollect(pending *peersPendingNotifications) {
	peers.ourself.RLock()
	_, reached := peers.ourself.routes(nil, false)
	peers.ourself.RUnlock()

	for name, peer := range peers.byName {
		if _, found := reached[peer.Name]; !found && peer.localRefCount == 0 {
			delete(peers.byName, name)
			peers.deleteByShortID(peer, pending)
			pending.removed = append(pending.removed, peer)
		}
	}

	if len(pending.removed) > 0 && peers.byShortID[peers.ourself.ShortID].peer != peers.ourself.Peer {
		// The local peer doesn't own its short ID. Garbage
		// collection might have freed some up, so try to
		// reassign.
		pending.reassignLocalShortID = true
	}
}

func (peers *Peers) decodeUpdate(update []byte) (newPeers map[PeerName]*Peer, decodedUpdate []*Peer, decodedConns [][]connectionSummary, err error) {
	newPeers = make(map[PeerName]*Peer)
	decodedUpdate = []*Peer{}
	decodedConns = [][]connectionSummary{}

	decoder := gob.NewDecoder(bytes.NewReader(update))

	for {
		summary, connSummaries, decErr := decodePeer(decoder)
		if decErr == io.EOF {
			break
		} else if decErr != nil {
			err = decErr
			return
		}
		newPeer := newPeerFromSummary(summary)
		decodedUpdate = append(decodedUpdate, newPeer)
		decodedConns = append(decodedConns, connSummaries)
		if _, found := peers.byName[newPeer.Name]; !found {
			newPeers[newPeer.Name] = newPeer
		}
	}

	for _, connSummaries := range decodedConns {
		for _, connSummary := range connSummaries {
			remoteName := PeerNameFromBin(connSummary.NameByte)
			if _, found := newPeers[remoteName]; found {
				continue
			}
			if _, found := peers.byName[remoteName]; found {
				continue
			}
			// Update refers to a peer which we have no knowledge of.
			newPeers[remoteName] = newPeerPlaceholder(remoteName)
		}
	}
	return
}

func (peers *Peers) applyDecodedUpdate(decodedUpdate []*Peer, decodedConns [][]connectionSummary, pending *peersPendingNotifications) peerNameSet {
	newUpdate := make(peerNameSet)
	for idx, newPeer := range decodedUpdate {
		connSummaries := decodedConns[idx]
		name := newPeer.Name
		// guaranteed to find peer in the peers.byName
		switch peer := peers.byName[name]; peer {
		case peers.ourself.Peer:
			if newPeer.UID != peer.UID {
				// The update contains information about an old
				// incarnation of ourselves. We increase our version
				// number beyond that which we received, so our
				// information supersedes the old one when it is
				// received by other peers.
				pending.localPeerModified = peers.ourself.setVersionBeyond(newPeer.Version)
			}
		case newPeer:
			peer.connections = makeConnsMap(peer, connSummaries, peers.byName)
			newUpdate[name] = struct{}{}
		default: // existing peer
			if newPeer.Version < peer.Version ||
				(newPeer.Version == peer.Version &&
					(newPeer.UID < peer.UID ||
						(newPeer.UID == peer.UID &&
							(!newPeer.HasShortID || peer.HasShortID)))) {
				continue
			}
			peer.Version = newPeer.Version
			peer.UID = newPeer.UID
			peer.NickName = newPeer.NickName
			peer.connections = makeConnsMap(peer, connSummaries, peers.byName)

			if newPeer.ShortID != peer.ShortID || newPeer.HasShortID != peer.HasShortID {
				peers.deleteByShortID(peer, pending)
				peer.ShortID = newPeer.ShortID
				peer.HasShortID = newPeer.HasShortID
				peers.addByShortID(peer, pending)
			}
			newUpdate[name] = struct{}{}
		}
	}
	return newUpdate
}

func (peer *Peer) encode(enc *gob.Encoder) {
	if err := enc.Encode(peer.peerSummary); err != nil {
		panic(err)
	}

	connSummaries := []connectionSummary{}
	for _, conn := range peer.connections {
		connSummaries = append(connSummaries, connectionSummary{
			conn.Remote().NameByte,
			conn.remoteTCPAddress(),
			conn.isOutbound(),
			conn.isEstablished(),
		})
	}

	if err := enc.Encode(connSummaries); err != nil {
		panic(err)
	}
}

func decodePeer(dec *gob.Decoder) (ps peerSummary, connSummaries []connectionSummary, err error) {
	if err = dec.Decode(&ps); err != nil {
		return
	}
	if err = dec.Decode(&connSummaries); err != nil {
		return
	}
	return
}

func makeConnsMap(peer *Peer, connSummaries []connectionSummary, byName map[PeerName]*Peer) map[PeerName]Connection {
	conns := make(map[PeerName]Connection)
	for _, connSummary := range connSummaries {
		name := PeerNameFromBin(connSummary.NameByte)
		remotePeer := byName[name]
		conn := newRemoteConnection(peer, remotePeer, connSummary.RemoteTCPAddr, connSummary.Outbound, connSummary.Established)
		conns[name] = conn
	}
	return conns
}

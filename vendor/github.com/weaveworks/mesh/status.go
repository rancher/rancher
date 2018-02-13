package mesh

import (
	"fmt"
	"net"
)

// Status is our current state as a peer, as taken from a router.
// This is designed to be used as diagnostic information.
type Status struct {
	Protocol           string
	ProtocolMinVersion int
	ProtocolMaxVersion int
	Encryption         bool
	PeerDiscovery      bool
	Name               string
	NickName           string
	Port               int
	Peers              []PeerStatus
	UnicastRoutes      []unicastRouteStatus
	BroadcastRoutes    []broadcastRouteStatus
	Connections        []LocalConnectionStatus
	TerminationCount   int
	Targets            []string
	OverlayDiagnostics interface{}
	TrustedSubnets     []string
}

// NewStatus returns a Status object, taken as a snapshot from the router.
func NewStatus(router *Router) *Status {
	return &Status{
		Protocol:           Protocol,
		ProtocolMinVersion: ProtocolMinVersion,
		ProtocolMaxVersion: ProtocolMaxVersion,
		Encryption:         router.usingPassword(),
		PeerDiscovery:      router.PeerDiscovery,
		Name:               router.Ourself.Name.String(),
		NickName:           router.Ourself.NickName,
		Port:               router.Port,
		Peers:              makePeerStatusSlice(router.Peers),
		UnicastRoutes:      makeUnicastRouteStatusSlice(router.Routes),
		BroadcastRoutes:    makeBroadcastRouteStatusSlice(router.Routes),
		Connections:        makeLocalConnectionStatusSlice(router.ConnectionMaker),
		TerminationCount:   router.ConnectionMaker.terminationCount,
		Targets:            router.ConnectionMaker.Targets(false),
		OverlayDiagnostics: router.Overlay.Diagnostics(),
		TrustedSubnets:     makeTrustedSubnetsSlice(router.TrustedSubnets),
	}
}

// PeerStatus is the current state of a peer in the mesh.
type PeerStatus struct {
	Name        string
	NickName    string
	UID         PeerUID
	ShortID     PeerShortID
	Version     uint64
	Connections []connectionStatus
}

// makePeerStatusSlice takes a snapshot of the state of peers.
func makePeerStatusSlice(peers *Peers) []PeerStatus {
	var slice []PeerStatus

	peers.forEach(func(peer *Peer) {
		var connections []connectionStatus
		if peer == peers.ourself.Peer {
			for conn := range peers.ourself.getConnections() {
				connections = append(connections, makeConnectionStatus(conn))
			}
		} else {
			// Modifying peer.connections requires a write lock on
			// Peers, and since we are holding a read lock (due to the
			// ForEach), access without locking the peer is safe.
			for _, conn := range peer.connections {
				connections = append(connections, makeConnectionStatus(conn))
			}
		}
		slice = append(slice, PeerStatus{
			peer.Name.String(),
			peer.NickName,
			peer.UID,
			peer.ShortID,
			peer.Version,
			connections,
		})
	})

	return slice
}

type connectionStatus struct {
	Name        string
	NickName    string
	Address     string
	Outbound    bool
	Established bool
}

func makeConnectionStatus(c Connection) connectionStatus {
	return connectionStatus{
		Name:        c.Remote().Name.String(),
		NickName:    c.Remote().NickName,
		Address:     c.remoteTCPAddress(),
		Outbound:    c.isOutbound(),
		Established: c.isEstablished(),
	}
}

// unicastRouteStatus is the current state of an established unicast route.
type unicastRouteStatus struct {
	Dest, Via string
}

// makeUnicastRouteStatusSlice takes a snapshot of the unicast routes in routes.
func makeUnicastRouteStatusSlice(r *routes) []unicastRouteStatus {
	r.RLock()
	defer r.RUnlock()

	var slice []unicastRouteStatus
	for dest, via := range r.unicast {
		slice = append(slice, unicastRouteStatus{dest.String(), via.String()})
	}
	return slice
}

// BroadcastRouteStatus is the current state of an established broadcast route.
type broadcastRouteStatus struct {
	Source string
	Via    []string
}

// makeBroadcastRouteStatusSlice takes a snapshot of the broadcast routes in routes.
func makeBroadcastRouteStatusSlice(r *routes) []broadcastRouteStatus {
	r.RLock()
	defer r.RUnlock()

	var slice []broadcastRouteStatus
	for source, via := range r.broadcast {
		var hops []string
		for _, hop := range via {
			hops = append(hops, hop.String())
		}
		slice = append(slice, broadcastRouteStatus{source.String(), hops})
	}
	return slice
}

// LocalConnectionStatus is the current state of a physical connection to a peer.
type LocalConnectionStatus struct {
	Address  string
	Outbound bool
	State    string
	Info     string
	Attrs    map[string]interface{}
}

// makeLocalConnectionStatusSlice takes a snapshot of the active local
// connections in the ConnectionMaker.
func makeLocalConnectionStatusSlice(cm *connectionMaker) []LocalConnectionStatus {
	resultChan := make(chan []LocalConnectionStatus)
	cm.actionChan <- func() bool {
		var slice []LocalConnectionStatus
		for conn := range cm.connections {
			state := "pending"
			if conn.isEstablished() {
				state = "established"
			}
			lc, _ := conn.(*LocalConnection)
			attrs := lc.OverlayConn.Attrs()
			name, ok := attrs["name"]
			if !ok {
				name = "none"
			}
			info := fmt.Sprintf("%-6v %v", name, conn.Remote())
			if lc.router.usingPassword() {
				if lc.untrusted() {
					info = fmt.Sprintf("%-11v %v", "encrypted", info)
					if attrs != nil {
						attrs["encrypted"] = true
					}
				} else {
					info = fmt.Sprintf("%-11v %v", "unencrypted", info)
				}
			}
			slice = append(slice, LocalConnectionStatus{conn.remoteTCPAddress(), conn.isOutbound(), state, info, attrs})
		}
		for address, target := range cm.targets {
			add := func(state, info string) {
				slice = append(slice, LocalConnectionStatus{address, true, state, info, nil})
			}
			switch target.state {
			case targetWaiting:
				until := "never"
				if !target.tryAfter.IsZero() {
					until = target.tryAfter.String()
				}
				if target.lastError == nil { // shouldn't happen
					add("waiting", "until: "+until)
				} else {
					add("failed", target.lastError.Error()+", retry: "+until)
				}
			case targetAttempting:
				if target.lastError == nil {
					add("connecting", "")
				} else {
					add("retrying", target.lastError.Error())
				}
			case targetConnected:
			case targetSuspended:
			}
		}
		resultChan <- slice
		return false
	}
	return <-resultChan
}

// makeTrustedSubnetsSlice makes a human-readable copy of the trustedSubnets.
func makeTrustedSubnetsSlice(trustedSubnets []*net.IPNet) []string {
	trustedSubnetStrs := []string{}
	for _, trustedSubnet := range trustedSubnets {
		trustedSubnetStrs = append(trustedSubnetStrs, trustedSubnet.String())
	}
	return trustedSubnetStrs
}

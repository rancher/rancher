// +build peer_name_mac !peer_name_alternative

package mesh

// The !peer_name_alternative effectively makes this the default,
// i.e. to choose an alternative, run
//
//   go build -tags 'peer_name_alternative peer_name_hash'
//
// Let peer names be MACs...
//
// MACs need to be unique across our network, or bad things will
// happen anyway. So they make pretty good candidates for peer
// names. And doing so is pretty efficient both computationally and
// network overhead wise.
//
// Note that we do not mandate *what* MAC should be used as the peer
// name. In particular it doesn't actually have to be the MAC of, say,
// the network interface the peer is sniffing on.

import (
	"fmt"
	"net"
)

// PeerName is used as a map key. Since net.HardwareAddr isn't suitable for
// that - it's a slice, and slices can't be map keys - we convert that to/from
// uint64.
type PeerName uint64

const (
	// PeerNameFlavour is the type of peer names we use.
	PeerNameFlavour = "mac"

	// NameSize is the number of bytes in a peer name.
	NameSize = 6

	// UnknownPeerName is used as a sentinel value.
	UnknownPeerName = PeerName(0)
)

// PeerNameFromUserInput parses PeerName from a user-provided string.
func PeerNameFromUserInput(userInput string) (PeerName, error) {
	return PeerNameFromString(userInput)
}

// PeerNameFromString parses PeerName from a generic string.
func PeerNameFromString(nameStr string) (PeerName, error) {
	var a, b, c, d, e, f uint64

	match := func(format string, args ...interface{}) bool {
		a, b, c, d, e, f = 0, 0, 0, 0, 0, 0
		n, err := fmt.Sscanf(nameStr+"\000", format+"\000", args...)
		return err == nil && n == len(args)
	}

	switch {
	case match("%2x:%2x:%2x:%2x:%2x:%2x", &a, &b, &c, &d, &e, &f):
	case match("::%2x:%2x:%2x:%2x", &c, &d, &e, &f):
	case match("%2x::%2x:%2x:%2x", &a, &d, &e, &f):
	case match("%2x:%2x::%2x:%2x", &a, &b, &e, &f):
	case match("%2x:%2x:%2x::%2x", &a, &b, &c, &f):
	case match("%2x:%2x:%2x:%2x::", &a, &b, &c, &d):
	case match("::%2x:%2x:%2x", &d, &e, &f):
	case match("%2x::%2x:%2x", &a, &e, &f):
	case match("%2x:%2x::%2x", &a, &b, &f):
	case match("%2x:%2x:%2x::", &a, &b, &c):
	case match("::%2x:%2x", &e, &f):
	case match("%2x::%2x", &a, &f):
	case match("%2x:%2x::", &a, &b):
	case match("::%2x", &f):
	case match("%2x::", &a):
	default:
		return UnknownPeerName, fmt.Errorf("invalid peer name format: %q", nameStr)
	}

	return PeerName(a<<40 | b<<32 | c<<24 | d<<16 | e<<8 | f), nil
}

// PeerNameFromBin parses PeerName from a byte slice.
func PeerNameFromBin(nameByte []byte) PeerName {
	return PeerName(macint(net.HardwareAddr(nameByte)))
}

// bytes encodes PeerName as a byte slice.
func (name PeerName) bytes() []byte {
	return intmac(uint64(name))
}

// String encodes PeerName as a string.
func (name PeerName) String() string {
	return intmac(uint64(name)).String()
}

func macint(mac net.HardwareAddr) (r uint64) {
	for _, b := range mac {
		r <<= 8
		r |= uint64(b)
	}
	return
}

func intmac(key uint64) (r net.HardwareAddr) {
	r = make([]byte, 6)
	for i := 5; i >= 0; i-- {
		r[i] = byte(key)
		key >>= 8
	}
	return
}

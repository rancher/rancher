package ntlm

import (
	"encoding/binary"
	"fmt"
)

// AV_PAIR identifiers, MS-NLMP 2.2.2.1.
const (
	avEOL             uint16 = 0x0000
	avNbComputerName  uint16 = 0x0001
	avNbDomainName    uint16 = 0x0002
	avDNSComputerName uint16 = 0x0003
	avDNSDomainName   uint16 = 0x0004
	avDNSTreeName     uint16 = 0x0005
	avFlags           uint16 = 0x0006
	avTimestamp       uint16 = 0x0007
	avSingleHost      uint16 = 0x0008
	avTargetName      uint16 = 0x0009
	avChannelBindings uint16 = 0x000A
)

// avFlagMICPresent is the MsvAvFlags bit stating the AUTHENTICATE message
// carries a MIC. Bit 0x00000004 (the client supplied an unverified target
// name) is deliberately never set: no SPN is supplied at all.
const avFlagMICPresent uint32 = 0x00000002

// avPair is one AV_PAIR from a TargetInfo buffer. The terminating MsvAvEOL is
// not represented; it is implied by the buffer and re-emitted on serialize.
type avPair struct {
	ID    uint16
	Value []byte
}

// securitySensitiveAVIDs may appear at most once in a server's TargetInfo. A
// duplicate would let a parser that keeps the first occurrence and one that
// keeps the last disagree about the channel binding in force.
var securitySensitiveAVIDs = map[uint16]bool{
	avFlags:           true,
	avTimestamp:       true,
	avTargetName:      true,
	avChannelBindings: true,
}

// parseTargetInfo decodes an AV_PAIR sequence, requiring it to end in exactly
// one MsvAvEOL with nothing after it.
func parseTargetInfo(buf []byte) ([]avPair, error) {
	pairs := []avPair{}
	seen := map[uint16]bool{}

	for offset := 0; ; {
		if len(buf)-offset < 4 {
			return nil, fmt.Errorf("ntlm: target info ends after %d bytes without an MsvAvEOL", offset)
		}

		id := binary.LittleEndian.Uint16(buf[offset : offset+2])
		length := int(binary.LittleEndian.Uint16(buf[offset+2 : offset+4]))
		offset += 4

		if id == avEOL {
			if length != 0 {
				return nil, fmt.Errorf("ntlm: MsvAvEOL declares a %d byte value", length)
			}
			if offset != len(buf) {
				return nil, fmt.Errorf("ntlm: %d bytes follow the MsvAvEOL", len(buf)-offset)
			}
			return pairs, nil
		}

		if length > len(buf)-offset {
			return nil, fmt.Errorf("ntlm: AV pair 0x%04x declares %d bytes but only %d remain", id, length, len(buf)-offset)
		}
		if securitySensitiveAVIDs[id] && seen[id] {
			return nil, fmt.Errorf("ntlm: AV pair 0x%04x appears more than once", id)
		}
		seen[id] = true

		value := make([]byte, length)
		copy(value, buf[offset:offset+length])
		pairs = append(pairs, avPair{ID: id, Value: value})
		offset += length
	}
}

// serializeTargetInfo encodes pairs followed by exactly one MsvAvEOL.
// An oversized value is an error, not a truncation (see putVarField).
func serializeTargetInfo(pairs []avPair) ([]byte, error) {
	size := 4
	for _, p := range pairs {
		if len(p.Value) > maxVarFieldLen {
			return nil, fmt.Errorf("ntlm: AV pair 0x%04x value of %d bytes exceeds the %d byte limit", p.ID, len(p.Value), maxVarFieldLen)
		}
		size += 4 + len(p.Value)
	}

	out := make([]byte, 0, size)
	for _, p := range pairs {
		out = binary.LittleEndian.AppendUint16(out, p.ID)
		out = binary.LittleEndian.AppendUint16(out, uint16(len(p.Value)))
		out = append(out, p.Value...)
	}
	return append(out, 0x00, 0x00, 0x00, 0x00), nil
}

// findAVPair returns the value of the first pair with the given id.
func findAVPair(pairs []avPair, id uint16) ([]byte, bool) {
	for _, p := range pairs {
		if p.ID == id {
			return p.Value, true
		}
	}
	return nil, false
}

// upsertAVPair replaces the value of an existing pair, preserving its
// position, or appends a new one.
func upsertAVPair(pairs []avPair, id uint16, value []byte) []avPair {
	for i := range pairs {
		if pairs[i].ID == id {
			pairs[i].Value = value
			return pairs
		}
	}
	return append(pairs, avPair{ID: id, Value: value})
}

// transformTargetInfo returns the TargetInfo the client sends back inside its
// NTLMv2 response: the server's pairs, preserved in order, with the channel
// binding token, an empty target name and the MsvAvFlags value upserted.
//
// It parses and re-serializes rather than appending. A well-formed TargetInfo
// ends in an MsvAvEOL, and a pair written after it is invisible to any parser
// that stops there, which would produce a bind carrying no effective channel
// binding at all.
//
// The empty MsvAvTargetName is the MS-NLMP 3.1.5.1.2 ClientSuppliedTargetName
// == NULL branch. It is not the same as omitting the pair.
// serverPairs must be the already-parsed server TargetInfo. It is passed in
// rather than re-parsed so there is exactly one parse of the buffer per
// exchange and no way for two parses to disagree.
func transformTargetInfo(serverPairs []avPair, cbt [16]byte, withMIC bool) ([]byte, error) {
	pairs := make([]avPair, len(serverPairs))
	copy(pairs, serverPairs)

	var flags uint32
	if existing, ok := findAVPair(pairs, avFlags); ok {
		if len(existing) != 4 {
			return nil, fmt.Errorf("ntlm: MsvAvFlags has %d bytes, expected 4", len(existing))
		}
		flags = binary.LittleEndian.Uint32(existing)
	}
	if withMIC {
		flags |= avFlagMICPresent
	}

	encodedFlags := make([]byte, 4)
	binary.LittleEndian.PutUint32(encodedFlags, flags)

	pairs = upsertAVPair(pairs, avFlags, encodedFlags)
	pairs = upsertAVPair(pairs, avTargetName, []byte{})
	pairs = upsertAVPair(pairs, avChannelBindings, cbt[:])

	return serializeTargetInfo(pairs)
}

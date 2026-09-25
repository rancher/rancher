package ntlm

import "encoding/binary"

// buildNegotiate returns a NEGOTIATE_MESSAGE (MS-NLMP 2.2.1.1) with empty
// domain and workstation buffers.
//
// The domain is deliberately not carried here. NEGOTIATE encodes it in the OEM
// character set, which would need a codepage conversion for non-ASCII domains,
// and the domain the server actually authenticates against is the Unicode one
// in AUTHENTICATE. go-ldap always passes an empty workstation.
func buildNegotiate(l layout) []byte {
	size := l.negotiateHeaderSize()
	msg := make([]byte, size)

	copy(msg[0:8], ntlmSignature)
	binary.LittleEndian.PutUint32(msg[8:12], messageTypeNegotiate)
	binary.LittleEndian.PutUint32(msg[12:16], l.flags())

	// Both payloads are empty, so the descriptors cannot exceed the security
	// buffer limit and the errors are unreachable.
	_ = putVarField(msg[16:24], 0, size, "negotiate domain")
	_ = putVarField(msg[24:32], 0, size, "negotiate workstation")

	if l.version {
		copy(msg[32:40], versionStructure[:])
	}

	return msg
}

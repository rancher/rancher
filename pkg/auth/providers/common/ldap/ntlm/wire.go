package ntlm

import (
	"encoding/binary"
	"fmt"
	"time"
	"unicode/utf16"
)

const (
	ntlmSignature = "NTLMSSP\x00"

	messageTypeNegotiate    = 1
	messageTypeChallenge    = 2
	messageTypeAuthenticate = 3

	// maxVarFieldLen is the largest payload an MS-NLMP security buffer can
	// describe. The length is a 16-bit field, so anything larger cannot be
	// represented and must be an error rather than a silent truncation.
	maxVarFieldLen = 65535
)

// MS-NLMP 2.2.2.5 NEGOTIATE flags.
const (
	flagUnicode                 uint32 = 0x00000001
	flagRequestTarget           uint32 = 0x00000004
	flagNegotiateSign           uint32 = 0x00000010
	flagNegotiateSeal           uint32 = 0x00000020
	flagLMKey                   uint32 = 0x00000080
	flagNTLM                    uint32 = 0x00000200
	flagAlwaysSign              uint32 = 0x00008000
	flagExtendedSessionSecurity uint32 = 0x00080000
	flagTargetInfo              uint32 = 0x00800000
	flagVersion                 uint32 = 0x02000000
	flagKeyExch                 uint32 = 0x40000000
)

// baseFlags is the set Rancher offers regardless of layout. Signing and
// sealing are omitted because the exchange always runs inside TLS, and key
// exchange is omitted so the exported session key is the session base key.
const baseFlags = flagUnicode |
	flagRequestTarget |
	flagNTLM |
	flagAlwaysSign |
	flagExtendedSessionSecurity |
	flagTargetInfo

// requiredChallengeFlags are the flags a CHALLENGE must return for this
// package to answer it at all.
//
// REQUEST_TARGET is deliberately absent. MS-NLMP 2.2.2.5 defines it as a
// request for the CHALLENGE TargetName; the channel binding token travels in
// TargetInfo, which NEGOTIATE_TARGET_INFO governs. Refusing a challenge for
// withholding REQUEST_TARGET would reject a server that is perfectly capable
// of the bind. It stays in baseFlags as an offer and is simply not asserted
// back if the server does not return it.
const requiredChallengeFlags = flagUnicode |
	flagNTLM |
	flagExtendedSessionSecurity |
	flagTargetInfo

func (l layout) flags() uint32 {
	if l.version {
		return baseFlags | flagVersion
	}
	return baseFlags
}

// versionStructure is the MS-NLMP 2.2.2.10 VERSION field: Windows 6.1 build
// 7601, NTLM revision 15. It is documented as debug information only. It is
// sent because the AUTHENTICATE header is positional, and a MIC is only found
// at its canonical offset when the version field precedes it.
var versionStructure = [8]byte{0x06, 0x01, 0xB1, 0x1D, 0x00, 0x00, 0x00, 0x0F}

// utf16LE encodes s as UTF-16 little-endian, the representation MS-NLMP uses
// for every string field once NTLMSSP_NEGOTIATE_UNICODE is set.
func utf16LE(s string) []byte {
	codes := utf16.Encode([]rune(s))
	out := make([]byte, 0, len(codes)*2)
	for _, c := range codes {
		out = binary.LittleEndian.AppendUint16(out, c)
	}
	return out
}

// fileTime converts t to a Windows FILETIME: 100-nanosecond intervals since
// 1601-01-01 UTC.
func fileTime(t time.Time) uint64 {
	const unixToFiletimeSeconds = 11644473600
	t = t.UTC()
	seconds := t.Unix()
	nanos := int64(t.Nanosecond())
	intervals := (seconds+unixToFiletimeSeconds)*10000000 + nanos/100
	return uint64(intervals)
}

// putVarField writes an MS-NLMP security buffer descriptor (length, maximum
// length, payload offset) at dst.
//
// It errors rather than truncating. The length field is 16 bits while the
// payloads it describes are built from server-supplied data, so an unchecked
// cast would silently emit a message whose descriptors disagree with its
// bytes — rejected by the domain controller as bad credentials, with nothing
// on the client indicating why.
func putVarField(dst []byte, length, offset int, name string) error {
	if length < 0 || length > maxVarFieldLen {
		return fmt.Errorf("ntlm: %s payload of %d bytes exceeds the %d byte security buffer limit", name, length, maxVarFieldLen)
	}
	binary.LittleEndian.PutUint16(dst[0:2], uint16(length))
	binary.LittleEndian.PutUint16(dst[2:4], uint16(length))
	binary.LittleEndian.PutUint32(dst[4:8], uint32(offset))
	return nil
}

// layout selects which optional AUTHENTICATE header fields are present. The
// header is positional, so the two flags move every payload offset and the MIC
// with them.
//
// The shipping configuration is both true. The other three combinations exist
// only so Task 8A can measure which one a real domain controller accepts; see
// the deviation note at the top of the plan.
type layout struct {
	version bool
	mic     bool
}

func (l layout) negotiateHeaderSize() int {
	if l.version {
		return 40
	}
	return 32
}

// micOffset is where the MIC begins, immediately after the version field when
// one is present. Only meaningful when l.mic is set.
func (l layout) micOffset() int {
	if l.version {
		return 72
	}
	return 64
}

func (l layout) authenticateHeaderSize() int {
	size := 64
	if l.version {
		size += 8
	}
	if l.mic {
		size += 16
	}
	return size
}

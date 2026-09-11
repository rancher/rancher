package ntlm

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUTF16LE(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []byte{'A', 0x00, 'B', 0x00}, utf16LE("AB"))
	assert.Equal(t, []byte{}, utf16LE(""))
	// U+00E9 LATIN SMALL LETTER E WITH ACUTE encodes as 0xE9 0x00.
	assert.Equal(t, []byte{0xE9, 0x00}, utf16LE("é"))
	// U+1F600 is outside the BMP and encodes as a surrogate pair.
	assert.Equal(t, []byte{0x3D, 0xD8, 0x00, 0xDE}, utf16LE("\U0001F600"))
}

func TestFileTime(t *testing.T) {
	t.Parallel()

	// The Windows epoch itself is zero 100ns intervals.
	assert.Equal(t, uint64(0), fileTime(time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)))
	// The Unix epoch is 11644473600 seconds later.
	assert.Equal(t, uint64(116444736000000000), fileTime(time.Unix(0, 0).UTC()))
}

func TestBuildNegotiate(t *testing.T) {
	t.Parallel()

	l := layout{version: true, mic: true}
	msg := buildNegotiate(l)

	require.Len(t, msg, l.negotiateHeaderSize(), "no payload is emitted, so the message is exactly the header")
	assert.Equal(t, ntlmSignature, string(msg[0:8]))
	assert.Equal(t, uint32(messageTypeNegotiate), binary.LittleEndian.Uint32(msg[8:12]))
	assert.Equal(t, l.flags(), binary.LittleEndian.Uint32(msg[12:16]))

	// Domain and workstation are empty security buffers whose offsets point at
	// the end of the message.
	for name, off := range map[string]int{"domain": 16, "workstation": 24} {
		assert.Equalf(t, uint16(0), binary.LittleEndian.Uint16(msg[off:off+2]), "%s length", name)
		assert.Equalf(t, uint16(0), binary.LittleEndian.Uint16(msg[off+2:off+4]), "%s max length", name)
		assert.Equalf(t, uint32(l.negotiateHeaderSize()), binary.LittleEndian.Uint32(msg[off+4:off+8]), "%s offset", name)
	}

	assert.Equal(t, versionStructure[:], msg[32:40])
}

func TestNegotiateFlagsValue(t *testing.T) {
	t.Parallel()

	// UNICODE | REQUEST_TARGET | NTLM | ALWAYS_SIGN | EXTENDED_SESSIONSECURITY
	// | TARGET_INFO | VERSION. Pinned so a flag change is a deliberate edit.
	shipping := layout{version: true, mic: true}
	assert.Equal(t, uint32(0x02888205), shipping.flags())
	assert.Equal(t, uint32(0x00888205), layout{}.flags(), "without VERSION")

	assert.Zero(t, baseFlags&flagKeyExch, "key exchange is not negotiated")
	assert.Zero(t, baseFlags&flagNegotiateSign, "TLS provides integrity; NTLM signing is not negotiated")
	assert.Zero(t, baseFlags&flagNegotiateSeal, "TLS provides confidentiality; NTLM sealing is not negotiated")

	// Nothing may be required that is not offered.
	assert.Zero(t, requiredChallengeFlags&^baseFlags, "required flags must be a subset of what is offered")

	// ALWAYS_SIGN and REQUEST_TARGET are offered but optional: neither is
	// needed to carry a channel binding token, so a server withholding them is
	// still usable. They are simply not asserted back in AUTHENTICATE.
	assert.Equal(t, flagAlwaysSign|flagRequestTarget, baseFlags&^requiredChallengeFlags)
}

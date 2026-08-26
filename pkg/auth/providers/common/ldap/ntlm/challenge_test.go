package ntlm

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTestChallenge assembles a CHALLENGE_MESSAGE: a 48-byte header followed
// by the target info payload. Payload offsets are absolute, so the header is
// laid out the same whether or not the server sent a version field.
func buildTestChallenge(flags uint32, serverChallenge [8]byte, targetInfo []byte) []byte {
	return buildTestChallengeWithDescriptor(flags, serverChallenge, targetInfo, len(targetInfo))
}

// buildTestChallengeWithDescriptor is the same, but writes declaredLen into the
// TargetInfo descriptor instead of the real payload length. Only the
// bounds-checking tests need the two to differ.
func buildTestChallengeWithDescriptor(flags uint32, serverChallenge [8]byte, targetInfo []byte, declaredLen int) []byte {
	msg := make([]byte, 48)
	copy(msg[0:8], ntlmSignature)
	binary.LittleEndian.PutUint32(msg[8:12], messageTypeChallenge)
	// TargetNameFields, empty.
	if err := putVarField(msg[12:20], 0, 48, "target name"); err != nil {
		panic(err)
	}
	binary.LittleEndian.PutUint32(msg[20:24], flags)
	copy(msg[24:32], serverChallenge[:])
	if err := putVarField(msg[40:48], declaredLen, 48, "target info"); err != nil {
		panic(err)
	}
	return append(msg, targetInfo...)
}

// testChallengeFlags is what a compliant server returns: everything in
// requiredChallengeFlags plus the optional flags Rancher offers. It must be a
// superset of requiredChallengeFlags or every nominal test rejects its own
// fixture.
func testChallengeFlags() uint32 {
	return baseFlags | flagVersion
}

func TestParseChallenge(t *testing.T) {
	t.Parallel()

	serverChallenge := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	targetInfo := []byte{0x00, 0x00, 0x00, 0x00} // MsvAvEOL only.

	got, err := parseChallenge(buildTestChallenge(testChallengeFlags(), serverChallenge, targetInfo))
	require.NoError(t, err)

	assert.Equal(t, serverChallenge, got.ServerChallenge)
	assert.Equal(t, testChallengeFlags(), got.Flags)
	assert.Equal(t, targetInfo, got.TargetInfo)
}

func TestParseChallengeRejects(t *testing.T) {
	t.Parallel()

	serverChallenge := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	eol := []byte{0x00, 0x00, 0x00, 0x00}
	valid := buildTestChallenge(testChallengeFlags(), serverChallenge, eol)

	corrupt := func(mutate func([]byte) []byte) []byte {
		clone := make([]byte, len(valid))
		copy(clone, valid)
		return mutate(clone)
	}

	tests := []struct {
		name string
		msg  []byte
	}{
		{name: "empty", msg: nil},
		{name: "truncated header", msg: valid[:40]},
		{
			name: "bad signature",
			msg:  corrupt(func(b []byte) []byte { b[0] = 'X'; return b }),
		},
		{
			name: "wrong message type",
			msg: corrupt(func(b []byte) []byte {
				binary.LittleEndian.PutUint32(b[8:12], messageTypeNegotiate)
				return b
			}),
		},
		{
			name: "target info offset past end",
			msg: corrupt(func(b []byte) []byte {
				binary.LittleEndian.PutUint32(b[44:48], 4096)
				return b
			}),
		},
		{
			name: "target info length past end",
			msg: corrupt(func(b []byte) []byte {
				binary.LittleEndian.PutUint16(b[40:42], 4096)
				return b
			}),
		},
		{
			name: "offset plus length overflows",
			msg: corrupt(func(b []byte) []byte {
				binary.LittleEndian.PutUint32(b[44:48], ^uint32(0))
				binary.LittleEndian.PutUint16(b[40:42], 8)
				return b
			}),
		},
		{
			name: "no target info flag",
			msg:  buildTestChallenge(testChallengeFlags()&^flagTargetInfo, serverChallenge, eol),
		},
		{
			name: "no extended session security flag",
			msg:  buildTestChallenge(testChallengeFlags()&^flagExtendedSessionSecurity, serverChallenge, eol),
		},
		{
			name: "no unicode flag",
			msg:  buildTestChallenge(testChallengeFlags()&^flagUnicode, serverChallenge, eol),
		},
		{
			name: "no ntlm flag",
			msg:  buildTestChallenge(testChallengeFlags()&^flagNTLM, serverChallenge, eol),
		},
		{
			name: "server demands key exchange",
			msg:  buildTestChallenge(testChallengeFlags()|flagKeyExch, serverChallenge, eol),
		},
		{
			name: "server demands lm key",
			msg:  buildTestChallenge(testChallengeFlags()|flagLMKey, serverChallenge, eol),
		},
		{name: "oversized", msg: make([]byte, maxNTLMChallengeSize+1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseChallenge(test.msg)
			require.Error(t, err)
		})
	}
}

func TestParseChallengeAcceptsExactlyMaximumSize(t *testing.T) {
	t.Parallel()

	// A message landing exactly on the cap, built from a legal descriptor.
	//
	// A TargetInfo descriptor is a 16-bit length, so it can only ever describe
	// 65 535 bytes; the cap is far above that. The message therefore reaches
	// maxNTLMChallengeSize as a maximum-length descriptor followed by trailing
	// bytes the descriptor does not cover. That is a legal NTLM message —
	// security buffer offsets are 32-bit and payloads may leave gaps — and it
	// is what lets the size limit be tested at all.
	//
	// This exercises the size cap and the descriptor bounds check. It does not
	// exercise AV-pair handling; parseChallenge never parses TargetInfo, and
	// the serializer's own length limit is covered separately.
	const declared = maxVarFieldLen

	targetInfo := make([]byte, 0, declared)
	targetInfo = binary.LittleEndian.AppendUint16(targetInfo, 0x000A) // unknown AV id
	targetInfo = binary.LittleEndian.AppendUint16(targetInfo, uint16(declared-8))
	targetInfo = append(targetInfo, make([]byte, declared-8)...)
	targetInfo = append(targetInfo, 0x00, 0x00, 0x00, 0x00) // MsvAvEOL
	require.Len(t, targetInfo, declared)

	// Pad the message out to exactly the cap. These bytes sit past the end of
	// the TargetInfo payload and are referenced by nothing.
	trailing := maxNTLMChallengeSize - 48 - declared
	require.Positive(t, trailing)

	msg := buildTestChallenge(testChallengeFlags(), [8]byte{}, targetInfo)
	msg = append(msg, make([]byte, trailing)...)
	require.Len(t, msg, maxNTLMChallengeSize)

	parsed, err := parseChallenge(msg)
	require.NoError(t, err)
	require.Len(t, parsed.TargetInfo, declared, "the descriptor bounds the payload, not the message")

	_, err = parseChallenge(append(msg, 0x00))
	require.Error(t, err, "one byte over the cap is refused")
}

func TestParseChallengeAcceptsWithoutOptionalFlags(t *testing.T) {
	t.Parallel()

	// REQUEST_TARGET and ALWAYS_SIGN are offered but not needed to carry a
	// channel binding token, so withholding them must not fail the exchange.
	for _, optional := range []struct {
		name string
		flag uint32
	}{
		{name: "request target", flag: flagRequestTarget},
		{name: "always sign", flag: flagAlwaysSign},
	} {
		t.Run(optional.name, func(t *testing.T) {
			t.Parallel()

			msg := buildTestChallenge(testChallengeFlags()&^optional.flag, [8]byte{1}, eolTerminated())

			parsed, err := parseChallenge(msg)
			require.NoError(t, err)
			assert.Zero(t, parsed.Flags&optional.flag)

			// And the client does not assert it back.
			assert.Zero(t, authenticateFlags(layout{version: true, mic: true}, parsed.Flags)&optional.flag)
		})
	}
}

func TestEffectiveLayoutRequiresNegotiatedVersion(t *testing.T) {
	t.Parallel()

	shipping := layout{version: true, mic: true}

	got, err := effectiveLayout(shipping, testChallengeFlags())
	require.NoError(t, err)
	assert.Equal(t, shipping, got)

	// A version-enabled layout against a server that withheld the flag fails
	// loudly. Emitting the field anyway would put the header and the flag word
	// in disagreement and relocate the MIC without saying so.
	_, err = effectiveLayout(shipping, testChallengeFlags()&^flagVersion)
	require.Error(t, err)

	// A version-disabled layout does not care either way.
	noVersion := layout{mic: true}
	got, err = effectiveLayout(noVersion, testChallengeFlags()&^flagVersion)
	require.NoError(t, err)
	assert.Equal(t, noVersion, got)
}

func TestBuildTestChallengeRejectsAnOversizedDescriptor(t *testing.T) {
	t.Parallel()

	// Guards the fixture builder itself: putVarField refuses a length a 16-bit
	// field cannot hold, so a future test cannot accidentally reintroduce a
	// truncated descriptor.
	assert.Panics(t, func() {
		buildTestChallengeWithDescriptor(testChallengeFlags(), [8]byte{}, nil, maxVarFieldLen+1)
	})
}

func FuzzParseChallenge(f *testing.F) {
	f.Add(buildTestChallenge(testChallengeFlags(), [8]byte{1}, []byte{0, 0, 0, 0}))
	f.Add([]byte(ntlmSignature))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, msg []byte) {
		// The contract is only that it never panics and never returns both a
		// nil error and a nil result.
		got, err := parseChallenge(msg)
		if err == nil && got == nil {
			t.Fatal("parseChallenge returned no result and no error")
		}
	})
}

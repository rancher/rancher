package ntlm

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCBT is a non-zero placeholder token. NewNegotiator rejects the all-zero
// value, so no test other than TestNewNegotiatorRejectsTheZeroToken may
// construct a negotiator with it.
var testCBT = [16]byte{0xCC, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}

func testNegotiator(t *testing.T, cbt [16]byte, extra ...Option) *Negotiator {
	t.Helper()

	require.NotEqual(t, [16]byte{}, cbt, "the constructor rejects the zero token; use testCBT")

	options := append([]Option{
		WithClock(func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) }),
		WithNonceSource(func(b []byte) error {
			for i := range b {
				b[i] = 0xAA
			}
			return nil
		}),
	}, extra...)

	n, err := NewNegotiator(cbt, options...)
	require.NoError(t, err)
	return n
}

func TestNegotiatorHappyPath(t *testing.T) {
	t.Parallel()

	n := testNegotiator(t, [16]byte{0xCC})

	negotiate, err := n.Negotiate(specDomain, "")
	require.NoError(t, err)
	assert.Equal(t, buildNegotiate(layout{version: true, mic: true}), negotiate)

	challengeBytes := buildTestChallenge(testChallengeFlags(), [8]byte{1, 2, 3, 4, 5, 6, 7, 8}, eolTerminated())

	authenticate, err := n.ChallengeResponse(challengeBytes, specUser, specNTHash)
	require.NoError(t, err)
	shipping := layout{version: true, mic: true}
	require.GreaterOrEqual(t, len(authenticate), shipping.authenticateHeaderSize())
	assert.Equal(t, uint32(messageTypeAuthenticate), binary.LittleEndian.Uint32(authenticate[8:12]))
	assert.NotEqual(t, make([]byte, 16), authenticate[shipping.micOffset():shipping.micOffset()+16], "the MIC is filled in")

	domain, err := readVarField(authenticate, 28, "domain")
	require.NoError(t, err)
	assert.Equal(t, utf16LE(specDomain), domain, "the domain from Negotiate is used")
}

func TestNegotiatorReturnsACopyOfTheNegotiateBytes(t *testing.T) {
	t.Parallel()

	n := testNegotiator(t, testCBT)

	negotiate, err := n.Negotiate(specDomain, "")
	require.NoError(t, err)

	// A caller mutating the returned slice must not change what the MIC covers.
	negotiate[0] = 'X'

	challengeBytes := buildTestChallenge(testChallengeFlags(), [8]byte{1}, eolTerminated())
	authenticate, err := n.ChallengeResponse(challengeBytes, specUser, specNTHash)
	require.NoError(t, err)

	fresh := testNegotiator(t, testCBT)
	_, err = fresh.Negotiate(specDomain, "")
	require.NoError(t, err)
	expected, err := fresh.ChallengeResponse(challengeBytes, specUser, specNTHash)
	require.NoError(t, err)

	assert.Equal(t, expected, authenticate)
}

func TestNegotiatorStateMachine(t *testing.T) {
	t.Parallel()

	challengeBytes := buildTestChallenge(testChallengeFlags(), [8]byte{1}, eolTerminated())

	t.Run("challenge response before negotiate", func(t *testing.T) {
		t.Parallel()

		n := testNegotiator(t, testCBT)
		_, err := n.ChallengeResponse(challengeBytes, specUser, specNTHash)
		require.ErrorIs(t, err, ErrInvalidExchangeState)
	})

	t.Run("second negotiate", func(t *testing.T) {
		t.Parallel()

		n := testNegotiator(t, testCBT)
		_, err := n.Negotiate(specDomain, "")
		require.NoError(t, err)

		_, err = n.Negotiate(specDomain, "")
		require.ErrorIs(t, err, ErrInvalidExchangeState)
	})

	t.Run("second challenge response", func(t *testing.T) {
		t.Parallel()

		n := testNegotiator(t, testCBT)
		_, err := n.Negotiate(specDomain, "")
		require.NoError(t, err)
		_, err = n.ChallengeResponse(challengeBytes, specUser, specNTHash)
		require.NoError(t, err)

		_, err = n.ChallengeResponse(challengeBytes, specUser, specNTHash)
		require.ErrorIs(t, err, ErrInvalidExchangeState)
	})

	t.Run("negotiate after challenge response", func(t *testing.T) {
		t.Parallel()

		n := testNegotiator(t, testCBT)
		_, err := n.Negotiate(specDomain, "")
		require.NoError(t, err)
		_, err = n.ChallengeResponse(challengeBytes, specUser, specNTHash)
		require.NoError(t, err)

		_, err = n.Negotiate(specDomain, "")
		require.ErrorIs(t, err, ErrInvalidExchangeState)
	})
}

func TestNegotiateRequiresADomain(t *testing.T) {
	t.Parallel()

	n := testNegotiator(t, testCBT)
	_, err := n.Negotiate("", "")
	require.ErrorIs(t, err, ErrMissingDomain)
}

func TestNegotiatorDomainChangesTheResponse(t *testing.T) {
	t.Parallel()

	challengeBytes := buildTestChallenge(testChallengeFlags(), [8]byte{1}, eolTerminated())

	respond := func(domain string) []byte {
		n := testNegotiator(t, testCBT)
		_, err := n.Negotiate(domain, "")
		require.NoError(t, err)
		out, err := n.ChallengeResponse(challengeBytes, specUser, specNTHash)
		require.NoError(t, err)
		return out
	}

	assert.NotEqual(t, respond("ALPHA"), respond("BETA"),
		"the domain feeds the response key, so identical credentials under two domains differ on the wire")
}

func TestChallengeResponseRejectsABadHash(t *testing.T) {
	t.Parallel()

	n := testNegotiator(t, testCBT)
	_, err := n.Negotiate(specDomain, "")
	require.NoError(t, err)

	challengeBytes := buildTestChallenge(testChallengeFlags(), [8]byte{1}, eolTerminated())

	_, err = n.ChallengeResponse(challengeBytes, specUser, "not hex")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrInvalidExchangeState))
}

func TestChallengeResponseRejectsAMalformedChallenge(t *testing.T) {
	t.Parallel()

	n := testNegotiator(t, testCBT)
	_, err := n.Negotiate(specDomain, "")
	require.NoError(t, err)

	_, err = n.ChallengeResponse([]byte("garbage"), specUser, specNTHash)
	require.Error(t, err)
}

func TestNewNegotiatorRejectsTheZeroToken(t *testing.T) {
	t.Parallel()

	// The all-zero token means "no channel binding" on the wire. A permissive
	// domain controller would accept such a bind, which is precisely the
	// outcome this feature exists to prevent, so it is refused at construction
	// rather than left to the caller to avoid.
	_, err := NewNegotiator([16]byte{})
	require.ErrorIs(t, err, ErrNoChannelBinding)
}

func TestNegotiatorCarriesTheTokenUnchanged(t *testing.T) {
	t.Parallel()

	cbt := [16]byte{0xDE, 0xAD, 0xBE, 0xEF}
	n := testNegotiator(t, cbt)
	_, err := n.Negotiate(specDomain, "")
	require.NoError(t, err)

	challengeBytes := buildTestChallenge(testChallengeFlags(), [8]byte{1}, eolTerminated())
	authenticate, err := n.ChallengeResponse(challengeBytes, specUser, specNTHash)
	require.NoError(t, err)

	ntResponse, err := readVarField(authenticate, 20, "nt response")
	require.NoError(t, err)
	pairs, err := parseTargetInfo(ntResponse[16+clientChallengeHeaderSize : len(ntResponse)-4])
	require.NoError(t, err)
	value, ok := findAVPair(pairs, avChannelBindings)
	require.True(t, ok)
	assert.Equal(t, cbt[:], value, "the token reaches the wire byte for byte")
}

func TestNegotiatorLayouts(t *testing.T) {
	t.Parallel()

	// All four header layouts must build and stay self-consistent. Only
	// version=true, mic=true ships; the rest exist for the Task 8A gate, and
	// they are unit-tested here so the spike measures working code rather than
	// discovering the layout surface at the domain controller.
	tests := []struct {
		name              string
		version, mic      bool
		wantMICOffset     int
		wantHeaderSize    int
		wantNegotiateSize int
	}{
		{name: "version and mic", version: true, mic: true, wantMICOffset: 72, wantHeaderSize: 88, wantNegotiateSize: 40},
		{name: "neither", version: false, mic: false, wantMICOffset: 64, wantHeaderSize: 64, wantNegotiateSize: 32},
		{name: "mic only", version: false, mic: true, wantMICOffset: 64, wantHeaderSize: 80, wantNegotiateSize: 32},
		{name: "version only", version: true, mic: false, wantMICOffset: 72, wantHeaderSize: 72, wantNegotiateSize: 40},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			l := layout{version: test.version, mic: test.mic}
			assert.Equal(t, test.wantMICOffset, l.micOffset())
			assert.Equal(t, test.wantHeaderSize, l.authenticateHeaderSize())
			assert.Equal(t, test.wantNegotiateSize, l.negotiateHeaderSize())

			n := testNegotiator(t, [16]byte{0x01}, withVersion(test.version), withMIC(test.mic))

			negotiate, err := n.Negotiate(specDomain, "")
			require.NoError(t, err)
			require.Len(t, negotiate, test.wantNegotiateSize)

			challengeBytes := buildTestChallenge(testChallengeFlags(), [8]byte{1}, eolTerminated())
			authenticate, err := n.ChallengeResponse(challengeBytes, specUser, specNTHash)
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(authenticate), test.wantHeaderSize)

			// Every payload sits at or after the header, so no payload byte can
			// land on the MIC field.
			for name, at := range map[string]int{
				"lm response": 12, "nt response": 20, "domain": 28,
				"user": 36, "workstation": 44, "session key": 52,
			} {
				offset := binary.LittleEndian.Uint32(authenticate[at+4 : at+8])
				assert.GreaterOrEqualf(t, int(offset), test.wantHeaderSize, "%s offset", name)
			}

			micField := authenticate[l.micOffset() : l.micOffset()+16]
			if test.mic {
				assert.NotEqual(t, make([]byte, 16), micField, "the MIC is filled in")
			}

			hasVersionFlag := binary.LittleEndian.Uint32(authenticate[60:64])&flagVersion != 0
			assert.Equal(t, test.version, hasVersionFlag)

			// The MsvAvFlags MIC-present bit must agree with the layout.
			ntResponse, err := readVarField(authenticate, 20, "nt response")
			require.NoError(t, err)
			pairs, err := parseTargetInfo(ntResponse[16+clientChallengeHeaderSize : len(ntResponse)-4])
			require.NoError(t, err)
			avFlagsValue, ok := findAVPair(pairs, avFlags)
			require.True(t, ok)
			micBitSet := binary.LittleEndian.Uint32(avFlagsValue)&avFlagMICPresent != 0
			assert.Equal(t, test.mic, micBitSet, "MsvAvFlags must not claim a MIC the message does not carry")
		})
	}
}

func TestNegotiatorSatisfiesGoLdapInterface(t *testing.T) {
	t.Parallel()

	n, err := NewNegotiator([16]byte{0x01})
	require.NoError(t, err)

	var _ ldapv3.NTLMNegotiator = n
}

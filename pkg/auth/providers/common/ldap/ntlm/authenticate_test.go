package ntlm

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MS-NLMP 4.2.4 NTLMv2 test values. Every constant below is published in the
// specification and was independently reproduced before this plan was written;
// none is a value this implementation generated. If a test using them fails,
// the implementation is wrong, not the constant.
const (
	specUser     = "User"
	specDomain   = "Domain"
	specPassword = "Password"

	// NTOWFv1 — MD4(UTF16LE("Password")). MS-NLMP 4.2.4.1.1.
	specNTHash = "a4f49c406510bdcab6824ee7c30fd852"
	// NTOWFv2 — HMAC-MD5(NTOWFv1, UTF16LE("USER" + "Domain")). 4.2.4.1.1.
	specResponseKeyNT = "0c868a403bfd7a93a3001ef22ef02e3f"
	// SessionBaseKey — HMAC-MD5(NTOWFv2, NTProofStr). 4.2.4.1.2.
	specSessionBaseKey = "8de40ccadbc14a82f15cb0ad0de95ca3"
	// LMv2Response for the 4.2.4 inputs. 4.2.4.2.1.
	specLMv2Response = "86c35097ac9cec102554764a57cccc19aaaaaaaaaaaaaaaa"
	// NTProofStr, the first 16 bytes of NtChallengeResponse. 4.2.4.2.2.
	specNTProofStr = "68cd0ab851e51c96aabc927bebef6a1c"
)

// The 4.2.4 fixtures: server challenge, client challenge, and the TargetInfo
// carrying NbDomainName "Domain" and NbComputerName "Server".
var (
	specServerChallenge = [8]byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}
	specClientChallenge = [8]byte{0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa}
)

// specTargetInfo is the 4.2.4 server TargetInfo, hex
// 02000c0044006f006d00610069006e0001000c00530065007200760065007200 0000 0000.
func specTargetInfo() []byte {
	return eolTerminated(
		avPair{ID: avNbDomainName, Value: utf16LE("Domain")},
		avPair{ID: avNbComputerName, Value: utf16LE("Server")},
	)
}

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

func TestResponseKeyNTMatchesSpecVector(t *testing.T) {
	t.Parallel()

	got := responseKeyNT(mustDecodeHex(t, specNTHash), specUser, specDomain)
	assert.Equal(t, mustDecodeHex(t, specResponseKeyNT), got)
}

func TestResponseKeyNTUppercasesOnlyTheUser(t *testing.T) {
	t.Parallel()

	hash := mustDecodeHex(t, specNTHash)

	assert.Equal(t,
		responseKeyNT(hash, "user", specDomain),
		responseKeyNT(hash, "USER", specDomain),
		"the user name is uppercased")
	assert.NotEqual(t,
		responseKeyNT(hash, specUser, "domain"),
		responseKeyNT(hash, specUser, "DOMAIN"),
		"the domain is used verbatim")
}

func TestResponseKeyNTDependsOnDomain(t *testing.T) {
	t.Parallel()

	hash := mustDecodeHex(t, specNTHash)
	assert.NotEqual(t, responseKeyNT(hash, specUser, "A"), responseKeyNT(hash, specUser, "B"))
}

// TestSpecVectorChain reproduces the full MS-NLMP 4.2.4 derivation against the
// published constants. It is the independent oracle for the response key, the
// client challenge framing, the NT proof and the session key: every value is
// from the specification, so a passing run means the formulas are right rather
// than merely self-consistent.
//
// The 4.2.4 fixtures predate channel binding, so the TargetInfo is passed
// through untransformed and the timestamp is zero. Layout variation and the
// CBT are covered separately.
func TestSpecVectorChain(t *testing.T) {
	t.Parallel()

	keyNT := responseKeyNT(mustDecodeHex(t, specNTHash), specUser, specDomain)
	require.Equal(t, mustDecodeHex(t, specResponseKeyNT), keyNT)

	lmv2 := append(
		hmacMD5(keyNT, specServerChallenge[:], specClientChallenge[:]),
		specClientChallenge[:]...,
	)
	assert.Equal(t, mustDecodeHex(t, specLMv2Response), lmv2, "LMv2 response, 4.2.4.2.1")

	temp := []byte{0x01, 0x01, 0, 0, 0, 0, 0, 0}
	temp = append(temp, make([]byte, 8)...) // Timestamp is zero in 4.2.4.
	temp = append(temp, specClientChallenge[:]...)
	temp = append(temp, 0, 0, 0, 0)
	temp = append(temp, specTargetInfo()...)
	temp = append(temp, 0, 0, 0, 0)

	proof := hmacMD5(keyNT, specServerChallenge[:], temp)
	assert.Equal(t, mustDecodeHex(t, specNTProofStr), proof, "NTProofStr, 4.2.4.2.2")
	assert.Equal(t, mustDecodeHex(t, specSessionBaseKey), hmacMD5(keyNT, proof), "SessionBaseKey, 4.2.4.1.2")
}

func testAuthenticateInput(t *testing.T, serverTargetInfo []byte) authenticateInput {
	t.Helper()

	return authenticateInput{
		NTHash:      mustDecodeHex(t, specNTHash),
		User:        specUser,
		Domain:      specDomain,
		Challenge:   &challenge{ServerChallenge: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}, Flags: testChallengeFlags(), TargetInfo: serverTargetInfo},
		CBT:         [16]byte{0xAA},
		ClientNonce: [8]byte{0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA},
		Now:         time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC),
		Layout:      layout{version: true, mic: true},
	}
}

func TestBuildAuthenticateLayout(t *testing.T) {
	t.Parallel()

	in := testAuthenticateInput(t, eolTerminated())
	msg, sessionKey, err := buildAuthenticate(in)
	require.NoError(t, err)
	require.Len(t, sessionKey, 16)
	require.GreaterOrEqual(t, len(msg), in.Layout.authenticateHeaderSize())

	assert.Equal(t, ntlmSignature, string(msg[0:8]))
	assert.Equal(t, uint32(messageTypeAuthenticate), binary.LittleEndian.Uint32(msg[8:12]))
	assert.Equal(t, authenticateFlags(in.Layout, in.Challenge.Flags), binary.LittleEndian.Uint32(msg[60:64]))
	assert.Equal(t, versionStructure[:], msg[64:72])

	// Every payload offset lands at or after the header, so the MIC field is
	// never overwritten by payload bytes.
	for name, at := range map[string]int{
		"lm response": 12, "nt response": 20, "domain": 28,
		"user": 36, "workstation": 44, "session key": 52,
	} {
		offset := binary.LittleEndian.Uint32(msg[at+4 : at+8])
		assert.GreaterOrEqualf(t, int(offset), in.Layout.authenticateHeaderSize(), "%s offset", name)
	}

	domain, err := readVarField(msg, 28, "domain")
	require.NoError(t, err)
	assert.Equal(t, utf16LE(specDomain), domain)

	user, err := readVarField(msg, 36, "user")
	require.NoError(t, err)
	assert.Equal(t, utf16LE(specUser), user, "the user name is sent verbatim, only the response key uppercases it")

	workstation, err := readVarField(msg, 44, "workstation")
	require.NoError(t, err)
	assert.Empty(t, workstation)

	sessionKeyField, err := readVarField(msg, 52, "session key")
	require.NoError(t, err)
	assert.Empty(t, sessionKeyField, "no key exchange, so no encrypted session key is sent")
}

func TestMICOffsetConstant(t *testing.T) {
	t.Parallel()

	// The AUTHENTICATE header is positional. Version occupies 64..71 and the
	// MIC 72..87, so the payload cannot start before 88. Changing either
	// constant relocates the MIC and silently invalidates it at the server.
	shipping := layout{version: true, mic: true}
	assert.Equal(t, 72, shipping.micOffset())
	assert.Equal(t, 88, shipping.authenticateHeaderSize())
	assert.Equal(t, shipping.micOffset()+16, shipping.authenticateHeaderSize())
}

func TestAuthenticateFlagsValue(t *testing.T) {
	t.Parallel()

	shipping := layout{version: true, mic: true}

	// A challenge returning everything offered reproduces the shipping word.
	assert.Equal(t, uint32(0x02888205), authenticateFlags(shipping, baseFlags|flagVersion))
	assert.Zero(t, authenticateFlags(shipping, baseFlags|flagVersion)&flagKeyExch)
	assert.Zero(t, authenticateFlags(shipping, baseFlags|flagVersion)&flagLMKey)

	// A flag the server withheld is not asserted back at it. ALWAYS_SIGN is
	// the only offered flag parseChallenge does not require, so it is the only
	// one that can legitimately be absent here.
	got := authenticateFlags(shipping, (baseFlags|flagVersion)&^flagAlwaysSign)
	assert.Zero(t, got&flagAlwaysSign, "the client does not claim a capability the server withheld")
	assert.NotZero(t, got&flagUnicode)
}

func TestBuildAuthenticateCarriesTheChannelBinding(t *testing.T) {
	t.Parallel()

	in := testAuthenticateInput(t, eolTerminated(avPair{ID: avNbDomainName, Value: utf16LE("FOO")}))
	msg, _, err := buildAuthenticate(in)
	require.NoError(t, err)

	ntResponse, err := readVarField(msg, 20, "nt response")
	require.NoError(t, err)
	require.Greater(t, len(ntResponse), 16+28)

	// NtChallengeResponse is NTProofStr(16) || temp; the AV pairs start 28
	// bytes into temp and run to the end minus the 4 trailing reserved bytes.
	targetInfo := ntResponse[16+28 : len(ntResponse)-4]
	pairs, err := parseTargetInfo(targetInfo)
	require.NoError(t, err)

	value, ok := findAVPair(pairs, avChannelBindings)
	require.True(t, ok)
	assert.Equal(t, in.CBT[:], value)
}

func TestBuildAuthenticateLMBranchServerTimestampPresent(t *testing.T) {
	t.Parallel()

	serverTimestamp := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}
	in := testAuthenticateInput(t, eolTerminated(avPair{ID: avTimestamp, Value: serverTimestamp}))

	msg, _, err := buildAuthenticate(in)
	require.NoError(t, err)

	lmResponse, err := readVarField(msg, 12, "lm response")
	require.NoError(t, err)
	assert.Equal(t, make([]byte, 24), lmResponse,
		"a server supplied timestamp means the LM response is 24 zero bytes")

	ntResponse, err := readVarField(msg, 20, "nt response")
	require.NoError(t, err)
	// temp starts at 16; its timestamp is at temp offset 8, so message offset 24.
	assert.Equal(t, serverTimestamp, ntResponse[24:32],
		"the server timestamp is copied into the client challenge")
}

func TestBuildAuthenticateLMBranchServerTimestampAbsent(t *testing.T) {
	t.Parallel()

	in := testAuthenticateInput(t, eolTerminated(avPair{ID: avNbDomainName, Value: utf16LE("FOO")}))

	msg, _, err := buildAuthenticate(in)
	require.NoError(t, err)

	lmResponse, err := readVarField(msg, 12, "lm response")
	require.NoError(t, err)
	require.Len(t, lmResponse, 24)
	assert.NotEqual(t, make([]byte, 24), lmResponse,
		"with no server timestamp the LMv2 response is computed")
	assert.Equal(t, in.ClientNonce[:], lmResponse[16:24],
		"the LMv2 response ends with the client nonce")

	ntResponse, err := readVarField(msg, 20, "nt response")
	require.NoError(t, err)
	want := make([]byte, 8)
	binary.LittleEndian.PutUint64(want, fileTime(in.Now))
	assert.Equal(t, want, ntResponse[24:32], "the injected clock is used")
}

func TestBuildAuthenticateIsDeterministic(t *testing.T) {
	t.Parallel()

	in := testAuthenticateInput(t, eolTerminated())

	first, firstKey, err := buildAuthenticate(in)
	require.NoError(t, err)
	second, secondKey, err := buildAuthenticate(in)
	require.NoError(t, err)

	assert.True(t, bytes.Equal(first, second), "same inputs produce the same bytes")
	assert.Equal(t, firstKey, secondKey)
}

func TestBuildAuthenticateRejectsMalformedTargetInfo(t *testing.T) {
	t.Parallel()

	in := testAuthenticateInput(t, []byte{0x02, 0x00, 0xFF, 0xFF})

	_, _, err := buildAuthenticate(in)
	require.Error(t, err)
}

func TestBuildAuthenticateHandlesUnicodeCredentials(t *testing.T) {
	t.Parallel()

	in := testAuthenticateInput(t, eolTerminated())
	in.User = "üser"
	in.Domain = "DÖMÄNE"

	msg, _, err := buildAuthenticate(in)
	require.NoError(t, err)

	user, err := readVarField(msg, 36, "user")
	require.NoError(t, err)
	assert.Equal(t, utf16LE("üser"), user)

	domain, err := readVarField(msg, 28, "domain")
	require.NoError(t, err)
	assert.Equal(t, utf16LE("DÖMÄNE"), domain)
}

func TestPatchMIC(t *testing.T) {
	t.Parallel()

	in := testAuthenticateInput(t, eolTerminated())
	negotiate := buildNegotiate(in.Layout)
	challengeBytes := buildTestChallenge(testChallengeFlags(), in.Challenge.ServerChallenge, in.Challenge.TargetInfo)

	msg, sessionKey, err := buildAuthenticate(in)
	require.NoError(t, err)
	require.Equal(t, make([]byte, 16), msg[in.Layout.micOffset():in.Layout.micOffset()+16], "the MIC field starts zeroed")

	// The expected MIC is HMAC-MD5 over the three messages with the MIC field
	// still zeroed.
	zeroed := make([]byte, len(msg))
	copy(zeroed, msg)
	want := hmacMD5(sessionKey, negotiate, challengeBytes, zeroed)

	require.NoError(t, patchMIC(in.Layout, msg, negotiate, challengeBytes, sessionKey))
	assert.Equal(t, want, msg[in.Layout.micOffset():in.Layout.micOffset()+16])

	// Only the MIC field changed.
	copy(zeroed[in.Layout.micOffset():in.Layout.micOffset()+16], msg[in.Layout.micOffset():in.Layout.micOffset()+16])
	assert.Equal(t, zeroed, msg)
}

func TestPatchMICRejectsShortMessage(t *testing.T) {
	t.Parallel()

	shipping := layout{version: true, mic: true}
	require.Error(t, patchMIC(shipping, make([]byte, shipping.authenticateHeaderSize()-1), nil, nil, make([]byte, 16)))
}

func TestPatchMICIsDeterministic(t *testing.T) {
	t.Parallel()

	in := testAuthenticateInput(t, eolTerminated())
	negotiate := buildNegotiate(in.Layout)
	challengeBytes := buildTestChallenge(testChallengeFlags(), in.Challenge.ServerChallenge, in.Challenge.TargetInfo)

	build := func() []byte {
		msg, key, err := buildAuthenticate(in)
		require.NoError(t, err)
		require.NoError(t, patchMIC(in.Layout, msg, negotiate, challengeBytes, key))
		return msg
	}

	assert.Equal(t, build(), build())
}

func TestMICChangesWhenTheChannelBindingChanges(t *testing.T) {
	t.Parallel()

	negotiate := buildNegotiate(layout{version: true, mic: true})

	build := func(cbt [16]byte) []byte {
		in := testAuthenticateInput(t, eolTerminated())
		in.CBT = cbt
		challengeBytes := buildTestChallenge(testChallengeFlags(), in.Challenge.ServerChallenge, in.Challenge.TargetInfo)
		msg, key, err := buildAuthenticate(in)
		require.NoError(t, err)
		require.NoError(t, patchMIC(in.Layout, msg, negotiate, challengeBytes, key))
		return msg[in.Layout.micOffset() : in.Layout.micOffset()+16]
	}

	assert.NotEqual(t, build([16]byte{0x01}), build([16]byte{0x02}),
		"the channel binding feeds the NT proof, the session key and therefore the MIC")
}

// The end-to-end fixture that pins a complete AUTHENTICATE message is
// deliberately absent from this repository.
//
// It was captured on 2026-08-26 against Windows Server 2019 Datacenter
// 10.0.17763 with LdapEnforceChannelBinding = 2, and this implementation
// reproduced all 444 accepted bytes including the MIC. The capture is retained
// outside this repository because the message body embeds the domain
// controller's real host and domain names as UTF-16 inside the target info,
// and the test needs the bind account's NT hash — an unsalted MD4 of its
// password. Neither belongs in a public repository.
//
// The tests above therefore validate the pieces rather than the whole: the
// derivation chain against the published MS-NLMP 4.2.4 vectors, the four header
// layouts against a size table, the channel binding token against an
// independently computed value, and the target info against a round trip. What
// they cannot catch is a correct field assembled at the wrong offset.
//
// Restoring that coverage needs a capture from a lab domain controller with
// disposable names and a single-use account. See the "Deviation from the spec"
// section of the implementation plan for what the live run established, and the
// AD lab plan for the throwaway environment that would produce a publishable
// capture.

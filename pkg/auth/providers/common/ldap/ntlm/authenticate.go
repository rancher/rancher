package ntlm

import (
	"crypto/hmac"
	"crypto/md5" // #nosec G501 -- HMAC-MD5 is mandated by MS-NLMP for NTLMv2.
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// clientChallengeHeaderSize is the fixed part of NTLMv2_CLIENT_CHALLENGE
// preceding the AV pairs: response type, reserved fields, timestamp and the
// client nonce.
const clientChallengeHeaderSize = 28

// responseKeyNT computes NTOWFv2: HMAC-MD5 of the uppercased user name
// concatenated with the domain, keyed by the NT hash. Only the user name is
// uppercased; MS-NLMP uses the domain verbatim.
func responseKeyNT(ntHash []byte, user, domain string) []byte {
	mac := hmac.New(md5.New, ntHash)
	mac.Write(utf16LE(strings.ToUpper(user) + domain))
	return mac.Sum(nil)
}

func hmacMD5(key []byte, parts ...[]byte) []byte {
	mac := hmac.New(md5.New, key)
	for _, p := range parts {
		mac.Write(p)
	}
	return mac.Sum(nil)
}

// authenticateInput is everything buildAuthenticate needs. ClientNonce and Now
// are parameters rather than generated internally so the output is
// deterministic and testable against fixed vectors.
type authenticateInput struct {
	NTHash      []byte
	User        string
	Domain      string
	Challenge   *challenge
	CBT         [16]byte
	ClientNonce [8]byte
	Now         time.Time
	Layout      layout
}

// buildAuthenticate returns the AUTHENTICATE_MESSAGE and the exported session
// key. When in.Layout.mic is set the MIC field is present and zeroed; the caller
// computes and patches it, because the MIC covers this message's own bytes.
func buildAuthenticate(in authenticateInput) ([]byte, []byte, error) {
	if in.Challenge == nil {
		return nil, nil, fmt.Errorf("ntlm: no challenge to respond to")
	}

	// Resolved here as well as in ChallengeResponse. Both derive it from the
	// same two inputs, so they cannot disagree, and validating here means a
	// direct caller cannot produce a message whose header contradicts its own
	// flag word.
	activeLayout, err := effectiveLayout(in.Layout, in.Challenge.Flags)
	if err != nil {
		return nil, nil, err
	}
	in.Layout = activeLayout

	serverPairs, err := parseTargetInfo(in.Challenge.TargetInfo)
	if err != nil {
		return nil, nil, err
	}

	targetInfo, err := transformTargetInfo(serverPairs, in.CBT, in.Layout.mic)
	if err != nil {
		return nil, nil, err
	}

	// MS-NLMP 3.1.5.1.2: when the server supplied a timestamp the client
	// echoes it and sends no LM response. Otherwise it stamps its own clock
	// and computes an LMv2 response. The branch is decided by what the server
	// sent, not by the client challenge, which always carries a timestamp.
	timestamp := make([]byte, 8)
	serverTimestamp, hasServerTimestamp := findAVPair(serverPairs, avTimestamp)
	if hasServerTimestamp {
		if len(serverTimestamp) != 8 {
			return nil, nil, fmt.Errorf("ntlm: MsvAvTimestamp has %d bytes, expected 8", len(serverTimestamp))
		}
		copy(timestamp, serverTimestamp)
	} else {
		binary.LittleEndian.PutUint64(timestamp, fileTime(in.Now))
	}

	// NTLMv2_CLIENT_CHALLENGE, MS-NLMP 2.2.2.7.
	temp := make([]byte, 0, clientChallengeHeaderSize+len(targetInfo)+4)
	temp = append(temp, 0x01, 0x01)             // RespType, HiRespType
	temp = append(temp, 0x00, 0x00)             // Reserved1
	temp = append(temp, 0x00, 0x00, 0x00, 0x00) // Reserved2
	temp = append(temp, timestamp...)
	temp = append(temp, in.ClientNonce[:]...)
	temp = append(temp, 0x00, 0x00, 0x00, 0x00) // Reserved3
	temp = append(temp, targetInfo...)
	temp = append(temp, 0x00, 0x00, 0x00, 0x00) // Reserved4

	keyNT := responseKeyNT(in.NTHash, in.User, in.Domain)
	ntProofStr := hmacMD5(keyNT, in.Challenge.ServerChallenge[:], temp)

	ntResponse := make([]byte, 0, len(ntProofStr)+len(temp))
	ntResponse = append(ntResponse, ntProofStr...)
	ntResponse = append(ntResponse, temp...)

	lmResponse := make([]byte, 24)
	if !hasServerTimestamp {
		// ResponseKeyLM equals ResponseKeyNT for NTLMv2.
		proof := hmacMD5(keyNT, in.Challenge.ServerChallenge[:], in.ClientNonce[:])
		copy(lmResponse[0:16], proof)
		copy(lmResponse[16:24], in.ClientNonce[:])
	}

	// No key exchange is negotiated, so the exported session key is the
	// session base key and no encrypted session key is sent.
	sessionKey := hmacMD5(keyNT, ntProofStr)

	domain := utf16LE(in.Domain)
	user := utf16LE(in.User)

	headerSize := in.Layout.authenticateHeaderSize()
	msg := make([]byte, headerSize)
	copy(msg[0:8], ntlmSignature)
	binary.LittleEndian.PutUint32(msg[8:12], messageTypeAuthenticate)

	offset := headerSize
	var payloadErr error
	appendPayload := func(at int, name string, payload []byte) {
		if payloadErr != nil {
			return
		}
		if err := putVarField(msg[at:at+8], len(payload), offset, name); err != nil {
			payloadErr = err
			return
		}
		msg = append(msg, payload...)
		offset += len(payload)
	}
	appendPayload(12, "LM response", lmResponse)
	appendPayload(20, "NT response", ntResponse)
	appendPayload(28, "domain", domain)
	appendPayload(36, "user", user)
	appendPayload(44, "workstation", nil) // go-ldap always binds with an empty one.
	appendPayload(52, "session key", nil) // Unused without key exchange.
	if payloadErr != nil {
		return nil, nil, payloadErr
	}

	binary.LittleEndian.PutUint32(msg[60:64], authenticateFlags(in.Layout, in.Challenge.Flags))
	if in.Layout.version {
		copy(msg[64:72], versionStructure[:])
	}
	// When in.Layout.mic is set, the 16 bytes at in.Layout.micOffset() are the
	// MIC field, left zeroed here for patchMIC to fill.

	return msg, sessionKey, nil
}

// patchMIC computes the message integrity code over the three exchanged
// messages and writes it into the AUTHENTICATE message in place.
//
// The MIC covers the AUTHENTICATE message including its own field, so that
// field must be present and zeroed when the HMAC is taken and only then
// overwritten. authenticate must be the exact bytes returned by
// buildAuthenticate with WithMIC set.
func patchMIC(l layout, authenticate, negotiate, challengeBytes, sessionKey []byte) error {
	if !l.mic {
		return nil
	}

	headerSize := l.authenticateHeaderSize()
	if len(authenticate) < headerSize {
		return fmt.Errorf("ntlm: authenticate message of %d bytes is shorter than the %d byte header", len(authenticate), headerSize)
	}

	at := l.micOffset()
	mic := hmacMD5(sessionKey, negotiate, challengeBytes, authenticate)
	copy(authenticate[at:at+16], mic)
	return nil
}

package ntlm

import (
	"encoding/binary"
	"fmt"
)

// maxNTLMChallengeSize bounds the CHALLENGE message the domain controller may
// send. Its two payloads are described by 16-bit lengths, so 256 KiB covers
// any well-formed message while keeping parsing work bounded.
const maxNTLMChallengeSize = 256 * 1024

// challengeHeaderSize is the CHALLENGE_MESSAGE header without the optional
// version field. Payload offsets are absolute, so a version field only shifts
// the payload and never the fields parsed here.
const challengeHeaderSize = 48

// challenge holds the parts of a CHALLENGE_MESSAGE (MS-NLMP 2.2.1.2) the
// client needs to build its response.
type challenge struct {
	ServerChallenge [8]byte
	Flags           uint32
	TargetInfo      []byte
}

// readVarField returns the payload a security buffer at msg[at:at+8] points
// to, rejecting any descriptor that does not lie wholly inside msg.
func readVarField(msg []byte, at int, name string) ([]byte, error) {
	if at < 0 || at+8 > len(msg) {
		return nil, fmt.Errorf("ntlm: %s descriptor at offset %d does not fit in the %d byte message", name, at, len(msg))
	}

	length := int(binary.LittleEndian.Uint16(msg[at : at+2]))
	offset := int(binary.LittleEndian.Uint32(msg[at+4 : at+8]))

	if offset < 0 || length < 0 || offset > len(msg) || length > len(msg)-offset {
		return nil, fmt.Errorf("ntlm: %s buffer at offset %d length %d is outside the %d byte message", name, offset, length, len(msg))
	}
	return msg[offset : offset+length], nil
}

// parseChallenge validates and decodes a CHALLENGE_MESSAGE.
//
// It rejects a challenge that does not support the exchange this package
// promises: without TARGET_INFO there is nowhere to carry the channel binding
// token, and without EXTENDED_SESSIONSECURITY the response would not be
// NTLMv2. It never downgrades.
func parseChallenge(msg []byte) (*challenge, error) {
	if len(msg) > maxNTLMChallengeSize {
		return nil, fmt.Errorf("ntlm: challenge of %d bytes exceeds the %d byte limit", len(msg), maxNTLMChallengeSize)
	}
	if len(msg) < challengeHeaderSize {
		return nil, fmt.Errorf("ntlm: challenge of %d bytes is shorter than the %d byte header", len(msg), challengeHeaderSize)
	}
	if string(msg[0:8]) != ntlmSignature {
		return nil, fmt.Errorf("ntlm: challenge has an invalid NTLMSSP signature")
	}
	if got := binary.LittleEndian.Uint32(msg[8:12]); got != messageTypeChallenge {
		return nil, fmt.Errorf("ntlm: expected message type %d, got %d", messageTypeChallenge, got)
	}

	flags := binary.LittleEndian.Uint32(msg[20:24])

	// Every flag the AUTHENTICATE message will assert is required here, so the
	// client can never claim a capability the server did not return. Checked
	// before the specific messages below so the diagnostics stay useful.
	if missing := requiredChallengeFlags &^ flags; missing != 0 {
		switch {
		case missing&flagTargetInfo != 0:
			return nil, fmt.Errorf("ntlm: server did not offer NTLMSSP_NEGOTIATE_TARGET_INFO, so the channel binding token cannot be sent")
		case missing&flagExtendedSessionSecurity != 0:
			return nil, fmt.Errorf("ntlm: server did not offer NTLMSSP_NEGOTIATE_EXTENDED_SESSIONSECURITY, which NTLMv2 requires")
		case missing&flagUnicode != 0:
			return nil, fmt.Errorf("ntlm: server did not offer NTLMSSP_NEGOTIATE_UNICODE; this package only sends UTF-16 strings")
		default:
			return nil, fmt.Errorf("ntlm: server did not offer required negotiate flags 0x%08x", missing)
		}
	}

	switch {
	case flags&flagKeyExch != 0:
		return nil, fmt.Errorf("ntlm: server requested NTLMSSP_NEGOTIATE_KEY_EXCH, which is not supported")
	case flags&flagLMKey != 0:
		return nil, fmt.Errorf("ntlm: server requested NTLMSSP_NEGOTIATE_LM_KEY, which is not supported")
	}

	targetInfo, err := readVarField(msg, 40, "target info")
	if err != nil {
		return nil, err
	}

	parsed := &challenge{Flags: flags, TargetInfo: targetInfo}
	copy(parsed.ServerChallenge[:], msg[24:32])
	return parsed, nil
}

// authenticateFlags is the flag word the client commits to for the session:
// the offered set intersected with what the server returned. Nothing is
// synthesized — a flag the server withheld is not asserted back at it.
//
// parseChallenge has already rejected any challenge missing a flag in
// requiredChallengeFlags, so the intersection cannot silently drop something
// this package depends on.
func authenticateFlags(l layout, challengeFlags uint32) uint32 {
	return l.flags() & challengeFlags
}

// effectiveLayout reports the layout to serialize with, given what the server
// negotiated.
//
// MS-NLMP 2.2.1.3 and 2.2.2.10 tie the presence of the VERSION field to a
// negotiated NTLMSSP_NEGOTIATE_VERSION. Emitting the field while clearing the
// flag — or setting the flag the server never returned — produces a message
// whose header does not match its own flag word, which is precisely the class
// of inconsistency that makes a domain controller reject a bind with no usable
// diagnostic.
//
// So a version-enabled layout requires the server to have returned the flag.
// Real Active Directory always does. If a server does not, this fails loudly
// with an actionable message rather than silently relocating the MIC.
func effectiveLayout(l layout, challengeFlags uint32) (layout, error) {
	if l.version && challengeFlags&flagVersion == 0 {
		return layout{}, fmt.Errorf("ntlm: server did not negotiate NTLMSSP_NEGOTIATE_VERSION, so the version field and the MIC offset it fixes cannot be used")
	}
	return l, nil
}

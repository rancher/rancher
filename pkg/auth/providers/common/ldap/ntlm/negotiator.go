package ntlm

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ErrInvalidExchangeState is returned when the two negotiator callbacks are
// invoked out of order or more than once.
var ErrInvalidExchangeState = errors.New("ntlm: negotiator methods called out of order")

// ErrMissingDomain is returned when no domain is supplied. The NTLMv2 response
// key is derived from the user name and the domain, so it is not optional.
var ErrMissingDomain = errors.New("ntlm: a domain is required")

type exchangeState int

const (
	stateNew exchangeState = iota
	stateNegotiated
	stateComplete
)

// Negotiator implements go-ldap's NTLMNegotiator for a single NTLMv2 bind
// carrying a channel binding token.
//
// It is single-use and stateful: go-ldap splits the exchange across two
// callbacks and passes the domain only to the first, so the domain and the
// exact NEGOTIATE bytes must be carried across. It is created and owned by one
// NTLMChallengeBind call and is not safe for concurrent use.
//
// Callers cannot match on the errors returned here. go-ldap (v3.4.14) renders
// negotiator failures with %s, not %w, so error identity is destroyed before
// NTLMChallengeBind returns. Do not build
// errors.Is checks on ErrInvalidExchangeState or ErrMissingDomain outside this
// package; that only becomes possible if go-ldap starts preserving %w itself.
type Negotiator struct {
	state          exchangeState
	cbt            [16]byte
	userDomain     string
	negotiateBytes []byte
	layout         layout

	now   func() time.Time
	nonce func([]byte) error
}

// Option adjusts a Negotiator at construction. The clock and nonce hooks exist
// so tests can pin the two non-deterministic inputs.
type Option func(*Negotiator)

// WithClock replaces the timestamp source used when the server supplies none.
func WithClock(now func() time.Time) Option {
	return func(n *Negotiator) { n.now = now }
}

// WithNonceSource replaces the client challenge source.
func WithNonceSource(read func([]byte) error) Option {
	return func(n *Negotiator) { n.nonce = read }
}

// withVersion controls whether the 8-byte VERSION field is emitted. Turning it
// off moves the MIC from offset 72 to 64 and the payload base from 88 to 80.
//
// Exists so the header layout can be measured against a real domain
// controller during development. Production code must not call it.
func withVersion(on bool) Option {
	return func(n *Negotiator) { n.layout.version = on }
}

// withMIC controls whether a message integrity code is emitted and whether the
// MsvAvFlags MIC-present bit is set. Same caveat as withVersion.
func withMIC(on bool) Option {
	return func(n *Negotiator) { n.layout.mic = on }
}

// ErrNoChannelBinding is returned when the negotiator is constructed with the
// all-zero token, which means "no channel binding" on the wire. A caller that
// cannot derive a real token must fail the bind rather than send an unbound
// one that a permissive DC might accept.
var ErrNoChannelBinding = errors.New("ntlm: refusing to bind with an all-zero channel binding token")

// NewNegotiator returns a single-use negotiator that binds the exchange to the
// TLS channel described by cbt.
func NewNegotiator(cbt [16]byte, options ...Option) (*Negotiator, error) {
	if cbt == ([16]byte{}) {
		return nil, ErrNoChannelBinding
	}

	n := &Negotiator{
		cbt:    cbt,
		layout: layout{version: true, mic: true},
		now:    time.Now,
		nonce: func(b []byte) error {
			_, err := rand.Read(b)
			return err
		},
	}
	for _, option := range options {
		option(n)
	}
	return n, nil
}

// Negotiate returns the NEGOTIATE message and records the domain the response
// key will be derived from. go-ldap always passes an empty workstation.
func (n *Negotiator) Negotiate(domain, workstation string) ([]byte, error) {
	if n.state != stateNew {
		return nil, ErrInvalidExchangeState
	}
	if domain == "" {
		return nil, ErrMissingDomain
	}

	n.userDomain = domain
	n.negotiateBytes = buildNegotiate(n.layout)
	n.state = stateNegotiated

	return bytes.Clone(n.negotiateBytes), nil
}

// ChallengeResponse returns the AUTHENTICATE message answering challengeBytes.
//
// hash is the hex-encoded NT hash go-ldap derived from the bind password; this
// package never hashes the password itself and never accepts a caller-supplied
// hash, so pass-the-hash stays unreachable.
func (n *Negotiator) ChallengeResponse(challengeBytes []byte, username, hash string) ([]byte, error) {
	if n.state != stateNegotiated {
		return nil, ErrInvalidExchangeState
	}

	ntHash, err := hex.DecodeString(hash)
	if err != nil {
		return nil, fmt.Errorf("ntlm: NT hash is not valid hex: %w", err)
	}
	if len(ntHash) != 16 {
		return nil, fmt.Errorf("ntlm: NT hash is %d bytes, expected 16", len(ntHash))
	}

	parsed, err := parseChallenge(challengeBytes)
	if err != nil {
		return nil, err
	}

	// Resolved once here so the flag word, the version field, the MIC offset
	// and patchMIC below all agree on the same layout.
	effective, err := effectiveLayout(n.layout, parsed.Flags)
	if err != nil {
		return nil, err
	}

	in := authenticateInput{
		NTHash:    ntHash,
		User:      username,
		Domain:    n.userDomain,
		Challenge: parsed,
		CBT:       n.cbt,
		Now:       n.now(),
		Layout:    effective,
	}
	if err := n.nonce(in.ClientNonce[:]); err != nil {
		return nil, fmt.Errorf("ntlm: cannot read a client challenge: %w", err)
	}

	authenticate, sessionKey, err := buildAuthenticate(in)
	if err != nil {
		return nil, err
	}
	if err := patchMIC(effective, authenticate, n.negotiateBytes, challengeBytes, sessionKey); err != nil {
		return nil, err
	}

	n.state = stateComplete
	return authenticate, nil
}

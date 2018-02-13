package mesh

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

const (
	// Protocol identifies a sort of major version of the protocol.
	Protocol = "weave"

	// ProtocolMinVersion establishes the lowest protocol version among peers
	// that we're willing to try to communicate with.
	ProtocolMinVersion = 1

	// ProtocolMaxVersion establishes the highest protocol version among peers
	// that we're willing to try to communicate with.
	ProtocolMaxVersion = 2
)

var (
	protocolBytes = []byte(Protocol)

	// How long we wait for the handshake phase of protocol negotiation.
	headerTimeout = 10 * time.Second

	// See filterV1Features.
	protocolV1Features = []string{
		"ConnID",
		"Name",
		"NickName",
		"PeerNameFlavour",
		"UID",
	}

	errExpectedCrypto   = fmt.Errorf("password specified, but peer requested an unencrypted connection")
	errExpectedNoCrypto = fmt.Errorf("no password specificed, but peer requested an encrypted connection")
)

type protocolIntroConn interface {
	io.ReadWriter

	// net.Conn's deadline methods
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

// The params necessary to negotiate a protocol intro with a remote peer.
type protocolIntroParams struct {
	MinVersion byte
	MaxVersion byte
	Features   map[string]string
	Conn       protocolIntroConn
	Password   []byte
	Outbound   bool
}

// The results from a successful protocol intro.
type protocolIntroResults struct {
	Features   map[string]string
	Receiver   tcpReceiver
	Sender     tcpSender
	SessionKey *[32]byte
	Version    byte
}

// DoIntro executes the protocol introduction.
func (params protocolIntroParams) doIntro() (res protocolIntroResults, err error) {
	if err = params.Conn.SetDeadline(time.Now().Add(headerTimeout)); err != nil {
		return
	}

	if res.Version, err = params.exchangeProtocolHeader(); err != nil {
		return
	}

	var pubKey, privKey *[32]byte
	if params.Password != nil {
		if pubKey, privKey, err = generateKeyPair(); err != nil {
			return
		}
	}

	if err = params.Conn.SetWriteDeadline(time.Time{}); err != nil {
		return
	}
	if err = params.Conn.SetReadDeadline(time.Now().Add(tcpHeartbeat * 2)); err != nil {
		return
	}

	switch res.Version {
	case 1:
		err = res.doIntroV1(params, pubKey, privKey)
	case 2:
		err = res.doIntroV2(params, pubKey, privKey)
	default:
		panic("unhandled protocol version")
	}

	return
}

func (params protocolIntroParams) exchangeProtocolHeader() (byte, error) {
	// Write in a separate goroutine to avoid the possibility of
	// deadlock.  The result channel is of size 1 so that the
	// goroutine does not linger even if we encounter an error on
	// the read side.
	sendHeader := append(protocolBytes, params.MinVersion, params.MaxVersion)
	writeDone := make(chan error, 1)
	go func() {
		_, err := params.Conn.Write(sendHeader)
		writeDone <- err
	}()

	header := make([]byte, len(protocolBytes)+2)
	if n, err := io.ReadFull(params.Conn, header); err != nil && n == 0 {
		return 0, fmt.Errorf("failed to receive remote protocol header: %s", err)
	} else if err != nil {
		return 0, fmt.Errorf("received incomplete remote protocol header (%d octets instead of %d): %v; error: %s",
			n, len(header), header[:n], err)
	}

	if !bytes.Equal(protocolBytes, header[:len(protocolBytes)]) {
		return 0, fmt.Errorf("remote protocol header not recognised: %v", header[:len(protocolBytes)])
	}

	theirMinVersion := header[len(protocolBytes)]
	minVersion := theirMinVersion
	if params.MinVersion > minVersion {
		minVersion = params.MinVersion
	}

	theirMaxVersion := header[len(protocolBytes)+1]
	maxVersion := theirMaxVersion
	if maxVersion > params.MaxVersion {
		maxVersion = params.MaxVersion
	}

	if minVersion > maxVersion {
		return 0, fmt.Errorf("remote version range [%d,%d] is incompatible with ours [%d,%d]",
			theirMinVersion, theirMaxVersion,
			params.MinVersion, params.MaxVersion)
	}

	if err := <-writeDone; err != nil {
		return 0, err
	}

	return maxVersion, nil
}

// The V1 procotol consists of the protocol identification/version
// header, followed by a stream of gobified values.  The first value
// is the encoded features map (never encrypted).  The subsequent
// values are the messages on the connection (encrypted for an
// encrypted connection).  For an encrypted connection, the public key
// is passed in the "PublicKey" feature as a string of hex digits.
func (res *protocolIntroResults) doIntroV1(params protocolIntroParams, pubKey, privKey *[32]byte) error {
	features := filterV1Features(params.Features)
	if pubKey != nil {
		features["PublicKey"] = hex.EncodeToString(pubKey[:])
	}

	enc := gob.NewEncoder(params.Conn)
	dec := gob.NewDecoder(params.Conn)

	// Encode in a separate goroutine to avoid the possibility of
	// deadlock.  The result channel is of size 1 so that the
	// goroutine does not linger even if we encounter an error on
	// the read side.
	encodeDone := make(chan error, 1)
	go func() {
		encodeDone <- enc.Encode(features)
	}()

	if err := dec.Decode(&res.Features); err != nil {
		return err
	}

	if err := <-encodeDone; err != nil {
		return err
	}

	res.Sender = newGobTCPSender(enc)
	res.Receiver = newGobTCPReceiver(dec)

	if pubKey == nil {
		if _, present := res.Features["PublicKey"]; present {
			return errExpectedNoCrypto
		}
	} else {
		remotePubKeyStr, ok := res.Features["PublicKey"]
		if !ok {
			return errExpectedCrypto
		}

		remotePubKey, err := hex.DecodeString(remotePubKeyStr)
		if err != nil {
			return err
		}

		res.setupCrypto(params, remotePubKey, privKey)
	}

	res.Features = filterV1Features(res.Features)
	return nil
}

// In the V1 protocol, the intro fields are sent unencrypted.  So we
// restrict them to an established subset of fields that are assumed
// to be safe.
func filterV1Features(intro map[string]string) map[string]string {
	safe := make(map[string]string)
	for _, k := range protocolV1Features {
		if val, ok := intro[k]; ok {
			safe[k] = val
		}
	}

	return safe
}

// The V2 procotol consists of the protocol identification/version
// header, followed by:
//
// - A single "encryption flag" byte: 0 for no encryption, 1 for
// encryption.
//
// - When the connection is encrypted, 32 bytes follow containing the
// public key.
//
// - Then a stream of length-prefixed messages, which are encrypted
// for an encrypted connection.
//
// The first message contains the encoded features map (so in contrast
// to V1, it will be encrypted on an encrypted connection).
func (res *protocolIntroResults) doIntroV2(params protocolIntroParams, pubKey, privKey *[32]byte) error {
	// Public key exchange
	var wbuf []byte
	if pubKey == nil {
		wbuf = []byte{0}
	} else {
		wbuf = make([]byte, 1+len(*pubKey))
		wbuf[0] = 1
		copy(wbuf[1:], (*pubKey)[:])
	}

	// Write in a separate goroutine to avoid the possibility of
	// deadlock.  The result channel is of size 1 so that the
	// goroutine does not linger even if we encounter an error on
	// the read side.
	writeDone := make(chan error, 1)
	go func() {
		_, err := params.Conn.Write(wbuf)
		writeDone <- err
	}()

	rbuf := make([]byte, 1)
	if _, err := io.ReadFull(params.Conn, rbuf); err != nil {
		return err
	}

	switch rbuf[0] {
	case 0:
		if pubKey != nil {
			return errExpectedCrypto
		}

		res.Sender = newLengthPrefixTCPSender(params.Conn)
		res.Receiver = newLengthPrefixTCPReceiver(params.Conn)

	case 1:
		if pubKey == nil {
			return errExpectedNoCrypto
		}

		rbuf = make([]byte, len(pubKey))
		if _, err := io.ReadFull(params.Conn, rbuf); err != nil {
			return err
		}

		res.Sender = newLengthPrefixTCPSender(params.Conn)
		res.Receiver = newLengthPrefixTCPReceiver(params.Conn)
		res.setupCrypto(params, rbuf, privKey)

	default:
		return fmt.Errorf("Bad encryption flag %d", rbuf[0])
	}

	if err := <-writeDone; err != nil {
		return err
	}

	// Features exchange
	go func() {
		buf := new(bytes.Buffer)
		if err := gob.NewEncoder(buf).Encode(&params.Features); err != nil {
			writeDone <- err
			return
		}

		writeDone <- res.Sender.Send(buf.Bytes())
	}()

	rbuf, err := res.Receiver.Receive()
	if err != nil {
		return err
	}

	if err := gob.NewDecoder(bytes.NewReader(rbuf)).Decode(&res.Features); err != nil {
		return err
	}

	if err := <-writeDone; err != nil {
		return err
	}

	return nil
}

func (res *protocolIntroResults) setupCrypto(params protocolIntroParams, remotePubKey []byte, privKey *[32]byte) {
	var remotePubKeyArr [32]byte
	copy(remotePubKeyArr[:], remotePubKey)
	res.SessionKey = formSessionKey(&remotePubKeyArr, privKey, params.Password)
	res.Sender = newEncryptedTCPSender(res.Sender, res.SessionKey, params.Outbound)
	res.Receiver = newEncryptedTCPReceiver(res.Receiver, res.SessionKey, params.Outbound)
}

// ProtocolTag identifies the type of msg encoded in a ProtocolMsg.
type protocolTag byte

const (
	// ProtocolHeartbeat identifies a heartbeat msg.
	ProtocolHeartbeat = iota
	// ProtocolReserved1 is a legacy overly control message.
	ProtocolReserved1
	// ProtocolReserved2 is a legacy overly control message.
	ProtocolReserved2
	// ProtocolReserved3 is a legacy overly control message.
	ProtocolReserved3
	// ProtocolGossip identifies a pure gossip msg.
	ProtocolGossip
	// ProtocolGossipUnicast identifies a gossip (unicast) msg.
	ProtocolGossipUnicast
	// ProtocolGossipBroadcast identifies a gossip (broadcast) msg.
	ProtocolGossipBroadcast
	// ProtocolOverlayControlMsg identifies a control msg.
	ProtocolOverlayControlMsg
)

// ProtocolMsg combines a tag and encoded msg.
type protocolMsg struct {
	tag protocolTag
	msg []byte
}

type protocolSender interface {
	SendProtocolMsg(m protocolMsg) error
}

package mesh

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"sync"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

// MaxTCPMsgSize is the hard limit on sends and receives. Larger messages will
// result in errors. This applies to the LengthPrefixTCP{Sender,Receiver} i.e.
// V2 of the protocol.
const maxTCPMsgSize = 10 * 1024 * 1024

// GenerateKeyPair is used during encrypted protocol introduction.
func generateKeyPair() (publicKey, privateKey *[32]byte, err error) {
	return box.GenerateKey(rand.Reader)
}

// FormSessionKey is used during encrypted protocol introduction.
func formSessionKey(remotePublicKey, localPrivateKey *[32]byte, secretKey []byte) *[32]byte {
	var sharedKey [32]byte
	box.Precompute(&sharedKey, remotePublicKey, localPrivateKey)
	sharedKeySlice := sharedKey[:]
	sharedKeySlice = append(sharedKeySlice, secretKey...)
	sessionKey := sha256.Sum256(sharedKeySlice)
	return &sessionKey
}

// TCP Senders/Receivers

// TCPCryptoState stores session key, nonce, and sequence state.
//
// The lowest 64 bits of the nonce contain the message sequence number. The
// top most bit indicates the connection polarity at the sender - '1' for
// outbound; the next indicates protocol type - '1' for TCP. The remaining 126
// bits are zero. The polarity is needed so that the two ends of a connection
// do not use the same nonces; the protocol type so that the TCP connection
// nonces are distinct from nonces used by overlay connections, if they share
// the session key. This is a requirement of the NaCl Security Model; see
// http://nacl.cr.yp.to/box.html.
type tcpCryptoState struct {
	sessionKey *[32]byte
	nonce      [24]byte
	seqNo      uint64
}

// NewTCPCryptoState returns a valid TCPCryptoState.
func newTCPCryptoState(sessionKey *[32]byte, outbound bool) *tcpCryptoState {
	s := &tcpCryptoState{sessionKey: sessionKey}
	if outbound {
		s.nonce[0] |= (1 << 7)
	}
	s.nonce[0] |= (1 << 6)
	return s
}

func (s *tcpCryptoState) advance() {
	s.seqNo++
	binary.BigEndian.PutUint64(s.nonce[16:24], s.seqNo)
}

// TCPSender describes anything that can send byte buffers.
// It abstracts over the different protocol version senders.
type tcpSender interface {
	Send([]byte) error
}

// GobTCPSender implements TCPSender and is used in the V1 protocol.
type gobTCPSender struct {
	encoder *gob.Encoder
}

func newGobTCPSender(encoder *gob.Encoder) *gobTCPSender {
	return &gobTCPSender{encoder: encoder}
}

// Send implements TCPSender by encoding the msg.
func (sender *gobTCPSender) Send(msg []byte) error {
	return sender.encoder.Encode(msg)
}

// LengthPrefixTCPSender implements TCPSender and is used in the V2 protocol.
type lengthPrefixTCPSender struct {
	writer io.Writer
}

func newLengthPrefixTCPSender(writer io.Writer) *lengthPrefixTCPSender {
	return &lengthPrefixTCPSender{writer: writer}
}

// Send implements TCPSender by writing the size of the msg as a big-endian
// uint32 before the msg. msgs larger than MaxTCPMsgSize are rejected.
func (sender *lengthPrefixTCPSender) Send(msg []byte) error {
	l := len(msg)
	if l > maxTCPMsgSize {
		return fmt.Errorf("outgoing message exceeds maximum size: %d > %d", l, maxTCPMsgSize)
	}
	// We copy the message so we can send it in a single Write
	// operation, thus making this thread-safe without locking.
	prefixedMsg := make([]byte, 4+l)
	binary.BigEndian.PutUint32(prefixedMsg, uint32(l))
	copy(prefixedMsg[4:], msg)
	_, err := sender.writer.Write(prefixedMsg)
	return err
}

// Implement TCPSender by wrapping an existing TCPSender with tcpCryptoState.
type encryptedTCPSender struct {
	sync.RWMutex
	sender tcpSender
	state  *tcpCryptoState
}

func newEncryptedTCPSender(sender tcpSender, sessionKey *[32]byte, outbound bool) *encryptedTCPSender {
	return &encryptedTCPSender{sender: sender, state: newTCPCryptoState(sessionKey, outbound)}
}

// Send implements TCPSender by sealing and sending the msg as-is.
func (sender *encryptedTCPSender) Send(msg []byte) error {
	sender.Lock()
	defer sender.Unlock()
	encodedMsg := secretbox.Seal(nil, msg, &sender.state.nonce, sender.state.sessionKey)
	sender.state.advance()
	return sender.sender.Send(encodedMsg)
}

// tcpReceiver describes anything that can receive byte buffers.
// It abstracts over the different protocol version receivers.
type tcpReceiver interface {
	Receive() ([]byte, error)
}

// gobTCPReceiver implements TCPReceiver and is used in the V1 protocol.
type gobTCPReceiver struct {
	decoder *gob.Decoder
}

func newGobTCPReceiver(decoder *gob.Decoder) *gobTCPReceiver {
	return &gobTCPReceiver{decoder: decoder}
}

// Receive implements TCPReciever by Gob decoding into a byte slice directly.
func (receiver *gobTCPReceiver) Receive() ([]byte, error) {
	var msg []byte
	err := receiver.decoder.Decode(&msg)
	return msg, err
}

// lengthPrefixTCPReceiver implements TCPReceiver, used in the V2 protocol.
type lengthPrefixTCPReceiver struct {
	reader io.Reader
}

func newLengthPrefixTCPReceiver(reader io.Reader) *lengthPrefixTCPReceiver {
	return &lengthPrefixTCPReceiver{reader: reader}
}

// Receive implements TCPReceiver by making a length-limited read into a byte buffer.
func (receiver *lengthPrefixTCPReceiver) Receive() ([]byte, error) {
	lenPrefix := make([]byte, 4)
	if _, err := io.ReadFull(receiver.reader, lenPrefix); err != nil {
		return nil, err
	}
	l := binary.BigEndian.Uint32(lenPrefix)
	if l > maxTCPMsgSize {
		return nil, fmt.Errorf("incoming message exceeds maximum size: %d > %d", l, maxTCPMsgSize)
	}
	msg := make([]byte, l)
	_, err := io.ReadFull(receiver.reader, msg)
	return msg, err
}

// encryptedTCPReceiver implements TCPReceiver by wrapping a TCPReceiver with TCPCryptoState.
type encryptedTCPReceiver struct {
	receiver tcpReceiver
	state    *tcpCryptoState
}

func newEncryptedTCPReceiver(receiver tcpReceiver, sessionKey *[32]byte, outbound bool) *encryptedTCPReceiver {
	return &encryptedTCPReceiver{receiver: receiver, state: newTCPCryptoState(sessionKey, !outbound)}
}

// Receive implements TCPReceiver by reading from the wrapped TCPReceiver and
// unboxing the encrypted message, returning the decoded message.
func (receiver *encryptedTCPReceiver) Receive() ([]byte, error) {
	msg, err := receiver.receiver.Receive()
	if err != nil {
		return nil, err
	}

	decodedMsg, success := secretbox.Open(nil, msg, &receiver.state.nonce, receiver.state.sessionKey)
	if !success {
		return nil, fmt.Errorf("Unable to decrypt TCP msg")
	}

	receiver.state.advance()
	return decodedMsg, nil
}

package ldap

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"gopkg.in/asn1-ber.v1"
)

// TestNilPacket tests that nil packets don't cause a panic.
func TestNilPacket(t *testing.T) {
	// Test for nil packet
	err := GetLDAPError(nil)
	if !IsErrorWithCode(err, ErrorUnexpectedResponse) {
		t.Errorf("Should have an 'ErrorUnexpectedResponse' error in nil packets, got: %v", err)
	}

	// Test for nil result
	kids := []*ber.Packet{
		{},  // Unused
		nil, // Can't be nil
	}
	pack := &ber.Packet{Children: kids}
	err = GetLDAPError(pack)

	if !IsErrorWithCode(err, ErrorUnexpectedResponse) {
		t.Errorf("Should have an 'ErrorUnexpectedResponse' error in nil packets, got: %v", err)
	}
}

// TestConnReadErr tests that an unexpected error reading from underlying
// connection bubbles up to the goroutine which makes a request.
func TestConnReadErr(t *testing.T) {
	conn := &signalErrConn{
		signals: make(chan error),
	}

	ldapConn := NewConn(conn, false)
	ldapConn.Start()

	// Make a dummy search request.
	searchReq := NewSearchRequest("dc=example,dc=com", ScopeWholeSubtree, DerefAlways, 0, 0, false, "(objectClass=*)", nil, nil)

	expectedError := errors.New("this is the error you are looking for")

	// Send the signal after a short amount of time.
	time.AfterFunc(10*time.Millisecond, func() { conn.signals <- expectedError })

	// This should block until the underlying conn gets the error signal
	// which should bubble up through the reader() goroutine, close the
	// connection, and
	_, err := ldapConn.Search(searchReq)
	if err == nil || !strings.Contains(err.Error(), expectedError.Error()) {
		t.Errorf("not the expected error: %s", err)
	}
}

// TestGetLDAPError tests parsing of result with a error response.
func TestGetLDAPError(t *testing.T) {
	diagnosticMessage := "Detailed error message"
	bindResponse := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ApplicationBindResponse, nil, "Bind Response")
	bindResponse.AppendChild(ber.Encode(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, int64(LDAPResultInvalidCredentials), "resultCode"))
	bindResponse.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "dc=example,dc=org", "matchedDN"))
	bindResponse.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, diagnosticMessage, "diagnosticMessage"))
	packet := ber.NewSequence("LDAPMessage")
	packet.AppendChild(ber.Encode(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, int64(0), "messageID"))
	packet.AppendChild(bindResponse)
	err := GetLDAPError(packet)
	if err == nil {
		t.Errorf("Did not get error response")
	}

	ldapError := err.(*Error)
	if ldapError.ResultCode != LDAPResultInvalidCredentials {
		t.Errorf("Got incorrect error code in LDAP error; got %v, expected %v", ldapError.ResultCode, LDAPResultInvalidCredentials)
	}
	if ldapError.Err.Error() != diagnosticMessage {
		t.Errorf("Got incorrect error message in LDAP error; got %v, expected %v", ldapError.Err.Error(), diagnosticMessage)
	}
}

// TestGetLDAPErrorSuccess tests parsing of a result with no error (resultCode == 0).
func TestGetLDAPErrorSuccess(t *testing.T) {
	bindResponse := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ApplicationBindResponse, nil, "Bind Response")
	bindResponse.AppendChild(ber.Encode(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, int64(0), "resultCode"))
	bindResponse.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "matchedDN"))
	bindResponse.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "diagnosticMessage"))
	packet := ber.NewSequence("LDAPMessage")
	packet.AppendChild(ber.Encode(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, int64(0), "messageID"))
	packet.AppendChild(bindResponse)
	err := GetLDAPError(packet)
	if err != nil {
		t.Errorf("Successful responses should not produce an error, but got: %v", err)
	}
}

// signalErrConn is a helpful type used with TestConnReadErr. It implements the
// net.Conn interface to be used as a connection for the test. Most methods are
// no-ops but the Read() method blocks until it receives a signal which it
// returns as an error.
type signalErrConn struct {
	signals chan error
}

// Read blocks until an error is sent on the internal signals channel. That
// error is returned.
func (c *signalErrConn) Read(b []byte) (n int, err error) {
	return 0, <-c.signals
}

func (c *signalErrConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (c *signalErrConn) Close() error {
	close(c.signals)
	return nil
}

func (c *signalErrConn) LocalAddr() net.Addr {
	return (*net.TCPAddr)(nil)
}

func (c *signalErrConn) RemoteAddr() net.Addr {
	return (*net.TCPAddr)(nil)
}

func (c *signalErrConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *signalErrConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *signalErrConn) SetWriteDeadline(t time.Time) error {
	return nil
}

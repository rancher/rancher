package ldap

import (
	"context"
	"crypto/tls"
	"time"

	ldapv3 "github.com/go-ldap/ldap/v3"
)

type FakeLdapConn struct {
	BindFunc             func(username, password string) error
	SearchFunc           func(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error)
	SearchWithPagingFunc func(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error)
}

func (m *FakeLdapConn) Start()                     { panic("unimplemented") }
func (m *FakeLdapConn) StartTLS(*tls.Config) error { panic("unimplemented") }
func (m *FakeLdapConn) Close() error               { panic("unimplemented") }
func (m *FakeLdapConn) GetLastError() error        { panic("unimplemented") }
func (m *FakeLdapConn) IsClosing() bool            { panic("unimplemented") }
func (m *FakeLdapConn) SetTimeout(time.Duration)   { panic("unimplemented") }
func (m *FakeLdapConn) TLSConnectionState() (tls.ConnectionState, bool) {
	panic("unimplemented")
}
func (m *FakeLdapConn) Bind(username, password string) error {
	if m.BindFunc != nil {
		return m.BindFunc(username, password)
	}
	return nil
}
func (m *FakeLdapConn) UnauthenticatedBind(username string) error { panic("unimplemented") }
func (m *FakeLdapConn) NTLMUnauthenticatedBind(domain, username string) error {
	panic("unimplemented")
}
func (m *FakeLdapConn) Unbind() error { panic("unimplemented") }
func (m *FakeLdapConn) SimpleBind(*ldapv3.SimpleBindRequest) (*ldapv3.SimpleBindResult, error) {
	panic("unimplemented")
}
func (m *FakeLdapConn) ExternalBind() error                    { panic("unimplemented") }
func (m *FakeLdapConn) Add(*ldapv3.AddRequest) error           { panic("unimplemented") }
func (m *FakeLdapConn) Del(*ldapv3.DelRequest) error           { panic("unimplemented") }
func (m *FakeLdapConn) Modify(*ldapv3.ModifyRequest) error     { panic("unimplemented") }
func (m *FakeLdapConn) ModifyDN(*ldapv3.ModifyDNRequest) error { panic("unimplemented") }
func (m *FakeLdapConn) ModifyWithResult(*ldapv3.ModifyRequest) (*ldapv3.ModifyResult, error) {
	panic("unimplemented")
}
func (m *FakeLdapConn) Extended(*ldapv3.ExtendedRequest) (*ldapv3.ExtendedResponse, error) {
	panic("unimplemented")
}
func (m *FakeLdapConn) Compare(dn, attribute, value string) (bool, error) { panic("unimplemented") }
func (m *FakeLdapConn) PasswordModify(*ldapv3.PasswordModifyRequest) (*ldapv3.PasswordModifyResult, error) {
	panic("unimplemented")
}
func (m *FakeLdapConn) Search(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(searchRequest)
	}
	return &ldapv3.SearchResult{}, nil
}
func (m *FakeLdapConn) SearchWithPaging(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
	if m.SearchWithPagingFunc != nil {
		return m.SearchWithPagingFunc(searchRequest, pagingSize)
	}
	return &ldapv3.SearchResult{}, nil
}
func (m *FakeLdapConn) SearchAsync(ctx context.Context, searchRequest *ldapv3.SearchRequest, bufferSize int) ldapv3.Response {
	panic("unimplemented")
}
func (m *FakeLdapConn) DirSync(searchRequest *ldapv3.SearchRequest, flags, maxAttrCount int64, cookie []byte) (*ldapv3.SearchResult, error) {
	panic("unimplemented")
}
func (m *FakeLdapConn) DirSyncAsync(ctx context.Context, searchRequest *ldapv3.SearchRequest, bufferSize int, flags, maxAttrCount int64, cookie []byte) ldapv3.Response {
	panic("unimplemented")
}
func (m *FakeLdapConn) Syncrepl(ctx context.Context, searchRequest *ldapv3.SearchRequest, bufferSize int, mode ldapv3.ControlSyncRequestMode, cookie []byte, reloadHint bool) ldapv3.Response {
	panic("unimplemented")
}

package ldap

import (
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
func (m *FakeLdapConn) Close()                     { panic("unimplemented") }
func (m *FakeLdapConn) IsClosing() bool            { panic("unimplemented") }
func (m *FakeLdapConn) SetTimeout(time.Duration)   { panic("unimplemented") }
func (m *FakeLdapConn) Bind(username, password string) error {
	if m.BindFunc != nil {
		return m.BindFunc(username, password)
	}
	return nil
}
func (m *FakeLdapConn) UnauthenticatedBind(username string) error { panic("unimplemented") }
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

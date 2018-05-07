package ldap

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	ldapv2 "gopkg.in/ldap.v2"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func NewLDAPConn(config *v3.ActiveDirectoryConfig, caPool *x509.CertPool) (*ldapv2.Conn, error) {
	logrus.Debug("Now creating Ldap connection")
	var lConn *ldapv2.Conn
	var err error
	var tlsConfig *tls.Config
	// TODO implment multi-server support
	if len(config.Servers) != 1 {
		return nil, errors.New("invalid server config. only exactly 1 server is currently supported")
	}
	server := config.Servers[0]
	if config.TLS {
		tlsConfig = &tls.Config{RootCAs: caPool, InsecureSkipVerify: false, ServerName: server}
		lConn, err = ldapv2.DialTLS("tcp", fmt.Sprintf("%s:%d", server, config.Port), tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("Error creating ssl connection: %v", err)
		}
	} else {
		lConn, err = ldapv2.Dial("tcp", fmt.Sprintf("%s:%d", server, config.Port))
		if err != nil {
			return nil, fmt.Errorf("Error creating connection: %v", err)
		}
	}

	lConn.SetTimeout(time.Duration(config.ConnectionTimeout) * time.Second)

	return lConn, nil
}

func GetUserExternalID(username string, loginDomain string) string {
	if strings.Contains(username, "\\") {
		return username
	} else if loginDomain != "" {
		return loginDomain + "\\" + username
	}
	return username
}

func HasPermission(attributes []*ldapv2.EntryAttribute, config *v3.ActiveDirectoryConfig) bool {
	var permission int64
	if !IsType(attributes, config.UserObjectClass) {
		return true
	}
	for _, attr := range attributes {
		if attr.Name == config.UserEnabledAttribute {
			if len(attr.Values) > 0 && attr.Values[0] != "" {
				intAttr, err := strconv.ParseInt(attr.Values[0], 10, 64)
				if err != nil {
					logrus.Errorf("Failed to get USER_ENABLED_ATTRIBUTE, error: %v", err)
					return false
				}
				permission = intAttr
			} else {
				return true
			}
		}
	}
	permission = permission & config.UserDisabledBitMask
	return permission != config.UserDisabledBitMask
}

func IsType(search []*ldapv2.EntryAttribute, varType string) bool {
	for _, attrib := range search {
		if attrib.Name == "objectClass" {
			for _, val := range attrib.Values {
				if val == varType {
					return true
				}
			}
		}
	}
	logrus.Debugf("Failed to determine if object is type: %s", varType)
	return false
}

func EscapeLDAPSearchFilter(filter string) string {
	buf := new(bytes.Buffer)
	for i := 0; i < len(filter); i++ {
		currChar := filter[i]
		switch currChar {
		case '\\':
			buf.WriteString("\\5c")
			break
		case '*':
			buf.WriteString("\\2a")
			break
		case '(':
			buf.WriteString("\\28")
			break
		case ')':
			buf.WriteString("\\29")
			break
		case '\u0000':
			buf.WriteString("\\00")
			break
		default:
			buf.WriteString(string(currChar))
		}
	}
	return buf.String()
}

func GetUserSearchAttributes(memberOfAttribute, objectClassAttribute string, config *v3.ActiveDirectoryConfig) []string {
	srchAttributes := strings.Split(config.UserSearchAttribute, "|")
	userSearchAttributes := []string{memberOfAttribute,
		objectClassAttribute,
		config.UserObjectClass,
		config.UserLoginAttribute,
		config.UserNameAttribute,
		config.UserEnabledAttribute}
	userSearchAttributes = append(userSearchAttributes, srchAttributes...)

	return userSearchAttributes
}

func GetGroupSearchAttributes(memberOfAttribute, objectClassAttribute string, config *v3.ActiveDirectoryConfig) []string {
	groupSeachAttributes := []string{memberOfAttribute,
		objectClassAttribute,
		config.GroupObjectClass,
		config.UserLoginAttribute,
		config.GroupNameAttribute,
		config.GroupSearchAttribute}
	return groupSeachAttributes
}

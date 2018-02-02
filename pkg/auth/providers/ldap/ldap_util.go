package ldap

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/sirupsen/logrus"
	ldapv2 "gopkg.in/ldap.v2"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type ConstantsConfig struct {
	UserScope            string
	GroupScope           string
	Scopes               []string
	MemberOfAttribute    string
	ObjectClassAttribute string
	CAPool               *x509.CertPool
}

func NewLDAPConn(config *v3.ActiveDirectoryConfig, constants *ConstantsConfig) (*ldapv2.Conn, error) {
	logrus.Debug("Now creating Ldap connection")
	var lConn *ldapv2.Conn
	var err error
	var tlsConfig *tls.Config
	if config.TLS {
		tlsConfig = &tls.Config{RootCAs: constants.CAPool, InsecureSkipVerify: false, ServerName: config.Server}
		lConn, err = ldapv2.DialTLS("tcp", fmt.Sprintf("%s:%d", config.Server, config.Port), tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("Error creating ssl connection: %v", err)
		}
	} else {
		lConn, err = ldapv2.Dial("tcp", fmt.Sprintf("%s:%d", config.Server, config.Port))
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

func GetUserSearchAttributes(config *v3.ActiveDirectoryConfig, c *ConstantsConfig) []string {
	userSearchAttributes := []string{c.MemberOfAttribute,
		c.ObjectClassAttribute,
		config.UserObjectClass,
		config.UserLoginField,
		config.UserNameField,
		config.UserSearchField,
		config.UserEnabledAttribute}

	return userSearchAttributes
}

func GetGroupSearchAttributes(config *v3.ActiveDirectoryConfig, c *ConstantsConfig) []string {
	groupSeachAttributes := []string{c.MemberOfAttribute,
		c.ObjectClassAttribute,
		config.GroupObjectClass,
		config.UserLoginField,
		config.GroupNameField,
		config.GroupSearchField}
	return groupSeachAttributes
}

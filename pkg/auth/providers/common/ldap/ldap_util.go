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
	"github.com/rancher/norman/httperror"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func NewLDAPConn(servers []string, TLS bool, port int64, connectionTimeout int64, caPool *x509.CertPool) (*ldapv2.Conn, error) {
	logrus.Debug("Now creating Ldap connection")
	var lConn *ldapv2.Conn
	var err error
	var tlsConfig *tls.Config
	ldapv2.DefaultTimeout = time.Duration(connectionTimeout) * time.Millisecond
	// TODO implment multi-server support
	if len(servers) != 1 {
		return nil, errors.New("invalid server config. only exactly 1 server is currently supported")
	}
	server := servers[0]
	if TLS {
		tlsConfig = &tls.Config{RootCAs: caPool, InsecureSkipVerify: false, ServerName: server}
		lConn, err = ldapv2.DialTLS("tcp", fmt.Sprintf("%s:%d", server, port), tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("Error creating ssl connection: %v", err)
		}
	} else {
		lConn, err = ldapv2.Dial("tcp", fmt.Sprintf("%s:%d", server, port))
		if err != nil {
			return nil, fmt.Errorf("Error creating connection: %v", err)
		}
	}

	lConn.SetTimeout(time.Duration(connectionTimeout) * time.Millisecond)

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

func HasPermission(attributes []*ldapv2.EntryAttribute, userObjectClass string, userEnabledAttribute string, userDisabledBitMask int64) bool {
	var permission int64
	if !IsType(attributes, userObjectClass) {
		return true
	}

	if userEnabledAttribute != "" {
		for _, attr := range attributes {
			if attr.Name == userEnabledAttribute {
				if len(attr.Values) > 0 && attr.Values[0] != "" {
					intAttr, err := strconv.ParseInt(attr.Values[0], 10, 64)
					if err != nil {
						logrus.Errorf("Failed to get USER_ENABLED_ATTRIBUTE, error: %v", err)
						return false
					}
					permission = intAttr
				}
			}
		}
	} else {
		return true
	}
	permission = permission & userDisabledBitMask
	return permission != userDisabledBitMask
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

func GetUserSearchAttributesForLDAP(config *v3.LdapConfig) []string {
	userSearchAttributes := []string{"dn", config.UserMemberAttribute,
		"objectClass",
		config.UserObjectClass,
		config.UserLoginAttribute,
		config.UserNameAttribute,
		config.UserEnabledAttribute}
	return userSearchAttributes
}

func GetGroupSearchAttributesForLDAP(config *v3.LdapConfig) []string {
	groupSeachAttributes := []string{config.GroupMemberUserAttribute,
		config.GroupMemberMappingAttribute,
		"objectClass",
		config.GroupObjectClass,
		config.UserLoginAttribute,
		config.GroupNameAttribute,
		config.GroupSearchAttribute}
	return groupSeachAttributes
}

func AuthenticateServiceAccountUser(enabled bool, serviceAccountPassword string, serviceAccountUsername string, lConn *ldapv2.Conn) (v3.Principal, []v3.Principal, map[string]string, error) {
	if !enabled { // TODO testing for enabled here might not be correct. Might be better to pass in an explicit testSvcAccount bool
		logrus.Debug("Bind service account username password")
		if serviceAccountPassword == "" {
			return v3.Principal{}, nil, nil, httperror.NewAPIError(httperror.MissingRequired, "service account password not provided")
		}
		sausername := GetUserExternalID(serviceAccountUsername, "")
		err := lConn.Bind(sausername, serviceAccountPassword)
		if err != nil {
			if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) {
				return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed")
			}
			return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.ServerError, "server error while authenticating")
		}
	}
	return v3.Principal{}, nil, nil, nil
}

func Min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

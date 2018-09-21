package ldap

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	ldapv2 "gopkg.in/ldap.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func GetUserSearchAttributes(memberOfAttribute, ObjectClass string, config *v3.ActiveDirectoryConfig) []string {
	userSearchAttributes := []string{memberOfAttribute,
		ObjectClass,
		config.UserObjectClass,
		config.UserLoginAttribute,
		config.UserNameAttribute,
		config.UserEnabledAttribute}
	return userSearchAttributes
}

func GetGroupSearchAttributes(memberOfAttribute, ObjectClass string, config *v3.ActiveDirectoryConfig) []string {
	groupSeachAttributes := []string{memberOfAttribute,
		ObjectClass,
		config.GroupObjectClass,
		config.UserLoginAttribute,
		config.GroupNameAttribute,
		config.GroupSearchAttribute}
	return groupSeachAttributes
}

func GetUserSearchAttributesForLDAP(ObjectClass string, config *v3.LdapConfig) []string {
	userSearchAttributes := []string{"dn", config.UserMemberAttribute,
		ObjectClass,
		config.UserObjectClass,
		config.UserLoginAttribute,
		config.UserNameAttribute,
		config.UserEnabledAttribute}
	return userSearchAttributes
}

func GetGroupSearchAttributesForLDAP(ObjectClass string, config *v3.LdapConfig) []string {
	groupSeachAttributes := []string{config.GroupMemberUserAttribute,
		config.GroupMemberMappingAttribute,
		ObjectClass,
		config.GroupObjectClass,
		config.UserLoginAttribute,
		config.GroupNameAttribute,
		config.GroupSearchAttribute}
	return groupSeachAttributes
}

func AuthenticateServiceAccountUser(serviceAccountPassword string, serviceAccountUsername string, defaultLoginDomain string, lConn *ldapv2.Conn) error {
	logrus.Debug("Binding service account username password")
	if serviceAccountPassword == "" {
		return httperror.NewAPIError(httperror.MissingRequired, "service account password not provided")
	}
	sausername := GetUserExternalID(serviceAccountUsername, defaultLoginDomain)
	err := lConn.Bind(sausername, serviceAccountPassword)
	if err != nil {
		if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) {
			return httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed")
		}
		return httperror.WrapAPIError(err, httperror.ServerError, "server error while authenticating")
	}

	return nil
}

func AttributesToPrincipal(attribs []*ldapv2.EntryAttribute, dnStr, scope, providerName, userObjectClass, userNameAttribute, userLoginAttribute, groupObjectClass, groupNameAttribute string) (*v3.Principal, error) {
	var externalIDType, accountName, externalID, login, kind string
	externalID = dnStr
	externalIDType = scope

	if IsType(attribs, userObjectClass) {
		for _, attr := range attribs {
			if attr.Name == userNameAttribute {
				if len(attr.Values) != 0 {
					accountName = attr.Values[0]
				} else {
					accountName = externalID
				}
			}
			if attr.Name == userLoginAttribute {
				if len(attr.Values) > 0 && attr.Values[0] != "" {
					login = attr.Values[0]
				}
			}
		}
		if login == "" {
			login = accountName
		}
		kind = "user"
	} else if IsType(attribs, groupObjectClass) {
		for _, attr := range attribs {
			if attr.Name == groupNameAttribute {
				if len(attr.Values) != 0 {
					accountName = attr.Values[0]
				} else {
					accountName = externalID
				}
			}
			if attr.Name == userLoginAttribute {
				if len(attr.Values) > 0 && attr.Values[0] != "" {
					login = attr.Values[0]
				}
			}
		}
		if login == "" {
			login = accountName
		}
		kind = "group"
	} else {
		return nil, fmt.Errorf("Failed to get attributes for %s", dnStr)
	}

	principal := &v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: externalIDType + "://" + externalID},
		DisplayName:   accountName,
		LoginName:     login,
		PrincipalType: kind,
		Me:            true,
		Provider:      providerName,
	}
	return principal, nil
}

func Min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

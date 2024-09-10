package ldap

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strconv"
	"strings"
	"time"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigAttributes struct {
	GroupMemberMappingAttribute string
	GroupNameAttribute          string
	GroupObjectClass            string
	GroupSearchAttribute        string
	ObjectClass                 string
	ProviderName                string
	UserLoginAttribute          string
	UserNameAttribute           string
	UserObjectClass             string
}

func Connect(config *v3.LdapConfig, caPool *x509.CertPool) (*ldapv3.Conn, error) {
	return NewLDAPConn(config.Servers, config.TLS, config.StartTLS, config.Port, config.ConnectionTimeout, caPool)
}

func NewLDAPConn(servers []string, TLS, startTLS bool, port int64, connectionTimeout int64, caPool *x509.CertPool) (*ldapv3.Conn, error) {
	logrus.Debug("Now creating Ldap connection")
	var lConn *ldapv3.Conn
	var err error
	var tlsConfig *tls.Config
	ldapv3.DefaultTimeout = time.Duration(connectionTimeout) * time.Millisecond
	if len(servers) < 1 {
		return nil, errors.New("invalid server config. at least 1 server needs to be configured")
	}
	for _, server := range servers {
		tlsConfig = &tls.Config{RootCAs: caPool, InsecureSkipVerify: false, ServerName: server}
		if TLS {
			lConn, err = ldapv3.DialTLS("tcp", fmt.Sprintf("%s:%d", server, port), tlsConfig)
			if err != nil {
				err = fmt.Errorf("Error creating ssl connection: %v", err)
			}
		} else if startTLS {
			lConn, err = ldapv3.Dial("tcp", fmt.Sprintf("%s:%d", server, port))
			if err != nil {
				err = fmt.Errorf("Error creating connection for startTLS: %v", err)
			} else if err = lConn.StartTLS(tlsConfig); err != nil {
				err = fmt.Errorf("Error upgrading startTLS connection: %v", err)
			}
		} else {
			lConn, err = ldapv3.Dial("tcp", fmt.Sprintf("%s:%d", server, port))
			if err != nil {
				err = fmt.Errorf("Error creating connection: %v", err)
			}
		}
		if err == nil {
			lConn.SetTimeout(time.Duration(connectionTimeout) * time.Millisecond)
			return lConn, nil
		}
	}

	return nil, err
}

func GetUserExternalID(username string, loginDomain string) string {
	if strings.Contains(username, "\\") {
		return username
	} else if loginDomain != "" {
		return loginDomain + "\\" + username
	}
	return username
}

func HasPermission(attributes []*ldapv3.EntryAttribute, userObjectClass string, userEnabledAttribute string, userDisabledBitMask int64) bool {
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

func IsType(search []*ldapv3.EntryAttribute, varType string) bool {
	for _, attrib := range search {
		if strings.EqualFold(attrib.Name, "objectClass") {
			for _, val := range attrib.Values {
				if strings.EqualFold(val, varType) {
					logrus.Debugf("ldap IsType found object of type %s", varType)
					return true
				}
			}
		}
	}
	logrus.Debugf("ldap IsType failed to determine if object is type: %s", varType)
	return false
}

func GetAttributeValuesByName(search []*ldapv3.EntryAttribute, attributeName string) []string {
	for _, attrib := range search {
		if attrib.Name == attributeName {
			return attrib.Values
		}
	}
	return []string{}
}

func GetUserSearchAttributes(memberOfAttribute, ObjectClass string, config *v32.ActiveDirectoryConfig) []string {
	userSearchAttributes := []string{memberOfAttribute,
		ObjectClass,
		config.UserObjectClass,
		config.UserLoginAttribute,
		config.UserNameAttribute,
		config.UserEnabledAttribute}
	return userSearchAttributes
}

func GetGroupSearchAttributes(config *v32.ActiveDirectoryConfig, searchAttributes ...string) []string {
	groupSeachAttributes := []string{
		config.GroupObjectClass,
		config.UserLoginAttribute,
		config.GroupNameAttribute,
		config.GroupSearchAttribute}
	groupSeachAttributes = append(groupSeachAttributes, searchAttributes...)
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
		ObjectClass,
		config.GroupObjectClass,
		config.UserLoginAttribute,
		config.GroupNameAttribute,
		config.GroupSearchAttribute}
	return groupSeachAttributes
}

func AuthenticateServiceAccountUser(serviceAccountPassword string, serviceAccountUsername string, defaultLoginDomain string, lConn ldapv3.Client) error {
	logrus.Debug("Binding service account username password")
	if serviceAccountPassword == "" {
		return httperror.NewAPIError(httperror.MissingRequired, "service account password not provided")
	}
	sausername := GetUserExternalID(serviceAccountUsername, defaultLoginDomain)
	err := lConn.Bind(sausername, serviceAccountPassword)
	if err != nil {
		if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultInvalidCredentials) {
			return httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed")
		}
		return httperror.WrapAPIError(err, httperror.ServerError, "server error while authenticating")
	}

	return nil
}

func AttributesToPrincipal(attribs []*ldapv3.EntryAttribute, dnStr, scope, providerName, userObjectClass, userNameAttribute, userLoginAttribute, groupObjectClass, groupNameAttribute string) (*v3.Principal, error) {
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

func GatherParentGroups(groupPrincipal v3.Principal, searchDomain string, groupScope string, config *ConfigAttributes, lConn ldapv3.Client,
	groupMap map[string]bool, nestedGroupPrincipals *[]v3.Principal, searchAttributes []string) error {
	groupMap[groupPrincipal.ObjectMeta.Name] = true
	principals := []v3.Principal{}
	//var searchAttributes []string
	parts := strings.SplitN(groupPrincipal.ObjectMeta.Name, ":", 2)
	if len(parts) != 2 {
		return errors.Errorf("invalid id %v", groupPrincipal.ObjectMeta.Name)
	}
	groupDN := strings.TrimPrefix(parts[1], "//")

	searchGroup := ldapv3.NewSearchRequest(searchDomain,
		ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(%v=%v)(%v=%v))", config.GroupMemberMappingAttribute, ldapv3.EscapeFilter(groupDN), config.ObjectClass, config.GroupObjectClass),
		searchAttributes, nil)
	resultGroups, err := lConn.SearchWithPaging(searchGroup, 1000)
	if err != nil {
		return err
	}

	for i := 0; i < len(resultGroups.Entries); i++ {
		entry := resultGroups.Entries[i]
		principal, err := AttributesToPrincipal(entry.Attributes, entry.DN, groupScope, config.ProviderName, config.UserObjectClass, config.UserNameAttribute, config.UserLoginAttribute, config.GroupObjectClass, config.GroupNameAttribute)
		if err != nil {
			logrus.Errorf("Error translating group result: %v", err)
			continue
		}
		principals = append(principals, *principal)
	}

	for _, gp := range principals {
		if _, ok := groupMap[gp.ObjectMeta.Name]; ok {
			continue
		} else {
			*nestedGroupPrincipals = append(*nestedGroupPrincipals, gp)
			err = GatherParentGroups(gp, searchDomain, groupScope, config, lConn, groupMap, nestedGroupPrincipals, searchAttributes)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func FindNonDuplicateBetweenGroupPrincipals(newGroupPrincipals []v3.Principal, groupPrincipals []v3.Principal, nonDupGroupPrincipals []v3.Principal) []v3.Principal {
	for _, gp := range newGroupPrincipals {
		counter := 0
		for _, usermembergp := range groupPrincipals {
			// check the groups ObjectMeta.Name and name fields value are the same, then they are the same group
			if gp.ObjectMeta.Name == usermembergp.ObjectMeta.Name && gp.DisplayName == usermembergp.DisplayName {
				break
			} else {
				counter++
			}
		}
		if counter == len(groupPrincipals) {
			nonDupGroupPrincipals = append(nonDupGroupPrincipals, gp)
		}
	}
	return nonDupGroupPrincipals
}

func Min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func NewCAPool(cert string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	pool.AppendCertsFromPEM([]byte(cert))
	return pool, nil
}

func ValidateLdapConfig(ldapConfig *v3.LdapConfig, certpool *x509.CertPool) (bool, error) {
	if len(ldapConfig.Servers) < 1 {
		return false, nil
	}

	lConn, err := Connect(ldapConfig, certpool)
	if err != nil {
		return false, err
	}
	defer lConn.Close()

	logrus.Debugf("validated ldap configuration: %s", strings.Join(ldapConfig.Servers, ","))
	return true, nil
}

package activedirectory

import (
	"fmt"
	ldapv2 "gopkg.in/ldap.v2"
	"reflect"
	"strings"

	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/sirupsen/logrus"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LClient is the ldap client
type LClient struct {
	ConstantsConfig *ldap.ConstantsConfig
}

func (l *LClient) LoginUser(adCredential v3.ActiveDirectoryCredential, config *v3.ActiveDirectoryConfig) (v3.Principal, []v3.Principal, map[string]string, int, error) {
	logrus.Debug("Now generating Ldap token")
	var status int
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal
	var providerInfo = make(map[string]string)

	username := adCredential.Username
	password := adCredential.Password

	externalID := ldap.GetUserExternalID(username, config.LoginDomain)

	if password == "" {
		return userPrincipal, groupPrincipals, providerInfo, 401, fmt.Errorf("Failed to login, password not provided")
	}

	lConn, err := ldap.NewLDAPConn(config, l.ConstantsConfig)
	if err != nil {
		return userPrincipal, groupPrincipals, providerInfo, status, err
	}

	if !config.Enabled {
		logrus.Debug("Bind service account username password")
		if config.ServiceAccountPassword == "" {
			status = 401
			return userPrincipal, groupPrincipals, providerInfo, status, fmt.Errorf("Failed to login, service account password not provided")
		}
		sausername := ldap.GetUserExternalID(config.ServiceAccountUsername, config.LoginDomain)
		err = lConn.Bind(sausername, config.ServiceAccountPassword)

		if err != nil {
			if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) {
				status = 401
			}
			defer lConn.Close()
			return userPrincipal, groupPrincipals, providerInfo, status, fmt.Errorf("Error in ldap bind of service account: %v", err)
		}
	}

	logrus.Debug("Binding username password")
	err = lConn.Bind(externalID, password)

	if err != nil {
		if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) {
			status = 401
		}
		return userPrincipal, groupPrincipals, providerInfo, status, fmt.Errorf("Error in ldap bind: %v", err)
	}
	defer lConn.Close()
	samName := username
	if strings.Contains(username, "\\") {
		samName = strings.SplitN(username, "\\\\", 2)[1]
	}
	query := "(" + config.UserLoginField + "=" + samName + ")"
	logrus.Debugf("LDAP Search query: {%s}", query)
	search := ldapv2.NewSearchRequest(config.Domain,
		ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
		query,
		ldap.GetUserSearchAttributes(config, l.ConstantsConfig), nil)

	userPrincipal, groupPrincipals, status, err = l.userRecord(search, lConn, config)
	return userPrincipal, groupPrincipals, providerInfo, status, err
}

func (l *LClient) userRecord(search *ldapv2.SearchRequest, lConn *ldapv2.Conn, config *v3.ActiveDirectoryConfig) (v3.Principal, []v3.Principal, int, error) {
	var status int
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal

	result, err := lConn.Search(search)
	if err != nil {
		return userPrincipal, groupPrincipals, status, err
	}

	if len(result.Entries) < 1 {
		return userPrincipal, groupPrincipals, status, fmt.Errorf("Cannot locate user information for %s", search.Filter)
	} else if len(result.Entries) > 1 {
		return userPrincipal, groupPrincipals, status, fmt.Errorf("Ldap user search found more than one result")
	}

	return l.getPrincipalsFromSearchResult(result, config)
}

func (l *LClient) getPrincipalsFromSearchResult(result *ldapv2.SearchResult, config *v3.ActiveDirectoryConfig) (v3.Principal, []v3.Principal, int, error) {
	var status int
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal

	c := l.ConstantsConfig
	entry := result.Entries[0]
	if !ldap.HasPermission(entry.Attributes, config) {
		return userPrincipal, groupPrincipals, status, fmt.Errorf("Permission denied")
	}

	memberOf := entry.GetAttributeValues(c.MemberOfAttribute)

	logrus.Debugf("ADConstants userMemberAttribute() {%s}", c.MemberOfAttribute)
	logrus.Debugf("SearchResult memberOf attribute {%s}", memberOf)

	// isType
	isType := false
	objectClass := entry.GetAttributeValues(c.ObjectClassAttribute)
	for _, obj := range objectClass {
		if strings.EqualFold(string(obj), config.UserObjectClass) {
			isType = true
		}
	}
	if !isType {
		return userPrincipal, groupPrincipals, status, nil
	}

	user, err := l.attributesToPrincipal(entry.Attributes, result.Entries[0].DN, c.UserScope, config)
	userPrincipal = *user
	if err != nil {
		return userPrincipal, groupPrincipals, status, err
	}

	if len(memberOf) != 0 {
		for _, attrib := range memberOf {
			group, err := l.getPrincipal(attrib, c.GroupScope, config)
			if err != nil {
				return userPrincipal, groupPrincipals, status, err
			}
			groupPrincipals = append(groupPrincipals, *group)
		}
	}
	return userPrincipal, groupPrincipals, status, nil
}

func (l *LClient) getPrincipal(distinguishedName string, scope string, config *v3.ActiveDirectoryConfig) (*v3.Principal, error) {
	c := l.ConstantsConfig

	var search *ldapv2.SearchRequest
	if c.Scopes[0] != scope && c.Scopes[1] != scope {
		return nil, fmt.Errorf("Invalid scope")
	}

	var attributes []*ldapv2.AttributeTypeAndValue
	var attribs []*ldapv2.EntryAttribute
	object, err := ldapv2.ParseDN(distinguishedName)
	if err != nil {
		return nil, err
	}
	for _, rdns := range object.RDNs {
		for _, attr := range rdns.Attributes {
			attributes = append(attributes, attr)
			entryAttr := ldapv2.NewEntryAttribute(attr.Type, []string{attr.Value})
			attribs = append(attribs, entryAttr)
		}
	}

	if !ldap.IsType(attribs, scope) && !ldap.HasPermission(attribs, config) {
		logrus.Errorf("Failed to get object %s", distinguishedName)
		return nil, nil
	}

	filter := "(" + c.ObjectClassAttribute + "=*)"
	logrus.Debugf("Query for getPrincipal(%s): %s", distinguishedName, filter)
	lConn, err := ldap.NewLDAPConn(config, l.ConstantsConfig)
	if err != nil {
		return nil, fmt.Errorf("Error %v creating connection", err)
	}
	defer lConn.Close()
	// Bind before query
	// If service acc bind fails, and auth is on, return principal formed using DN
	serviceAccountUsername := ldap.GetUserExternalID(config.ServiceAccountUsername, config.LoginDomain)
	err = lConn.Bind(serviceAccountUsername, config.ServiceAccountPassword)

	if err != nil {
		if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) && config.Enabled {
			var kind string
			if strings.EqualFold(c.UserScope, scope) {
				kind = "user"
			} else if strings.EqualFold(c.GroupScope, scope) {
				kind = "group"
			}
			principal := &v3.Principal{
				ObjectMeta:  metav1.ObjectMeta{Name: scope + "://" + distinguishedName},
				DisplayName: distinguishedName,
				LoginName:   distinguishedName,
				Kind:        kind,
			}

			return principal, nil
		}
		return nil, fmt.Errorf("Error in ldap bind: %v", err)
	}

	if strings.EqualFold(c.UserScope, scope) {
		search = ldapv2.NewSearchRequest(distinguishedName,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			filter,
			ldap.GetUserSearchAttributes(config, l.ConstantsConfig), nil)
	} else {
		search = ldapv2.NewSearchRequest(distinguishedName,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			filter,
			ldap.GetGroupSearchAttributes(config, l.ConstantsConfig), nil)
	}

	result, err := lConn.Search(search)
	if err != nil {
		return nil, fmt.Errorf("Error %v in search query : %v", err, filter)
	}

	if len(result.Entries) < 1 {
		return nil, fmt.Errorf("No identities can be retrieved")
	} else if len(result.Entries) > 1 {
		return nil, fmt.Errorf("More than one result found")
	}

	entry := result.Entries[0]
	entryAttributes := entry.Attributes
	if !ldap.HasPermission(entry.Attributes, config) {
		return nil, fmt.Errorf("Permission denied")
	}

	principal, err := l.attributesToPrincipal(entryAttributes, distinguishedName, scope, config)
	if err != nil {
		return nil, err
	}
	if principal == nil {
		return nil, fmt.Errorf("Principal not returned for LDAP")
	}
	return principal, nil
}

func (l *LClient) attributesToPrincipal(attribs []*ldapv2.EntryAttribute, dnStr string, scope string, config *v3.ActiveDirectoryConfig) (*v3.Principal, error) {
	var externalIDType, accountName, externalID, login, kind string
	externalID = dnStr
	externalIDType = scope

	if ldap.IsType(attribs, config.UserObjectClass) {
		for _, attr := range attribs {
			if attr.Name == config.UserNameField {
				if len(attr.Values) != 0 {
					accountName = attr.Values[0]
				} else {
					accountName = externalID
				}
			}
			if attr.Name == config.UserLoginField {
				login = attr.Values[0]
			}
		}
		kind = "user"
	} else if ldap.IsType(attribs, config.GroupObjectClass) {
		for _, attr := range attribs {
			if attr.Name == config.GroupNameField {
				if len(attr.Values) != 0 {
					accountName = attr.Values[0]
				} else {
					accountName = externalID
				}
			}
			if attr.Name == config.UserLoginField {
				if len(attr.Values) > 0 && attr.Values[0] != "" {
					login = attr.Values[0]
				}
			} else {
				login = accountName
			}
		}
		kind = "group"
	} else {
		logrus.Errorf("Failed to get attributes for %s", dnStr)
		return nil, nil
	}

	principal := &v3.Principal{
		ObjectMeta:  metav1.ObjectMeta{Name: externalIDType + "://" + login}, //TODO: Change to {externalIDType + "://" + externalID} when user auto-creation is added
		DisplayName: accountName,
		LoginName:   login,
		Kind:        kind,
	}

	return principal, nil
}

///Search

//SearchPrincipals returns the principal by name
func (l *LClient) SearchPrincipals(name string, exactMatch bool, config *v3.ActiveDirectoryConfig) ([]v3.Principal, error) {
	c := l.ConstantsConfig
	principals := []v3.Principal{}
	for _, scope := range c.Scopes {
		principalList, err := l.searchPrincipals(name, scope, exactMatch, config)
		if err != nil {
			return []v3.Principal{}, err
		}
		principals = append(principals, principalList...)
	}
	return principals, nil
}

func (l *LClient) searchPrincipals(name string, scope string, exactMatch bool, config *v3.ActiveDirectoryConfig) ([]v3.Principal, error) {
	c := l.ConstantsConfig
	name = ldap.EscapeLDAPSearchFilter(name)
	if strings.EqualFold(c.UserScope, scope) {
		return l.searchUser(name, exactMatch, config)
	} else if strings.EqualFold(c.GroupScope, scope) {
		return l.searchGroup(name, exactMatch, config)
	} else {
		return nil, fmt.Errorf("Invalid scope")
	}
}

func (l *LClient) searchUser(name string, exactMatch bool, config *v3.ActiveDirectoryConfig) ([]v3.Principal, error) {
	c := l.ConstantsConfig
	var query string
	if exactMatch {
		query = "(&(" + config.UserSearchField + "=" + name + ")(" + c.ObjectClassAttribute + "=" +
			config.UserObjectClass + "))"
	} else {
		query = "(&(" + config.UserSearchField + "=*" + name + "*)(" + c.ObjectClassAttribute + "=" +
			config.UserObjectClass + "))"
	}
	logrus.Debugf("LDAPProvider searchUser query: %s", query)
	return l.searchLdap(query, c.UserScope, config)
}

func (l *LClient) searchGroup(name string, exactMatch bool, config *v3.ActiveDirectoryConfig) ([]v3.Principal, error) {
	c := l.ConstantsConfig
	var query string
	if exactMatch {
		query = "(&(" + config.GroupSearchField + "=" + name + ")(" + c.ObjectClassAttribute + "=" +
			config.GroupObjectClass + "))"
	} else {
		query = "(&(" + config.GroupSearchField + "=*" + name + "*)(" + c.ObjectClassAttribute + "=" +
			config.GroupObjectClass + "))"
	}
	logrus.Debugf("LDAPProvider searchGroup query: %s", query)
	return l.searchLdap(query, c.GroupScope, config)
}

func (l *LClient) searchLdap(query string, scope string, config *v3.ActiveDirectoryConfig) ([]v3.Principal, error) {
	c := l.ConstantsConfig
	principals := []v3.Principal{}
	var search *ldapv2.SearchRequest

	searchDomain := config.Domain
	if strings.EqualFold(c.UserScope, scope) {
		search = ldapv2.NewSearchRequest(searchDomain,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			query,
			ldap.GetUserSearchAttributes(config, l.ConstantsConfig), nil)
	} else {
		if config.GroupSearchDomain != "" {
			searchDomain = config.GroupSearchDomain
		}
		search = ldapv2.NewSearchRequest(searchDomain,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			query,
			ldap.GetGroupSearchAttributes(config, l.ConstantsConfig), nil)
	}

	lConn, err := ldap.NewLDAPConn(config, l.ConstantsConfig)
	if err != nil {
		return []v3.Principal{}, fmt.Errorf("Error %v creating connection", err)
	}
	// Bind before query
	serviceAccountUsername := ldap.GetUserExternalID(config.ServiceAccountUsername, config.LoginDomain)
	err = lConn.Bind(serviceAccountUsername, config.ServiceAccountPassword)
	if err != nil {
		return nil, fmt.Errorf("Error %v in ldap bind", err)
	}
	defer lConn.Close()

	results, err := lConn.Search(search)
	if err != nil {
		ldapErr, ok := reflect.ValueOf(err).Interface().(*ldapv2.Error)
		if ok && ldapErr.ResultCode != ldapv2.LDAPResultNoSuchObject {
			return []v3.Principal{}, fmt.Errorf("When searching ldap, Failed to search: %s, error: %#v", query, err)
		}
	}

	for i := 0; i < len(results.Entries); i++ {
		entry := results.Entries[i]
		principal, err := l.attributesToPrincipal(entry.Attributes, results.Entries[i].DN, scope, config)
		if err != nil {
			return []v3.Principal{}, err
		}
		principals = append(principals, *principal)
	}

	return principals, nil
}

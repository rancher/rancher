package activedirectory

import (
	"crypto/x509"
	"fmt"
	"reflect"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/sirupsen/logrus"
	ldapv2 "gopkg.in/ldap.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *adProvider) loginUser(adCredential *v3public.BasicLogin, config *v3.ActiveDirectoryConfig, caPool *x509.CertPool) (v3.Principal, []v3.Principal, map[string]string, error) {
	logrus.Debug("Now generating Ldap token")

	username := adCredential.Username
	password := adCredential.Password
	externalID := ldap.GetUserExternalID(username, config.DefaultLoginDomain)

	lConn, err := ldap.NewLDAPConn(config, caPool)
	if err != nil {
		return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.ServerError, "error creating connection")
	}
	defer lConn.Close()

	if !config.Enabled { // TODO testing for enabled here might not be correct. Might be better to pass in an explicit testSvcAccount bool
		logrus.Debug("Bind service account username password")
		sausername := ldap.GetUserExternalID(config.ServiceAccountUsername, config.DefaultLoginDomain)
		err = lConn.Bind(sausername, config.ServiceAccountPassword)
		if err != nil {
			if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) {
				return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed")
			}
			return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.ServerError, "server error while authenticating")
		}
	}

	logrus.Debug("Binding username password")
	err = lConn.Bind(externalID, password)
	if err != nil {
		if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) {
			return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed")
		}
		return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.ServerError, "server error while authenticating")
	}

	samName := username
	if strings.Contains(username, "\\") {
		samName = strings.SplitN(username, "\\\\", 2)[1]
	}
	query := "(" + config.UserLoginAttribute + "=" + samName + ")"
	logrus.Debugf("LDAP Search query: {%s}", query)
	search := ldapv2.NewSearchRequest(config.UserSearchBase,
		ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
		query,
		ldap.GetUserSearchAttributes(MemberOfAttribute, ObjectClassAttribute, config), nil)

	userPrincipal, groupPrincipals, err := p.userRecord(search, lConn, config, caPool)
	if err != nil {
		return v3.Principal{}, nil, nil, err
	}

	allowed, err := p.userMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipal, groupPrincipals)
	if err != nil {
		return v3.Principal{}, nil, nil, err
	}
	if !allowed {
		return v3.Principal{}, nil, nil, httperror.NewAPIError(httperror.Unauthorized, "unauthorized")
	}

	return userPrincipal, groupPrincipals, map[string]string{}, err
}

func (p *adProvider) userRecord(search *ldapv2.SearchRequest, lConn *ldapv2.Conn, config *v3.ActiveDirectoryConfig, caPool *x509.CertPool) (v3.Principal, []v3.Principal, error) {
	result, err := lConn.Search(search)
	if err != nil {
		return v3.Principal{}, nil, err
	}

	if len(result.Entries) < 1 {
		return v3.Principal{}, nil, fmt.Errorf("Cannot locate user information for %s", search.Filter)
	} else if len(result.Entries) > 1 {
		return v3.Principal{}, nil, fmt.Errorf("ldap user search found more than one result")
	}

	return p.getPrincipalsFromSearchResult(result, config, caPool)
}

func (p *adProvider) getPrincipalsFromSearchResult(result *ldapv2.SearchResult, config *v3.ActiveDirectoryConfig, caPool *x509.CertPool) (v3.Principal, []v3.Principal, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal

	entry := result.Entries[0]
	if !ldap.HasPermission(entry.Attributes, config) {
		return v3.Principal{}, nil, fmt.Errorf("Permission denied")
	}

	memberOf := entry.GetAttributeValues(MemberOfAttribute)

	logrus.Debugf("ADConstants userMemberAttribute() {%s}", MemberOfAttribute)
	logrus.Debugf("SearchResult memberOf attribute {%s}", memberOf)

	isType := false
	objectClass := entry.GetAttributeValues(ObjectClassAttribute)
	for _, obj := range objectClass {
		if strings.EqualFold(string(obj), config.UserObjectClass) {
			isType = true
		}
	}
	if !isType {
		return v3.Principal{}, nil, nil
	}

	user, err := p.attributesToPrincipal(entry.Attributes, result.Entries[0].DN, UserScope, config)
	userPrincipal = *user
	if err != nil {
		return userPrincipal, groupPrincipals, err
	}

	if len(memberOf) != 0 {
		for _, attrib := range memberOf {
			group, err := p.getPrincipal(attrib, GroupScope, config, caPool)
			if err != nil {
				return userPrincipal, groupPrincipals, err
			}
			groupPrincipals = append(groupPrincipals, *group)
		}
	}
	return userPrincipal, groupPrincipals, nil
}

func (p *adProvider) getPrincipal(distinguishedName string, scope string, config *v3.ActiveDirectoryConfig, caPool *x509.CertPool) (*v3.Principal, error) {
	var search *ldapv2.SearchRequest
	if !slice.ContainsString(scopes, scope) {
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

	filter := "(" + ObjectClassAttribute + "=*)"
	logrus.Debugf("Query for getPrincipal(%s): %s", distinguishedName, filter)
	lConn, err := ldap.NewLDAPConn(config, caPool)
	if err != nil {
		return nil, fmt.Errorf("Error %v creating connection", err)
	}
	defer lConn.Close()
	// Bind before query
	// If service acc bind fails, and auth is on, return principal formed using DN
	serviceAccountUsername := ldap.GetUserExternalID(config.ServiceAccountUsername, config.DefaultLoginDomain)
	err = lConn.Bind(serviceAccountUsername, config.ServiceAccountPassword)

	if err != nil {
		if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) && config.Enabled {
			var kind string
			if strings.EqualFold(UserScope, scope) {
				kind = "user"
			} else if strings.EqualFold(GroupScope, scope) {
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

	if strings.EqualFold(UserScope, scope) {
		search = ldapv2.NewSearchRequest(distinguishedName,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			filter,
			ldap.GetUserSearchAttributes(MemberOfAttribute, ObjectClassAttribute, config), nil)
	} else {
		search = ldapv2.NewSearchRequest(distinguishedName,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			filter,
			ldap.GetGroupSearchAttributes(MemberOfAttribute, ObjectClassAttribute, config), nil)
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

	principal, err := p.attributesToPrincipal(entryAttributes, distinguishedName, scope, config)
	if err != nil {
		return nil, err
	}
	if principal == nil {
		return nil, fmt.Errorf("Principal not returned for LDAP")
	}
	return principal, nil
}

func (p *adProvider) attributesToPrincipal(attribs []*ldapv2.EntryAttribute, dnStr string, scope string, config *v3.ActiveDirectoryConfig) (*v3.Principal, error) {
	var externalIDType, accountName, externalID, login, kind string
	externalID = dnStr
	externalIDType = scope

	if ldap.IsType(attribs, config.UserObjectClass) {
		for _, attr := range attribs {
			if attr.Name == config.UserNameAttribute {
				if len(attr.Values) != 0 {
					accountName = attr.Values[0]
				} else {
					accountName = externalID
				}
			}
			if attr.Name == config.UserLoginAttribute {
				login = attr.Values[0]
			}
		}
		kind = "user"
	} else if ldap.IsType(attribs, config.GroupObjectClass) {
		for _, attr := range attribs {
			if attr.Name == config.GroupNameAttribute {
				if len(attr.Values) != 0 {
					accountName = attr.Values[0]
				} else {
					accountName = externalID
				}
			}
			if attr.Name == config.UserLoginAttribute {
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

func (p *adProvider) searchPrincipals(name, principalType string, exactMatch bool, config *v3.ActiveDirectoryConfig, caPool *x509.CertPool) ([]v3.Principal, error) {
	name = ldap.EscapeLDAPSearchFilter(name)

	var principals []v3.Principal

	if principalType == "" || principalType == "user" {
		princs, err := p.searchUser(name, exactMatch, config, caPool)
		if err != nil {
			return nil, err
		}
		principals = append(principals, princs...)
	}

	if principalType == "" || principalType == "group" {
		princs, err := p.searchGroup(name, exactMatch, config, caPool)
		if err != nil {
			return nil, err
		}
		principals = append(principals, princs...)
	}

	return principals, nil
}

func (p *adProvider) searchUser(name string, exactMatch bool, config *v3.ActiveDirectoryConfig, caPool *x509.CertPool) ([]v3.Principal, error) {
	var query string
	if exactMatch {
		query = "(&(" + config.UserSearchAttribute + "=" + name + ")(" + ObjectClassAttribute + "=" +
			config.UserObjectClass + "))"
	} else {
		query = "(&(" + config.UserSearchAttribute + "=*" + name + "*)(" + ObjectClassAttribute + "=" +
			config.UserObjectClass + "))"
	}
	logrus.Debugf("LDAPProvider searchUser query: %s", query)
	return p.searchLdap(query, UserScope, config, caPool)
}

func (p *adProvider) searchGroup(name string, exactMatch bool, config *v3.ActiveDirectoryConfig, caPool *x509.CertPool) ([]v3.Principal, error) {
	var query string
	if exactMatch {
		query = "(&(" + config.GroupSearchAttribute + "=" + name + ")(" + ObjectClassAttribute + "=" +
			config.GroupObjectClass + "))"
	} else {
		query = "(&(" + config.GroupSearchAttribute + "=*" + name + "*)(" + ObjectClassAttribute + "=" +
			config.GroupObjectClass + "))"
	}
	logrus.Debugf("LDAPProvider searchGroup query: %s", query)
	return p.searchLdap(query, GroupScope, config, caPool)
}

func (p *adProvider) searchLdap(query string, scope string, config *v3.ActiveDirectoryConfig, caPool *x509.CertPool) ([]v3.Principal, error) {
	principals := []v3.Principal{}
	var search *ldapv2.SearchRequest

	searchDomain := config.UserSearchBase
	if strings.EqualFold(UserScope, scope) {
		search = ldapv2.NewSearchRequest(searchDomain,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			query,
			ldap.GetUserSearchAttributes(MemberOfAttribute, ObjectClassAttribute, config), nil)
	} else {
		if config.GroupSearchBase != "" {
			searchDomain = config.GroupSearchBase
		}
		search = ldapv2.NewSearchRequest(searchDomain,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			query,
			ldap.GetGroupSearchAttributes(MemberOfAttribute, ObjectClassAttribute, config), nil)
	}

	lConn, err := ldap.NewLDAPConn(config, caPool)
	if err != nil {
		return []v3.Principal{}, fmt.Errorf("Error %v creating connection", err)
	}
	// Bind before query
	serviceAccountUsername := ldap.GetUserExternalID(config.ServiceAccountUsername, config.DefaultLoginDomain)
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
		principal, err := p.attributesToPrincipal(entry.Attributes, results.Entries[i].DN, scope, config)
		if err != nil {
			return []v3.Principal{}, err
		}
		principals = append(principals, *principal)
	}

	return principals, nil
}

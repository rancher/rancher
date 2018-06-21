package ldap

import (
	"crypto/x509"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/sirupsen/logrus"
	ldapv2 "gopkg.in/ldap.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var operationalAttrList = []string{"1.1", "+", "*"}

func (p *ldapProvider) loginUser(credential *v3public.BasicLogin, config *v3.LdapConfig, caPool *x509.CertPool) (v3.Principal, []v3.Principal, map[string]string, error) {
	logrus.Debug("Now generating Ldap token")

	username := credential.Username
	password := credential.Password

	if password == "" {
		return v3.Principal{}, nil, nil, httperror.NewAPIError(httperror.MissingRequired, "password not provided")
	}

	lConn, err := p.ldapConnection(config, caPool)
	if err != nil {
		return v3.Principal{}, nil, nil, err
	}
	defer lConn.Close()

	enabled := config.Enabled
	serviceAccountPassword := config.ServiceAccountPassword
	serviceAccountUserName := config.ServiceAccountDistinguishedName
	ldap.AuthenticateServiceAccountUser(enabled, serviceAccountPassword, serviceAccountUserName, lConn)

	logrus.Debug("Binding username password")

	searchRequest := ldapv2.NewSearchRequest(config.UserSearchBase,
		ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=%v)(%v=%v))", config.UserObjectClass, config.UserLoginAttribute, ldap.EscapeLDAPSearchFilter(username)),
		p.getUserSearchAttributes(config), nil)
	result, err := lConn.Search(searchRequest)
	if err != nil {
		return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed") // need to reload this error
	}

	if len(result.Entries) < 1 {
		return v3.Principal{}, nil, nil, fmt.Errorf("Cannot locate user information for %s", searchRequest.Filter)
	} else if len(result.Entries) > 1 {
		return v3.Principal{}, nil, nil, fmt.Errorf("ldap user search found more than one result")
	}

	userDN := result.Entries[0].DN //userDN is externalID
	err = lConn.Bind(userDN, password)
	if err != nil {
		if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) {
			return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed")
		}
		return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.ServerError, "server error while authenticating")
	}

	searchOpRequest := ldapv2.NewSearchRequest(userDN,
		ldapv2.ScopeBaseObject, ldapv2.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(objectClass=%v)", config.UserObjectClass),
		operationalAttrList, nil)
	opResult, err := lConn.Search(searchOpRequest)
	if err != nil {
		return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed") // need to reload this error
	}
	userPrincipal, groupPrincipals, err := p.getPrincipalsFromSearchResult(result, opResult, config, lConn)
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

func (p *ldapProvider) getPrincipalsFromSearchResult(result *ldapv2.SearchResult, opResult *ldapv2.SearchResult, config *v3.LdapConfig, lConn *ldapv2.Conn) (v3.Principal, []v3.Principal, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal
	var nonDupGroupPrincipals []v3.Principal
	var userScope, groupScope string
	entry := result.Entries[0]
	userAttributes := entry.Attributes

	if !p.permissionCheck(userAttributes, config) {
		return v3.Principal{}, nil, fmt.Errorf("Permission denied")
	}

	logrus.Debugf("getPrincipals: user attributes: %v ", userAttributes)

	userMemberAttribute := entry.GetAttributeValues(config.UserMemberAttribute)
	if len(userMemberAttribute) == 0 {
		userMemberAttribute = opResult.Entries[0].GetAttributeValues(config.UserMemberAttribute)
	}

	logrus.Debugf("SearchResult memberOf attribute {%s}", userMemberAttribute)

	isType := false
	objectClass := entry.GetAttributeValues("objectClass")
	for _, obj := range objectClass {
		if strings.EqualFold(string(obj), config.UserObjectClass) {
			isType = true
		}
	}
	if !isType {
		return v3.Principal{}, nil, nil
	}

	userScope = p.userScope
	groupScope = p.groupScope

	user, err := p.attributesToPrincipal(entry.Attributes, result.Entries[0].DN, userScope, config)
	userPrincipal = *user
	if err != nil {
		return userPrincipal, groupPrincipals, err
	}

	if len(userMemberAttribute) > 0 {
		for _, dn := range userMemberAttribute {
			query := fmt.Sprintf("(&(%v=%v)(objectClass=%v))", config.GroupDNAttribute, dn, config.GroupObjectClass)
			userMemberGroupPrincipals, err := p.searchLdap(query, groupScope, config, lConn)
			groupPrincipals = append(groupPrincipals, userMemberGroupPrincipals...)
			if err != nil {
				return userPrincipal, groupPrincipals, err
			}
		}
	}

	opEntry := opResult.Entries[0]
	opAttributes := opEntry.Attributes

	groupMemberUserAttribute := entry.GetAttributeValues(config.GroupMemberUserAttribute)
	if len(groupMemberUserAttribute) == 0 {
		for _, attr := range opAttributes {
			if attr.Name == config.GroupMemberUserAttribute {
				groupMemberUserAttribute = attr.Values
			}
		}
	}

	if len(groupMemberUserAttribute) > 0 {
		query := fmt.Sprintf("(&(%v=%v)(objectClass=%v))", config.GroupMemberMappingAttribute, ldapv2.EscapeFilter(groupMemberUserAttribute[0]), config.GroupObjectClass)
		newGroupPrincipals, err := p.searchLdap(query, groupScope, config, lConn)
		//deduplicate groupprincipals get from userMemberAttribute
		nonDupGroupPrincipals = p.findNonDuplicateGroupPrincipals(newGroupPrincipals, groupPrincipals, nonDupGroupPrincipals)
		groupPrincipals = append(groupPrincipals, nonDupGroupPrincipals...)
		if err != nil {
			return userPrincipal, groupPrincipals, err
		}
	}

	return userPrincipal, groupPrincipals, nil
}

func (p *ldapProvider) findNonDuplicateGroupPrincipals(newGroupPrincipals []v3.Principal, groupPrincipals []v3.Principal, nonDupGroupPrincipals []v3.Principal) []v3.Principal {
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

func (p *ldapProvider) getPrincipal(distinguishedName string, scope string, config *v3.LdapConfig, caPool *x509.CertPool) (*v3.Principal, error) {
	var search *ldapv2.SearchRequest
	var filter string
	if !slice.ContainsString(freeIpaScopes, scope) && !slice.ContainsString(openLdapScopes, scope) {
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

	if !ldap.IsType(attribs, scope) && !p.permissionCheck(attribs, config) {
		logrus.Errorf("Failed to get object %v", distinguishedName)
		return nil, nil
	}

	userType := strings.Split(scope, "_")[1]
	if strings.EqualFold("user", userType) {
		filter = fmt.Sprintf("(objectClass=%v)", config.UserObjectClass)
	} else {
		filter = fmt.Sprintf("(objectClass=%v)", config.GroupObjectClass)
	}

	logrus.Debugf("Query for getPrincipal(%v): %v", distinguishedName, filter)

	lConn, err := p.ldapConnection(config, caPool)
	if err != nil {
		return nil, err
	}
	defer lConn.Close()
	// Bind before query
	// If service acc bind fails, and auth is on, return principal formed using DN
	serviceAccountUsername := ldap.GetUserExternalID(config.ServiceAccountDistinguishedName, "")
	err = lConn.Bind(serviceAccountUsername, config.ServiceAccountPassword)

	if err != nil {
		if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) && config.Enabled {
			var kind string
			if strings.EqualFold("user", userType) {
				kind = "user"
			} else if strings.EqualFold("group", userType) {
				kind = "group"
			}
			principal := &v3.Principal{
				ObjectMeta:    metav1.ObjectMeta{Name: scope + "://" + distinguishedName},
				DisplayName:   distinguishedName,
				LoginName:     distinguishedName,
				PrincipalType: kind,
			}

			return principal, nil
		}
		return nil, fmt.Errorf("Error in ldap bind: %v", err)
	}

	if strings.EqualFold("user", userType) {
		search = ldapv2.NewSearchRequest(distinguishedName,
			ldapv2.ScopeBaseObject, ldapv2.NeverDerefAliases, 0, 0, false,
			filter,
			p.getUserSearchAttributes(config), nil)
	} else {
		search = ldapv2.NewSearchRequest(distinguishedName,
			ldapv2.ScopeBaseObject, ldapv2.NeverDerefAliases, 0, 0, false,
			filter,
			p.getGroupSearchAttributes(config), nil)
	}

	result, err := lConn.Search(search)
	if err != nil {
		if ldapErr, ok := err.(*ldapv2.Error); ok && ldapErr.ResultCode == 32 {
			return nil, httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("%v not found", distinguishedName))
		}
		return nil, httperror.WrapAPIError(errors.Wrapf(err, "server returned error for search %v %v: %v", search.BaseDN, filter, err), httperror.ServerError, "Internal server error")
	}

	if len(result.Entries) < 1 {
		return nil, fmt.Errorf("No identities can be retrieved")
	} else if len(result.Entries) > 1 {
		return nil, fmt.Errorf("More than one result found")
	}

	entry := result.Entries[0]
	entryAttributes := entry.Attributes

	if !p.permissionCheck(entry.Attributes, config) {
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

func (p *ldapProvider) attributesToPrincipal(attribs []*ldapv2.EntryAttribute, dnStr string, scope string, config *v3.LdapConfig) (*v3.Principal, error) {
	var externalIDType, accountName, externalID, login string
	var principal *v3.Principal
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
				if len(attr.Values) > 0 && attr.Values[0] != "" {
					login = attr.Values[0]
				}
			}
		}
		if login == "" {
			login = accountName
		}
		principal = &v3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: externalIDType + "://" + externalID},
			DisplayName:   accountName,
			LoginName:     login,
			PrincipalType: "user",
			Me:            true,
			Provider:      p.providerName,
		}
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
			}
		}
		if login == "" {
			login = accountName
		}
		principal = &v3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: externalIDType + "://" + externalID},
			DisplayName:   accountName,
			LoginName:     login,
			PrincipalType: "group",
			MemberOf:      true,
			Provider:      p.providerName,
		}
	} else {
		logrus.Errorf("Failed to get attributes for %s", dnStr)
		return nil, nil
	}

	return principal, nil
}

func (p *ldapProvider) searchPrincipals(name, principalType string, config *v3.LdapConfig, lConn *ldapv2.Conn) ([]v3.Principal, error) {
	name = ldap.EscapeLDAPSearchFilter(name)

	var principals []v3.Principal

	if principalType == "" || principalType == "user" {
		princs, err := p.searchUser(name, config, lConn)
		if err != nil {
			return nil, err
		}
		principals = append(principals, princs...)
	}

	if principalType == "" || principalType == "group" {
		princs, err := p.searchGroup(name, config, lConn)
		if err != nil {
			return nil, err
		}
		principals = append(principals, princs...)
	}

	return principals, nil
}

func (p *ldapProvider) searchUser(name string, config *v3.LdapConfig, lConn *ldapv2.Conn) ([]v3.Principal, error) {
	var userScope string
	srchAttributes := strings.Split(config.UserSearchAttribute, "|")
	query := fmt.Sprintf("(&(objectClass=%v)", config.UserObjectClass)
	srchAttrs := "(|"
	for _, attr := range srchAttributes {
		srchAttrs += fmt.Sprintf("(%v=%v*)", attr, name)
	}
	query += srchAttrs + "))"
	logrus.Debugf("%s searchUser query: %s", p.providerName, query)
	userScope = p.userScope

	return p.searchLdap(query, userScope, config, lConn)
}

func (p *ldapProvider) searchGroup(name string, config *v3.LdapConfig, lConn *ldapv2.Conn) ([]v3.Principal, error) {
	var groupScope string
	query := fmt.Sprintf("(&(%v=*%v*)(objectClass=%v))", config.GroupSearchAttribute, name, config.GroupObjectClass)

	logrus.Debugf("%s searchGroup query: %s", p.providerName, query)

	groupScope = p.groupScope

	return p.searchLdap(query, groupScope, config, lConn)
}

func (p *ldapProvider) searchLdap(query string, scope string, config *v3.LdapConfig, lConn *ldapv2.Conn) ([]v3.Principal, error) {
	principals := []v3.Principal{}
	var search *ldapv2.SearchRequest

	userType := strings.Split(scope, "_")[1]
	searchDomain := config.UserSearchBase
	if strings.EqualFold("user", userType) {
		search = ldapv2.NewSearchRequest(searchDomain,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			query,
			p.getUserSearchAttributes(config), nil)
	} else {
		if config.GroupSearchBase != "" {
			searchDomain = config.GroupSearchBase
		}
		search = ldapv2.NewSearchRequest(searchDomain,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			query,
			p.getGroupSearchAttributes(config), nil)
	}

	// Bind before query
	serviceAccountUsername := ldap.GetUserExternalID(config.ServiceAccountDistinguishedName, "")
	err := lConn.Bind(serviceAccountUsername, config.ServiceAccountPassword)
	if err != nil {
		return nil, fmt.Errorf("Error %v in ldap bind", err)
	}

	results, err := lConn.SearchWithPaging(search, 1000)
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

func (p *ldapProvider) ldapConnection(config *v3.LdapConfig, caPool *x509.CertPool) (*ldapv2.Conn, error) {
	servers := config.Servers
	TLS := config.TLS
	port := config.Port
	connectionTimeout := config.ConnectionTimeout
	return ldap.NewLDAPConn(servers, TLS, port, connectionTimeout, caPool)
}

func (p *ldapProvider) permissionCheck(attributes []*ldapv2.EntryAttribute, config *v3.LdapConfig) bool {
	userObjectClass := config.UserObjectClass
	userEnabledAttribute := config.UserEnabledAttribute
	userDisabledBitMask := config.UserDisabledBitMask
	return ldap.HasPermission(attributes, userObjectClass, userEnabledAttribute, userDisabledBitMask)
}

func (p *ldapProvider) getUserSearchAttributes(config *v3.LdapConfig) []string {
	ldapConfig := &v3.LdapConfig{
		UserMemberAttribute:  config.UserMemberAttribute,
		UserObjectClass:      config.UserObjectClass,
		UserLoginAttribute:   config.UserLoginAttribute,
		UserNameAttribute:    config.UserNameAttribute,
		UserEnabledAttribute: config.UserEnabledAttribute}
	return ldap.GetUserSearchAttributesForLDAP(ldapConfig)
}

func (p *ldapProvider) getGroupSearchAttributes(config *v3.LdapConfig) []string {
	ldapConfig := &v3.LdapConfig{
		GroupMemberUserAttribute: config.GroupMemberUserAttribute,
		GroupObjectClass:         config.GroupObjectClass,
		UserLoginAttribute:       config.UserLoginAttribute,
		GroupNameAttribute:       config.GroupNameAttribute,
		GroupSearchAttribute:     config.GroupSearchAttribute}
	return ldap.GetGroupSearchAttributesForLDAP(ldapConfig)
}

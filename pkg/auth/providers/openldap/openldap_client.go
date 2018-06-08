package openldap

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

func (p *openldapProvider) loginUser(credential *v3public.BasicLogin, config *v3.OpenLDAPConfig, caPool *x509.CertPool) (v3.Principal, []v3.Principal, map[string]string, error) {
	logrus.Debug("Now generating Ldap token")

	username := credential.Username
	password := credential.Password

	if password == "" {
		return v3.Principal{}, nil, nil, httperror.NewAPIError(httperror.MissingRequired, "password not provided")
	}

	lConn, err := ldap.NewLDAPConnForOpenLDAP(config, caPool)
	if err != nil {
		return v3.Principal{}, nil, nil, err
	}
	defer lConn.Close()

	if !config.Enabled { // TODO testing for enabled here might not be correct. Might be better to pass in an explicit testSvcAccount bool
		logrus.Debug("Bind service account username password")
		if config.ServiceAccountPassword == "" {
			return v3.Principal{}, nil, nil, httperror.NewAPIError(httperror.MissingRequired, "service account password not provided")
		}
		sausername := ldap.GetUserExternalID(config.ServiceAccountUsername, "")
		err = lConn.Bind(sausername, config.ServiceAccountPassword)
		if err != nil {
			if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) {
				return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed")
			}
			return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.ServerError, "server error while authenticating")
		}
	}

	logrus.Debug("Binding username password")
	searchRequest := ldapv2.NewSearchRequest(config.UserSearchBase,
		ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
		"(&("+ObjectClassAttribute+"="+config.UserObjectClass+")(uid="+username+"))", ldap.GetUserSearchAttributesForOpenLDAP(config), nil)
	result, err := lConn.Search(searchRequest)
	if err != nil {
		return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed") // need to reload this error
	}

	if len(result.Entries) < 1 {
		return v3.Principal{}, nil, nil, fmt.Errorf("Cannot locate user information for %s", searchRequest.Filter)
	} else if len(result.Entries) > 1 {
		return v3.Principal{}, nil, nil, fmt.Errorf("ldap user search found more than one result")
	}

	userDN := result.Entries[0].DN //userdn is externalID
	err = lConn.Bind(userDN, password)
	if err != nil {
		if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) {
			return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed")
		}
		return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.ServerError, "server error while authenticating")
	}

	operationalAttrList := []string{"1.1", "+", "*"}
	searchOpRequest := ldapv2.NewSearchRequest(userDN,
		ldapv2.ScopeBaseObject, ldapv2.NeverDerefAliases, 0, 0, false,
		"("+ObjectClassAttribute+"="+config.UserObjectClass+")",
		operationalAttrList, nil)
	opResult, err := lConn.Search(searchOpRequest)
	if err != nil {
		return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed") // need to reload this error
	}
	userPrincipal, groupPrincipals, err := p.getPrincipalsFromSearchResult(result, opResult, config, caPool)
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

func (p *openldapProvider) getPrincipalsFromSearchResult(result *ldapv2.SearchResult, opResult *ldapv2.SearchResult, config *v3.OpenLDAPConfig, caPool *x509.CertPool) (v3.Principal, []v3.Principal, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal
	var nonDupGroupPrincipals []v3.Principal

	entry := result.Entries[0]
	userAttributes := entry.Attributes
	if !ldap.HasPermissionForOpenLDAP(userAttributes, config) {
		return v3.Principal{}, nil, fmt.Errorf("Permission denied")
	}

	logrus.Debugf("getPrincipals: user attributes: %v ", userAttributes)

	userMemberAttribute := entry.GetAttributeValues(config.UserMemberAttribute)
	if userMemberAttribute == nil {
		userMemberAttribute = opResult.Entries[0].GetAttributeValues(config.UserMemberAttribute)
	}

	logrus.Debugf("SearchResult memberOf attribute {%s}", userMemberAttribute)

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

	if userMemberAttribute != nil {
		for _, dn := range userMemberAttribute {
			query := "(&(" + config.GroupDNAttribute + "=" + dn + ")(" + ObjectClassAttribute + "=" +
				config.GroupObjectClass + "))"
			userMemberGroupPrincipals, err := p.searchLdap(query, GroupScope, config, caPool)
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
		query := "(&(" + config.GroupMemberMappingAttribute + "=" + groupMemberUserAttribute[0] + ")(" + ObjectClassAttribute + "=" +
			config.GroupObjectClass + "))"
		newGroupPrincipals, err := p.searchLdap(query, GroupScope, config, caPool)
		//deduplicate groupprincipals get from userMemberAttribute
		nonDupGroupPrincipals = p.findNonDuplicateGroupPrincipals(newGroupPrincipals, groupPrincipals, nonDupGroupPrincipals)
		groupPrincipals = append(groupPrincipals, nonDupGroupPrincipals...)
		if err != nil {
			return userPrincipal, groupPrincipals, err
		}
	}

	return userPrincipal, groupPrincipals, nil
}

func (p *openldapProvider) findNonDuplicateGroupPrincipals(newGroupPrincipals []v3.Principal, groupPrincipals []v3.Principal, nonDupGroupPrincipals []v3.Principal) []v3.Principal {
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

func (p *openldapProvider) getPrincipal(distinguishedName string, scope string, config *v3.OpenLDAPConfig, caPool *x509.CertPool) (*v3.Principal, error) {
	var search *ldapv2.SearchRequest
	var filter string
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

	if !ldap.IsType(attribs, scope) && !ldap.HasPermissionForOpenLDAP(attribs, config) {
		logrus.Errorf("Failed to get object %s", distinguishedName)
		return nil, nil
	}

	if strings.EqualFold(UserScope, scope) {
		filter = "(" + ObjectClassAttribute + "=" + config.UserObjectClass + ")"
	} else {
		filter = "(" + ObjectClassAttribute + "=" + config.GroupObjectClass + ")"
	}

	logrus.Debugf("Query for getPrincipal(%s): %s", distinguishedName, filter)
	lConn, err := ldap.NewLDAPConnForOpenLDAP(config, caPool)
	if err != nil {
		return nil, err
	}
	defer lConn.Close()
	// Bind before query
	// If service acc bind fails, and auth is on, return principal formed using DN
	serviceAccountUsername := ldap.GetUserExternalID(config.ServiceAccountUsername, "")
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
				ObjectMeta:    metav1.ObjectMeta{Name: scope + "://" + distinguishedName},
				DisplayName:   distinguishedName,
				LoginName:     distinguishedName,
				PrincipalType: kind,
			}

			return principal, nil
		}
		return nil, fmt.Errorf("Error in ldap bind: %v", err)
	}

	if strings.EqualFold(UserScope, scope) {
		search = ldapv2.NewSearchRequest(distinguishedName,
			ldapv2.ScopeBaseObject, ldapv2.NeverDerefAliases, 0, 0, false,
			filter,
			ldap.GetUserSearchAttributesForOpenLDAP(config), nil)
	} else {
		search = ldapv2.NewSearchRequest(distinguishedName,
			ldapv2.ScopeBaseObject, ldapv2.NeverDerefAliases, 0, 0, false,
			filter,
			ldap.GetGroupSearchAttributesForOpenLDAP(config), nil)
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
	if !ldap.HasPermissionForOpenLDAP(entry.Attributes, config) {
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

func (p *openldapProvider) attributesToPrincipal(attribs []*ldapv2.EntryAttribute, dnStr string, scope string, config *v3.OpenLDAPConfig) (*v3.Principal, error) {
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
				login = attr.Values[0]
			}
		}
		principal = &v3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: externalIDType + "://" + externalID},
			DisplayName:   accountName,
			LoginName:     login,
			PrincipalType: "user",
			Me:            true,
			Provider:      Name,
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
			} else {
				login = accountName
			}
		}
		principal = &v3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: externalIDType + "://" + externalID},
			DisplayName:   accountName,
			LoginName:     login,
			PrincipalType: "group",
			MemberOf:      true,
			Provider:      Name,
		}
	} else {
		logrus.Errorf("Failed to get attributes for %s", dnStr)
		return nil, nil
	}

	return principal, nil
}

func (p *openldapProvider) searchPrincipals(name, principalType string, config *v3.OpenLDAPConfig, caPool *x509.CertPool) ([]v3.Principal, error) {
	name = ldap.EscapeLDAPSearchFilter(name)

	var principals []v3.Principal

	if principalType == "" || principalType == "user" {
		princs, err := p.searchUser(name, config, caPool)
		if err != nil {
			return nil, err
		}
		principals = append(principals, princs...)
	}

	if principalType == "" || principalType == "group" {
		princs, err := p.searchGroup(name, config, caPool)
		if err != nil {
			return nil, err
		}
		principals = append(principals, princs...)
	}

	return principals, nil
}

func (p *openldapProvider) searchUser(name string, config *v3.OpenLDAPConfig, caPool *x509.CertPool) ([]v3.Principal, error) {
	query := "(&(" + config.UserSearchAttribute + "=*" + name + "*)(" + ObjectClassAttribute + "=" +
		config.UserObjectClass + "))"
	logrus.Debugf("OpenLDAPProvider searchUser query: %s", query)
	return p.searchLdap(query, UserScope, config, caPool)
}

func (p *openldapProvider) searchGroup(name string, config *v3.OpenLDAPConfig, caPool *x509.CertPool) ([]v3.Principal, error) {
	query := "(&(" + config.GroupSearchAttribute + "=*" + name + "*)(" + ObjectClassAttribute + "=" +
		config.GroupObjectClass + "))"
	logrus.Debugf("OpenLDAPProvider searchGroup query: %s", query)
	return p.searchLdap(query, GroupScope, config, caPool)
}

func (p *openldapProvider) searchLdap(query string, scope string, config *v3.OpenLDAPConfig, caPool *x509.CertPool) ([]v3.Principal, error) {
	principals := []v3.Principal{}
	var search *ldapv2.SearchRequest

	searchDomain := config.UserSearchBase
	if strings.EqualFold(UserScope, scope) {
		search = ldapv2.NewSearchRequest(searchDomain,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			query,
			ldap.GetUserSearchAttributesForOpenLDAP(config), nil)
	} else {
		if config.GroupSearchBase != "" {
			searchDomain = config.GroupSearchBase
		}
		search = ldapv2.NewSearchRequest(searchDomain,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			query,
			ldap.GetGroupSearchAttributesForOpenLDAP(config), nil)
	}

	lConn, err := ldap.NewLDAPConnForOpenLDAP(config, caPool)
	if err != nil {
		return []v3.Principal{}, err
	}
	// Bind before query
	serviceAccountUsername := ldap.GetUserExternalID(config.ServiceAccountUsername, "")
	err = lConn.Bind(serviceAccountUsername, config.ServiceAccountPassword)
	if err != nil {
		return nil, fmt.Errorf("Error %v in ldap bind", err)
	}
	defer lConn.Close()

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

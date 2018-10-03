package activedirectory

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

func (p *adProvider) loginUser(adCredential *v3public.BasicLogin, config *v3.ActiveDirectoryConfig, caPool *x509.CertPool, testServiceAccountBind bool) (v3.Principal, []v3.Principal, error) {
	logrus.Debug("Now generating Ldap token")

	username := adCredential.Username
	password := adCredential.Password
	if password == "" {
		return v3.Principal{}, nil, httperror.NewAPIError(httperror.MissingRequired, "password not provided")
	}
	externalID := ldap.GetUserExternalID(username, config.DefaultLoginDomain)

	lConn, err := p.ldapConnection(config, caPool)
	if err != nil {
		return v3.Principal{}, nil, err
	}
	defer lConn.Close()

	serviceAccountPassword := config.ServiceAccountPassword
	serviceAccountUserName := config.ServiceAccountUsername
	if testServiceAccountBind {
		err = ldap.AuthenticateServiceAccountUser(serviceAccountPassword, serviceAccountUserName, config.DefaultLoginDomain, lConn)
		if err != nil {
			return v3.Principal{}, nil, err
		}
	}

	logrus.Debug("Binding username password")
	err = lConn.Bind(externalID, password)
	if err != nil {
		if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) {
			return v3.Principal{}, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed")
		}
		return v3.Principal{}, nil, httperror.WrapAPIError(err, httperror.ServerError, "server error while authenticating")
	}

	samName := username
	if strings.Contains(username, `\`) {
		samName = strings.SplitN(username, `\`, 2)[1]
	}
	query := fmt.Sprintf("(%v=%v)", config.UserLoginAttribute, ldapv2.EscapeFilter(samName))
	logrus.Debugf("LDAP Search query: {%s}", query)
	search := ldapv2.NewSearchRequest(config.UserSearchBase,
		ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
		query,
		ldap.GetUserSearchAttributes(MemberOfAttribute, ObjectClass, config), nil)

	result, err := lConn.Search(search)
	if err != nil {
		return v3.Principal{}, nil, err
	}

	if len(result.Entries) < 1 {
		return v3.Principal{}, nil, fmt.Errorf("Cannot locate user information for %s", search.Filter)
	} else if len(result.Entries) > 1 {
		return v3.Principal{}, nil, fmt.Errorf("ldap user search found more than one result")
	}

	userPrincipal, groupPrincipals, err := p.getPrincipalsFromSearchResult(result, config, lConn)
	if err != nil {
		return v3.Principal{}, nil, err
	}

	allowed, err := p.userMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipal, groupPrincipals)
	if err != nil {
		return v3.Principal{}, nil, err
	}
	if !allowed {
		return v3.Principal{}, nil, httperror.NewAPIError(httperror.PermissionDenied, "Permission denied")
	}

	return userPrincipal, groupPrincipals, err
}

func (p *adProvider) getPrincipalsFromSearchResult(result *ldapv2.SearchResult, config *v3.ActiveDirectoryConfig, lConn *ldapv2.Conn) (v3.Principal, []v3.Principal, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal

	entry := result.Entries[0]

	if !p.permissionCheck(entry.Attributes, config) {
		return v3.Principal{}, nil, fmt.Errorf("Permission denied")
	}

	memberOf := entry.GetAttributeValues(MemberOfAttribute)

	logrus.Debugf("ADConstants userMemberAttribute() {%v}", MemberOfAttribute)
	logrus.Debugf("SearchResult memberOf attribute {%s}", memberOf)

	isType := false
	objectClass := entry.GetAttributeValues(ObjectClass)
	for _, obj := range objectClass {
		if strings.EqualFold(string(obj), config.UserObjectClass) {
			isType = true
		}
	}
	if !isType {
		return v3.Principal{}, nil, nil
	}

	user, err := ldap.AttributesToPrincipal(entry.Attributes, result.Entries[0].DN, UserScope, Name, config.UserObjectClass, config.UserNameAttribute, config.UserLoginAttribute, config.GroupObjectClass, config.GroupNameAttribute)
	if err != nil {
		return userPrincipal, groupPrincipals, err
	}

	userPrincipal = *user
	userPrincipal.Me = true

	// config.NestedGroupMembershipEnabled nil or false = false
	if config.NestedGroupMembershipEnabled != nil {
		if *config.NestedGroupMembershipEnabled == true {
			// As per https://msdn.microsoft.com/en-us/library/aa746475%28VS.85%29.aspx, `(member:1.2.840.113556.1.4.1941:=cn=user1,cn=users,DC=x)`
			// query can fetch all groups that the user is a member of, including nested groups
			userDN := result.Entries[0].DN
			// config.GroupMemberMappingAttribute is a required field post 2.0.1, so if an upgraded setup doesn't have its value, we set it to `member`
			if config.GroupMemberMappingAttribute == "" {
				config.GroupMemberMappingAttribute = "member"
			}
			nestedGroupsQuery := fmt.Sprintf("(%v%v%v)", config.GroupMemberMappingAttribute, ":1.2.840.113556.1.4.1941:=", ldapv2.EscapeFilter(userDN))
			logrus.Debugf("AD: Query for pulling user's groups: %v", nestedGroupsQuery)
			searchBase := config.UserSearchBase
			if config.GroupSearchBase != "" {
				searchBase = config.GroupSearchBase
			}

			// Call common method for getting group principals
			groupPrincipals, err = p.getGroupPrincipalsFromSearch(searchBase, nestedGroupsQuery, config, lConn, memberOf)
			if err != nil {
				return userPrincipal, groupPrincipals, err
			}

			return userPrincipal, groupPrincipals, nil
		}
	}
	if len(memberOf) != 0 {
		for i := 0; i < len(memberOf); i += 50 {
			batch := memberOf[i:ldap.Min(i+50, len(memberOf))]
			filter := fmt.Sprintf("(%v=%v)", ObjectClass, config.GroupObjectClass)
			query := "(|"
			for _, attrib := range batch {
				query += fmt.Sprintf("(distinguishedName=%v)", ldapv2.EscapeFilter(attrib))
			}
			query += ")"
			query = fmt.Sprintf("(&%v%v)", filter, query)
			// Pulling user's groups
			logrus.Debugf("AD: Query for pulling user's groups: %v", query)
			searchDomain := config.UserSearchBase
			if config.GroupSearchBase != "" {
				searchDomain = config.GroupSearchBase
			}

			// Call common method for getting group principals
			groupPrincipalListBatch, err := p.getGroupPrincipalsFromSearch(searchDomain, query, config, lConn, batch)
			if err != nil {
				return userPrincipal, groupPrincipals, err
			}
			groupPrincipals = append(groupPrincipals, groupPrincipalListBatch...)
		}
	}
	return userPrincipal, groupPrincipals, nil

}

func (p *adProvider) getGroupPrincipalsFromSearch(searchBase string, filter string, config *v3.ActiveDirectoryConfig, lConn *ldapv2.Conn,
	groupDN []string) ([]v3.Principal, error) {
	var groupPrincipals []v3.Principal
	var nilPrincipal []v3.Principal

	search := ldapv2.NewSearchRequest(searchBase,
		ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
		filter,
		ldap.GetGroupSearchAttributes(MemberOfAttribute, ObjectClass, config), nil)

	serviceAccountUsername := ldap.GetUserExternalID(config.ServiceAccountUsername, config.DefaultLoginDomain)
	err := lConn.Bind(serviceAccountUsername, config.ServiceAccountPassword)

	if err != nil {
		if ldapv2.IsErrorWithCode(err, ldapv2.LDAPResultInvalidCredentials) && config.Enabled {
			// If bind fails because service account password has changed, just return identities formed from groups in `memberOf`
			groupList := []v3.Principal{}
			for _, dn := range groupDN {
				grp := v3.Principal{
					ObjectMeta:    metav1.ObjectMeta{Name: GroupScope + "://" + dn},
					DisplayName:   dn,
					LoginName:     dn,
					PrincipalType: GroupScope,
					MemberOf:      true,
				}
				groupList = append(groupList, grp)
			}
			return groupList, nil
		}
		return groupPrincipals, err
	}

	result, err := lConn.SearchWithPaging(search, 1000)
	if err != nil {
		return groupPrincipals, httperror.WrapAPIError(errors.Wrapf(err, "server returned error for search %v %v: %v", search.BaseDN, search.Filter, err), httperror.ServerError, err.Error())
	}

	for _, e := range result.Entries {
		principal, err := ldap.AttributesToPrincipal(e.Attributes, e.DN, GroupScope, Name, config.UserObjectClass, config.UserNameAttribute, config.UserLoginAttribute, config.GroupObjectClass, config.GroupNameAttribute)
		if err != nil {
			logrus.Errorf("AD: Error in getting principal for group entry %v: %v", e, err)
			continue
		}
		if !reflect.DeepEqual(principal, nilPrincipal) {
			principal.MemberOf = true
			groupPrincipals = append(groupPrincipals, *principal)
		}
	}

	return groupPrincipals, nil
}

func (p *adProvider) getPrincipal(distinguishedName string, scope string, config *v3.ActiveDirectoryConfig, caPool *x509.CertPool) (*v3.Principal, error) {
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

	if !ldap.IsType(attribs, scope) && !p.permissionCheck(attribs, config) {
		logrus.Errorf("Failed to get object %s", distinguishedName)
		return nil, nil
	}

	if strings.EqualFold(UserScope, scope) {
		filter = fmt.Sprintf("(%v=%v)", ObjectClass, config.UserObjectClass)
	} else {
		filter = fmt.Sprintf("(%v=%v)", ObjectClass, config.GroupObjectClass)
	}

	logrus.Debugf("Query for getPrincipal(%s): %s", distinguishedName, filter)
	lConn, err := p.ldapConnection(config, caPool)
	if err != nil {
		return nil, err
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
			ldap.GetUserSearchAttributes(MemberOfAttribute, ObjectClass, config), nil)
	} else {
		search = ldapv2.NewSearchRequest(distinguishedName,
			ldapv2.ScopeBaseObject, ldapv2.NeverDerefAliases, 0, 0, false,
			filter,
			ldap.GetGroupSearchAttributes(MemberOfAttribute, ObjectClass, config), nil)
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

	principal, err := ldap.AttributesToPrincipal(entryAttributes, distinguishedName, scope, Name, config.UserObjectClass, config.UserNameAttribute, config.UserLoginAttribute, config.GroupObjectClass, config.GroupNameAttribute)
	if err != nil {
		return nil, err
	}
	return principal, nil
}

func (p *adProvider) searchPrincipals(name, principalType string, config *v3.ActiveDirectoryConfig, lConn *ldapv2.Conn) ([]v3.Principal, error) {
	name = ldapv2.EscapeFilter(name)

	var principals []v3.Principal

	if principalType == "" || principalType == "user" {
		userPrincipals, err := p.searchUser(name, config, lConn)
		if err != nil {
			return nil, err
		}
		principals = append(principals, userPrincipals...)
	}

	if principalType == "" || principalType == "group" {
		groupPrincipals, err := p.searchGroup(name, config, lConn)
		if err != nil {
			return nil, err
		}
		principals = append(principals, groupPrincipals...)
	}

	return principals, nil
}

func (p *adProvider) searchUser(name string, config *v3.ActiveDirectoryConfig, lConn *ldapv2.Conn) ([]v3.Principal, error) {
	srchAttributes := strings.Split(config.UserSearchAttribute, "|")
	query := fmt.Sprintf("(&(%v=%v)", ObjectClass, config.UserObjectClass)
	srchAttrs := "(|"
	for _, attr := range srchAttributes {
		srchAttrs += fmt.Sprintf("(%v=%v*)", attr, name)
	}
	// UserSearchFilter should be follow AD search filter syntax, enclosed by parentheses
	query += srchAttrs + ")" + config.UserSearchFilter + ")"
	logrus.Debugf("LDAPProvider searchUser query: %s", query)
	return p.searchLdap(query, UserScope, config, lConn)
}

func (p *adProvider) searchGroup(name string, config *v3.ActiveDirectoryConfig, lConn *ldapv2.Conn) ([]v3.Principal, error) {
	// GroupSearchFilter should be follow AD search filter syntax, enclosed by parentheses
	query := "(&(" + ObjectClass + "=" + config.GroupObjectClass + ")(" + config.GroupSearchAttribute + "=" + name + "*)" + config.GroupSearchFilter + ")"
	logrus.Debugf("LDAPProvider searchGroup query: %s", query)
	return p.searchLdap(query, GroupScope, config, lConn)
}

func (p *adProvider) searchLdap(query string, scope string, config *v3.ActiveDirectoryConfig, lConn *ldapv2.Conn) ([]v3.Principal, error) {
	var principals []v3.Principal
	var search *ldapv2.SearchRequest

	searchDomain := config.UserSearchBase
	if strings.EqualFold(UserScope, scope) {
		search = ldapv2.NewSearchRequest(searchDomain,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			query,
			ldap.GetUserSearchAttributes(MemberOfAttribute, ObjectClass, config), nil)
	} else {
		if config.GroupSearchBase != "" {
			searchDomain = config.GroupSearchBase
		}
		search = ldapv2.NewSearchRequest(searchDomain,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			query,
			ldap.GetGroupSearchAttributes(MemberOfAttribute, ObjectClass, config), nil)
	}

	// Bind before query
	serviceAccountUsername := ldap.GetUserExternalID(config.ServiceAccountUsername, config.DefaultLoginDomain)
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
		principal, err := ldap.AttributesToPrincipal(entry.Attributes, results.Entries[i].DN, scope, Name, config.UserObjectClass, config.UserNameAttribute, config.UserLoginAttribute, config.GroupObjectClass, config.GroupNameAttribute)
		if err != nil {
			logrus.Errorf("Error translating search result: %v", err)
			continue
		}
		principals = append(principals, *principal)
	}

	return principals, nil
}

func (p *adProvider) ldapConnection(config *v3.ActiveDirectoryConfig, caPool *x509.CertPool) (*ldapv2.Conn, error) {
	servers := config.Servers
	TLS := config.TLS
	port := config.Port
	connectionTimeout := config.ConnectionTimeout
	return ldap.NewLDAPConn(servers, TLS, port, connectionTimeout, caPool)
}
func (p *adProvider) permissionCheck(attributes []*ldapv2.EntryAttribute, config *v3.ActiveDirectoryConfig) bool {
	userObjectClass := config.UserObjectClass
	userEnabledAttribute := config.UserEnabledAttribute
	userDisabledBitMask := config.UserDisabledBitMask
	return ldap.HasPermission(attributes, userObjectClass, userEnabledAttribute, userDisabledBitMask)
}

package googleoauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
)

func (g *googleOauthProvider) getUserInfoAndGroups(adminSvc *admin.Service, gOAuthToken *oauth2.Token, config *v32.GoogleOauthConfig, testAndEnableAction bool) (v3.Principal, []v3.Principal, error) {
	var userPrincipal v3.Principal
	var groupPrincipals []v3.Principal
	// use the access token to make requests, get user info
	user, err := g.goauthClient.getUser(gOAuthToken.AccessToken, config)
	if err != nil {
		return userPrincipal, groupPrincipals, err
	}
	if testAndEnableAction {
		if user.HostedDomain != config.Hostname {
			return userPrincipal, groupPrincipals, fmt.Errorf("invalid hostname provided")
		}
	}
	userPrincipal = g.toPrincipal(userType, *user, nil)
	userPrincipal.Me = true
	logrus.Debugf("[Google OAuth] loginuser: Obtained userinfo using oauth access token")

	groupPrincipals, err = g.getGroupsUserBelongsTo(adminSvc, user.SubjectUniqueID, user.HostedDomain, config)
	if err != nil {
		// The error for this group request could be 403, because svc acc was not provided, and we're relying on individual
		// users' creds to get groups
		if config.ServiceAccountCredential == "" {
			if gErr, ok := err.(*googleapi.Error); !ok || gErr.Code != http.StatusForbidden {
				// if the error is not forbidden, return the error
				return userPrincipal, groupPrincipals, err
			}
			// if the error is forbidden, don't throw any error, just no group principals will be returned
		} else {
			// getting any error in spite of svc acc creds must be returned
			return userPrincipal, groupPrincipals, err
		}
	}
	if config.NestedGroupMembershipEnabled {
		groupPrincipals, err = g.fetchParentGroups(config, groupPrincipals, adminSvc, user.HostedDomain)
		if err != nil {
			return userPrincipal, groupPrincipals, err
		}
	}

	logrus.Debugf("[Google OAuth] loginuser: Retrieved user's groups using admin directory")
	return userPrincipal, groupPrincipals, nil
}

func (g *googleOauthProvider) fetchParentGroups(config *v32.GoogleOauthConfig, groupPrincipals []v3.Principal, adminSvc *admin.Service, hostedDomain string) ([]v3.Principal, error) {
	groupMap := make(map[string]bool)
	var nestedGroupPrincipals []v3.Principal
	for _, principal := range groupPrincipals {
		principals, err := g.gatherParentGroups(principal, adminSvc, config, hostedDomain, groupMap)
		if err != nil {
			return groupPrincipals, err
		}
		if len(principals) > 0 {
			nestedGroupPrincipals = append(nestedGroupPrincipals, principals...)
		}
	}
	groupPrincipals = append(groupPrincipals, nestedGroupPrincipals...)
	return groupPrincipals, nil
}

func (g *googleOauthProvider) getGroupsUserBelongsTo(adminSvc *admin.Service, userKey string, hostedDomain string, config *v32.GoogleOauthConfig) ([]v3.Principal, error) {
	var groupPrincipals []v3.Principal
	_, groups, err := g.paginateResults(adminSvc, hostedDomain, userKey, "", "", groupType, false)
	if err != nil {
		// The error for this group request could be 403, because svc acc was not provided, and we're relying on individual
		// users' creds to get groups
		if config.ServiceAccountCredential == "" {
			// used the client creds, if error is forbidden, don't throw error
			if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == http.StatusForbidden {
				return groupPrincipals, nil
			}
		}
		// getting any error in spite of svc acc creds must be returned
		return nil, err
	}
	for _, gr := range groups {
		group := Account{Name: gr.Name, Email: gr.Email, SubjectUniqueID: gr.Id}
		groupPrincipal := g.toPrincipal(groupType, group, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}
	return groupPrincipals, nil
}

func (g *googleOauthProvider) searchPrincipals(adminSvc *admin.Service, searchKey, principalType string, config *v32.GoogleOauthConfig) ([]Account, error) {
	var accounts []Account

	if principalType == "" || principalType == "user" {
		userAccounts, err := g.searchUsers(adminSvc, searchKey, config, false)
		if err != nil {
			if config.ServiceAccountCredential == "" {
				// used the client creds, must try once with viewType domain_public
				if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == http.StatusForbidden {
					userAccounts, err = g.searchUsers(adminSvc, searchKey, config, true)
					if err != nil {
						return nil, err
					}
				} else {
					return nil, err
				}
			} else {
				// getting any error in spite of svc acc creds must be returned
				return nil, err
			}
		}
		accounts = append(accounts, userAccounts...)
	}
	if principalType == "" || principalType == "group" {
		groupAccounts, err := g.searchGroups(adminSvc, searchKey, config)
		if err != nil {
			if config.ServiceAccountCredential == "" {
				// used the client creds, if error is forbidden, don't throw error
				if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == http.StatusForbidden {
					return accounts, nil
				}
			}
			// getting any error in spite of svc acc creds must be returned
			return nil, err
		}
		accounts = append(accounts, groupAccounts...)
	}
	return accounts, nil
}

func (g *googleOauthProvider) searchUsers(adminSvc *admin.Service, searchKey string, config *v32.GoogleOauthConfig, viewPublic bool) ([]Account, error) {
	var users []*admin.User
	var accounts []Account
	users, _, err := g.paginateResults(adminSvc, config.Hostname, "", searchKey, "", userType, viewPublic)
	if err != nil {
		return accounts, err
	}
	for _, u := range users {
		a := Account{
			SubjectUniqueID: u.Id,
			Email:           u.PrimaryEmail,
			PictureURL:      u.ThumbnailPhotoUrl,
			Type:            userType,
		}
		if u.Name != nil {
			a.Name = u.Name.FullName
			a.GivenName = u.Name.GivenName
			a.FamilyName = u.Name.FamilyName
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (g *googleOauthProvider) searchGroups(adminSvc *admin.Service, searchKey string, config *v32.GoogleOauthConfig) ([]Account, error) {
	var accounts []Account
	groupsMap := map[string]*admin.Group{}
	for _, attr := range []string{"name", "email"} {
		_, resGroups, err := g.paginateResults(adminSvc, config.Hostname, "", searchKey, attr, groupType, false)
		if err != nil {
			return accounts, err
		}
		//dedup groups
		for _, gr := range resGroups {
			if _, ok := groupsMap[gr.Id]; !ok {
				groupsMap[gr.Id] = gr
			}
		}
	}

	for _, g := range groupsMap {
		accounts = append(accounts, Account{SubjectUniqueID: g.Id, Email: g.Email, Name: g.Name, Type: groupType})
	}
	return accounts, nil
}

func (g *googleOauthProvider) paginateResults(adminSvc *admin.Service, hostedDomain, memberKey, searchKey, searchAttr, accountType string, viewPublic bool) ([]*admin.User, []*admin.Group, error) {
	var groups []*admin.Group
	var users []*admin.User
	switch accountType {
	case userType:
		// in case of list users call, using the searchKey itself as query searches all attributes for the search key
		userListCall := adminSvc.Users.List().Domain(hostedDomain).Query(searchKey)
		if viewPublic {
			userListCall.ViewType(domainPublicViewType)
		}
		for {
			res, err := userListCall.Do()
			if err != nil {
				return users, nil, err
			}
			users = append(users, res.Users...)
			if res.NextPageToken == "" {
				return users, nil, nil
			}
			userListCall.PageToken(res.NextPageToken)
		}
	case groupType:
		groupListCall := adminSvc.Groups.List().Domain(hostedDomain)
		if memberKey != "" {
			groupListCall.UserKey(memberKey)
		} else if searchKey != "" {
			// in case of list groups call, we need to specify search attributes
			groupListCall.Query(searchAttr + ":" + searchKey + "*")
		}
		for {
			res, err := groupListCall.Do()
			if err != nil {
				return nil, groups, err
			}
			groups = append(groups, res.Groups...)
			if res.NextPageToken == "" {
				return nil, groups, nil
			}
			groupListCall.PageToken(res.NextPageToken)
		}
	default:
		return nil, nil, fmt.Errorf("paginateResults: Invalid principal type")
	}
}

func (g *googleOauthProvider) gatherParentGroups(groupPrincipal v3.Principal, adminSvc *admin.Service, config *v32.GoogleOauthConfig, hostedDomain string, groupMap map[string]bool) ([]v3.Principal, error) {
	var principals []v3.Principal
	if groupMap[groupPrincipal.ObjectMeta.Name] {
		return principals, nil
	}
	groupMap[groupPrincipal.ObjectMeta.Name] = true
	parts := strings.SplitN(groupPrincipal.ObjectMeta.Name, ":", 2)
	if len(parts) != 2 {
		return principals, fmt.Errorf("error while gathering parent groups: invalid id %v", groupPrincipal.ObjectMeta.Name)
	}
	groupID := strings.TrimPrefix(parts[1], "//")
	groups, err := g.getGroupsUserBelongsTo(adminSvc, groupID, hostedDomain, config)
	if err != nil {
		return principals, err
	}

	for _, group := range groups {
		if groupMap[group.ObjectMeta.Name] {
			continue
		} else {
			principals = append(principals, group)
			nestedGroupPrincipals, err := g.gatherParentGroups(group, adminSvc, config, hostedDomain, groupMap)
			if err != nil {
				return principals, err
			}
			if len(nestedGroupPrincipals) > 0 {
				principals = append(principals, nestedGroupPrincipals...)
			}
		}
	}
	return principals, nil
}

func (g *googleOauthProvider) getdirectoryServiceFromStoredToken(storedOauthToken string, config *v32.GoogleOauthConfig) (*admin.Service, error) {
	var oauthToken oauth2.Token
	if err := json.Unmarshal([]byte(storedOauthToken), &oauthToken); err != nil {
		return nil, err
	}

	oauth2Config, err := google.ConfigFromJSON([]byte(config.OauthCredential), scopes...)
	if err != nil {
		return nil, err
	}
	adminSvc, err := g.getDirectoryService(g.ctx, config.AdminEmail, []byte(config.ServiceAccountCredential), oauth2Config.TokenSource(g.ctx, &oauthToken))
	if err != nil {
		return nil, err
	}
	return adminSvc, nil
}

func getUIDFromPrincipalID(principalID string) (string, string, error) {
	var externalID string
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid id %v", principalID)
	}
	externalID = strings.TrimPrefix(parts[1], "//")
	parts = strings.SplitN(parts[0], "_", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid id %v", principalID)
	}
	return externalID, parts[1], nil
}

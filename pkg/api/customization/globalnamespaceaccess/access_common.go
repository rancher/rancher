package globalnamespaceaccess

import (
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type MemberAccess struct {
	Users              v3.UserInterface
	PrtbLister         v3.ProjectRoleTemplateBindingLister
	RoleTemplateLister v3.RoleTemplateLister
}

const (
	ImpersonateUserHeader  = "Impersonate-User"
	ImpersonateGroupHeader = "Impersonate-Group"
	AllAccess              = "all"
	UpdateAccess           = "update"
	ReadonlyAccess         = "readonly"
	localPrincipalPrefix   = "local://"
)

var accessVerbs = map[string][]string{
	AllAccess:      {"create", "update", "delete", "list"},
	UpdateAccess:   {"update", "get", "list"},
	ReadonlyAccess: {"get", "list"},
}

func (ma *MemberAccess) CheckCreatorAndMembersAccessToTargets(request *types.APIContext, targets []string, data map[string]interface{}, resourceType, apiGroup, resource string) error {
	creatorID := request.Request.Header.Get(ImpersonateUserHeader)
	if err := ma.getGroupsAndCheckAccess(data, targets, resource, resourceType); err != nil {
		return err
	}
	memberIDAccessType, err := ma.getMemberAccessTypeMap(creatorID, data)
	if err != nil {
		return err
	}
	defer request.Request.Header.Set(ImpersonateUserHeader, creatorID)
	for userID, accessType := range memberIDAccessType {
		userAPIContext := *request
		userRequest := *(request.Request)
		userAPIContext.Request = &userRequest
		userAPIContext.Request.Header.Set(ImpersonateUserHeader, userID)
		for _, targetID := range targets {
			switch resourceType {
			case client.GlobalDNSType:
				newObj := make(map[string]interface{})
				newObj["id"] = targetID
				if _, err := checkAccess(&userAPIContext, accessVerbs[ReadonlyAccess], newObj, apiGroup, resource); err != nil {
					return fmt.Errorf("user %v cannot access %v %v", userAPIContext.Request.Header.Get(ImpersonateUserHeader), resource, targetID)
				}
			case client.MultiClusterAppType:
				split := strings.SplitN(targetID, ":", 2)
				if len(split) != 2 {
					return fmt.Errorf("project Id is incorrect")
				}
				projectNamespace := split[1]
				newObj := make(map[string]interface{})
				newObj["namespaceId"] = projectNamespace
				if verb, err := checkAccess(&userAPIContext, accessVerbs[accessType], newObj, apiGroup, resource); err != nil {
					return fmt.Errorf("user %v cannot %v apps in project %v", userAPIContext.Request.Header.Get(ImpersonateUserHeader), verb, targetID)
				}
			}
		}
	}
	return nil
}

func (ma *MemberAccess) getMemberAccessTypeMap(creatorID string, data map[string]interface{}) (map[string]string, error) {
	memberIDAccessType := make(map[string]string)
	memberIDAccessType[creatorID] = AllAccess
	membersMapSlice, _ := values.GetSlice(data, client.MultiClusterAppFieldMembers)
	// find user and pass ID
	for _, m := range membersMapSlice {
		if userPrincipalID, ok := m[client.MemberFieldUserPrincipalID].(string); ok {
			if _, ok := m[client.MemberFieldGroupPrincipalID].(string); ok {
				return memberIDAccessType, fmt.Errorf("member cannot have both userPrincipalID and groupPrincipalID set")
			}
			user, err := ma.getUserFromUserPrincipalID(userPrincipalID)
			if err != nil {
				return memberIDAccessType, err
			}
			if user == nil {
				return memberIDAccessType, fmt.Errorf("no user found for principal %v", userPrincipalID)
			}
			if accessType, exists := m[client.MemberFieldAccessType].(string); exists {
				memberIDAccessType[user.Name] = accessType
			} else {
				memberIDAccessType[user.Name] = ReadonlyAccess
			}
		}
	}
	return memberIDAccessType, nil
}

func (ma *MemberAccess) getGroupsAndCheckAccess(data map[string]interface{}, targets []string, resource, resourceType string) error {
	membersMapSlice, _ := values.GetSlice(data, client.MultiClusterAppFieldMembers)
	groupAccessType := make(map[string]string)
	var groups []string
	for _, m := range membersMapSlice {
		if groupPrincipalID, ok := m[client.MemberFieldGroupPrincipalID].(string); ok && groupPrincipalID != "" {
			groups = append(groups, groupPrincipalID)
			if accessType, exists := m[client.MemberFieldAccessType].(string); exists {
				groupAccessType[groupPrincipalID] = accessType
			} else {
				groupAccessType[groupPrincipalID] = ReadonlyAccess
			}
		}
	}
	return CheckGroupAccess(groupAccessType, targets, ma.PrtbLister, ma.RoleTemplateLister, resource, resourceType)
}

func CheckGroupAccess(groups map[string]string, targets []string, prtbLister v3.ProjectRoleTemplateBindingLister, rtLister v3.RoleTemplateLister, resource, resourceType string) error {
	for _, t := range targets {
		split := strings.SplitN(t, ":", 2)
		if len(split) != 2 {
			return fmt.Errorf("Target project name %s is invalid", t)
		}
		projectNS := split[1]
		prtbs, err := prtbLister.List(projectNS, labels.NewSelector())
		if err != nil {
			return err
		}
		prtbMap := make(map[string]string)
		for _, prtb := range prtbs {
			if prtb.GroupPrincipalName != "" {
				prtbMap[prtb.GroupPrincipalName] = prtb.RoleTemplateName
			}
		}
		// see if all groups are in prtbMap
		for g, accessType := range groups {
			if rtName, ok := prtbMap[g]; ok {
				if resourceType == client.MultiClusterAppType {
					rt, err := rtLister.Get("", rtName)
					if err != nil {
						return err
					}
					for _, r := range rt.Rules {
						if !slice.ContainsString(r.Resources, resource) {
							continue
						}
						if slice.ContainsString(r.Verbs, "*") {
							continue
						}
						for _, v := range accessVerbs[accessType] {
							if !slice.ContainsString(r.Verbs, v) {
								return fmt.Errorf("member (group) %v cannot %v %v in project %v", g, v, resource, t)
							}
						}

					}
				}
			} else {
				return fmt.Errorf("member (group) %v does not have access to project %v", g, t)
			}
		}
	}
	return nil
}

func (ma *MemberAccess) getUserFromUserPrincipalID(userPrincipalID string) (*v3.User, error) {
	encodedPrincipalID := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(userPrincipalID))
	if len(encodedPrincipalID) > 63 {
		encodedPrincipalID = encodedPrincipalID[:63]
	}
	set := labels.Set(map[string]string{encodedPrincipalID: "hashed-principal-name"})
	usersList, err := ma.Users.List(v1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return nil, err
	}

	if len(usersList.Items) == 0 {
		// check for local auth principals
		if strings.HasPrefix(userPrincipalID, localPrincipalPrefix) {
			userID := strings.TrimPrefix(userPrincipalID, localPrincipalPrefix)
			user, err := ma.Users.Controller().Lister().Get("", userID)
			return user, err
		}
		return nil, nil
	}

	var match *v3.User
	for _, u := range usersList.Items {
		if slice.ContainsString(u.PrincipalIDs, userPrincipalID) {
			if match != nil {
				// error out on duplicates
				return nil, fmt.Errorf("can't find unique user for principal %v", userPrincipalID)
			}
			match = &u
		}
	}
	return match, nil
}

func checkAccess(apiContext *types.APIContext, verbs []string, newObj map[string]interface{}, apiGroup, resource string) (string, error) {
	for _, verb := range verbs {
		if err := apiContext.AccessControl.CanDo(apiGroup, resource, verb, apiContext, newObj, apiContext.Schema); err != nil {
			return verb, err
		}
	}
	return "", nil
}

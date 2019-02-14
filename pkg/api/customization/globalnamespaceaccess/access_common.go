package globalnamespaceaccess

import (
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"

	"github.com/rancher/norman/api/access"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type MemberAccess struct {
	Users              v3.UserInterface
	RoleTemplateLister v3.RoleTemplateLister
	PrtbLister         v3.ProjectRoleTemplateBindingLister
	CrtbLister         v3.ClusterRoleTemplateBindingLister
}

const (
	ImpersonateUserHeader  = "Impersonate-User"
	ImpersonateGroupHeader = "Impersonate-Group"
	OwnerAccess            = "owner"
	ReadonlyAccess         = "read-only"
	localPrincipalPrefix   = "local://"
)

func (ma *MemberAccess) CheckCallerAccessToTargets(request *types.APIContext, targets []string, resourceType string, into interface{}) error {
	for _, targetID := range targets {
		if err := access.ByID(request, &managementschema.Version, resourceType, targetID, into); err != nil {
			return err
		}
	}
	return nil
}

func (ma *MemberAccess) EnsureRoleInTargets(targetProjects, roleTemplates []string, callerID string) error {
	newProjectRoleTemplateMap := make(map[string]bool)
	newClusterRoleTemplateMap := make(map[string]bool)
	projects := make(map[string]bool)
	clusters := make(map[string]bool)
	for _, r := range roleTemplates {
		rt, err := ma.RoleTemplateLister.Get("", r)
		if err != nil {
			return err
		}
		switch rt.Context {
		case "project":
			if !newProjectRoleTemplateMap[r] {
				newProjectRoleTemplateMap[r] = true
			}
		case "cluster":
			if !newClusterRoleTemplateMap[r] {
				newClusterRoleTemplateMap[r] = true
			}
		}
	}
	for _, t := range targetProjects {
		split := strings.SplitN(t, ":", 2)
		if len(split) != 2 {
			return fmt.Errorf("invalid project ID %v", t)
		}
		clusterName := split[0]
		projectName := split[1]

		if !projects[projectName] {
			projects[projectName] = true
		}
		if !clusters[clusterName] {
			clusters[clusterName] = true
		}
	}

	for pname := range projects {
		projectRoleTemplateFoundCount := 0
		prtbs, err := ma.PrtbLister.List(pname, labels.NewSelector())
		if err != nil {
			return err
		}
		for _, prtb := range prtbs {
			if prtb.UserName == callerID && newProjectRoleTemplateMap[prtb.RoleTemplateName] {
				projectRoleTemplateFoundCount++
			}
		}
		if projectRoleTemplateFoundCount != len(newProjectRoleTemplateMap) {
			return fmt.Errorf("user %v does not have all project roles provided in project %v", callerID, pname)
		}
	}
	for cname := range clusters {
		clusterRoleTemplateFoundCount := 0
		crtbs, err := ma.CrtbLister.List(cname, labels.NewSelector())
		if err != nil {
			return err
		}
		for _, crtb := range crtbs {
			if crtb.UserName == callerID && newClusterRoleTemplateMap[crtb.RoleTemplateName] {
				clusterRoleTemplateFoundCount++
			}
		}
		if clusterRoleTemplateFoundCount != len(newClusterRoleTemplateMap) {
			return fmt.Errorf("user %v does not have all cluster roles provided in cluster %v", callerID, cname)
		}
	}
	return nil
}

// CheckAccessToUpdateMembers checks if the request is updating members list, and if the caller has permission to do so
func CheckAccessToUpdateMembers(members []v3.Member, data map[string]interface{}, ownerAccess bool) error {
	var requestUpdatesMembers bool
	// Check if members are being updated, if yes, make sure only member with owner permission is making this update request
	newMembers := convert.ToMapSlice(data[client.GlobalDNSFieldMembers])
	originalMembers := members
	if len(newMembers) != len(originalMembers) && !ownerAccess {
		return fmt.Errorf("only members with owner access can update members")
	}

	newMemberAccessType := make(map[string]string)
	for _, m := range newMembers {
		if _, ok := m[client.MemberFieldUserPrincipalID]; ok {
			newMemberAccessType[convert.ToString(m[client.MemberFieldUserPrincipalID])] = convert.ToString(m[client.MemberFieldAccessType])
		}
		if _, ok := m[client.MemberFieldGroupPrincipalID]; ok {
			newMemberAccessType[convert.ToString(m[client.MemberFieldGroupPrincipalID])] = convert.ToString(m[client.MemberFieldAccessType])
		}
	}

	originalMemberAccessType := make(map[string]string)
	originalMembersFoundInRequest := make(map[string]bool) // map to check whether all existing members are present in the current request, if not then this request is trying to update members list
	for _, m := range originalMembers {
		if m.UserPrincipalName != "" {
			originalMemberAccessType[m.UserPrincipalName] = m.AccessType
		}
		if m.GroupPrincipalName != "" {
			originalMemberAccessType[m.GroupPrincipalName] = m.AccessType
		}
	}

	// go through all members in the current request, check if each exists in the original global DNS
	// if it exists, check that the access type hasn't changed, if it has changed, this means the req is updating members
	// if the member from req doesn't exist in original global DNS, this means the new request is adding a new member, hence updating members list
	for key, accessType := range newMemberAccessType {
		if val, ok := originalMemberAccessType[key]; ok {
			if val != accessType {
				requestUpdatesMembers = true
				break
			}
			// mark this original member as present in the current request
			originalMembersFoundInRequest[key] = true
		} else {
			requestUpdatesMembers = true
			break
		}
	}
	if requestUpdatesMembers && !ownerAccess {
		return fmt.Errorf("only members with owner access can add new members")
	}

	// at this point, all members in the new request have been found in the original global DNS with the same access type
	// but we need to check if all members from the original global DNS are also present in the current request
	for member := range originalMemberAccessType {
		// if any member is not found, this means the current request is updating members list by deleting this member
		if !originalMembersFoundInRequest[member] && !ownerAccess {
			return fmt.Errorf("only members with owner access can delete existing members")
		}
	}
	return nil
}

func (ma *MemberAccess) GetAccessTypeOfCaller(callerID, creatorID, name string, members []v3.Member) (string, error) {
	var username string
	if callerID == creatorID {
		return OwnerAccess, nil
	}
	for _, m := range members {
		if m.UserName == "" && m.UserPrincipalName != "" {
			user, err := ma.getUserFromUserPrincipalID(m.UserPrincipalName)
			if err != nil {
				return "", err
			}
			if user == nil {
				return "", fmt.Errorf("no user found for principal %v", m.UserPrincipalName)
			}
			if user.Name == callerID {
				username = user.Name
			}
		} else if m.UserName == callerID {
			username = m.UserName
		}
		if username != "" { // found the caller
			return m.AccessType, nil
		}
	}
	return "", fmt.Errorf("user %v is not in members list", callerID)
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

package globalnamespaceaccess

import (
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type MemberAccess struct {
	Users              v3.UserInterface
	RoleTemplateLister v3.RoleTemplateLister
	PrtbLister         v3.ProjectRoleTemplateBindingLister
	CrtbLister         v3.ClusterRoleTemplateBindingLister
	GrbLister          v3.GlobalRoleBindingLister
	GrLister           v3.GlobalRoleLister
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

func (ma *MemberAccess) IsAdmin(callerID string) (bool, error) {
	u, err := ma.Users.Controller().Lister().Get("", callerID)
	if err != nil {
		return false, err
	}
	if u == nil {
		return false, fmt.Errorf("No user found with ID %v", callerID)
	}
	// Get globalRoleBinding for this user
	grbs, err := ma.GrbLister.List("", labels.NewSelector())
	if err != nil {
		return false, err
	}
	var callerRole string
	for _, grb := range grbs {
		if grb.UserName == callerID {
			callerRole = grb.GlobalRoleName
			break
		}
	}
	gr, err := ma.GrLister.Get("", callerRole)
	if err != nil {
		return false, err
	}
	if gr != nil {
		for _, rule := range gr.Rules {
			// admin roles have all resources and all verbs allowed
			if slice.ContainsString(rule.Resources, "*") && slice.ContainsString(rule.APIGroups, "*") && slice.ContainsString(rule.Verbs, "*") {
				// caller is global admin
				return true, nil
			}
		}
	}
	return false, nil
}

func (ma *MemberAccess) EnsureRoleInTargets(targetProjects, roleTemplates []string, callerID string) error {
	isAdmin, err := ma.IsAdmin(callerID)
	if err != nil {
		return err
	}
	if isAdmin {
		// relax this check for the global admin
		return nil
	}
	if len(roleTemplates) == 0 {
		// avoid prtb/crtb calculation; create method in mcapp store will assign creator's inherited roles;
		// create store method is called after validator
		return nil
	}
	newProjectRoleTemplateMap := make(map[string]bool)
	newClusterRoleTemplateMap := make(map[string]bool)
	projects := make(map[string]bool)
	clusters := make(map[string]bool)
	for _, r := range roleTemplates {
		rt, err := ma.RoleTemplateLister.Get("", r)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "Role "+r+" does not exist")
			}
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

	fmt.Printf("\n\nnewProjectRoleTemplateMap: %v\n\n", newProjectRoleTemplateMap)
	fmt.Printf("\n\ncallerID: %v\n\n", callerID)
	for pname := range projects {
		projectRoleTemplateFoundCount := 0
		prtbs, err := ma.PrtbLister.List(pname, labels.NewSelector())
		if err != nil {
			return err
		}
		for _, prtb := range prtbs {
			fmt.Printf("\n\nprtb.UserName: %v\n\n", prtb.UserName)
			if prtb.UserName == callerID && newProjectRoleTemplateMap[prtb.RoleTemplateName] {
				fmt.Printf("\nFound %v\n", prtb.RoleTemplateName)
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

func (ma *MemberAccess) DeriveRolesInTargets(callerID string, targets []string) (map[string][]string, error) {
	targetToRoles := make(map[string][]string)
	isAdmin, err := ma.IsAdmin(callerID)
	if err != nil {
		return nil, err
	}
	if isAdmin {
		projectRolesToAddMap := make(map[string]bool)
		clusterRolesToAddMap := make(map[string]bool)
		// assign clusterCreatorDefault and projectCreatorDefault roles
		rts, err := ma.RoleTemplateLister.List("", labels.NewSelector())
		if err != nil {
			return nil, err
		}
		for _, rt := range rts {
			if rt.ClusterCreatorDefault && !clusterRolesToAddMap[rt.Name] {
				clusterRolesToAddMap[rt.Name] = true
			} else if rt.ProjectCreatorDefault && !projectRolesToAddMap[rt.Name] {
				projectRolesToAddMap[rt.Name] = true
			}
		}
		if len(projectRolesToAddMap) == 0 && len(clusterRolesToAddMap) == 0 {
			return nil, fmt.Errorf("Admin has passed no roles to multiclusterapp, and no cluster/project owner roles found")
		}
		// add projectCreatorDefault roles in all target projects
		projectRolesToAdd := make([]string, len(projectRolesToAddMap))
		i := 0
		for role := range projectRolesToAddMap {
			projectRolesToAdd[i] = role
			i++
		}
		// add clusterCreatorDefault roles in all target projects's clusters
		clusterRolesToAdd := make([]string, len(clusterRolesToAddMap))
		i = 0
		for role := range clusterRolesToAddMap {
			clusterRolesToAdd[i] = role
			i++
		}
		targetToRoles := make(map[string][]string)
		for _, target := range targets {
			targetToRoles[target] = projectRolesToAdd
			split := strings.SplitN(target, ":", 2)
			if len(split) != 2 {
				return nil, fmt.Errorf("invalid project name: %v", target)
			}
			clusterName := split[0]
			if _, ok := targetToRoles[clusterName]; !ok {
				targetToRoles[clusterName] = clusterRolesToAdd
			}
		}
	} else {
		for _, target := range targets {
			projectRoleFound := false
			isClusterCreator := false
			split := strings.SplitN(target, ":", 2)
			if len(split) != 2 {
				return targetToRoles, fmt.Errorf("invalid project name: %v", target)
			}
			clusterName := split[0]
			projectName := split[1]
			// get roles from this project for this creator
			prtbs, err := ma.PrtbLister.List(projectName, labels.NewSelector())
			if err != nil {
				return targetToRoles, err
			}
			for _, prtb := range prtbs {
				if prtb.UserName == callerID {
					projectRoleFound = true
					if roles, ok := targetToRoles[target]; ok {
						if !slice.ContainsString(roles, prtb.RoleTemplateName) {
							targetToRoles[target] = append(roles, prtb.RoleTemplateName)
						}
					} else {
						targetToRoles[target] = []string{prtb.RoleTemplateName}
					}
				}
			}

			if !projectRoleFound {
				// get roles from this cluster for this creator, see if the creator is the cluster-creator and hence should have
				// access to all projects
				crtbs, err := ma.CrtbLister.List(clusterName, labels.NewSelector())
				if err != nil {
					return targetToRoles, err
				}
				for _, crtb := range crtbs {
					if crtb.UserName == callerID {
						roleTemplate, err := ma.RoleTemplateLister.Get("", crtb.RoleTemplateName)
						if err != nil {
							return targetToRoles, err
						}
						if roleTemplate != nil && roleTemplate.ClusterCreatorDefault {
							isClusterCreator = true
							if roles, ok := targetToRoles[clusterName]; ok {
								if !slice.ContainsString(roles, crtb.RoleTemplateName) {
									targetToRoles[clusterName] = append(roles, crtb.RoleTemplateName)
								}
							} else {
								targetToRoles[clusterName] = []string{crtb.RoleTemplateName}
							}
						}
					}
				}
			}
			if !projectRoleFound && !isClusterCreator {
				// creator has no roles in target projects and is not even a cluster-owner
				return nil, fmt.Errorf("Multiclusterapp creator %v has no roles in target project %v", callerID, target)
			}
		}
	}
	return targetToRoles, nil
}

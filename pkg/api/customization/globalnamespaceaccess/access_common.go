package globalnamespaceaccess

import (
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/set"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	client "github.com/rancher/types/client/management/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type MemberAccess struct {
	Users              v3.UserInterface
	RoleTemplateLister v3.RoleTemplateLister
	PrtbLister         v3.ProjectRoleTemplateBindingLister
	CrtbLister         v3.ClusterRoleTemplateBindingLister
	GrbLister          v3.GlobalRoleBindingLister
	GrLister           v3.GlobalRoleLister
	Prtbs              v3.ProjectRoleTemplateBindingInterface
	Crtbs              v3.ClusterRoleTemplateBindingInterface
	ProjectLister      v3.ProjectLister
	ClusterLister      v3.ClusterLister
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

func (ma *MemberAccess) CanCreateRKETemplate(callerID string) (bool, error) {
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
					if slice.ContainsString(rule.Resources, "clustertemplates") && slice.ContainsString(rule.APIGroups, "management.cattle.io") && slice.ContainsString(rule.Verbs, "create") {
						// caller can create RKE templates
						return true, nil
					}
				}
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
		for _, t := range targetProjects {
			if err := ma.checkProjectExists(t); err != nil {
				return err
			}
		}
		// relax memberAccess check for the global admin
		return nil
	}
	newProjectRoleTemplateMap := make(map[string]*v3.RoleTemplate)
	newClusterRoleTemplateMap := make(map[string]*v3.RoleTemplate)
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
			if _, ok := newProjectRoleTemplateMap[r]; !ok {
				newProjectRoleTemplateMap[r] = rt
			}
		case "cluster":
			if _, ok := newClusterRoleTemplateMap[r]; !ok {
				newClusterRoleTemplateMap[r] = rt
			}
		}
	}
	clustersCallerIsOwnerOf := make(map[string]bool)
	errMsg := "User does not have "
	roleMissing := false
	for _, t := range targetProjects {
		if err := ma.checkProjectExists(t); err != nil {
			return err
		}
		cname, pname := ref.Parse(t)
		if !clusters[cname] {
			clusters[cname] = true
		}
		projectRoleTemplateFoundCount := 0
		projectRoleTemplateFoundMap := make(map[string]bool)
		callerIsProjectOwner := false
		callerIsProjectMember := false
		callerIsClusterOwner := false
		prtbs, err := ma.PrtbLister.List(pname, labels.NewSelector())
		if err != nil {
			return err
		}
		for _, prtb := range prtbs {
			if prtb.UserName == callerID {
				if _, ok := newProjectRoleTemplateMap[prtb.RoleTemplateName]; ok {
					projectRoleTemplateFoundMap[prtb.RoleTemplateName] = true
					projectRoleTemplateFoundCount++
				}
				if callerIsProjectOwner && callerIsProjectMember {
					continue
				}
				rt, err := ma.RoleTemplateLister.Get("", prtb.RoleTemplateName)
				if err != nil {
					return err
				}
				if rt.ProjectCreatorDefault && rt.Builtin {
					callerIsProjectOwner = true
				}
				if rt.Name == "project-member" {
					callerIsProjectMember = true
				}
			}
		}
		if projectRoleTemplateFoundCount != len(newProjectRoleTemplateMap) {
			// user does not have prtbs for all input roles in this project, find the roles for which there are no prtbs
			customRolesFound := false
			inputRolesContainProjectOwnerRole := false
			projectRolesToAddMap := make(map[string]bool)
			for role := range newProjectRoleTemplateMap {
				projectRolesToAddMap[role] = true
			}
			_, rolesNotFound, _ := set.Diff(projectRoleTemplateFoundMap, projectRolesToAddMap)
			// find if any of the roles for which prtbs aren't found are custom
			for _, r := range rolesNotFound {
				if rt, ok := newProjectRoleTemplateMap[r]; ok {
					if !rt.Builtin {
						customRolesFound = true
					}
					if rt.ProjectCreatorDefault && rt.Builtin {
						// this is the "project-owner" role
						inputRolesContainProjectOwnerRole = true
					}
				}
			}
			// check if caller is project-owner, project-member or cluster-owner
			if callerIsProjectOwner && !customRolesFound {
				// project-owner should be allowed to add any built-in project roles
				continue
			}
			if callerIsProjectMember && !customRolesFound && !inputRolesContainProjectOwnerRole {
				// project-member should be allowed to add any built-in project role, EXCEPT the project-owner role
				continue
			}

			// check if caller is cluster-owner
			crtbs, err := ma.CrtbLister.List(cname, labels.NewSelector())
			if err != nil {
				return err
			}
			for _, crtb := range crtbs {
				if crtb.UserName == callerID {
					rt, err := ma.RoleTemplateLister.Get("", crtb.RoleTemplateName)
					if err != nil {
						return err
					}
					if rt.ClusterCreatorDefault && rt.Builtin {
						// caller is the owner of the cluster that this project belongs to, no need to check other crtbs
						callerIsClusterOwner = true
						clustersCallerIsOwnerOf[cname] = true
						break
					}
				}
			}
			if callerIsClusterOwner && !customRolesFound {
				// cluster-owner should be allowed to add any built-in roles
				continue
			}
			// either the user is not one of these: project-owner, project-member or cluster-owner, OR
			// the passed in roles have some custom roles which the user does not have prtbs/crtbs for
			p, err := ma.ProjectLister.Get(cname, pname)
			if err != nil {
				return err
			}
			projectName := pname
			if p.Spec.DisplayName != "" {
				projectName = p.Spec.DisplayName
			}
			// get display name of cluster
			c, err := ma.ClusterLister.Get("", cname)
			if err != nil {
				return err
			}
			clusterName := cname
			if c.Spec.DisplayName != "" {
				clusterName = c.Spec.DisplayName
			}
			roleMissing = true
			missingRoles := strings.Join(rolesNotFound, ",")
			projErr := fmt.Sprintf("roles %v in project %v of cluster %v, ", missingRoles, projectName, clusterName)
			errMsg += projErr
		}
	}

	if len(newClusterRoleTemplateMap) == 0 && !roleMissing {
		return nil
	}
	for cname := range clusters {
		clusterRoleTemplateFoundCount := 0
		clusterRoleTemplateFoundMap := make(map[string]bool)
		clusterOwner := clustersCallerIsOwnerOf[cname]
		crtbs, err := ma.CrtbLister.List(cname, labels.NewSelector())
		if err != nil {
			return err
		}
		for _, crtb := range crtbs {
			if crtb.UserName == callerID {
				if _, ok := newClusterRoleTemplateMap[crtb.RoleTemplateName]; ok {
					clusterRoleTemplateFoundMap[crtb.RoleTemplateName] = true
					clusterRoleTemplateFoundCount++
				}
				if clusterOwner {
					// we already found a crtb with roletemplate cluster-owner for the caller in this cluster
					continue
				}
				rt, err := ma.RoleTemplateLister.Get("", crtb.RoleTemplateName)
				if err != nil {
					return err
				}
				if rt.ClusterCreatorDefault && rt.Builtin {
					clusterOwner = true
				}
			}
		}
		if clusterRoleTemplateFoundCount != len(newClusterRoleTemplateMap) {
			customRolesFound := false
			clusterRolesToAddMap := make(map[string]bool)
			for role := range newClusterRoleTemplateMap {
				clusterRolesToAddMap[role] = true
			}
			_, rolesNotFound, _ := set.Diff(clusterRoleTemplateFoundMap, clusterRolesToAddMap)
			// find if any of the roles for which prtbs aren't found are builtin
			for _, r := range rolesNotFound {
				if rt, ok := newProjectRoleTemplateMap[r]; ok {
					if !rt.Builtin {
						customRolesFound = true
					}
				}
			}
			if clusterOwner && !customRolesFound {
				// caller is cluster-owner of current cluster, relax this check
				continue
			}

			// get cluster's displayName
			c, err := ma.ClusterLister.Get("", cname)
			if err != nil {
				return err
			}
			clusterName := cname
			if c.Spec.DisplayName != "" {
				clusterName = c.Spec.DisplayName
			}
			roleMissing = true
			missingRoles := strings.Join(rolesNotFound, ",")
			clusErr := fmt.Sprintf("roles %v in cluster %v, ", missingRoles, clusterName)
			errMsg += clusErr
		}
	}
	if roleMissing {
		errMsg := strings.TrimRight(errMsg, ", ")
		return httperror.NewAPIError(httperror.PermissionDenied, errMsg)
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
		if userPrincipalName, ok := m[client.MemberFieldUserPrincipalID]; ok && userPrincipalName != nil {
			newMemberAccessType[convert.ToString(m[client.MemberFieldUserPrincipalID])] = convert.ToString(m[client.MemberFieldAccessType])
		}
		if groupPrincipalName, ok := m[client.MemberFieldGroupPrincipalID]; ok && groupPrincipalName != nil {
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
	isAdmin, err := ma.IsAdmin(callerID)
	if err != nil {
		return "", err
	}
	if isAdmin {
		// global admins should be allowed to update mcapp, irrespective of their accessType if they're added as member, or
		// even if they aren't added as member at all
		return OwnerAccess, nil
	}
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
		if m.GroupPrincipalName == "*" {
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

func (ma *MemberAccess) RemoveRolesFromTargets(targetProjects, rolesToRemove []string, mcappName string, removeAllRoles bool) error {
	systemUserPrincipalID := fmt.Sprintf("system://%s", mcappName)
	// from given targets, remove prtbs/crtbs created for user with system account's userID
	rolesToRemoveMap := make(map[string]bool)
	if !removeAllRoles {
		for _, role := range rolesToRemove {
			rolesToRemoveMap[role] = true
		}
	}

	for _, target := range targetProjects {
		split := strings.SplitN(target, ":", 2)
		if len(split) != 2 {
			errMsg := fmt.Sprintf("Invalid project ID: %v", target)
			return httperror.NewAPIError(httperror.InvalidBodyContent, errMsg)
		}
		clusterName, projectName := split[0], split[1]
		clustersCovered := make(map[string]bool)
		prtbs, err := ma.PrtbLister.List(projectName, labels.NewSelector())
		if err != nil {
			return err
		}
		for _, prtb := range prtbs {
			if prtb.UserPrincipalName == systemUserPrincipalID {
				if removeAllRoles || rolesToRemoveMap[prtb.RoleTemplateName] {
					if err = ma.Prtbs.DeleteNamespaced(projectName, prtb.Name, &v1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
						return err
					}
				}
			}
		}

		if !clustersCovered[clusterName] {
			crtbs, err := ma.CrtbLister.List(clusterName, labels.NewSelector())
			if err != nil {
				return err
			}
			for _, crtb := range crtbs {
				if crtb.UserPrincipalName == systemUserPrincipalID {
					if removeAllRoles || rolesToRemoveMap[crtb.RoleTemplateName] {
						if err = ma.Crtbs.DeleteNamespaced(clusterName, crtb.Name, &v1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
							return err
						}
					}
				}
			}
			clustersCovered[clusterName] = true
		}
	}
	return nil
}

func (ma *MemberAccess) checkProjectExists(target string) error {
	split := strings.SplitN(target, ":", 2)
	if len(split) != 2 {
		return httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("invalid project ID %s", target))
	}
	_, err := ma.ProjectLister.Get(split[0], split[1])
	if err != nil {
		if apierrors.IsNotFound(err) {
			return httperror.NewAPIError(httperror.InvalidOption, fmt.Sprintf("project is not found %s", target))
		}
		return httperror.NewAPIError(httperror.InvalidOption, fmt.Sprintf("error getting project %s: %v", target, err))
	}
	return nil
}

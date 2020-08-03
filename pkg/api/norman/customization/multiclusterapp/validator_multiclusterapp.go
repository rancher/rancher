package multiclusterapp

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/set"
	"github.com/rancher/norman/types/values"
	gaccess "github.com/rancher/rancher/pkg/api/norman/customization/globalnamespaceaccess"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type Wrapper struct {
	MultiClusterApps              v3.MultiClusterAppInterface
	MultiClusterAppLister         v3.MultiClusterAppLister
	MultiClusterAppRevisionLister v3.MultiClusterAppRevisionLister
	PrtbLister                    v3.ProjectRoleTemplateBindingLister
	CrtbLister                    v3.ClusterRoleTemplateBindingLister
	RoleTemplateLister            v3.RoleTemplateLister
	Users                         v3.UserInterface
	GrbLister                     v3.GlobalRoleBindingLister
	GrLister                      v3.GlobalRoleLister
	Prtbs                         v3.ProjectRoleTemplateBindingInterface
	Crtbs                         v3.ClusterRoleTemplateBindingInterface
	ProjectLister                 v3.ProjectLister
	ClusterLister                 v3.ClusterLister
	Apps                          pv3.AppInterface
	TemplateVersionLister         v3.CatalogTemplateVersionLister
}

const (
	mcAppLabel   = "io.cattle.field/multiClusterAppId"
	creatorIDAnn = "field.cattle.io/creatorId"
)

func (w Wrapper) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "revisions":
		var app client.MultiClusterApp
		if err := access.ByID(apiContext, &managementschema.Version, client.MultiClusterAppType, apiContext.ID, &app); err != nil {
			return err
		}
		var appRevisions, filtered []map[string]interface{}
		if err := access.List(apiContext, &managementschema.Version, client.MultiClusterAppRevisionType, &types.QueryOptions{}, &appRevisions); err != nil {
			return err
		}
		for _, revision := range appRevisions {
			labels := convert.ToMapInterface(revision["labels"])
			if reflect.DeepEqual(labels[mcAppLabel], app.Name) {
				filtered = append(filtered, revision)
			}
		}
		sort.SliceStable(filtered, func(i, j int) bool {
			val1, err := time.Parse(time.RFC3339, convert.ToString(values.GetValueN(filtered[i], "created")))
			if err != nil {
				logrus.Infof("error parsing time %v", err)
			}
			val2, err := time.Parse(time.RFC3339, convert.ToString(values.GetValueN(filtered[j], "created")))
			if err != nil {
				logrus.Infof("error parsing time %v", err)
			}
			return val1.After(val2)
		})
		apiContext.Type = client.MultiClusterAppRevisionType
		apiContext.WriteResponse(http.StatusOK, filtered)
		return nil
	}
	return nil
}

func (w Wrapper) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	if request.Method != http.MethodPut && request.Method != http.MethodPost {
		return nil
	}
	ma := gaccess.MemberAccess{
		Users:              w.Users,
		PrtbLister:         w.PrtbLister,
		CrtbLister:         w.CrtbLister,
		RoleTemplateLister: w.RoleTemplateLister,
		GrbLister:          w.GrbLister,
		GrLister:           w.GrLister,
		Prtbs:              w.Prtbs,
		Crtbs:              w.Crtbs,
		ProjectLister:      w.ProjectLister,
		ClusterLister:      w.ClusterLister,
	}
	var targetProjects []string
	callerID := request.Request.Header.Get(gaccess.ImpersonateUserHeader)
	if request.Method == http.MethodPost {
		// create request, caller is owner/creator
		// Request is POST, hence multiclusterapp is being created.
		// check if creator has the given roles in all target projects
		targets, _ := values.GetSlice(data, client.MultiClusterAppFieldTargets)
		for _, t := range targets {
			targetProjects = append(targetProjects, convert.ToString(t[client.TargetFieldProjectID]))
		}
		roleTemplates := convert.ToStringSlice(data[client.MultiClusterAppFieldRoles])
		return ma.EnsureRoleInTargets(targetProjects, roleTemplates, callerID)
	}
	// edit request, only editing members, roles and answers is allowed through this
	split := strings.SplitN(request.ID, ":", 2)
	if len(split) != 2 {
		return fmt.Errorf("incorrect multiclusterapp ID %v", request.ID)
	}
	mcapp, err := w.MultiClusterApps.GetNamespaced(split[0], split[1], v1.GetOptions{})
	if err != nil {
		return err
	}
	metaAccessor, err := meta.Accessor(mcapp)
	if err != nil {
		return err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
	if !ok {
		return fmt.Errorf("MultiClusterApp %v has no creatorId annotation", metaAccessor.GetName())
	}
	accessType, err := ma.GetAccessTypeOfCaller(callerID, creatorID, mcapp.Name, mcapp.Spec.Members)
	if err != nil {
		return err
	}
	if accessType == gaccess.ReadonlyAccess {
		return fmt.Errorf("read-only members cannot update multiclusterapp")
	}
	ownerAccess := accessType == gaccess.OwnerAccess
	// only members and roles list, and templateversion/answers can be edited through PUT, for updating target projects, we need to use actions only
	// that's why target projects field has been made non updatable in rancher/types
	if err := gaccess.CheckAccessToUpdateMembers(mcapp.Spec.Members, data, ownerAccess); err != nil {
		return err
	}

	// check whether roles are being edited, if yes then only owner should be allowed to, and should have this role in all projects
	roles := convert.ToStringSlice(data[client.MultiClusterAppFieldRoles])
	newRoles := make(map[string]bool)
	for _, r := range roles {
		newRoles[r] = true
	}
	originalRoles := make(map[string]bool)
	for _, r := range mcapp.Spec.Roles {
		originalRoles[r] = true
	}
	// get difference between new and original roles
	rolesToAdd, rolesToRemove, _ := set.Diff(newRoles, originalRoles)
	if len(rolesToAdd) == 0 && len(rolesToRemove) == 0 {
		// this UPDATE request is not modifying the multiclusterapp roles.
		// So return without calling EnsureRoleInTargets, because this UPDATE could be called by a mcapp member with access type member,
		// just to update the templateversion or answers; this member is allowed to not have all roles in all targets
		return nil
	}
	if (len(rolesToAdd) != 0 || len(rolesToRemove) != 0) && !ownerAccess {
		return fmt.Errorf("user %v is not an owner of multiclusterapp %v, cannot edit roles", callerID, mcapp.Name)
	}
	// check if modifier has all roles listed in toAdd in all projects
	for _, t := range mcapp.Spec.Targets {
		targetProjects = append(targetProjects, t.ProjectName)
	}
	if err = ma.EnsureRoleInTargets(targetProjects, rolesToAdd, callerID); err != nil {
		return err
	}
	// at this point we know the roles have changed, and that the user has the new roles in targets,
	// retry underlying app upgrade
	for _, t := range mcapp.Spec.Targets {
		_, projectNS := ref.Parse(t.ProjectName)
		if t.AppName != "" {
			err := wait.ExponentialBackoff(backoff, func() (bool, error) {
				app, err := w.Apps.GetNamespaced(projectNS, t.AppName, v1.GetOptions{})
				if err != nil || app == nil {
					return false, fmt.Errorf("error %v getting app %s in %s", err, t.AppName, projectNS)
				}
				if val, ok := app.Labels["mcapp"]; !ok || val != mcapp.Name {
					return false, fmt.Errorf("app %s in %s missing multi cluster app label", t.AppName, projectNS)
				}
				v32.AppConditionUserTriggeredAction.True(app)
				if _, err := w.Apps.Update(app); err != nil {
					if apierrors.IsConflict(err) {
						return false, nil
					}
					return false, err
				}
				return true, nil
			})
			if err != nil {
				return err
			}
		}
	}
	return ma.RemoveRolesFromTargets(targetProjects, rolesToRemove, mcapp.Name, false)
}

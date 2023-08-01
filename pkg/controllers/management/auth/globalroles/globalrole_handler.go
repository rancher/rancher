package globalroles

import (
	"reflect"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	globalRoleLabel       = map[string]string{"authz.management.cattle.io/globalrole": "true"}
	crNameAnnotation      = "authz.management.cattle.io/cr-name"
	initialSyncAnnotation = "authz.management.cattle.io/initial-sync"
	clusterRoleKind       = "ClusterRole"
)

func newGlobalRoleLifecycle(management *config.ManagementContext) *globalRoleLifecycle {
	return &globalRoleLifecycle{
		crLister: management.RBAC.ClusterRoles("").Controller().Lister(),
		crClient: management.RBAC.ClusterRoles(""),
		rLister:  management.RBAC.Roles("").Controller().Lister(),
		rClient:  management.RBAC.Roles(""),
	}
}

type globalRoleLifecycle struct {
	crLister rbacv1.ClusterRoleLister
	crClient rbacv1.ClusterRoleInterface
	rLister  rbacv1.RoleLister
	rClient  rbacv1.RoleInterface
}

func (gr *globalRoleLifecycle) Create(obj *v3.GlobalRole) (runtime.Object, error) {
	var returnError error
	err := gr.reconcileGlobalRole(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	err = gr.reconcileCatalogRole(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	return obj, returnError
}

func (gr *globalRoleLifecycle) Updated(obj *v3.GlobalRole) (runtime.Object, error) {
	var returnError error
	err := gr.reconcileGlobalRole(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	err = gr.reconcileCatalogRole(obj)
	if err != nil {
		returnError = multierror.Append(returnError, err)
	}
	return nil, returnError
}

func (gr *globalRoleLifecycle) Remove(obj *v3.GlobalRole) (runtime.Object, error) {
	// Don't need to delete the created ClusterRole because owner reference will take care of that
	return nil, nil
}

func (gr *globalRoleLifecycle) reconcileGlobalRole(globalRole *v3.GlobalRole) error {
	crName := getCRName(globalRole)

	clusterRole, _ := gr.crLister.Get("", crName)
	if clusterRole != nil {
		if !reflect.DeepEqual(globalRole.Rules, clusterRole.Rules) {
			clusterRole.Rules = globalRole.Rules
			logrus.Infof("[%v] Updating clusterRole %v. GlobalRole rules have changed. Have: %+v. Want: %+v", grController, clusterRole.Name, clusterRole.Rules, globalRole.Rules)
			if _, err := gr.crClient.Update(clusterRole); err != nil {
				return errors.Wrapf(err, "couldn't update ClusterRole %v", clusterRole.Name)
			}
		}
		return nil
	}

	logrus.Infof("[%v] Creating clusterRole %v for corresponding GlobalRole", grController, crName)
	_, err := gr.crClient.Create(&v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: globalRole.TypeMeta.APIVersion,
					Kind:       globalRole.TypeMeta.Kind,
					Name:       globalRole.Name,
					UID:        globalRole.UID,
				},
			},
			Labels: globalRoleLabel,
		},
		Rules: globalRole.Rules,
	})
	if err != nil {
		return err
	}
	// Add an annotation to the globalrole indicating the name we used for future updates
	if globalRole.Annotations == nil {
		globalRole.Annotations = map[string]string{}
	}
	globalRole.Annotations[crNameAnnotation] = crName
	return nil
}

func (gr *globalRoleLifecycle) reconcileCatalogRole(globalRole *v3.GlobalRole) error {
	// rules which give template/template version access need to have a specific namespaced role created, since the
	// backend resources that they grant access to are namespaced resources
	var catalogRules []v1.PolicyRule
	for _, rule := range globalRole.Rules {
		ruleGivesTemplateAccess := rbac.RuleGivesResourceAccess(rule, TemplateResourceRule)
		ruleGivesTemplateVersionAccess := rbac.RuleGivesResourceAccess(rule, TemplateVersionResourceRule)
		if !(ruleGivesTemplateAccess || ruleGivesTemplateVersionAccess) {
			// if rule doesn't give access to templates or template versions, move on without evaluating further
			continue
		}
		ruleCopy := rule.DeepCopy()
		ruleCopy.APIGroups = []string{mgmt.GroupName}
		ruleCopy.Resources = []string{}
		// NonResource URLS are only used for ClusterRoles - these roles are namespaced, so no need to include
		ruleCopy.NonResourceURLs = []string{}
		if ruleGivesTemplateAccess {
			ruleCopy.Resources = append(ruleCopy.Resources, catalogTemplateResourceRule)
		}
		if ruleGivesTemplateVersionAccess {
			ruleCopy.Resources = append(ruleCopy.Resources, catalogTemplateVersionResourceRule)
		}
		catalogRules = append(catalogRules, *ruleCopy)
	}
	if len(catalogRules) == 0 {
		return nil
	}
	// if this GR gives access to templates/template versions, create a role in cattle-global-data for access
	roleName := globalRole.Name + "-" + GlobalCatalogRole
	role, err := gr.rLister.Get(namespace.GlobalNamespace, roleName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		role = &v1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleName,
				Namespace: namespace.GlobalNamespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: globalRole.APIVersion,
						Kind:       globalRole.Kind,
						Name:       globalRole.Name,
						UID:        globalRole.UID,
					},
				},
			},
			Rules: catalogRules,
		}
		_, err = gr.rClient.Create(role)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	} else {
		// if we found the role, make sure that the rules are up-to-date and give the same access as their grs
		updateRule := false
		for _, rule := range catalogRules {
			ruleFound := false
			for _, existingRule := range globalRole.Rules {
				if reflect.DeepEqual(rule, existingRule) {
					ruleFound = true
					break
				}
			}
			if !ruleFound {
				// if we need to update any individual rule, just replace them all
				updateRule = true
				break
			}
		}
		if updateRule {
			newRole := role.DeepCopy()
			newRole.Rules = catalogRules
			_, err := gr.rClient.Update(newRole)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getCRName(globalRole *v3.GlobalRole) string {
	if crName, ok := globalRole.Annotations[crNameAnnotation]; ok {
		return crName
	}
	return generateCRName(globalRole.Name)
}

func generateCRName(name string) string {
	return "cattle-globalrole-" + name
}

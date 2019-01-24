package auth

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"

	"k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	globalRoleBindingLabel = map[string]string{"authz.management.cattle.io/globalrolebinding": "true"}
	crbNameAnnotation      = "authz.management.cattle.io/crb-name"
	crbNamePrefix          = "cattle-globalrolebinding-"
	grbController          = "mgmt-auth-grb-controller"
)

const (
	globalCatalogRole                  = "global-catalog"
	globalCatalogRoleBinding           = "global-catalog-binding"
	templateResourceRule               = "templates"
	templateVersionResourceRule        = "templateversions"
	catalogTemplateResourceRule        = "catalogtemplates"
	catalogTemplateVersionResourceRule = "catalogtemplateversions"
)

func newGlobalRoleBindingLifecycle(management *config.ManagementContext) *globalRoleBindingLifecycle {
	return &globalRoleBindingLifecycle{
		crbLister:         management.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crbClient:         management.RBAC.ClusterRoleBindings(""),
		grLister:          management.Management.GlobalRoles("").Controller().Lister(),
		roles:             management.RBAC.Roles(""),
		roleLister:        management.RBAC.Roles("").Controller().Lister(),
		roleBindings:      management.RBAC.RoleBindings(""),
		roleBindingLister: management.RBAC.RoleBindings("").Controller().Lister(),
	}
}

type globalRoleBindingLifecycle struct {
	crbLister         rbacv1.ClusterRoleBindingLister
	grLister          v3.GlobalRoleLister
	crbClient         rbacv1.ClusterRoleBindingInterface
	roles             rbacv1.RoleInterface
	roleLister        rbacv1.RoleLister
	roleBindings      rbacv1.RoleBindingInterface
	roleBindingLister rbacv1.RoleBindingLister
}

func (grb *globalRoleBindingLifecycle) Create(obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	err := grb.reconcileGlobalRoleBinding(obj)
	return obj, err
}

func (grb *globalRoleBindingLifecycle) Updated(obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	err := grb.reconcileGlobalRoleBinding(obj)
	return nil, err
}

func (grb *globalRoleBindingLifecycle) Remove(obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	// Don't need to delete the created ClusterRole because owner reference will take care of that
	return nil, nil
}

func (grb *globalRoleBindingLifecycle) reconcileGlobalRoleBinding(globalRoleBinding *v3.GlobalRoleBinding) error {
	var catalogTemplateRule, catalogTemplateVersionRule *v1.PolicyRule
	crbName, ok := globalRoleBinding.Annotations[crbNameAnnotation]
	if !ok {
		crbName = crbNamePrefix + globalRoleBinding.Name
	}
	subject := v1.Subject{
		Kind:     "User",
		Name:     globalRoleBinding.UserName,
		APIGroup: rbacv1.GroupName,
	}
	crb, _ := grb.crbLister.Get("", crbName)
	if crb != nil {
		subjects := []v1.Subject{subject}
		updateSubject := !reflect.DeepEqual(subjects, crb.Subjects)

		updateRoleRef := false
		var roleRef v1.RoleRef
		gr, _ := grb.grLister.Get("", globalRoleBinding.GlobalRoleName)
		if gr != nil {
			crNameFromGR := getCRName(gr)
			if crNameFromGR != crb.RoleRef.Name {
				updateRoleRef = true
				roleRef = v1.RoleRef{
					Name: crNameFromGR,
					Kind: clusterRoleKind,
				}
			}
		}
		if updateSubject || updateRoleRef {
			crb = crb.DeepCopy()
			if updateRoleRef {
				crb.RoleRef = roleRef
			}
			crb.Subjects = subjects
			logrus.Infof("[%v] Updating clusterRoleBinding %v for globalRoleBinding %v user %v", grbController, crb.Name, globalRoleBinding.Name, globalRoleBinding.UserName)
			if _, err := grb.crbClient.Update(crb); err != nil {
				return errors.Wrapf(err, "couldn't update ClusterRoleBinding %v", crb.Name)
			}
		}
		return nil
	}

	logrus.Infof("Creating new GlobalRoleBinding for GlobalRoleBinding %v", globalRoleBinding.Name)
	gr, _ := grb.grLister.Get("", globalRoleBinding.GlobalRoleName)
	var crName string
	if gr != nil {
		crName = getCRName(gr)
	} else {
		crName = generateCRName(globalRoleBinding.GlobalRoleName)
	}
	logrus.Infof("[%v] Creating clusterRoleBinding for globalRoleBinding %v for user %v with role %v", grbController, globalRoleBinding.Name, globalRoleBinding.UserName, crName)
	_, err := grb.crbClient.Create(&v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: crbName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: globalRoleBinding.TypeMeta.APIVersion,
					Kind:       globalRoleBinding.TypeMeta.Kind,
					Name:       globalRoleBinding.Name,
					UID:        globalRoleBinding.UID,
				},
			},
			Labels: globalRoleBindingLabel,
		},
		Subjects: []v1.Subject{subject},
		RoleRef: v1.RoleRef{
			Name: crName,
			Kind: clusterRoleKind,
		},
	})
	if err != nil {
		return err
	}
	// Add an annotation to the globalrole indicating the name we used for future updates
	if globalRoleBinding.Annotations == nil {
		globalRoleBinding.Annotations = map[string]string{}
	}
	globalRoleBinding.Annotations[crbNameAnnotation] = crbName

	// Check if the current globalRole has rules for templates and templateversions
	gr, _ = grb.grLister.Get("", globalRoleBinding.GlobalRoleName)
	if gr != nil {
		for _, rule := range gr.Rules {
			for _, resource := range rule.Resources {
				if resource == templateResourceRule {
					catalogTemplateRule = &v1.PolicyRule{
						APIGroups: rule.APIGroups,
						Resources: []string{catalogTemplateResourceRule},
						Verbs:     rule.Verbs,
					}
				} else if resource == templateVersionResourceRule {
					catalogTemplateVersionRule = &v1.PolicyRule{
						APIGroups: rule.APIGroups,
						Resources: []string{catalogTemplateVersionResourceRule},
						Verbs:     rule.Verbs,
					}
				}
			}
		}
	}
	// If rules for "templates and "templateversions" exists, create a role for the granting access to
	// catalogtemplates and catalogtemplateversions in the global namespace
	var rules []v1.PolicyRule
	if catalogTemplateRule != nil {
		rules = append(rules, *catalogTemplateRule)
	}
	if catalogTemplateVersionRule != nil {
		rules = append(rules, *catalogTemplateVersionRule)
	}
	if len(rules) > 0 {
		_, err = grb.roles.GetNamespaced(namespace.GlobalNamespace, globalCatalogRole, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				role := &v1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      globalCatalogRole,
						Namespace: namespace.GlobalNamespace,
					},
					Rules: rules,
				}
				_, err = grb.roles.Create(role)
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return err
				}
			} else {
				return err
			}
		}
		// Create a rolebinding, referring the above role, and using globalrole user.Username as the subject
		// Check if rb exists first!
		grbName := globalRoleBinding.UserName + "-" + globalCatalogRoleBinding
		_, err = grb.roleBindings.GetNamespaced(namespace.GlobalNamespace, grbName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				rb := &v1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      grbName,
						Namespace: namespace.GlobalNamespace,
					},
					Subjects: []v1.Subject{subject},
					RoleRef: v1.RoleRef{
						Kind: "Role",
						Name: globalCatalogRole,
					},
				}
				_, err = grb.roleBindings.Create(rb)
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return err
				}
			} else {
				return err
			}
		}
	}
	return nil
}

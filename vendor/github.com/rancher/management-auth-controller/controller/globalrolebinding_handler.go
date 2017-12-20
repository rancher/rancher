package controller

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	globalRoleBindingLabel = map[string]string{"authz.management.cattle.io/globalrolebinding": "true"}
	crbNameAnnotation      = "authz.management.cattle.io/crb-name"
	crbNamePrefix          = "cattle-globalrolebinding-"
)

func newGlobalRoleBindingLifecycle(management *config.ManagementContext) *globalRoleBindingLifecycle {
	return &globalRoleBindingLifecycle{
		crbLister: management.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crbClient: management.RBAC.ClusterRoleBindings(""),
		grLister:  management.Management.GlobalRoles("").Controller().Lister(),
	}
}

type globalRoleBindingLifecycle struct {
	crbLister rbacv1.ClusterRoleBindingLister
	grLister  v3.GlobalRoleLister
	crbClient rbacv1.ClusterRoleBindingInterface
}

func (grb *globalRoleBindingLifecycle) Create(obj *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
	err := grb.reconcileGlobalRoleBinding(obj)
	return obj, err
}

func (grb *globalRoleBindingLifecycle) Updated(obj *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
	err := grb.reconcileGlobalRoleBinding(obj)
	return nil, err
}

func (grb *globalRoleBindingLifecycle) Remove(obj *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
	// Don't need to delete the created ClusterRole because owner reference will take care of that
	return nil, nil
}

func (grb *globalRoleBindingLifecycle) reconcileGlobalRoleBinding(globalRoleBinding *v3.GlobalRoleBinding) error {
	crbName, ok := globalRoleBinding.Annotations[crbNameAnnotation]
	if !ok {
		crbName = crbNamePrefix + globalRoleBinding.Name
	}
	crb, _ := grb.crbLister.Get("", crbName)
	if crb != nil {
		subjects := []v1.Subject{globalRoleBinding.Subject}
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
		Subjects: []v1.Subject{globalRoleBinding.Subject},
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

	return nil
}

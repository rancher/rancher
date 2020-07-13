package auth

import (
	"reflect"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	globalRoleLabel  = map[string]string{"authz.management.cattle.io/globalrole": "true"}
	crNameAnnotation = "authz.management.cattle.io/cr-name"
	clusterRoleKind  = "ClusterRole"
	grController     = "mgmt-auth-gr-controller"
)

func newGlobalRoleLifecycle(management *config.ManagementContext) *globalRoleLifecycle {
	return &globalRoleLifecycle{
		crLister: management.RBAC.ClusterRoles("").Controller().Lister(),
		crClient: management.RBAC.ClusterRoles(""),
	}
}

type globalRoleLifecycle struct {
	crLister rbacv1.ClusterRoleLister
	crClient rbacv1.ClusterRoleInterface
}

func (gr *globalRoleLifecycle) Create(obj *v3.GlobalRole) (runtime.Object, error) {
	err := gr.reconcileGlobalRole(obj)
	return obj, err
}

func (gr *globalRoleLifecycle) Updated(obj *v3.GlobalRole) (runtime.Object, error) {
	err := gr.reconcileGlobalRole(obj)
	return nil, err
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

func getCRName(globalRole *v3.GlobalRole) string {
	if crName, ok := globalRole.Annotations[crNameAnnotation]; ok {
		return crName
	}
	return generateCRName(globalRole.Name)
}

func generateCRName(name string) string {
	return "cattle-globalrole-" + name
}

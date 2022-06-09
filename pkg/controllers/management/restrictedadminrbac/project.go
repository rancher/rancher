package restrictedadminrbac

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	k8srbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *rbaccontroller) projectRBACSync(key string, project *apimgmtv3.Project) (runtime.Object, error) {
	if project == nil || project.DeletionTimestamp != nil {
		return nil, nil
	}

	if project.Namespace == "local" {
		return nil, nil
	}

	var returnErr error
	grbs, err := r.grbIndexer.ByIndex(grbByRoleIndex, rbac.GlobalRestrictedAdmin)
	if err != nil {
		return nil, err
	}
	for _, x := range grbs {
		grb, _ := x.(*v3.GlobalRoleBinding)
		rbName := fmt.Sprintf("%s-%s", grb.Name, rbac.RestrictedAdminProjectRoleBinding)
		rb, err := r.rbLister.Get(project.Name, rbName)
		if err != nil && !k8serrors.IsNotFound(err) {
			returnErr = multierror.Append(returnErr, err)
			continue
		}
		if rb != nil {
			continue
		}
		_, err = r.roleBindings.Create(&k8srbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rbName,
				Namespace: project.Name,
				Labels:    map[string]string{rbac.RestrictedAdminProjectRoleBinding: "true"},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: grb.TypeMeta.APIVersion,
						Kind:       grb.TypeMeta.Kind,
						UID:        grb.UID,
						Name:       grb.Name,
					},
				},
			},
			RoleRef: k8srbac.RoleRef{
				Name: rbac.ProjectCRDsClusterRole,
				Kind: "ClusterRole",
			},
			Subjects: []k8srbac.Subject{
				rbac.GetGRBSubject(grb),
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			returnErr = multierror.Append(returnErr, err)
		}

	}
	return nil, returnErr
}

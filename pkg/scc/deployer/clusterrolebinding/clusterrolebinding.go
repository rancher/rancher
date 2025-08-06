package clusterrolebinding

import (
	"context"
	"fmt"

	rbacControllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/deployer/types"
	"github.com/rancher/rancher/pkg/scc/util/log"
)

// Deployer implements ResourceDeployer for ClusterRoleBinding resources
type Deployer struct {
	log                 log.StructuredLogger
	clusterRoleBindings rbacControllers.ClusterRoleBindingController
}

func NewDeployer(log log.StructuredLogger, clusterRoleBindings rbacControllers.ClusterRoleBindingController) *Deployer {
	return &Deployer{
		log:                 log.WithField("deployer", "cluster-role-binding"),
		clusterRoleBindings: clusterRoleBindings,
	}
}

func (d Deployer) Ensure(_ context.Context, labels map[string]string) error {
	_, err := d.clusterRoleBindings.Get(consts.ClusterRoleBindingName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error checking for cluster role binding %s: %w", consts.ClusterRoleBindingName, err)
	}

	if errors.IsNotFound(err) {
		// ClusterRoleBinding to give the service account cluster-admin
		desiredClusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: consts.ClusterRoleBindingName,
				Labels: map[string]string{
					consts.LabelK8sManagedBy: "rancher",
					consts.LabelK8sPartOf:    consts.DeploymentName,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     consts.ClusterRoleName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Namespace: consts.DefaultSCCNamespace,
					Name:      consts.ServiceAccountName,
				},
			},
		}

		_, err = d.clusterRoleBindings.Create(desiredClusterRoleBinding)
		if err != nil {
			return fmt.Errorf("error creating cluster role binding %s: %w", consts.ClusterRoleBindingName, err)
		}

		d.log.Infof("Created cluster role binding: %s", consts.ClusterRoleBindingName)
	}

	return nil
}

var _ types.ResourceDeployer = &Deployer{}

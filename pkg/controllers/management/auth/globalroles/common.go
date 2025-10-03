package globalroles

import (
	"errors"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	mgmtconv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteAdminClusterRoleBindings(
	clusterClient mgmtconv3.ClusterClient,
	clusterManager *clustermanager.Manager,
	grb *v3.GlobalRoleBinding,
) error {
	// Explicit API call to ensure we have the most recent cluster info when deleting admin bindings
	clusters, err := clusterClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	crbName := rbac.GrbCRBName(grb)

	var errs error // Collect all the errors to delete as many user context bindings as possible.
	for _, cluster := range clusters.Items {
		userContext, err := clusterManager.UserContext(cluster.Name)
		if err != nil {
			if !clustermanager.IsClusterUnavailableErr(err) {
				errs = errors.Join(errs, fmt.Errorf("can't get user context for cluster %s: %w", cluster.Name, err))
			}
			continue
		}

		crb, err := userContext.RBACw.ClusterRoleBinding().Cache().Get(crbName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				errs = errors.Join(errs, fmt.Errorf("can't get ClusterRoleBinding %s for cluster %s: %w", crbName, cluster.Name, err))
			}
			continue
		}

		logrus.Infof("Deleting ClusterRoleBinding %s for admin GlobalRoleBinding %s for cluster %s", crbName, grb.Name, cluster.Name)

		err = userContext.RBACw.ClusterRoleBinding().Delete(crb.Name, &metav1.DeleteOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				errs = errors.Join(errs, fmt.Errorf("can't delete admin ClusterRoleBinding %s for cluster %s: %w", crbName, cluster.Name, err))
			}
			continue
		}
	}

	return errs
}

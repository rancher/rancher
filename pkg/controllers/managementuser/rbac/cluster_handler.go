package rbac

import (
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	k8srbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	grbByRoleIndex = "management.cattle.io/grb-by-role"
)

func newClusterHandler(workload *config.UserContext) v3.ClusterHandlerFunc { //*clusterHandler {
	informer := workload.Management.Management.GlobalRoleBindings("").Controller().Informer()

	ch := &clusterHandler{
		grbIndexer: informer.GetIndexer(),
		// Management level resources
		grbController: workload.Management.Management.GlobalRoleBindings("").Controller(),
		clusters:      workload.Management.Management.Clusters(""),
		// User context resources
		userGRB:       workload.RBAC.ClusterRoleBindings(""),
		userGRBLister: workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
	}
	return ch.sync
}

type clusterHandler struct {
	grbIndexer cache.Indexer
	// Management level resources
	grbController v3.GlobalRoleBindingController
	clusters      v3.ClusterInterface
	// User context resources
	userGRB       v1.ClusterRoleBindingInterface
	userGRBLister v1.ClusterRoleBindingLister
}

func (h *clusterHandler) sync(key string, obj *v3.Cluster) (runtime.Object, error) {
	if key == "" || obj == nil {
		return nil, nil
	}

	if !v32.ClusterConditionGlobalAdminsSynced.IsTrue(obj) {
		err := h.doSync(obj)
		if err != nil {
			return nil, err
		}
		return h.clusters.Update(obj)
	}
	return obj, nil
}

func (h *clusterHandler) doSync(cluster *v3.Cluster) error {
	_, err := v32.ClusterConditionGlobalAdminsSynced.DoUntilTrue(cluster, func() (runtime.Object, error) {
		grbs, err := h.grbIndexer.ByIndex(grbByRoleIndex, "admin")
		if err != nil {
			return nil, err
		}

		for _, x := range grbs {
			grb, _ := x.(*v3.GlobalRoleBinding)
			bindingName := rbac.GrbCRBName(grb)
			b, err := h.userGRBLister.Get("", bindingName)
			if err != nil && !k8serrors.IsNotFound(err) {
				return nil, err
			}

			if b != nil {
				// binding exists, nothing to do
				continue
			}

			_, err = h.userGRB.Create(&k8srbac.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: bindingName,
				},
				Subjects: []k8srbac.Subject{
					{
						Kind: "User",
						Name: grb.UserName,
					},
				},
				RoleRef: k8srbac.RoleRef{
					Name: "cluster-admin",
					Kind: "ClusterRole",
				},
			})
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

func grbByRole(obj interface{}) ([]string, error) {
	grb, ok := obj.(*v3.GlobalRoleBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{grb.GlobalRoleName}, nil
}

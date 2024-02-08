package rbac

import (
	"fmt"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	k8srbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

const (
	grbByRoleIndex = "management.cattle.io/grb-by-role"
)

var (
	errNotFound     = apierrors.NewNotFound(schema.GroupResource{}, "")
	errAlreadyExist = apierrors.NewAlreadyExists(schema.GroupResource{}, "")
)

func newClusterHandler(workload *config.UserContext) v3.ClusterHandlerFunc { //*clusterHandler {
	informer := workload.Management.Management.GlobalRoleBindings("").Controller().Informer()

	ch := &clusterHandler{
		clusterName: workload.ClusterName,
		grbIndexer:  informer.GetIndexer(),
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
	clusterName string
	grbIndexer  cache.Indexer
	// Management level resources
	grbController v3.GlobalRoleBindingController
	clusters      v3.ClusterInterface
	// User context resources
	userGRB       v1.ClusterRoleBindingInterface
	userGRBLister v1.ClusterRoleBindingLister
}

func (h *clusterHandler) sync(key string, obj *v3.Cluster) (runtime.Object, error) {
	// We receive clusters with no data, when that happens no checks will work so just ignore them
	if key == "" || obj == nil || obj.Name == "" {
		return nil, nil
	}

	// Don't operate on a cluster this handler isn't created for
	if h.clusterName != obj.Name {
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

// doSync syncs CRBs for all GlobalAdmins to the clustere role 'cluster-admin'.
func (h *clusterHandler) doSync(cluster *v3.Cluster) error {
	_, err := v32.ClusterConditionGlobalAdminsSynced.DoUntilTrue(cluster, func() (runtime.Object, error) {
		grbs, err := h.grbIndexer.ByIndex(grbByRoleIndex, rbac.GlobalAdmin)
		if err != nil {
			return nil, fmt.Errorf("failed to list GlobalRoleBindings for global-admin: %w", err)
		}

		for _, x := range grbs {
			grb, ok := x.(*v32.GlobalRoleBinding)
			if !ok || grb == nil {
				continue
			}
			bindingName := rbac.GrbCRBName(grb)
			_, err := h.userGRBLister.Get("", bindingName)
			if !k8serrors.IsNotFound(err) {
				return nil, fmt.Errorf("failed to get GlobalRoleBinding for '%s': %w", bindingName, err)
			}

			if err == nil {
				// binding exists, nothing to do
				continue
			}

			_, err = h.userGRB.Create(&k8srbac.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: bindingName,
				},
				Subjects: []k8srbac.Subject{
					rbac.GetGRBSubject(grb),
				},
				RoleRef: k8srbac.RoleRef{
					Name: "cluster-admin",
					Kind: "ClusterRole",
				},
			})
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				return nil, fmt.Errorf("failed to create new ClusterRoleBinding for GlobalRoleBinding '%s': %w", grb.Name, err)
			}
		}
		return nil, nil
	})
	return err
}

func grbByRole(obj interface{}) ([]string, error) {
	grb, ok := obj.(*v32.GlobalRoleBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{grb.GlobalRoleName}, nil
}

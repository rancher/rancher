package rbac

import (
	"fmt"
	"time"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/status"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

const (
	grbByUserAndRoleIndex            = "authz.cluster.cattle.io/grb-by-user-and-role"
	grbHandlerName                   = "grb-cluster-sync"
	clusterAdminRoleExists           = "ClusterAdminRoleExists"
	failedToCreateClusterRoleBinding = "FailedToCreateClusterRoleBinding"
	failedToGetClusterRoleBinding    = "FailedToGetClusterRoleBinding"
)

func RegisterIndexers(scaledContext *config.ScaledContext) error {
	informer := scaledContext.Management.GlobalRoleBindings("").Controller().Informer()
	indexers := map[string]cache.IndexFunc{
		grbByUserAndRoleIndex: grbByUserAndRole,
		grbByRoleIndex:        grbByRole,
	}
	if err := informer.AddIndexers(indexers); err != nil {
		return err
	}

	// Add cache informer to project role template bindings
	prtbInformer := scaledContext.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	prtbIndexers := map[string]cache.IndexFunc{
		prtbByProjectIndex:               prtbByProjectName,
		prtbByProjecSubjectIndex:         prtbByProjectAndSubject,
		rtbByClusterAndRoleTemplateIndex: rtbByClusterAndRoleTemplateName,
		prtbByUIDIndex:                   prtbByUID,
		prtbByNsAndNameIndex:             prtbByNsName,
		rtbByClusterAndUserIndex:         rtbByClusterAndUserNotDeleting,
	}
	if err := prtbInformer.AddIndexers(prtbIndexers); err != nil {
		return err
	}

	crtbInformer := scaledContext.Management.ClusterRoleTemplateBindings("").Controller().Informer()
	crtbIndexers := map[string]cache.IndexFunc{
		rtbByClusterAndRoleTemplateIndex: rtbByClusterAndRoleTemplateName,
		rtbByClusterAndUserIndex:         rtbByClusterAndUserNotDeleting,
	}
	return crtbInformer.AddIndexers(crtbIndexers)
}

func newGlobalRoleBindingHandler(workload *config.UserContext) v3.GlobalRoleBindingHandlerFunc {

	h := &grbHandler{
		clusterName:         workload.ClusterName,
		clusterRoleBindings: workload.RBAC.ClusterRoleBindings(""),
		crbLister:           workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		// The following clients/controllers all point at the management cluster
		grLister:  workload.Management.Management.GlobalRoles("").Controller().Lister(),
		grbLister: workload.Management.Wrangler.Mgmt.GlobalRoleBinding().Cache(),
		grbClient: workload.Management.Wrangler.Mgmt.GlobalRoleBinding(),
		status:    status.NewStatus(),
	}

	return h.sync
}

// grbHandler ensures the global admins have full access to every cluster. If a globalRoleBinding is created that uses
// the admin role, then the user in that binding gets a clusterRoleBinding in every user cluster to the cluster-admin role
type grbHandler struct {
	clusterName         string
	clusterRoleBindings rbacv1.ClusterRoleBindingInterface
	crbLister           rbacv1.ClusterRoleBindingLister
	grLister            v3.GlobalRoleLister
	grbLister           mgmtv3.GlobalRoleBindingCache
	grbClient           mgmtv3.GlobalRoleBindingController
	status              *status.Status
}

func (c *grbHandler) sync(key string, obj *apisv3.GlobalRoleBinding) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}
	var remoteConditions []metav1.Condition
	isAdmin, err := rbac.IsAdminGlobalRole(obj.GlobalRoleName, c.grLister)
	if err != nil {
		return nil, err
	}
	if !isAdmin {
		return obj, nil
	}

	logrus.Debugf("%v is an admin role", obj.GlobalRoleName)
	if err := c.ensureClusterAdminBinding(obj, &remoteConditions); err != nil {
		return nil, err
	}

	return obj, c.updateStatus(obj, remoteConditions)
}

// ensureClusterAdminBinding creates a ClusterRoleBinding for GRB subject to
// the Kubernetes "cluster-admin" ClusterRole in the downstream cluster.
func (c *grbHandler) ensureClusterAdminBinding(obj *apisv3.GlobalRoleBinding, conditions *[]metav1.Condition) error {
	condition := metav1.Condition{Type: clusterAdminRoleExists}
	bindingName := rbac.GrbCRBName(obj)
	_, err := c.crbLister.Get("", bindingName)
	if err != nil && !apierrors.IsNotFound(err) {
		c.status.AddCondition(conditions, condition, failedToGetClusterRoleBinding, err)
		return fmt.Errorf("failed to get ClusterRoleBinding '%s' from the cache: %w", bindingName, err)
	}

	if err == nil {
		// binding exists, nothing to do
		c.status.AddCondition(conditions, condition, clusterAdminRoleExists, nil)
		return nil
	}

	_, err = c.clusterRoleBindings.Create(&v12.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
		},
		Subjects: []v12.Subject{rbac.GetGRBSubject(obj)},
		RoleRef: v12.RoleRef{
			Name: "cluster-admin",
			Kind: "ClusterRole",
		},
	})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		c.status.AddCondition(conditions, condition, failedToCreateClusterRoleBinding, err)
		return fmt.Errorf("failed to create ClusterRoleBinding '%s' for admin in downstream '%s': %w", bindingName, c.clusterName, err)
	}

	c.status.AddCondition(conditions, condition, clusterAdminRoleExists, nil)
	return nil
}

func grbByUserAndRole(obj interface{}) ([]string, error) {
	grb, ok := obj.(*apisv3.GlobalRoleBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{rbac.GetGRBTargetKey(grb) + "-" + grb.GlobalRoleName}, nil
}

func (c *grbHandler) updateStatus(grb *apisv3.GlobalRoleBinding, remoteConditions []metav1.Condition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		grbFromCluster, err := c.grbLister.Get(grb.Name)
		if err != nil {
			return err
		}
		if len(grb.Status.RemoteConditions) > 0 && status.CompareConditions(grbFromCluster.Status.RemoteConditions, remoteConditions) {
			return nil
		}
		if len(remoteConditions) == 0 && grbFromCluster.Status.SummaryRemote == status.SummaryCompleted {
			return nil
		}

		grbFromCluster.Status.SummaryRemote = status.SummaryCompleted
		if grbFromCluster.Status.SummaryLocal == status.SummaryCompleted {
			grbFromCluster.Status.Summary = status.SummaryCompleted
		}
		for _, c := range remoteConditions {
			if c.Status != metav1.ConditionTrue {
				grbFromCluster.Status.Summary = status.SummaryError
				grbFromCluster.Status.SummaryRemote = status.SummaryError
				break
			}
		}
		grbFromCluster.Status.LastUpdateTime = c.status.TimeNow().Format(time.RFC3339)
		grbFromCluster.Status.ObservedGenerationRemote = grb.ObjectMeta.Generation
		grbFromCluster.Status.RemoteConditions = remoteConditions
		grbFromCluster, err = c.grbClient.UpdateStatus(grbFromCluster)
		if err != nil {
			return err
		}
		return nil
	})
}

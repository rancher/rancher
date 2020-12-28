package healthsyncer

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/condition"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/pkg/ticker"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const (
	syncInterval = 15 * time.Second
)

type ClusterControllerLifecycle interface {
	Stop(cluster *v3.Cluster)
}

type HealthSyncer struct {
	ctx               context.Context
	clusterName       string
	clusterLister     v3.ClusterLister
	clusters          v3.ClusterInterface
	componentStatuses corev1.ComponentStatusInterface
	namespaces        corev1.NamespaceInterface
	k8s               kubernetes.Interface
}

func Register(ctx context.Context, workload *config.UserContext) {
	h := &HealthSyncer{
		ctx:               ctx,
		clusterName:       workload.ClusterName,
		clusterLister:     workload.Management.Management.Clusters("").Controller().Lister(),
		clusters:          workload.Management.Management.Clusters(""),
		componentStatuses: workload.Core.ComponentStatuses(""),
		namespaces:        workload.Core.Namespaces(""),
		k8s:               workload.K8sClient,
	}

	go h.syncHealth(ctx, syncInterval)
}

func (h *HealthSyncer) syncHealth(ctx context.Context, syncHealth time.Duration) {
	for range ticker.Context(ctx, syncHealth) {
		err := h.updateClusterHealth()
		if err != nil && !apierrors.IsConflict(err) {
			logrus.Error(err)
		}
	}
}

func (h *HealthSyncer) getComponentStatus(cluster *v3.Cluster) error {
	ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
	defer cancel()

	// Prior to k8s v1.14, we only needed to list the ComponentStatuses from the user cluster.
	// As of k8s v1.14, kubeapi returns a successful ComponentStatuses response even if etcd is not available.
	// To work around this, now we try to get a namespace from the API, even if not found, it means the API is up.
	if _, err := h.k8s.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return condition.Error("ComponentStatsFetchingFailure", errors.Wrap(err, "Failed to communicate with API server"))
	}

	cses, err := h.componentStatuses.List(metav1.ListOptions{})
	if err != nil {
		return condition.Error("ComponentStatsFetchingFailure", errors.Wrap(err, "Failed to communicate with API server"))
	}
	cluster.Status.ComponentStatuses = []v32.ClusterComponentStatus{}
	for _, cs := range cses.Items {
		clusterCS := convertToClusterComponentStatus(&cs)
		cluster.Status.ComponentStatuses = append(cluster.Status.ComponentStatuses, *clusterCS)
	}
	sort.Slice(cluster.Status.ComponentStatuses, func(i, j int) bool {
		return cluster.Status.ComponentStatuses[i].Name < cluster.Status.ComponentStatuses[j].Name
	})
	return nil
}

func (h *HealthSyncer) updateClusterHealth() error {
	oldCluster, err := h.getCluster()
	if err != nil {
		return err
	}
	cluster := oldCluster.DeepCopy()
	if !v32.ClusterConditionProvisioned.IsTrue(cluster) {
		logrus.Debugf("Skip updating cluster health - cluster [%s] not provisioned yet", h.clusterName)
		return nil
	}

	wasReady := v32.ClusterConditionReady.IsTrue(cluster)
	newObj, err := v32.ClusterConditionReady.Do(cluster, func() (runtime.Object, error) {
		for i := 0; ; i++ {
			err := h.getComponentStatus(cluster)
			if err == nil && !wasReady {
				err = h.checkClusterAgent()
			}
			if err == nil || i > 1 {
				return cluster, errors.Wrap(err, "cluster health check failed")
			}
			select {
			case <-h.ctx.Done():
				return cluster, err
			case <-time.After(5 * time.Second):
			}
		}
	})

	if err == nil {
		v32.ClusterConditionWaiting.True(newObj)
		v32.ClusterConditionWaiting.Message(newObj, "")
	}

	if !reflect.DeepEqual(oldCluster, newObj) {
		if _, err := h.clusters.Update(newObj.(*v3.Cluster)); err != nil {
			return errors.Wrapf(err, "[updateClusterHealth] Failed to update cluster [%s]", cluster.Name)
		}
	}

	// Purposefully not return error.  This is so when the cluster goes unavailable we don't just keep failing
	// which will essentially keep the controller alive forever, instead of shutting down.
	return nil
}

func (h *HealthSyncer) checkClusterAgent() error {
	if c, err := h.clusterLister.Get("", h.clusterName); err == nil && c.Spec.Internal {
		return nil
	}

	deployment, err := h.k8s.AppsV1().Deployments(namespace.System).Get(h.ctx, "cattle-cluster-agent", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if condition.Cond(appsv1.DeploymentAvailable).IsTrue(deployment) {
		return nil
	}
	return fmt.Errorf("cluster agent is not ready")
}

func (h *HealthSyncer) getCluster() (*v3.Cluster, error) {
	return h.clusterLister.Get("", h.clusterName)
}

func convertToClusterComponentStatus(cs *v1.ComponentStatus) *v32.ClusterComponentStatus {
	return &v32.ClusterComponentStatus{
		Name:       cs.Name,
		Conditions: cs.Conditions,
	}
}

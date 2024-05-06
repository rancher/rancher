package healthsyncer

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"time"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types/slice"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/clusterconnected"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/ticker"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	syncInterval = 15 * time.Second
)

// kubernetes/apimachinery/pkg/util/version/version.go
var versionMatchRE = regexp.MustCompile(`^\s*v?([0-9]+(?:\.[0-9]+)*)(.*)*$`)

var excludedComponentMap = map[string][]string{
	"aks":     {"controller-manager", "scheduler"},
	"tencent": {"controller-manager", "scheduler"},
	"lke":     {"controller-manager", "scheduler"},
}

// ComponentStatus is disabled with k8s v1.22
var componentStatusDisabledRange, _ = semver.ParseRange(">=1.22.0")

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
	// Prior to k8s v1.14, we only needed to list the ComponentStatuses from the user cluster.
	// As of k8s v1.14, kubeapi returns a successful ComponentStatuses response even if etcd is not available.
	// To work around this, now we try to get a namespace from the API, even if not found, it means the API is up.
	if err := IsAPIUp(h.ctx, h.k8s.CoreV1().Namespaces()); err != nil {
		return condition.Error("ComponentStatusFetchingFailure", errors.Wrap(err, "Failed to communicate with API server during namespace check"))
	}

	cluster.Status.ComponentStatuses = []v32.ClusterComponentStatus{}
	if cluster.Status.Version == nil {
		return nil
	}
	parts := versionMatchRE.FindStringSubmatch(cluster.Status.Version.String())
	if parts == nil || len(parts) < 2 {
		return condition.Error("ComponentStatusFetchingFailure", fmt.Errorf("Failed to parse cluster status version %s",
			cluster.Status.Version.String()))
	}
	k8sVersion, err := semver.Parse(parts[1])
	if err != nil {
		return condition.Error("ComponentStatusFetchingFailure", fmt.Errorf("Failed to parse cluster k8s version %s",
			cluster.Status.Version.String()))
	}
	if componentStatusDisabledRange(k8sVersion) {
		return nil
	}
	cses, err := h.componentStatuses.List(metav1.ListOptions{})
	if err != nil {
		return condition.Error("ComponentStatusFetchingFailure", errors.Wrap(err, "Failed to communicate with API server"))
	}
	clusterType := cluster.Status.Provider // the provider detector is more accurate but we can fall back on the driver in some cases
	if clusterType == "" {
		clusterType = cluster.Status.Driver
	}
	excludedComponents, ok := excludedComponentMap[clusterType]
	for _, cs := range cses.Items {
		if ok && slice.ContainsString(excludedComponents, cs.Name) {
			continue
		}
		clusterCS := convertToClusterComponentStatus(&cs)
		cluster.Status.ComponentStatuses = append(cluster.Status.ComponentStatuses, *clusterCS)
	}
	sort.Slice(cluster.Status.ComponentStatuses, func(i, j int) bool {
		return cluster.Status.ComponentStatuses[i].Name < cluster.Status.ComponentStatuses[j].Name
	})

	return nil
}

// IsAPIUp checks if the Kubernetes API server is up and etcd is available.
// It gets a namespace from the API, even if not found, it means the API is up.
// It returns nil if the API is up, otherwise it returns an error.
func IsAPIUp(ctx context.Context, ns typedv1.NamespaceInterface) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if _, err := ns.Get(ctx, "kube-system", metav1.GetOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
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

	// cluster condition ready is set to false if connected is false, return to avoid setting it to true incorrectly
	if clusterconnected.Connected.IsFalse(cluster) {
		logrus.Debugf("Skip updating cluster condition ready - cluster agent for [%s] isn't connected yet", h.clusterName)
		return nil
	}

	newObj, err := v32.ClusterConditionReady.Do(cluster, func() (runtime.Object, error) {
		for i := 0; ; i++ {
			err := h.getComponentStatus(cluster)
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

func (h *HealthSyncer) getCluster() (*v3.Cluster, error) {
	return h.clusterLister.Get("", h.clusterName)
}

func convertToClusterComponentStatus(cs *v1.ComponentStatus) *v32.ClusterComponentStatus {
	return &v32.ClusterComponentStatus{
		Name:       cs.Name,
		Conditions: cs.Conditions,
	}
}

package healthsyncer

import (
	"fmt"
	"time"

	"context"

	"github.com/rancher/cluster-agent/utils"
	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	syncInterval = 15 * time.Second
	msgReady     = "Cluster ready to serve API"
	msgNotReady  = "Cluster not ready to serve API"
)

type HealthSyncer struct {
	clusterName       string
	Clusters          v3.ClusterInterface
	ComponentStatuses corev1.ComponentStatusInterface
}

func Register(ctx context.Context, workload *config.ClusterContext) {
	h := &HealthSyncer{
		clusterName:       workload.ClusterName,
		Clusters:          workload.Management.Management.Clusters(""),
		ComponentStatuses: workload.Core.ComponentStatuses(""),
	}

	go h.syncHealth(ctx, syncInterval)
}

func (h *HealthSyncer) syncHealth(ctx context.Context, syncHealth time.Duration) {
	for range utils.TickerContext(ctx, syncHealth) {
		err := h.updateClusterHealth()
		if err != nil {
			logrus.Info(err)
		}
	}
}

func (h *HealthSyncer) updateClusterHealth() error {
	cluster, err := h.getCluster()
	if err != nil {
		return err
	}
	if cluster == nil {
		logrus.Info("Skip updating cluster health, cluster [%s] deleted", h.clusterName)
		return nil
	}
	if !utils.IsClusterProvisioned(cluster) {
		return fmt.Errorf("Skip updating cluster health - cluster [%s] not provisioned yet", h.clusterName)
	}
	cses, err := h.ComponentStatuses.List(metav1.ListOptions{})
	if err != nil {
		logrus.Debugf("Error getting componentstatuses for server health %v", err)
		updateConditionStatus(cluster, v3.ClusterConditionReady, v1.ConditionFalse, msgNotReady)
		return nil
	}
	updateConditionStatus(cluster, v3.ClusterConditionReady, v1.ConditionTrue, msgReady)
	logrus.Debugf("Cluster [%s] Condition Ready", h.clusterName)

	h.updateClusterStatus(cluster, cses.Items)
	_, err = h.Clusters.Update(cluster)
	if err != nil {
		return fmt.Errorf("Failed to update cluster [%s] %v", cluster.Name, err)
	}
	logrus.Debugf("Updated cluster health successfully [%s]", h.clusterName)
	return nil
}

func (h *HealthSyncer) updateClusterStatus(cluster *v3.Cluster, cses []v1.ComponentStatus) {
	cluster.Status.ComponentStatuses = []v3.ClusterComponentStatus{}
	for _, cs := range cses {
		clusterCS := convertToClusterComponentStatus(&cs)
		cluster.Status.ComponentStatuses = append(cluster.Status.ComponentStatuses, *clusterCS)
	}
}

func (h *HealthSyncer) getCluster() (*v3.Cluster, error) {
	return h.Clusters.Get(h.clusterName, metav1.GetOptions{})
}

func convertToClusterComponentStatus(cs *v1.ComponentStatus) *v3.ClusterComponentStatus {
	return &v3.ClusterComponentStatus{
		Name:       cs.Name,
		Conditions: cs.Conditions,
	}
}

func updateConditionStatus(cluster *v3.Cluster, conditionType v3.ClusterConditionType, status v1.ConditionStatus, msg string) {
	pos, condition := getConditionByType(cluster, conditionType)
	currTime := time.Now().UTC().Format(time.RFC3339)

	if condition != nil {
		if condition.Status != status {
			condition.Status = status
			condition.LastTransitionTime = currTime
		}
		condition.LastUpdateTime = currTime
		condition.Reason = msg
		cluster.Status.Conditions[pos] = *condition
	}
}

func getConditionByType(cluster *v3.Cluster, conditionType v3.ClusterConditionType) (int, *v3.ClusterCondition) {
	for index, condition := range cluster.Status.Conditions {
		if condition.Type == conditionType {
			return index, &condition
		}
	}
	return -1, nil
}

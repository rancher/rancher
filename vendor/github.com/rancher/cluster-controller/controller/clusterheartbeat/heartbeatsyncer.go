package clusterheartbeat

import (
	"time"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	syncInterval = 20 * time.Second
)

var clusterToLastUpdated map[string]time.Time

type HeartBeatSyncer struct {
	ClusterClient v3.ClusterInterface
}

func Register(management *config.ManagementContext) {
	h := &HeartBeatSyncer{
		ClusterClient: management.Management.Clusters(""),
	}
	management.Management.Clusters("").Controller().AddHandler(h.sync)

	clusterToLastUpdated = make(map[string]time.Time)
	go h.syncHeartBeat(syncInterval)
}

func (h *HeartBeatSyncer) sync(key string, cluster *v3.Cluster) error {
	logrus.Debugf("Syncing cluster [%s] ", key)
	if cluster == nil {
		// cluster has been deleted
		if _, exists := clusterToLastUpdated[key]; exists {
			delete(clusterToLastUpdated, key)
			logrus.Debugf("Cluster [%s] already deleted", key)
		}
	} else {
		condition := getConditionIfReady(cluster)
		if condition != nil {
			lastUpdateTime, _ := time.Parse(time.RFC3339, condition.LastUpdateTime)
			clusterToLastUpdated[key] = lastUpdateTime
			logrus.Debugf("Synced cluster [%s] successfully", key)
		}
	}
	logrus.Debugf("Syncing cluster [%s] complete ", key)
	return nil
}

func (h *HeartBeatSyncer) syncHeartBeat(syncInterval time.Duration) {
	for _ = range time.Tick(syncInterval) {
		logrus.Debugf("Start heartbeat")
		h.checkHeartBeat()
		logrus.Debugf("Heartbeat complete")
	}
}

func (h *HeartBeatSyncer) checkHeartBeat() {
	for clusterName, lastUpdatedTime := range clusterToLastUpdated {
		if lastUpdatedTime.Add(syncInterval).Before(time.Now().UTC()) {
			cluster, err := h.ClusterClient.Get(clusterName, metav1.GetOptions{})
			if err != nil {
				logrus.Errorf("Error getting Cluster [%s] - %v", clusterName, err)
				continue
			}
			setConditionStatus(cluster, v3.ClusterConditionReady, corev1.ConditionUnknown)
			logrus.Infof("Cluster [%s] condition status unknown", clusterName)
			err = h.update(cluster)
			if err != nil {
				logrus.Errorf("Error getting Cluster [%s] - %v", clusterName, err)
				continue
			}
		}
	}
}

func (h *HeartBeatSyncer) update(cluster *v3.Cluster) error {
	_, err := h.ClusterClient.Update(cluster)
	return err
}

func getConditionByType(cluster *v3.Cluster, conditionType v3.ClusterConditionType) (int, *v3.ClusterCondition) {
	for index, condition := range cluster.Status.Conditions {
		if condition.Type == conditionType {
			return index, &condition
		}
	}
	return -1, nil
}

// Condition is Ready if conditionType is Ready and conditionStatus is True/False but not unknown.
func getConditionIfReady(cluster *v3.Cluster) *v3.ClusterCondition {
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == v3.ClusterConditionReady && condition.Status != corev1.ConditionUnknown {
			return &condition
		}
	}
	return nil
}

func setConditionStatus(cluster *v3.Cluster, conditionType v3.ClusterConditionType, status corev1.ConditionStatus) {
	pos, condition := getConditionByType(cluster, conditionType)
	currTime := time.Now().Format(time.RFC3339)
	if condition != nil {
		if condition.Status != status {
			condition.Status = status
			condition.LastTransitionTime = currTime
		}
		condition.LastUpdateTime = currTime
		cluster.Status.Conditions[pos] = *condition
	}
}

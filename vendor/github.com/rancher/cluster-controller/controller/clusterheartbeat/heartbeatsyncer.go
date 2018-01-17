package clusterheartbeat

import (
	"context"
	"sync"
	"time"

	"github.com/rancher/cluster-agent/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	syncInterval       = 20 * time.Second
	msgBehindOnPing    = "Not received heartbeat from cluster-agent"
	reasonBehindOnPing = "BehindOnPing"
)

type updateData struct {
	updated        bool //True if updated by cluster update from cluster-agent. False if updated by periodic sync.
	lastUpdateTime time.Time
}

var (
	clusterToLastUpdated map[string]*updateData
	mapLock              = sync.Mutex{}
)

type HeartBeatSyncer struct {
	ClusterLister v3.ClusterLister
}

func Register(ctx context.Context, management *config.ManagementContext) {
	clustersClient := management.Management.Clusters("")

	h := &HeartBeatSyncer{
		ClusterLister: clustersClient.Controller().Lister(),
	}
	clustersClient.AddLifecycle(h.GetName(), h)

	clusterToLastUpdated = make(map[string]*updateData)
	go h.syncHeartBeat(ctx, syncInterval)
}

func (h *HeartBeatSyncer) Create(cluster *v3.Cluster) (*v3.Cluster, error) {
	h.storeLastUpdateTime(cluster)
	return nil, nil
}

func (h *HeartBeatSyncer) Updated(cluster *v3.Cluster) (*v3.Cluster, error) {
	h.storeLastUpdateTime(cluster)
	return nil, nil
}

func (h *HeartBeatSyncer) Remove(cluster *v3.Cluster) (*v3.Cluster, error) {
	mapLock.Lock()
	defer mapLock.Unlock()

	key := cluster.Name
	if _, exists := clusterToLastUpdated[key]; exists {
		delete(clusterToLastUpdated, key)
		logrus.Debugf("Cluster [%s] deleted", key)
	}
	return nil, nil
}

func isChanged(cluster *v3.Cluster) (bool, time.Time) {
	condition := getConditionIfReady(cluster)
	if condition != nil {
		lastUpdateTime, _ := time.Parse(time.RFC3339, condition.LastUpdateTime)
		if lastUpdateTime != clusterToLastUpdated[cluster.Name].lastUpdateTime {
			return true, lastUpdateTime
		}
	}
	return false, time.Now()
}

func (h *HeartBeatSyncer) storeLastUpdateTime(cluster *v3.Cluster) {
	mapLock.Lock()
	defer mapLock.Unlock()

	key := cluster.Name
	if _, exists := clusterToLastUpdated[key]; !exists {
		clusterToLastUpdated[key] = &updateData{}
	}
	changed, lastUpdateTime := isChanged(cluster)
	if changed {
		clusterToLastUpdated[key].lastUpdateTime = lastUpdateTime
		clusterToLastUpdated[key].updated = true

		logrus.Debugf("Synced cluster [%s] successfully", key)
	}
}

func (h *HeartBeatSyncer) syncHeartBeat(ctx context.Context, syncInterval time.Duration) {
	for range utils.TickerContext(ctx, syncInterval) {
		logrus.Debugf("Start heartbeat")
		h.checkHeartBeat()
		logrus.Debugf("Heartbeat complete")
	}
}

func (h *HeartBeatSyncer) checkHeartBeat() {
	mapLock.Lock()
	defer mapLock.Unlock()

	for clusterName := range clusterToLastUpdated {
		if !clusterToLastUpdated[clusterName].updated {
			cluster, err := h.ClusterLister.Get("", clusterName)
			if err != nil {
				logrus.Errorf("Error getting Cluster [%s] - %v", clusterName, err)
				continue
			}

			v3.ClusterConditionReady.False(cluster)
			v3.ClusterConditionReady.Message(cluster, msgBehindOnPing)
			v3.ClusterConditionReady.Reason(cluster, reasonBehindOnPing)

			logrus.Debugf("Cluster [%s] condition status unknown", clusterName)
		} else {
			clusterToLastUpdated[clusterName].updated = false
		}
	}
}

// Condition is Ready if conditionType is Ready and conditionStatus is True
func getConditionIfReady(cluster *v3.Cluster) *v3.ClusterCondition {
	for _, condition := range cluster.Status.Conditions {
		if string(condition.Type) == string(v3.ClusterConditionReady) && condition.Status == corev1.ConditionTrue {
			return &condition
		}
	}
	return nil
}

func (h *HeartBeatSyncer) GetName() string {
	return "cluster-heartbeatsync-controller"
}

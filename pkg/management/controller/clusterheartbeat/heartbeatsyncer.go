package clusterheartbeat

import (
	"context"
	"sync"
	"time"

	"github.com/rancher/rancher/pkg/cluster/utils"
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

var clusterToLastUpdated sync.Map

type HeartBeatSyncer struct {
	ClusterLister v3.ClusterLister
	Clusters      v3.ClusterInterface
}

func Register(ctx context.Context, management *config.ManagementContext) {
	clustersClient := management.Management.Clusters("")

	h := &HeartBeatSyncer{
		ClusterLister: clustersClient.Controller().Lister(),
		Clusters:      clustersClient,
	}
	clustersClient.Controller().AddHandler(h.GetName(), h.sync)

	go h.syncHeartBeat(ctx, syncInterval)
}

func (h *HeartBeatSyncer) sync(key string, cluster *v3.Cluster) error {
	if cluster == nil {
		clusterToLastUpdated.Delete(key)
	} else {
		h.storeLastUpdateTime(key, cluster)

	}
	return nil
}

func isChanged(key string, cluster *v3.Cluster) (bool, time.Time) {
	condition := getConditionIfReady(cluster)
	if condition != nil {
		lastUpdateTime, _ := time.Parse(time.RFC3339, condition.LastUpdateTime)
		value, ok := clusterToLastUpdated.Load(key)
		if !ok || lastUpdateTime != value.(updateData).lastUpdateTime {
			return true, lastUpdateTime
		}
	}
	return false, time.Now()
}

func (h *HeartBeatSyncer) storeLastUpdateTime(key string, cluster *v3.Cluster) {
	changed, lastUpdateTime := isChanged(key, cluster)
	if changed {
		newData := updateData{
			lastUpdateTime: lastUpdateTime,
			updated:        true,
		}
		clusterToLastUpdated.Store(key, newData)
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
	clusterToLastUpdated.Range(func(k, v interface{}) bool {
		u := v.(updateData)
		if u.updated {
			u.updated = false
			clusterToLastUpdated.Store(k, u)
			return true
		}
		clusterName := k.(string)
		cluster, err := h.ClusterLister.Get("", clusterName)
		if err != nil {
			logrus.Errorf("Error getting Cluster [%s] - %v", clusterName, err)
			return true
		}

		v3.ClusterConditionReady.False(cluster)
		v3.ClusterConditionReady.Message(cluster, msgBehindOnPing)
		v3.ClusterConditionReady.Reason(cluster, reasonBehindOnPing)
		_, err = h.Clusters.Update(cluster)
		if err != nil {
			logrus.Errorf("Error updating Cluster [%s] - %v", clusterName, err)
		} else {
			logrus.Debugf("Cluster [%s] condition status unknown", clusterName)
		}
		return true
	})
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

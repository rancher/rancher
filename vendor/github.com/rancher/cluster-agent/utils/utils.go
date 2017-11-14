package utils

import (
	"context"
	"time"

	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func IsClusterProvisioned(cluster *v3.Cluster) bool {
	isProvisioned := getClusterConditionByType(cluster, v3.ClusterConditionProvisioned)
	if isProvisioned == nil {
		return false
	}
	return isProvisioned.Status == "True"
}

func getClusterConditionByType(cluster *v3.Cluster, conditionType v3.ClusterConditionType) *v3.ClusterCondition {
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

func TickerContext(ctx context.Context, duration time.Duration) <-chan time.Time {
	ticker := time.NewTicker(duration)
	go func() {
		<-ctx.Done()
		ticker.Stop()
	}()
	return ticker.C
}

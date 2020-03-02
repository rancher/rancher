package cis

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rancher/rancher/pkg/controllers/user/cis"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	DefaultCronSchedule    = "0 0 * * *"
	DefaultRetention       = 3
	ScheduledScanKeyPrefix = "scheduledscan/"
	separator              = ":"
)

type scheduleInfo struct {
	cronSchedule       string
	lastScanLaunchedAt time.Time
	nextScanAt         time.Time
}

type scheduledScanHandler struct {
	sync.Mutex
	managementClient  managementv3.Interface
	clusterClient     v3.ClusterInterface
	clusterLister     v3.ClusterLister
	clusterScanClient v3.ClusterScanInterface
	clusterScanLister v3.ClusterScanLister
	clustersMap       map[string]*scheduleInfo
}

func newScheduleScanHandler(
	managementClient managementv3.Interface,
	clusterClient v3.ClusterInterface,
	clusterLister v3.ClusterLister,
	clusterScanClient v3.ClusterScanInterface,
	clusterScanLister v3.ClusterScanLister,
) *scheduledScanHandler {
	return &scheduledScanHandler{
		managementClient:  managementClient,
		clusterClient:     clusterClient,
		clusterLister:     clusterLister,
		clusterScanClient: clusterScanClient,
		clusterScanLister: clusterScanLister,
		clustersMap:       make(map[string]*scheduleInfo),
	}
}

func (i *scheduleInfo) String() string {
	return fmt.Sprintf("cronSchedule: %v lastAt: %v nextAt: %v",
		i.cronSchedule, i.lastScanLaunchedAt.String(), i.nextScanAt.String())
}

func (ssh *scheduledScanHandler) clusterSync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil ||
		cluster.DeletionTimestamp != nil ||
		cluster.Spec.ScheduledClusterScan == nil {
		return nil, nil
	}

	ssh.Lock()
	defer ssh.Unlock()
	clusterInfo, ok := ssh.clustersMap[cluster.Name]
	if cluster.Spec.ScheduledClusterScan.Enabled {
		if !ok {
			// Do stuff to enable scheduling
			cronSchedule := getCronSchedule(cluster)
			clusterInfo = &scheduleInfo{
				cronSchedule: cronSchedule,
			}
			logrus.Infof("scheduledScanHandler: clusterSync: Scheduled Scan enabled for cluster %v: %v",
				cluster.Name, cronSchedule)
			if err := scheduleScan(cluster, ssh.clusterScanClient, clusterInfo); err != nil {
				return nil, fmt.Errorf("scheduledScanHandler: clusterSync: error scheduling scan: %v", err)
			}
			ssh.clustersMap[cluster.Name] = clusterInfo
		} else {
			if cluster.Spec.ScheduledClusterScan.ScheduleConfig != nil &&
				cluster.Spec.ScheduledClusterScan.ScheduleConfig.CronSchedule != "" &&
				cluster.Spec.ScheduledClusterScan.ScheduleConfig.CronSchedule != clusterInfo.cronSchedule {
				clusterInfo.cronSchedule = cluster.Spec.ScheduledClusterScan.ScheduleConfig.CronSchedule
				logrus.Infof("scheduledScanHandler: clusterSync: cron schedule modified for cluster %v: %v",
					cluster.Name, clusterInfo.cronSchedule)
			}
		}
	} else {
		if ok {
			logrus.Infof("scheduledScanHandler: clusterSync: Scheduled Scan disabled for cluster: %v",
				cluster.Name)
			delete(ssh.clustersMap, cluster.Name)
		}
	}
	return nil, nil
}

func (ssh *scheduledScanHandler) sync(key string, _ *v3.ClusterScan) (runtime.Object, error) {
	if !strings.HasPrefix(key, ScheduledScanKeyPrefix) {
		return nil, nil
	}
	splits := strings.Split(strings.TrimPrefix(key, ScheduledScanKeyPrefix), separator)
	clusterID := splits[0]
	pastCronSchedule := splits[1]
	ssh.Lock()
	clusterInfo, ok := ssh.clustersMap[clusterID]
	ssh.Unlock()
	if !ok {
		logrus.Debugf("scheduledScanHandler: sync: scheduled scan has been disabled, ignoring")
		return nil, nil
	}
	cluster, err := ssh.clusterLister.Get("", clusterID)
	if err != nil {
		return nil, fmt.Errorf("scheduledScanHandler: sync: error fetching cluster %v: %v", clusterID, err)
	}

	if !cluster.Spec.ScheduledClusterScan.Enabled {
		logrus.Debugf("scheduledScanHandler: sync: scheduled scan is disabled, ignoring the current one")
		return nil, nil
	}

	if err := cis.ValidateClusterBeforeLaunchingScan(cluster); err != nil {
		return nil, err
	}

	currentCronSchedule := getCronSchedule(cluster)
	if currentCronSchedule != pastCronSchedule {
		logrus.Infof("scheduledScanHandler: sync: cron schedule has changed for cluster: %v, hence skipping",
			clusterID)
		return nil, nil
	}

	if _, err = checkAndLaunchScan(cluster, ssh.clusterClient, ssh.clusterScanClient, clusterInfo); err != nil {
		return nil, fmt.Errorf("scheduledScanHandler: clusterSync: cluster %v: error launching scan: %v",
			cluster.Name, err)
	}

	retention := getRetention(cluster)
	clusterScanClient := ssh.managementClient.ClusterScans(clusterID)
	if err := cleanOldScans(clusterID, retention, clusterScanClient, ssh.clusterScanLister); err != nil {
		return nil, fmt.Errorf("scheduledScanHandler: clusterSync: error cleaning old scans: %v", err)
	}

	return nil, nil
}

func checkAndLaunchScan(cluster *v3.Cluster,
	clusterClient v3.ClusterInterface,
	clusterScanClient v3.ClusterScanInterface,
	clusterInfo *scheduleInfo,
) (*v3.ClusterScan, error) {
	var cisScanConfig *v3.CisScanConfig
	if cluster.Spec.ScheduledClusterScan.ScanConfig == nil ||
		cluster.Spec.ScheduledClusterScan.ScanConfig.CisScanConfig == nil {
		cisScanConfig = &v3.CisScanConfig{
			Profile: v3.CisScanProfileTypePermissive,
		}
	}

	nextAt := clusterInfo.nextScanAt
	now := time.Now().Round(time.Second)
	if !nextAt.Equal(now) && !now.After(nextAt) {
		logrus.Debugf("scheduledScanHandler: checkAndLaunchScan: cluster %v next: %v, now: %v",
			cluster.Name, nextAt.String(), now.String())
		return nil, nil
	}
	isManual := false
	cisScan, err := cis.LaunchScan(
		isManual,
		cisScanConfig,
		cluster,
		clusterClient,
		clusterScanClient,
		cis.RetryIntervalInMilliseconds,
		cis.NumberOfRetriesForClusterUpdate,
	)
	if err != nil {
		return nil, fmt.Errorf("scheduledScanHandler: checkAndLaunchScan: error launching scan: %v", err)
	}
	clusterInfo.lastScanLaunchedAt = time.Now().Round(time.Second)
	logrus.Debugf("scheduledScanHandler: checkAndLaunchScan: launched scan for cluster %v at: %v",
		cluster.Name, clusterInfo.lastScanLaunchedAt.String())
	if err := scheduleScan(cluster, clusterScanClient, clusterInfo); err != nil {
		return nil, fmt.Errorf("scheduledScanHandler: clusterSync: cluster %v: error scheduling next scan: %v",
			cluster.Name, err)
	}
	return cisScan, nil
}

func scheduleScan(
	cluster *v3.Cluster,
	clusterScanClient v3.ClusterScanInterface,
	clusterInfo *scheduleInfo,
) error {
	cronSchedule := getCronSchedule(cluster)
	key := fmt.Sprintf("%v%v%v%v", ScheduledScanKeyPrefix, cluster.Name, separator, cronSchedule)
	timeAfter, next, err := getTimeAfterAndNext(cronSchedule)
	if err != nil {
		return err
	}
	clusterScanClient.Controller().EnqueueAfter("", key, timeAfter)
	clusterInfo.nextScanAt = next.Round(time.Second)
	logrus.Debugf("scheduledScanHandler: scheduleScan: cluster %v now: %v setting nextScanAt: %v", cluster.Name, time.Now().String(), next.String())
	return nil
}

func getTimeAfterAndNext(cronSchedule string) (time.Duration, time.Time, error) {
	var timeAfter time.Duration
	var next time.Time
	schedule, err := cron.ParseStandard(cronSchedule)
	if err != nil {
		return timeAfter, next, fmt.Errorf("error parsing cron schedule %v: %v", cronSchedule, err)
	}
	now := time.Now()
	next = schedule.Next(now)
	timeAfter = next.Sub(now)
	return timeAfter, next, nil
}

func getCronSchedule(cluster *v3.Cluster) string {
	cronSchedule := DefaultCronSchedule
	if cluster.Spec.ScheduledClusterScan.ScheduleConfig != nil &&
		cluster.Spec.ScheduledClusterScan.ScheduleConfig.CronSchedule != "" {
		cronSchedule = cluster.Spec.ScheduledClusterScan.ScheduleConfig.CronSchedule
	}
	return cronSchedule
}

func getRetention(cluster *v3.Cluster) int {
	retention := DefaultRetention
	if cluster.Spec.ScheduledClusterScan.ScheduleConfig != nil &&
		cluster.Spec.ScheduledClusterScan.ScheduleConfig.Retention > 0 {
		retention = cluster.Spec.ScheduledClusterScan.ScheduleConfig.Retention
	}
	return retention
}
func cleanOldScans(
	clusterID string,
	retention int,
	clusterScanClient v3.ClusterScanInterface,
	clusterScanLister v3.ClusterScanLister,
) error {
	clusterScans, err := clusterScanLister.List(clusterID, labels.Everything())
	if err != nil {
		return fmt.Errorf("error listing cluster scans for cluster %v: %v", clusterID, err)
	}
	if len(clusterScans) <= retention {
		return nil
	}
	sort.Slice(clusterScans, func(i, j int) bool {
		return !clusterScans[i].CreationTimestamp.Before(&clusterScans[j].CreationTimestamp)
	})
	for _, cs := range clusterScans[retention:] {
		logrus.Debugf("scheduledScanHandler: cleanOldScans: deleting cs: %v %v", cs.Name, cs.CreationTimestamp.String())
		if err := deleteClusterScanWithRetry(clusterScanClient, cs.Name); err != nil {
			logrus.Errorf("scheduledScanHandler: cleanOldScans: error deleting cluster scan: %v: %v",
				cs.Name, err)
		}
	}
	return nil
}

func deleteClusterScanWithRetry(clusterScanClient v3.ClusterScanInterface, name string) error {
	var err error
	for retry := NumberOfRetriesForScheduledScanRemoval; retry > 0; retry-- {
		if err = clusterScanClient.Delete(name, &metav1.DeleteOptions{}); err == nil {
			return nil
		}
		time.Sleep(RetryIntervalInMilliseconds * time.Millisecond)
	}
	return err
}

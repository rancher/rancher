package clusterupstreamrefresher

import (
	"fmt"
	"reflect"
	"strings"

	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

//isKubernetesEngineIndexer
const (
	isKEIndexer = "clusters.management.cattle.io/is-ke"
)

var (
	// for other cloud drivers, please edit HERE
	cloudDrivers             = []string{apimgmtv3.ClusterDriverEKS}
	clusterUpstreamRefresher *clusterRefreshController
)

// for other cloud drivers, please edit HERE
type clusterConfig struct {
	eksConfig *eksv1.EKSClusterConfigSpec
}

func init() {
	// possible settings controller, which references refresh
	// cron job, will run prior to .
	// This ensure the CronJob will not be nil
	clusterUpstreamRefresher = &clusterRefreshController{
		refreshCronJob: cron.New(),
	}
}

type clusterRefreshController struct {
	refreshCronJob *cron.Cron
	secretsCache   wranglerv1.SecretCache
	clusterClient  v3.ClusterClient
	clusterCache   v3.ClusterCache
}

func StartClusterUpstreamCronJob(wContext *wrangler.Context) {
	clusterUpstreamRefresher.secretsCache = wContext.Core.Secret().Cache()
	clusterUpstreamRefresher.clusterClient = wContext.Mgmt.Cluster()
	clusterUpstreamRefresher.clusterCache = wContext.Mgmt.Cluster().Cache()

	clusterUpstreamRefresher.clusterCache.AddIndexer(isKEIndexer, func(obj *apimgmtv3.Cluster) ([]string, error) {
		// for other cloud drivers, please edit HERE
		switch {
		case obj.Spec.EKSConfig != nil:
			if obj.Status.EKSStatus.UpstreamSpec == nil {
				logrus.Infof("initial upstream spec for cluster [%s] has not been set by cluster handler yet, skipping", obj.Name)
				return []string{}, nil
			}
			return []string{apimgmtv3.ClusterDriverEKS}, nil
		default:
			return []string{}, nil
		}
	})

	schedule, err := cron.ParseStandard(settings.ClusterUpstreamRefreshCron.Get())
	if err != nil {
		logrus.Errorf("Error parsing upstream cluster refresh cron. Upstream state will not be refreshed: %v", err)
		return
	}
	clusterUpstreamRefresher.refreshCronJob.Schedule(schedule, cron.FuncJob(clusterUpstreamRefresher.refreshAllUpstreamStates))
	clusterUpstreamRefresher.refreshCronJob.Start()
}

func (c *clusterRefreshController) refreshAllUpstreamStates() {
	logrus.Debugf("Refreshing clusters' upstream states")
	for _, cloudDriver := range cloudDrivers {
		clusters, err := c.clusterCache.GetByIndex(isKEIndexer, cloudDriver)
		if err != nil {
			logrus.Error("error trying to refresh clusters' upstream states")
			return
		}

		for _, cluster := range clusters {
			if _, err := c.refreshClusterUpstreamSpec(cluster, cloudDriver); err != nil {
				logrus.Errorf("error refreshing cluster [%s] upstream state", cluster.Name)
			}
		}
	}
}

func (c *clusterRefreshController) refreshClusterUpstreamSpec(cluster *mgmtv3.Cluster, cloudDriver string) (*mgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}

	logrus.Infof("checking cluster [%s] upstream state for changes", cluster.Name)

	// In this call, it is possible to get errors back with non-nil upstreamSpec.
	// If upstreamSpec is nil then the syncing failed for some reason. This is reported to the user, and this function returns at the end of this if-statement.
	// If upstreamSpec is non-nil then the syncing occurred as expected, but the node groups have health issues that are reported to the user.
	// In this second case, the message is set on the Updated condition, but execution continues because the sync was successful.
	upstreamConfig, err := getComparableUpstreamSpec(c.secretsCache, cluster)
	if err != nil {
		var statusErr error
		var syncFailed string
		if upstreamConfig == nil {
			syncFailed = ": syncing failed"
		}
		cluster = cluster.DeepCopy()
		apimgmtv3.ClusterConditionUpdated.False(cluster)
		apimgmtv3.ClusterConditionUpdated.Message(cluster, fmt.Sprintf("[Syncing error%s] %s", syncFailed, err.Error()))

		cluster, statusErr = c.clusterClient.Update(cluster)
		if statusErr != nil {
			return cluster, statusErr
		}

		if upstreamConfig == nil {
			return cluster, err
		}
	} else if strings.Contains(apimgmtv3.ClusterConditionUpdated.GetMessage(cluster), "[Syncing error") {
		cluster = cluster.DeepCopy()
		apimgmtv3.ClusterConditionUpdated.True(cluster)
		apimgmtv3.ClusterConditionUpdated.Message(cluster, "")
		cluster, err = c.clusterClient.Update(cluster)
		if err != nil {
			return cluster, err
		}
	}

	var initialClusterConfig, appliedClusterConfig, upstreamClusterConfig, upstreamSpec interface{}
	// for other cloud drivers, please edit HERE
	switch cloudDriver {
	case apimgmtv3.ClusterDriverEKS:
		initialClusterConfig = cluster.Spec.EKSConfig
		appliedClusterConfig = cluster.Status.AppliedSpec.EKSConfig
		upstreamClusterConfig = cluster.Status.EKSStatus.UpstreamSpec
		upstreamSpec = upstreamConfig.eksConfig
	}

	// compare saved cluster.Status...UpstreamSpec with upstreamSpec,
	// if there is difference then update cluster.Status...UpstreamSpec
	if !reflect.DeepEqual(upstreamClusterConfig, upstreamSpec) {
		logrus.Infof("updating cluster [%s], upstream change detected", cluster.Name)
		cluster = cluster.DeepCopy()
		// for other cloud drivers, please edit HERE
		switch cloudDriver {
		case apimgmtv3.ClusterDriverEKS:
			cluster.Status.EKSStatus.UpstreamSpec = upstreamConfig.eksConfig
		}
		cluster, err = c.clusterClient.Update(cluster)
		if err != nil {
			return cluster, err
		}
	}

	// check if cluster is still updating changes
	if !reflect.DeepEqual(initialClusterConfig, appliedClusterConfig) {
		logrus.Infof("cluster [%s] currently updating, skipping spec sync", cluster.Name)
		return cluster, nil
	}

	// check for changes between upstream spec on cluster and initial ClusterConfig object
	specMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(initialClusterConfig)
	if err != nil {
		return cluster, err
	}

	upstreamSpecMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(upstreamSpec)
	if err != nil {
		return cluster, err
	}

	var updateClusterConfig bool
	for key, value := range upstreamSpecMap {
		if specMap[key] == nil {
			continue
		}
		if reflect.DeepEqual(specMap[key], value) {
			continue
		}
		updateClusterConfig = true
		specMap[key] = value
	}

	if !updateClusterConfig {
		logrus.Infof("cluster [%s] matches upstream, skipping spec sync", cluster.Name)
		return cluster, nil
	}

	// for other cloud drivers, please edit HERE
	switch cloudDriver {
	case apimgmtv3.ClusterDriverEKS:
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(specMap, cluster.Spec.EKSConfig)
	}
	if err != nil {
		return cluster, err
	}

	return c.clusterClient.Update(cluster)
}

func getComparableUpstreamSpec(secretsCache wranglerv1.SecretCache, cluster *mgmtv3.Cluster) (*clusterConfig, error) {
	clusterCfg := &clusterConfig{}

	// for other cloud drivers, please edit HERE
	switch cluster.Status.Driver {
	case apimgmtv3.ClusterDriverEKS:
		eksConfig, err := BuildEKSUpstreamSpec(secretsCache, cluster)
		clusterCfg.eksConfig = eksConfig
		return clusterCfg, err
	default:
		return nil, fmt.Errorf("unsupported cloud driver")
	}
}

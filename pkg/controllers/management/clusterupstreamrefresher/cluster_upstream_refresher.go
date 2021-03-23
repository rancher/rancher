package clusterupstreamrefresher

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	noKEv2Provider         = "none"
	clusterLastRefreshTime = "clusters.management.cattle.io/ke-last-refresh"
)

type clusterRefreshController struct {
	secretsCache        wranglerv1.SecretCache
	clusterClient       v3.ClusterClient
	clusterCache        v3.ClusterCache
	clusterEnqueueAfter func(name string, duration time.Duration)
}

// for other cloud drivers, please edit HERE
type clusterConfig struct {
	eksConfig *eksv1.EKSClusterConfigSpec
}

func Register(ctx context.Context, wContext *wrangler.Context) {
	c := clusterRefreshController{
		secretsCache:        wContext.Core.Secret().Cache(),
		clusterClient:       wContext.Mgmt.Cluster(),
		clusterCache:        wContext.Mgmt.Cluster().Cache(),
		clusterEnqueueAfter: wContext.Mgmt.Cluster().EnqueueAfter,
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "cluster-refresher-controller", c.onClusterChange)
}

func (c *clusterRefreshController) onClusterChange(key string, cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	provider, ready := getProviderAndReadyStatus(cluster)
	if provider == noKEv2Provider {
		// not a KEv2 cluster
		return cluster, nil
	}

	if !ready {
		return cluster, nil
	}

	var lastRefreshTime string
	var err error

	if cluster.Annotations != nil {
		lastRefreshTime = cluster.Annotations[clusterLastRefreshTime]
	}

	providerRefreshInterval, err := getProviderRefreshInterval(provider)
	if err != nil {
		return cluster, err
	}

	shouldRefresh, err := shouldRefreshCluster(providerRefreshInterval, lastRefreshTime)
	if err != nil {
		return cluster, err
	}

	if shouldRefresh {
		return cluster, err
	}

	cluster, err = c.refreshClusterUpstreamSpec(cluster, provider)
	if err != nil {
		return cluster, err
	}

	c.clusterEnqueueAfter(key, providerRefreshInterval)

	return cluster, nil
}

// getProviderAndReadyStatus returns the managed cluster provider of the given
// cluster and whether it is ready to be refresh and synced.
func getProviderAndReadyStatus(cluster *mgmtv3.Cluster) (string, bool) {
	// for other cloud drivers, please edit HERE
	switch {
	case cluster.Spec.EKSConfig != nil:
		if cluster.Status.EKSStatus.UpstreamSpec == nil {
			logrus.Infof("initial upstream spec for cluster [%s] has not been set by cluster handler yet, skipping", cluster.Name)
			return apimgmtv3.ClusterDriverEKS, false
		}
		return apimgmtv3.ClusterDriverEKS, true
	default:
		return noKEv2Provider, false
	}
}

// getProviderRefreshInterval returns the duration that should pass between
// refreshing a cluster created by the given cloud provider.
func getProviderRefreshInterval(provider string) (time.Duration, error) {
	var refreshInterval int

	// for other cloud drivers, please edit HERE
	switch provider {
	case apimgmtv3.ClusterDriverEKS:
		refreshInterval = settings.EKSUpstreamRefresh.GetInt()
	default:
		return 300, nil
	}

	return time.Duration(refreshInterval) * time.Second, nil
}

// shouldRefreshCluster checks lastRefreshTime and refreshInterval and returns
// true when upstream cluster should be refreshed
func shouldRefreshCluster(refreshInterval time.Duration, lastRefreshTime string) (bool, error) {
	if lastRefreshTime == "" {
		return false, fmt.Errorf("lastRefreshTime is required")
	}

	lastRefreshUnix, err := strconv.ParseInt(lastRefreshTime, 10, 64)
	if err != nil {
		return false, fmt.Errorf("unable to parse last KEv2 refresh time [%s]: %v", lastRefreshTime, err)
	}

	return !time.Now().After(time.Unix(lastRefreshUnix, 0).Add(refreshInterval)), nil
}

func (c *clusterRefreshController) refreshClusterUpstreamSpec(cluster *mgmtv3.Cluster, cloudDriver string) (*mgmtv3.Cluster, error) {

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
		logrus.Debugf("cluster [%s] matches upstream, skipping spec sync", cluster.Name)
		return cluster, nil
	}

	// for other cloud drivers, please edit HERE
	switch cloudDriver {
	case apimgmtv3.ClusterDriverEKS:
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(specMap, cluster.Spec.EKSConfig)
		if err != nil {
			return cluster, err
		}
	}

	if cluster.Annotations == nil {
		cluster.Annotations = make(map[string]string)
	}
	cluster.Annotations[clusterLastRefreshTime] = strconv.FormatInt(time.Now().Unix(), 10)

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

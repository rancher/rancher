package clusterupstreamrefresher

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	aksv1 "github.com/rancher/aks-operator/pkg/apis/aks.cattle.io/v1"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	gkev1 "github.com/rancher/gke-operator/pkg/apis/gke.cattle.io/v1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	noKEv2Provider         = "none"
	clusterLastRefreshTime = "clusters.management.cattle.io/ke-last-refresh"
	refreshSettingFormat   = "%s-refresh"
)

type clusterRefreshController struct {
	secretsCache        wranglerv1.SecretCache
	secretClient        wranglerv1.SecretClient
	clusterClient       v3.ClusterClient
	clusterCache        v3.ClusterCache
	clusterEnqueueAfter func(name string, duration time.Duration)
}

// for other cloud drivers, please edit HERE
type clusterConfig struct {
	aksConfig *aksv1.AKSClusterConfigSpec
	eksConfig *eksv1.EKSClusterConfigSpec
	gkeConfig *gkev1.GKEClusterConfigSpec
	aliConfig *aliv1.AliClusterConfigSpec
}

func Register(ctx context.Context, wContext *wrangler.Context) {
	c := clusterRefreshController{
		secretsCache:        wContext.Core.Secret().Cache(),
		secretClient:        wContext.Core.Secret(),
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
	if provider == noKEv2Provider || !ready {
		return cluster, nil
	}

	providerRefreshInterval, err := getProviderRefreshInterval(provider)
	if err != nil {
		return cluster, err
	}

	nextRefresh, err := nextRefreshTime(providerRefreshInterval, cluster.Annotations[clusterLastRefreshTime])
	if err != nil {
		return cluster, err
	}

	now := time.Now()
	if nextRefresh.Before(now) {
		cluster, err = c.refreshClusterUpstreamSpec(cluster, provider)
		if err != nil {
			return cluster, err
		}
		c.clusterEnqueueAfter(key, providerRefreshInterval)
	} else {
		c.clusterEnqueueAfter(key, nextRefresh.Sub(now))
	}

	return cluster, nil
}

// getProviderAndReadyStatus returns the managed cluster provider of the given
// cluster and whether it is ready to be refresh and synced.
func getProviderAndReadyStatus(cluster *mgmtv3.Cluster) (string, bool) {
	// for other cloud drivers, please edit HERE
	switch {
	case cluster.Spec.AKSConfig != nil:
		if cluster.Status.AKSStatus.UpstreamSpec == nil {
			logrus.Debugf("initial upstream spec for cluster [%s] has not been set by cluster handler yet, skipping", cluster.Name)
			return apimgmtv3.ClusterDriverAKS, false
		}
		return apimgmtv3.ClusterDriverAKS, true
	case cluster.Spec.EKSConfig != nil:
		if cluster.Status.EKSStatus.UpstreamSpec == nil {
			logrus.Debugf("initial upstream spec for cluster [%s] has not been set by cluster handler yet, skipping", cluster.Name)
			return apimgmtv3.ClusterDriverEKS, false
		}
		return apimgmtv3.ClusterDriverEKS, true
	case cluster.Spec.GKEConfig != nil:
		if cluster.Status.GKEStatus.UpstreamSpec == nil {
			logrus.Debugf("initial upstream spec for cluster [%s] has not been set by cluster handler yet, skipping", cluster.Name)
			return apimgmtv3.ClusterDriverGKE, false
		}
		return apimgmtv3.ClusterDriverGKE, true
	case cluster.Spec.AliConfig != nil:
		if cluster.Status.AliStatus.UpstreamSpec == nil {
			logrus.Debugf("initial upstream spec for cluster [%s] has not been set by cluster handler yet, skipping", cluster.Name)
			return apimgmtv3.ClusterDriverAlibaba, false
		}
		return apimgmtv3.ClusterDriverAlibaba, true
	default:
		return noKEv2Provider, false
	}
}

// getProviderRefreshInterval returns the duration that should pass between
// refreshing a cluster created by the given cloud provider.
func getProviderRefreshInterval(provider string) (time.Duration, error) {
	providerRefreshInterval := settings.GetSettingByID(fmt.Sprintf(refreshSettingFormat, strings.ToLower(provider)))
	if providerRefreshInterval == "" {
		return 300 * time.Second, nil
	}
	refreshInterval, err := strconv.Atoi(providerRefreshInterval)
	if err != nil {
		return 300 * time.Second, err
	}

	return time.Duration(refreshInterval) * time.Second, nil
}

// nextRefreshTime checks lastRefreshTime and refreshInterval and when the next refresh should occur
func nextRefreshTime(refreshInterval time.Duration, lastRefreshTime string) (time.Time, error) {
	if lastRefreshTime == "" {
		return time.Time{}, nil
	}

	lastRefreshUnix, err := strconv.ParseInt(lastRefreshTime, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("unable to parse last KEv2 refresh time [%s]: %v", lastRefreshTime, err)
	}

	return time.Unix(lastRefreshUnix, 0).Add(refreshInterval), nil
}

func (c *clusterRefreshController) refreshClusterUpstreamSpec(cluster *mgmtv3.Cluster, cloudDriver string) (*mgmtv3.Cluster, error) {
	logrus.Debugf("checking cluster [%s] upstream state for changes", cluster.Name)

	// In this call, it is possible to get errors back with non-nil upstreamSpec.
	// If upstreamSpec is nil then the syncing failed for some reason. This is reported to the user, and this function returns at the end of this if-statement.
	// If upstreamSpec is non-nil then the syncing occurred as expected, but the node groups have health issues that are reported to the user.
	// In this second case, the message is set on the Updated condition, but execution continues because the sync was successful.
	upstreamConfig, err := getComparableUpstreamSpec(c.secretsCache, c.secretClient, cluster)

	// Track what needs updating
	var setConditionFalse bool
	var conditionMsg string
	var clearSyncError bool
	var syncFailedCompletely bool

	if err != nil {
		syncFailedCompletely = upstreamConfig == nil || (upstreamConfig.gkeConfig == nil && upstreamConfig.eksConfig == nil && upstreamConfig.aksConfig == nil && upstreamConfig.aliConfig == nil)
		var syncFailedStr string
		if syncFailedCompletely {
			syncFailedStr = ": syncing failed"
		}
		setConditionFalse = true
		conditionMsg = fmt.Sprintf("[Syncing error%s] %s", syncFailedStr, err.Error())

		// Only continue if one of the configs on upstreamConfig is not nil.
		// Otherwise, an error occurred and no syncing should occur.
		if syncFailedCompletely {
			// Update status with fresh fetch and set condition
			cluster, err = c.updateClusterStatus(cluster.Name, cloudDriver, true, conditionMsg, false, nil)
			if err != nil {
				return cluster, err
			}

			// Update refreshTime annotation
			return c.updateRefreshAnnotation(cluster)
		}
	} else if strings.Contains(apimgmtv3.ClusterConditionUpdated.GetMessage(cluster), "[Syncing error") {
		clearSyncError = true
	}

	var initialClusterConfig, appliedClusterConfig, upstreamClusterConfig, upstreamSpec interface{}
	// for other cloud drivers, please edit HERE
	switch cloudDriver {
	case apimgmtv3.ClusterDriverAKS:
		initialClusterConfig = cluster.Spec.AKSConfig
		appliedClusterConfig = cluster.Status.AppliedSpec.AKSConfig
		upstreamClusterConfig = cluster.Status.AKSStatus.UpstreamSpec
		upstreamSpec = upstreamConfig.aksConfig
	case apimgmtv3.ClusterDriverEKS:
		initialClusterConfig = cluster.Spec.EKSConfig
		appliedClusterConfig = cluster.Status.AppliedSpec.EKSConfig
		upstreamClusterConfig = cluster.Status.EKSStatus.UpstreamSpec
		upstreamSpec = upstreamConfig.eksConfig
	case apimgmtv3.ClusterDriverGKE:
		initialClusterConfig = cluster.Spec.GKEConfig
		appliedClusterConfig = cluster.Status.AppliedSpec.GKEConfig
		upstreamClusterConfig = cluster.Status.GKEStatus.UpstreamSpec
		upstreamSpec = upstreamConfig.gkeConfig
	case apimgmtv3.ClusterDriverAlibaba:
		initialClusterConfig = cluster.Spec.AliConfig
		appliedClusterConfig = cluster.Status.AppliedSpec.AliConfig
		upstreamClusterConfig = cluster.Status.AliStatus.UpstreamSpec
		upstreamSpec = upstreamConfig.aliConfig
	}

	// Check if upstream spec changed
	upstreamSpecChanged := !reflect.DeepEqual(upstreamClusterConfig, upstreamSpec)
	if upstreamSpecChanged {
		logrus.Debugf("updating cluster [%s], upstream change detected", cluster.Name)
	}

	// check if cluster is still updating changes
	skipSpecSync := !reflect.DeepEqual(initialClusterConfig, appliedClusterConfig)
	if skipSpecSync {
		logrus.Debugf("cluster [%s] currently updating, skipping spec sync", cluster.Name)
	}

	// Determine spec config changes
	var specChanged bool
	var specMap map[string]interface{}
	if !skipSpecSync {
		var err error
		specMap, err = runtime.DefaultUnstructuredConverter.ToUnstructured(initialClusterConfig)
		if err != nil {
			return cluster, err
		}

		upstreamSpecMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(upstreamSpec)
		if err != nil {
			return cluster, err
		}

		for key, value := range upstreamSpecMap {
			if specMap[key] == nil {
				continue
			}
			if reflect.DeepEqual(specMap[key], value) {
				continue
			}
			specChanged = true
			specMap[key] = value
		}

		if specChanged {
			logrus.Debugf("change detected for cluster [%s], updating spec", cluster.Name)
		} else {
			logrus.Debugf("cluster [%s] matches upstream, skipping spec sync", cluster.Name)
		}
	}

	// Update spec if needed
	if specChanged {
		cluster = cluster.DeepCopy()
		// for other cloud drivers, please edit HERE
		switch cloudDriver {
		case apimgmtv3.ClusterDriverAKS:
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(specMap, cluster.Spec.AKSConfig)
			if err != nil {
				return cluster, err
			}
		case apimgmtv3.ClusterDriverEKS:
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(specMap, cluster.Spec.EKSConfig)
			if err != nil {
				return cluster, err
			}
		case apimgmtv3.ClusterDriverGKE:
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(specMap, cluster.Spec.GKEConfig)
			if err != nil {
				return cluster, err
			}
		case apimgmtv3.ClusterDriverAlibaba:
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(specMap, cluster.Spec.AliConfig)
			if err != nil {
				return cluster, err
			}
		}
		cluster, err = c.clusterClient.Update(cluster)
		if err != nil {
			return cluster, err
		}
	}

	// Update status if needed (conditions or upstreamSpec changed)
	if setConditionFalse || clearSyncError || upstreamSpecChanged {
		// Only pass upstreamConfig when it actually changed
		var configToPass *clusterConfig
		if upstreamSpecChanged {
			configToPass = upstreamConfig
		}

		// Update status with fresh fetch
		cluster, err = c.updateClusterStatus(cluster.Name, cloudDriver, setConditionFalse, conditionMsg, clearSyncError, configToPass)
		if err != nil {
			return cluster, err
		}
	}

	// Update refreshTime annotation
	return c.updateRefreshAnnotation(cluster)
}

// updateClusterStatus updates the cluster status.
// It fetches a fresh cluster, optionally sets the ClusterConditionUpdated state,
// and updates UpstreamSpec if upstreamConfig is provided.
func (c *clusterRefreshController) updateClusterStatus(clusterName, cloudDriver string, setConditionFalse bool, conditionMsg string, clearSyncError bool, upstreamConfig *clusterConfig) (*mgmtv3.Cluster, error) {
	toUpdate, err := c.clusterClient.Get(clusterName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Set condition directly on fresh cluster if needed
	if setConditionFalse {
		apimgmtv3.ClusterConditionUpdated.False(toUpdate)
		apimgmtv3.ClusterConditionUpdated.Message(toUpdate, conditionMsg)
	} else if clearSyncError {
		apimgmtv3.ClusterConditionUpdated.True(toUpdate)
		apimgmtv3.ClusterConditionUpdated.Message(toUpdate, "")
	}

	// Update UpstreamSpec if provided
	if upstreamConfig != nil {
		// for other cloud drivers, please edit HERE
		switch cloudDriver {
		case apimgmtv3.ClusterDriverAKS:
			toUpdate.Status.AKSStatus.UpstreamSpec = upstreamConfig.aksConfig
		case apimgmtv3.ClusterDriverEKS:
			toUpdate.Status.EKSStatus.UpstreamSpec = upstreamConfig.eksConfig
		case apimgmtv3.ClusterDriverGKE:
			toUpdate.Status.GKEStatus.UpstreamSpec = upstreamConfig.gkeConfig
		case apimgmtv3.ClusterDriverAlibaba:
			toUpdate.Status.AliStatus.UpstreamSpec = upstreamConfig.aliConfig
		}
	}

	return c.clusterClient.UpdateStatus(toUpdate)
}

// updateRefreshAnnotation updates the cluster refreshTime annotation
func (c *clusterRefreshController) updateRefreshAnnotation(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	cluster = cluster.DeepCopy()
	if cluster.Annotations == nil {
		cluster.Annotations = make(map[string]string)
	}
	cluster.Annotations[clusterLastRefreshTime] = strconv.FormatInt(time.Now().Unix(), 10)
	return c.clusterClient.Update(cluster)
}

func getComparableUpstreamSpec(secretsCache wranglerv1.SecretCache, secretClient wranglerv1.SecretClient, cluster *mgmtv3.Cluster) (*clusterConfig, error) {
	clusterCfg := &clusterConfig{}

	// for other cloud drivers, please edit HERE
	switch cluster.Status.Driver {
	case apimgmtv3.ClusterDriverAKS:
		aksConfig, err := BuildAKSUpstreamSpec(secretsCache, secretClient, cluster)
		clusterCfg.aksConfig = aksConfig
		return clusterCfg, err
	case apimgmtv3.ClusterDriverEKS:
		eksConfig, err := BuildEKSUpstreamSpec(secretClient, cluster)
		clusterCfg.eksConfig = eksConfig
		return clusterCfg, err
	case apimgmtv3.ClusterDriverGKE:
		gkeConfig, err := BuildGKEUpstreamSpec(secretsCache, secretClient, cluster)
		clusterCfg.gkeConfig = gkeConfig
		return clusterCfg, err
	case apimgmtv3.ClusterDriverAlibaba:
		aliConfig, err := BuildAlibabaUpstreamSpec(secretsCache, cluster)
		clusterCfg.aliConfig = aliConfig
		return clusterCfg, err
	default:
		return nil, fmt.Errorf("unsupported cloud driver")
	}
}

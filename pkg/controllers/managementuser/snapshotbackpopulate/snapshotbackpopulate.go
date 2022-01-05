package snapshotbackpopulate

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	cluster2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	configMapNames = map[string]bool{
		"k3s-etcd-snapshots":  true,
		"rke2-etcd-snapshots": true,
	}
)

type handler struct {
	clusterName  string
	clusterCache provisioningcontrollers.ClusterCache
	clusters     provisioningcontrollers.ClusterClient
}

// Register sets up the v2provisioning snapshot backpopulate controller. This controller is responsible for monitoring
// the downstream etcd-snapshots configmap and backpopulating snapshots to the Rancher management cluster
func Register(ctx context.Context, userContext *config.UserContext) error {
	h := handler{
		clusterName:  userContext.ClusterName,
		clusterCache: userContext.Management.Wrangler.Provisioning.Cluster().Cache(),
		clusters:     userContext.Management.Wrangler.Provisioning.Cluster(),
	}

	// We want to watch two specific objects, not all config maps.  So we setup a custom controller
	// to just watch those names.
	clientFactory, err := client.NewSharedClientFactory(&userContext.RESTConfig, nil)
	if err != nil {
		return err
	}

	for configMapName := range configMapNames {
		cacheFactory := cache.NewSharedCachedFactory(clientFactory, &cache.SharedCacheFactoryOptions{
			DefaultNamespace: "kube-system",
			DefaultTweakList: func(options *metav1.ListOptions) {
				options.FieldSelector = fmt.Sprintf("metadata.name=%s", configMapName)
			},
		})
		controllerFactory := controller.NewSharedControllerFactory(cacheFactory, nil)

		controller := corecontrollers.New(controllerFactory)
		controller.ConfigMap().OnChange(ctx, "snapshotbackpopulate", h.OnChange)
		if err := controllerFactory.Start(ctx, 1); err != nil {
			return err
		}
	}

	return nil
}

func (h *handler) OnChange(key string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	if configMap == nil {
		return nil, nil
	}

	if configMap.Namespace != "kube-system" || !configMapNames[configMap.Name] {
		return configMap, nil
	}

	cluster, err := h.clusterCache.GetByIndex(cluster2.ByCluster, h.clusterName)
	if err != nil || len(cluster) != 1 {
		return configMap, err
	}

	fromConfigMap, err := h.configMapToSnapshots(configMap)
	if err != nil {
		return configMap, err
	}

	if !equality.Semantic.DeepEqual(cluster[0].Status.ETCDSnapshots, fromConfigMap) {
		cluster := cluster[0].DeepCopy()
		cluster.Status.ETCDSnapshots = fromConfigMap
		_, err = h.clusters.UpdateStatus(cluster)
		return configMap, err
	}

	return configMap, nil
}

func (h *handler) configMapToSnapshots(configMap *corev1.ConfigMap) (result []rkev1.ETCDSnapshot, _ error) {
	for k, v := range configMap.Data {
		file := &snapshotFile{}
		if err := json.Unmarshal([]byte(v), file); err != nil {
			logrus.Errorf("invalid non-json value in %s/%s for key %s in cluster %s", configMap.Namespace, configMap.Name, k, h.clusterName)
			return nil, nil
		}
		snapshot := rkev1.ETCDSnapshot{
			Name:      file.Name,
			Location:  file.Location,
			Metadata:  file.Metadata,
			Message:   file.Message,
			NodeName:  file.NodeName,
			CreatedAt: file.CreatedAt,
			Size:      file.Size,
			Status:    file.Status,
		}
		if file.S3 != nil {
			snapshot.S3 = &rkev1.ETCDSnapshotS3{
				Endpoint:      file.S3.Endpoint,
				EndpointCA:    file.S3.EndpointCA,
				SkipSSLVerify: file.S3.SkipSSLVerify,
				Bucket:        file.S3.Bucket,
				Region:        file.S3.Region,
				Folder:        file.S3.Folder,
			}
		}
		result = append(result, snapshot)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

type s3Config struct {
	Endpoint      string `json:"endpoint,omitempty"`
	EndpointCA    string `json:"endpointCA,omitempty"`
	SkipSSLVerify bool   `json:"skipSSLVerify,omitempty"`
	Bucket        string `json:"bucket,omitempty"`
	Region        string `json:"region,omitempty"`
	Folder        string `json:"folder,omitempty"`
}

// snapshotFile represents a single snapshot and it's
// metadata.
type snapshotFile struct {
	Name      string       `json:"name"`
	Location  string       `json:"location,omitempty"`
	NodeName  string       `json:"nodeName,omitempty"`
	Metadata  string       `json:"metadata,omitempty"`
	Message   string       `json:"message,omitempty"`
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
	Size      int64        `json:"size,omitempty"`
	Status    string       `json:"status,omitempty"`
	S3        *s3Config    `json:"s3Config,omitempty"`
}

package snapshotbackpopulate

import (
	"context"
	"encoding/json"
	"sort"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	cluster2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	snapshotNames = map[string]bool{
		"k3s-etcd-snapshots":  true,
		"rke2-etcd-snapshots": true,
	}
)

type handler struct {
	clusterName  string
	clusterCache provisioningcontrollers.ClusterCache
	clusters     provisioningcontrollers.ClusterClient
}

func Register(ctx context.Context, userContext *config.UserContext) {
	h := handler{
		clusterName:  userContext.ClusterName,
		clusterCache: userContext.Management.Wrangler.Provisioning.Cluster().Cache(),
		clusters:     userContext.Management.Wrangler.Provisioning.Cluster(),
	}
	userContext.Core.ConfigMaps("kube-system").AddHandler(ctx, "snapshotbackpopulate", h.OnChange)
}

func (h *handler) OnChange(key string, configMap *corev1.ConfigMap) (runtime.Object, error) {
	if configMap == nil {
		return nil, nil
	}

	if configMap.Namespace != "kube-system" || !snapshotNames[configMap.Name] {
		return configMap, nil
	}

	cluster, err := h.clusterCache.GetByIndex(cluster2.ByCluster, h.clusterName)
	if err != nil || len(cluster) != 1 {
		return configMap, err
	}

	fromConfigMap, err := configMapToSnapshots(configMap)
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

func configMapToSnapshots(configMap *corev1.ConfigMap) (result []rkev1.ETCDSnapshot, _ error) {
	for _, v := range configMap.Data {
		file := &snapshotFile{}
		if err := json.Unmarshal([]byte(v), file); err != nil {
			return nil, err
		}
		snapshot := rkev1.ETCDSnapshot{
			Name:      file.Name,
			NodeName:  file.NodeName,
			CreatedAt: file.CreatedAt,
			Size:      file.Size,
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
	NodeName  string       `json:"nodeName,omitempty"`
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
	Size      int64        `json:"size,omitempty"`
	S3        *s3Config    `json:"s3Config,omitempty"`
}

package snapshotextrametadata

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	configMapName = "rke2-etcd-snapshot-extra-metadata"

	resourcesKey    = "resources"
	restoreModesKey = "restoreModes"

	// mgmtClusterResourceKey is the key the sanitized management cluster is published under in the
	// resources section, and the first segment of any restore mode selector targeting that cluster.
	mgmtClusterResourceKey = "cluster.management.cattle.io"
)

// renderedFields are the only top-level fields sanitize keeps. Everything else, i.e. status and any
// other subresource-backed field, is purged: subresources are not restorable from a snapshot.
var renderedFields = []string{"apiVersion", "kind", "metadata", "spec"}

type handler struct {
	configMap corecontrollers.ConfigMapClient
}

func Register(ctx context.Context, userContext *config.UserContext, capiCtx *wrangler.CAPIContext, cluster *apimgmtv3.Cluster) {
	logrus.Debugf("[snapshotextrametadata] Registering controller for cluster %s", userContext.ClusterName)

	h := &handler{
		configMap: userContext.Corew.ConfigMap(),
	}

	userContext.Management.Wrangler.Mgmt.Cluster().OnChange(ctx, "snapshotextrametadata", h.onChange)
}

func (h *handler) onChange(_ string, cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster == nil {
		return nil, nil
	}

	data, err := renderData(cluster)
	if err != nil {
		return nil, err
	}

	cm, err := h.configMap.Get(metav1.NamespaceSystem, configMapName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = h.configMap.Create(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: metav1.NamespaceSystem,
				Name:      configMapName,
			},
			Data: data,
		})
		if err != nil {
			return nil, err
		}
		return cluster, nil
	}
	if err != nil {
		return nil, err
	}

	if maps.Equal(cm.Data, data) {
		return cluster, nil
	}

	cm = cm.DeepCopy()
	cm.Data = data

	if _, err = h.configMap.Update(cm); err != nil {
		return nil, err
	}

	return cluster, nil
}

// renderData builds the data of the etcd snapshot extra metadata ConfigMap. RKE2/K3s copy every key
// in it into the snapshot's metadata, so the resources section carries the objects to restore from
// and the restoreModes section carries the selector each restore mode applies to those resources.
func renderData(cluster *apimgmtv3.Cluster) (map[string]string, error) {
	resources := map[string]any{}

	sanitized, err := sanitize(cluster)
	if err != nil {
		return nil, err
	}
	resources[mgmtClusterResourceKey] = sanitized

	out, err := json.Marshal(resources)
	if err != nil {
		return nil, err
	}

	data := map[string]string{
		resourcesKey: string(out),
	}

	restoreModes := map[string]any{
		"all":               "*",
		"kubernetesVersion": fmt.Sprintf("$['%s'].spec.rke2Config.kubernetesVersion", mgmtClusterResourceKey),
		"none":              "",
	}

	out, err = json.Marshal(restoreModes)
	if err != nil {
		return nil, err
	}

	data[restoreModesKey] = string(out)

	return data, nil
}

// sanitize converts obj to its unstructured form, purged of all subresources.
func sanitize(obj any) (any, error) {
	umap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	for field := range umap {
		if !slices.Contains(renderedFields, field) {
			delete(umap, field)
		}
	}

	return umap, nil
}

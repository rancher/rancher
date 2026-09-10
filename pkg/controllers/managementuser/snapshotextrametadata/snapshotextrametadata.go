package snapshotextrametadata

import (
	"context"
	"encoding/json"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

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
	cm, err := h.configMap.Get(metav1.NamespaceSystem, "rke2-etcd-snapshot-extra-metadata", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// create
	}
	if err != nil {
		return nil, err
	}

	resources := map[string]any{}

	resources["cluster.management.cattle.io"], err = sanitize(cluster)
	if err != nil {
		return nil, err
	}

	cm.Data = map[string]string{}

	out, err := json.Marshal(resources)
	if err != nil {
		return nil, err
	}

	cm.Data["resources"] = string(out)

	restoreModes := map[string]any{
		"all": "*",
		"kubernetesVersion": "$.spec.kubernetesVerison",
		"none": "",
	}

	out, err = json.Marshal(restoreModes)
	if err != nil {
		return nil, err
	}

	cm.Data["restoreModes"] = string(out)

	return cm, nil
}

func sanitize(obj any) (any, error) {
	umap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}


}

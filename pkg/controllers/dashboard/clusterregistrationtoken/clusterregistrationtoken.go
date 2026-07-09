package clusterregistrationtoken

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v32 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/labels"
)

type handler struct {
	clusterRegistrationTokenCache      v32.ClusterRegistrationTokenCache
	clusterRegistrationTokenController v32.ClusterRegistrationTokenController
	clusters                           v32.ClusterCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clusterRegistrationTokenController: clients.Mgmt.ClusterRegistrationToken(),
		clusterRegistrationTokenCache:      clients.Mgmt.ClusterRegistrationToken().Cache(),
		clusters:                           clients.Mgmt.Cluster().Cache(),
	}
	clients.Mgmt.ClusterRegistrationToken().OnChange(ctx, "cluster-registration-token", h.onChange)
	clients.Mgmt.Cluster().OnChange(ctx, "cluster-registration-token-trigger", h.onClusterChange)

}

func (h *handler) onClusterChange(key string, obj *v3.Cluster) (*v3.Cluster, error) {
	if obj == nil {
		return obj, nil
	}

	crts, err := h.clusterRegistrationTokenCache.List(obj.Name, labels.Everything())
	if err != nil {
		return obj, nil
	}

	for _, crt := range crts {
		h.clusterRegistrationTokenController.Enqueue(crt.Namespace, crt.Name)
	}

	return obj, nil
}

func (h *handler) onChange(key string, obj *v3.ClusterRegistrationToken) (_ *v3.ClusterRegistrationToken, err error) {
	if obj == nil {
		return obj, nil
	}

	if obj.Status.Token != "" {
		newStatus, err := h.assignStatus(obj)
		if err != nil {
			return nil, err
		}
		if !equality.Semantic.DeepEqual(obj.Status, newStatus) {
			obj = obj.DeepCopy()
			obj.Status = newStatus
			return h.clusterRegistrationTokenController.Update(obj)
		}
		return obj, nil
	}

	obj = obj.DeepCopy()
	obj.Status.Token, err = randomtoken.Generate()
	if err != nil {
		return nil, err
	}

	return h.clusterRegistrationTokenController.Update(obj)
}

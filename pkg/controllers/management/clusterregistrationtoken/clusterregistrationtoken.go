package clusterregistrationtoken

import (
	"context"

	"github.com/rancher/wrangler/pkg/randomtoken"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/labels"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v33 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

type handler struct {
	clusterRegistrationTokenCache      v33.ClusterRegistrationTokenLister
	clusterRegistrationTokenClient     v33.ClusterRegistrationTokenInterface
	clusterRegistrationTokenController v33.ClusterRegistrationTokenController
	clusters                           v33.ClusterLister
}

func Register(ctx context.Context, mgmt *config.ManagementContext) {
	h := &handler{
		clusterRegistrationTokenClient:     mgmt.Management.ClusterRegistrationTokens(""),
		clusterRegistrationTokenController: mgmt.Management.ClusterRegistrationTokens("").Controller(),
		clusterRegistrationTokenCache:      mgmt.Management.ClusterRegistrationTokens("").Controller().Lister(),
		clusters:                           mgmt.Management.Clusters("").Controller().Lister(),
	}
	mgmt.Management.ClusterRegistrationTokens("").Controller().AddHandler(ctx, "cluster-registration-token", h.onChange)
	mgmt.Management.Clusters("").Controller().AddHandler(ctx, "cluster-registration-token-trigger", h.onClusterChange)

}

func (h *handler) onClusterChange(key string, obj *v3.Cluster) (runtime.Object, error) {
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

func (h *handler) onChange(key string, obj *v3.ClusterRegistrationToken) (_ runtime.Object, err error) {
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
			return h.clusterRegistrationTokenClient.Update(obj)
		}
		return obj, nil
	}

	obj = obj.DeepCopy()
	obj.Status.Token, err = randomtoken.Generate()
	if err != nil {
		return nil, err
	}

	return h.clusterRegistrationTokenClient.Update(obj)
}

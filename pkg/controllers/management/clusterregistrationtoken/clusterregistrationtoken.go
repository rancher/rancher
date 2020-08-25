package clusterregistrationtoken

import (
	"context"

	"github.com/rancher/wrangler/pkg/randomtoken"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v33 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

type handler struct {
	clusterRegistrationTokenClient v33.ClusterRegistrationTokenInterface
}

func Register(ctx context.Context, mgmt *config.ManagementContext) {
	h := &handler{
		clusterRegistrationTokenClient: mgmt.Management.ClusterRegistrationTokens(""),
	}
	mgmt.Management.ClusterRegistrationTokens("").Controller().AddHandler(ctx, "cluster-registration-token", h.onChange)
}

func (h *handler) onChange(key string, obj *v3.ClusterRegistrationToken) (_ runtime.Object, err error) {
	if obj == nil {
		return obj, nil
	}

	if obj.Status.Token != "" {
		return obj, nil
	}

	obj = obj.DeepCopy()
	obj.Status.Token, err = randomtoken.Generate()
	if err != nil {
		return nil, err
	}

	return h.clusterRegistrationTokenClient.Update(obj)
}

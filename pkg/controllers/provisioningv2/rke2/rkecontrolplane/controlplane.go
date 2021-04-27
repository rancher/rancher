package rkecontrolplane

import (
	"context"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(ctx context.Context, clients *wrangler.Context) {
	v1.RegisterRKEControlPlaneStatusHandler(ctx, clients.RKE.RKEControlPlane(),
		"", "rke-control-plane",
		func(obj *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
			status.Ready = true
			status.ObservedGeneration = obj.Generation
			return status, nil
		})
}

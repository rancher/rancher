package managedos

import (
	"context"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/managesystemagent"
	fleetcontrollers "github.com/rancher/rancher/pkg/generated/controllers/fleet.cattle.io/v1alpha1"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/relatedresource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		bundleCache: clients.Fleet.Bundle().Cache(),
	}

	relatedresource.Watch(ctx,
		"mcc-from-bundle-trigger",
		relatedresource.OwnerResolver(true, provv1.SchemeGroupVersion.String(), "ManagedOS"),
		clients.Provisioning.ManagedOS(),
		clients.Fleet.Bundle())
	provisioningcontrollers.RegisterManagedOSGeneratingHandler(ctx,
		clients.Provisioning.ManagedOS(),
		clients.Apply.
			WithSetOwnerReference(true, true).
			WithCacheTypes(
				clients.Provisioning.ManagedOS(),
				clients.Fleet.Bundle()),
		"Defined",
		"mos-bundle",
		h.OnChange,
		nil)
}

type handler struct {
	bundleCache fleetcontrollers.BundleCache
}

func (h *handler) OnChange(mos *provv1.ManagedOS, status provv1.ManagedOSStatus) ([]runtime.Object, provv1.ManagedOSStatus, error) {
	if mos.Spec.OSImage == "" {
		return nil, status, nil
	}

	resources, err := managesystemagent.ToResources(objects(mos))
	if err != nil {
		return nil, status, err
	}

	bundle := &v1alpha1.Bundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.SafeConcatName("mos", mos.Name),
			Namespace: mos.Namespace,
		},
		Spec: v1alpha1.BundleSpec{
			Resources:               resources,
			BundleDeploymentOptions: v1alpha1.BundleDeploymentOptions{},
			Paused:                  mos.Spec.Paused,
			RolloutStrategy:         mos.Spec.ClusterRolloutStrategy,
			Targets:                 mos.Spec.Targets,
		},
	}

	status, err = h.updateStatus(status, bundle)
	return []runtime.Object{
		bundle,
	}, status, err
}

func (h *handler) updateStatus(status provv1.ManagedOSStatus, bundle *v1alpha1.Bundle) (provv1.ManagedOSStatus, error) {
	bundle, err := h.bundleCache.Get(bundle.Namespace, bundle.Name)
	if apierrors.IsNotFound(err) {
		return status, nil
	} else if err != nil {
		return status, err
	}

	status.BundleStatus = bundle.Status
	return status, nil
}

package fleetworkspace

import (
	"context"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/yaml"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	managed = "provisioning.cattle.io/managed"
)

type handle struct {
	workspaceCache mgmtcontrollers.FleetWorkspaceCache
	namespaceCache v1.NamespaceCache
	workspaces     mgmtcontrollers.FleetWorkspaceClient
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handle{
		workspaceCache: clients.Mgmt.FleetWorkspace().Cache(),
		workspaces:     clients.Mgmt.FleetWorkspace(),
		namespaceCache: clients.Core.Namespace().Cache(),
	}

	if features.MCM.Enabled() {
		clients.Mgmt.Setting().OnChange(ctx, "default-workspace", h.OnSetting)
	}

	mgmtcontrollers.RegisterFleetWorkspaceGeneratingHandler(ctx,
		clients.Mgmt.FleetWorkspace(),
		clients.Apply.
			WithCacheTypes(clients.Core.Namespace()),
		"",
		"workspace",
		h.OnChange,
		&generic.GeneratingHandlerOptions{
			AllowClusterScoped: true,
		})

	clients.Fleet.Cluster().OnChange(ctx, "workspace-backport-cluster",
		func(s string, obj *fleet.Cluster) (*fleet.Cluster, error) {
			if obj == nil {
				return nil, nil
			}
			return obj, h.onFleetObject(obj)
		})
	clients.Fleet.Bundle().OnChange(ctx, "workspace-backport-bundle",
		func(s string, obj *fleet.Bundle) (*fleet.Bundle, error) {
			if obj == nil {
				return nil, nil
			}
			return obj, h.onFleetObject(obj)
		})
}

func (h *handle) OnSetting(key string, setting *mgmt.Setting) (*mgmt.Setting, error) {
	if setting == nil || setting.Name != "fleet-default-workspace-name" {
		return setting, nil
	}

	value := setting.Value
	if value == "" {
		value = setting.Default
	}

	if value == "" {
		return setting, nil
	}

	_, err := h.workspaceCache.Get(value)
	if apierror.IsNotFound(err) {
		_, err = h.workspaces.Create(&mgmt.FleetWorkspace{
			ObjectMeta: metav1.ObjectMeta{
				Name: value,
			},
		})
	}

	return setting, err
}

func (h *handle) OnChange(workspace *mgmt.FleetWorkspace, status mgmt.FleetWorkspaceStatus) ([]runtime.Object, mgmt.FleetWorkspaceStatus, error) {
	if workspace.Annotations[managed] == "false" {
		return nil, status, nil
	}

	return []runtime.Object{
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   workspace.Name,
				Labels: yaml.CleanAnnotationsForExport(workspace.Labels),
			},
		},
	}, status, nil
}

func (h *handle) onFleetObject(obj runtime.Object) error {
	m, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	_, err = h.workspaceCache.Get(m.GetNamespace())
	if apierror.IsNotFound(err) {
		ns, err := h.namespaceCache.Get(m.GetNamespace())
		if err != nil {
			return err
		}

		_, err = h.workspaces.Create(&mgmt.FleetWorkspace{
			ObjectMeta: metav1.ObjectMeta{
				Name: m.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Namespace",
						Name:       ns.Name,
						UID:        ns.UID,
					},
				},
				Annotations: map[string]string{
					managed: "false",
				},
			},
			Status: mgmt.FleetWorkspaceStatus{},
		})
		if apierror.IsAlreadyExists(err) {
			return nil
		}
	}

	return err
}

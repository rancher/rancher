package fleetworkspace

import (
	"context"
	"fmt"

	"github.com/gorilla/mux"
	"github.com/rancher/dynamiclistener/server"
	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ext"
	"github.com/rancher/rancher/pkg/features"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	apiv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiregistration.k8s.io/v1"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/yaml"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

var (
	managed = "provisioning.cattle.io/managed"
)

type handle struct {
	workspaceCache mgmtcontrollers.FleetWorkspaceCache
	namespaceCache v1.NamespaceCache
	workspaces     mgmtcontrollers.FleetWorkspaceClient
	serviceClient  v1.ServiceClient
	apiClient      apiv1.APIServiceClient
	wContext       *wrangler.Context
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handle{
		workspaceCache: clients.Mgmt.FleetWorkspace().Cache(),
		workspaces:     clients.Mgmt.FleetWorkspace(),
		namespaceCache: clients.Core.Namespace().Cache(),
		serviceClient:  clients.Core.Service(),
		apiClient:      clients.API.APIService(),
		wContext:       clients,
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
	if setting == nil {
		return setting, nil
	}
	switch setting.Name {
	case "fleet-default-workspace-name":
		return h.onWorkspaceSetting(setting)
	case "imperative-api-extension":
		return h.onExtensionSetting(setting)
	default:
		return setting, nil
	}
}

func (h *handle) onExtensionSetting(setting *mgmt.Setting) (*mgmt.Setting, error) {
	value := getEffectiveValue(setting)
	apiSvcName := fmt.Sprintf("%s.%s", "v1alpha1", "ext.cattle.io")
	svcName := "api-extension"
	svcNamespace := "cattle-system"
	switch value {
	case "on":
		var port int32 = 5555
		apisvc := &apiregv1.APIService{
			ObjectMeta: metav1.ObjectMeta{
				Name: apiSvcName,
			},
			Spec: apiregv1.APIServiceSpec{
				Service: &apiregv1.ServiceReference{
					Namespace: svcNamespace,
					Name:      svcName,
					Port:      &port,
				},
				Group:   "ext.cattle.io",
				Version: "v1alpha1",
				// Note: this done for POC sake, but shouldn't be done for prod
				InsecureSkipTLSVerify: true,
				GroupPriorityMinimum:  100,
				VersionPriority:       100,
			},
		}
		_, err := h.apiClient.Create(apisvc)
		if err != nil && !apierror.IsAlreadyExists(err) {
			return nil, fmt.Errorf("unable to enable extension server: %w", err)
		}
		router := mux.NewRouter()
		ext.RegisterSubRoutes(router, h.wContext)
		go func() {
			err := server.ListenAndServe(context.Background(), 555, 0, router, nil)
			logrus.Errorf("extension server exited with: %s", err.Error())
		}()

	case "off":
		err := h.apiClient.Delete(apiSvcName, &metav1.DeleteOptions{})
		if err != nil && !apierror.IsNotFound(err) {
			return nil, fmt.Errorf("unable to delete api service: %w", err)
		}
	default:
		// invalid, take no action
		logrus.Errorf("imperative-api-extension had unexpected value %s, no action will be taken", value)
	}
	return setting, nil

}

func (h *handle) onWorkspaceSetting(setting *mgmt.Setting) (*mgmt.Setting, error) {
	value := getEffectiveValue(setting)

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

func getEffectiveValue(setting *mgmt.Setting) string {
	value := setting.Value
	if value == "" {
		value = setting.Default
	}
	return value
}

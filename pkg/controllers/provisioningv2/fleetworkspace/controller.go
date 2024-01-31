package fleetworkspace

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	"github.com/rancher/rancher/pkg/features"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/wrangler"
	v1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/yaml"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	managed                             = "provisioning.cattle.io/managed"
	errorFleetWorkspacesProjectNotFound = errors.New("can't find FleetWorkspaces project")
)

type handle struct {
	workspaceCache mgmtcontrollers.FleetWorkspaceCache
	namespaceCache v1.NamespaceCache
	workspaces     mgmtcontrollers.FleetWorkspaceClient
	projectsCache  mgmtcontrollers.ProjectCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handle{
		workspaceCache: clients.Mgmt.FleetWorkspace().Cache(),
		workspaces:     clients.Mgmt.FleetWorkspace(),
		namespaceCache: clients.Core.Namespace().Cache(),
		projectsCache:  clients.Mgmt.Project().Cache(),
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
	project, err := getFleetWorkspacesProject(h.projectsCache)
	if err != nil {
		return setting, err
	}

	_, err = h.workspaceCache.Get(value)
	if apierror.IsNotFound(err) {
		_, err = h.workspaces.Create(&mgmt.FleetWorkspace{
			ObjectMeta: metav1.ObjectMeta{
				Name: value,
				Annotations: map[string]string{
					nslabels.ProjectIDFieldLabel: fmt.Sprintf("%v:%v", project.Spec.ClusterName, project.Name),
				},
			},
		})
	}

	return setting, err
}

func (h *handle) OnChange(workspace *mgmt.FleetWorkspace, status mgmt.FleetWorkspaceStatus) ([]runtime.Object, mgmt.FleetWorkspaceStatus, error) {
	if workspace.Annotations[managed] == "false" {
		return nil, status, nil
	}

	project, err := getFleetWorkspacesProject(h.projectsCache)
	if err != nil {
		return nil, mgmt.FleetWorkspaceStatus{}, err
	}

	return []runtime.Object{
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   workspace.Name,
				Labels: yaml.CleanAnnotationsForExport(workspace.Labels),
				Annotations: map[string]string{
					nslabels.ProjectIDFieldLabel: fmt.Sprintf("%v:%v", project.Spec.ClusterName, project.Name),
				},
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

func getFleetWorkspacesProject(projectLister mgmtv3.ProjectLister) (*mgmt.Project, error) {
	projects, err := projectLister.List("local", labels.Set(project.FleetWorkspacesProjectLabel).AsSelector())
	if err != nil {
		return nil, errors.Wrapf(err, "list project failed")
	}

	if len(projects) == 0 {
		return nil, errorFleetWorkspacesProjectNotFound
	}

	return projects[0], nil
}

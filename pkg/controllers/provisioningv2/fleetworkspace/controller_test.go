package fleetworkspace

import (
	"fmt"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/wrangler/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierros "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestOnChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fleetWorkspacesProject := v3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "FleetWorkspaces",
		},
		Spec: v3.ProjectSpec{
			ClusterName: "local",
		},
	}
	workspace := v3.FleetWorkspace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "workspace",
		},
	}

	tests := map[string]struct {
		projectsMock       func() mgmtcontrollers.ProjectCache
		expectedNamespaces []runtime.Object
		expectedErr        string
	}{
		"FleetWorkspaces project exists": {
			projectsMock: func() mgmtcontrollers.ProjectCache {
				projectsMock := fake.NewMockCacheInterface[*v3.Project](ctrl)
				projectsMock.EXPECT().List("local", labels.Set(project.FleetWorkspacesProjectLabel).AsSelector()).Return([]*v3.Project{&fleetWorkspacesProject}, nil)

				return projectsMock
			},
			expectedNamespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: workspace.Name,
						Annotations: map[string]string{
							nslabels.ProjectIDFieldLabel: fmt.Sprintf("%v:%v", fleetWorkspacesProject.Spec.ClusterName, fleetWorkspacesProject.Name),
						},
						Labels: map[string]string{},
					},
				},
			},
		},
		"FleetWorkspaces project does not exists": {
			projectsMock: func() mgmtcontrollers.ProjectCache {
				ctrl := gomock.NewController(t)
				projectsMock := fake.NewMockCacheInterface[*v3.Project](ctrl)
				projectsMock.EXPECT().List("local", labels.Set(project.FleetWorkspacesProjectLabel).AsSelector()).Return([]*v3.Project{}, nil)

				return projectsMock
			},
			expectedErr: errorFleetWorkspacesProjectNotFound.Error(),
		},
		"List projects return an error": {
			projectsMock: func() mgmtcontrollers.ProjectCache {
				ctrl := gomock.NewController(t)
				projectsMock := fake.NewMockCacheInterface[*v3.Project](ctrl)
				projectsMock.EXPECT().List("local", labels.Set(project.FleetWorkspacesProjectLabel).AsSelector()).Return(nil, fmt.Errorf("projects can't be fetched"))

				return projectsMock
			},
			expectedErr: "list project failed: projects can't be fetched",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			h := handle{
				projectsCache: test.projectsMock(),
			}

			namespaces, _, err := h.OnChange(&workspace, v3.FleetWorkspaceStatus{})

			assert.True(t, errorContains(err, test.expectedErr))
			assert.Equal(t, test.expectedNamespaces, namespaces)
		})
	}
}

func TestOnSetting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	fleetWorkspacesProject := v3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "FleetWorkspaces",
		},
		Spec: v3.ProjectSpec{
			ClusterName: "local",
		},
	}
	setting := v3.Setting{
		Value: "setting",
		ObjectMeta: metav1.ObjectMeta{
			Name: "fleet-default-workspace-name",
		},
	}

	tests := map[string]struct {
		projectsMock        func() mgmtcontrollers.ProjectCache
		workspacesCacheMock func() mgmtcontrollers.FleetWorkspaceCache
		workspacesMock      func() mgmtcontrollers.FleetWorkspaceClient
		expectedErr         error
	}{
		"workspace is created when it doesn't exist": {
			projectsMock: func() mgmtcontrollers.ProjectCache {
				projectsMock := fake.NewMockCacheInterface[*v3.Project](ctrl)
				projectsMock.EXPECT().List("local", labels.Set(project.FleetWorkspacesProjectLabel).AsSelector()).Return([]*v3.Project{&fleetWorkspacesProject}, nil)

				return projectsMock
			},
			workspacesCacheMock: func() mgmtcontrollers.FleetWorkspaceCache {
				workspacesCacheMock := fake.NewMockNonNamespacedCacheInterface[*v3.FleetWorkspace](ctrl)
				workspacesCacheMock.EXPECT().Get(gomock.Any()).Return(nil, apierros.NewNotFound(schema.GroupResource{}, ""))

				return workspacesCacheMock
			},
			workspacesMock: func() mgmtcontrollers.FleetWorkspaceClient {
				workspacesMock := fake.NewMockNonNamespacedControllerInterface[*v3.FleetWorkspace, *v3.FleetWorkspaceList](ctrl)
				workspacesMock.EXPECT().Create(&v3.FleetWorkspace{
					ObjectMeta: metav1.ObjectMeta{
						Name: setting.Value,
						Annotations: map[string]string{
							nslabels.ProjectIDFieldLabel: fmt.Sprintf("%v:%v", fleetWorkspacesProject.Spec.ClusterName, fleetWorkspacesProject.Name),
						},
					},
				}).Return(nil, nil)

				return workspacesMock
			},
		},
		"workspace is not created when it exists": {
			projectsMock: func() mgmtcontrollers.ProjectCache {
				projectsMock := fake.NewMockCacheInterface[*v3.Project](ctrl)
				projectsMock.EXPECT().List("local", labels.Set(project.FleetWorkspacesProjectLabel).AsSelector()).Return([]*v3.Project{&fleetWorkspacesProject}, nil)

				return projectsMock
			},
			workspacesCacheMock: func() mgmtcontrollers.FleetWorkspaceCache {
				workspacesCacheMock := fake.NewMockNonNamespacedCacheInterface[*v3.FleetWorkspace](ctrl)
				workspacesCacheMock.EXPECT().Get(gomock.Any()).Return(nil, nil)

				return workspacesCacheMock
			},
			workspacesMock: func() mgmtcontrollers.FleetWorkspaceClient {
				return fake.NewMockNonNamespacedControllerInterface[*v3.FleetWorkspace, *v3.FleetWorkspaceList](ctrl)
			},
		},
		"error returned if FleetWorkspaces project doesn't exist": {
			projectsMock: func() mgmtcontrollers.ProjectCache {
				projectsMock := fake.NewMockCacheInterface[*v3.Project](ctrl)
				projectsMock.EXPECT().List("local", labels.Set(project.FleetWorkspacesProjectLabel).AsSelector()).Return([]*v3.Project{}, nil)

				return projectsMock
			},
			workspacesCacheMock: func() mgmtcontrollers.FleetWorkspaceCache {
				return fake.NewMockNonNamespacedCacheInterface[*v3.FleetWorkspace](ctrl)
			},
			workspacesMock: func() mgmtcontrollers.FleetWorkspaceClient {
				return fake.NewMockNonNamespacedControllerInterface[*v3.FleetWorkspace, *v3.FleetWorkspaceList](ctrl)
			},
			expectedErr: errorFleetWorkspacesProjectNotFound,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			h := handle{
				projectsCache:  test.projectsMock(),
				workspaces:     test.workspacesMock(),
				workspaceCache: test.workspacesCacheMock(),
			}

			_, err := h.OnSetting("", &setting)

			assert.Equal(t, test.expectedErr, err)
		})
	}
}

func errorContains(err error, want string) bool {
	if err == nil {
		return want == ""
	}
	if want == "" {
		return false
	}
	return strings.Contains(err.Error(), want)
}

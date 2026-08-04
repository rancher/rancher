package privateregistry

import (
	"fmt"
	"strings"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/controllers/managementuser/secret"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	errDefault       = fmt.Errorf("error")
	errNotFound      = apierrors.NewNotFound(schema.GroupResource{}, "")
	errAlreadyExists = apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, "")
)

func readyCluster(name string, importedCfg *v3.ImportedConfig) *v3.Cluster {
	return &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v3.ClusterSpec{
			ImportedConfig:     importedCfg,
			FleetWorkspaceName: "fleet-default",
		},
		Status: v3.ClusterStatus{
			Conditions: []v3.ClusterCondition{
				{Type: "SystemProjectCreated", Status: corev1.ConditionTrue},
				{Type: "AgentDeployed", Status: corev1.ConditionTrue},
			},
		},
	}
}

func systemProject(clusterName, projectName string, extraLabels map[string]string) *v3.Project {
	l := map[string]string{
		project.SystemProjectLabelKey: "true",
	}
	for k, v := range extraLabels {
		l[k] = v
	}
	return &v3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: projectName, Labels: l},
		Spec:       v3.ProjectSpec{ClusterName: clusterName},
		Status:     v3.ProjectStatus{BackingNamespace: clusterName + "-" + projectName},
	}
}

func withSettings(t *testing.T, values map[settings.Setting]string) {
	t.Helper()
	oldValues := make(map[settings.Setting]string)
	for setting, v := range values {
		oldValues[setting] = setting.Get()
		if err := setting.Set(v); err != nil {
			t.Error(err)
			t.FailNow()
		}
	}
	t.Cleanup(func() {
		for setting, value := range oldValues {
			if err := setting.Set(value); err != nil {
				t.Error(err)
				t.FailNow()
			}
		}
	})
}

func Test_handler_labelSystemProject(t *testing.T) {
	tests := []struct {
		name               string
		cluster            *v3.Cluster
		registryURL        string
		pullSecrets        string
		setupProjectCache  func(*fake.MockCacheInterface[*v3.Project])
		setupProjectClient func(*fake.MockControllerInterface[*v3.Project, *v3.ProjectList])
		wantErr            bool
	}{
		{
			name:    "nil cluster",
			cluster: nil,
		},
		{
			name: "cluster not ready, not local",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-abcde"},
			},
		},
		{
			name:        "local cluster skipped entirely",
			cluster:     &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "local"}},
			registryURL: "my-registry.example.com",
			pullSecrets: "pull-secret",
		},
		{
			name:    "provisioned cluster skipped, name does not match MgmtNameRegexp",
			cluster: readyCluster("c-m-123", nil),
		},
		{
			name:        "no system project found despite cluster condition, error returned",
			cluster:     readyCluster("c-abcde", nil),
			registryURL: "my-registry.example.com",
			pullSecrets: "pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return(nil, nil)
			},
			wantErr: true,
		},
		{
			name:        "error getting system project for cluster, err returned",
			cluster:     readyCluster("c-abcde", nil),
			registryURL: "my-registry.example.com",
			pullSecrets: "pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name:        "global registry with pull secrets configured, label added to system project",
			cluster:     readyCluster("c-abcde", nil),
			registryURL: "my-registry.example.com",
			pullSecrets: "pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return([]*v3.Project{
					systemProject("c-abcde", "p-system", nil),
				}, nil)
			},
			setupProjectClient: func(f *fake.MockControllerInterface[*v3.Project, *v3.ProjectList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(p *v3.Project) (*v3.Project, error) {
					assert.Equal(t, "true", p.Labels[secret.NeedsGlobalPrivateRegistryPullSecret])
					return p, nil
				})
			},
		},
		{
			name:        "global registry with pull secrets, label already present, no update",
			cluster:     readyCluster("c-abcde", nil),
			registryURL: "my-registry.example.com",
			pullSecrets: "pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return([]*v3.Project{
					systemProject("c-abcde", "p-system", map[string]string{
						secret.NeedsGlobalPrivateRegistryPullSecret: "true",
					}),
				}, nil)
			},
		},
		{
			name:        "global registry configuration removed, label removed from system project",
			cluster:     readyCluster("c-abcde", nil),
			registryURL: "",
			pullSecrets: "",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return([]*v3.Project{
					systemProject("c-abcde", "p-system", map[string]string{
						secret.NeedsGlobalPrivateRegistryPullSecret: "true",
					}),
				}, nil)
			},
			setupProjectClient: func(f *fake.MockControllerInterface[*v3.Project, *v3.ProjectList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(p *v3.Project) (*v3.Project, error) {
					_, exists := p.Labels[secret.NeedsGlobalPrivateRegistryPullSecret]
					assert.False(t, exists)
					return p, nil
				})
			},
		},
		{
			name: "cluster-level registry overrides global, label removed from system project",
			cluster: readyCluster("c-abcde", &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			registryURL: "global-registry.example.com",
			pullSecrets: "global-pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return([]*v3.Project{
					systemProject("c-abcde", "p-system", map[string]string{
						secret.NeedsGlobalPrivateRegistryPullSecret: "true",
					}),
				}, nil)
			},
			setupProjectClient: func(f *fake.MockControllerInterface[*v3.Project, *v3.ProjectList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(p *v3.Project) (*v3.Project, error) {
					_, exists := p.Labels[secret.NeedsGlobalPrivateRegistryPullSecret]
					assert.False(t, exists)
					return p, nil
				})
			},
		},
		{
			name:        "global registry with no pull secrets, label not added",
			cluster:     readyCluster("c-abcde", nil),
			registryURL: "my-registry.example.com",
			pullSecrets: "",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return([]*v3.Project{
					systemProject("c-abcde", "p-system", nil),
				}, nil)
			},
		},
		{
			name:        "error updating project on label add",
			cluster:     readyCluster("c-abcde", nil),
			registryURL: "my-registry.example.com",
			pullSecrets: "pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return([]*v3.Project{
					systemProject("c-abcde", "p-system", nil),
				}, nil)
			},
			setupProjectClient: func(f *fake.MockControllerInterface[*v3.Project, *v3.ProjectList]) {
				f.EXPECT().Update(gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name:        "error updating project on label remove",
			cluster:     readyCluster("c-abcde", nil),
			registryURL: "",
			pullSecrets: "",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return([]*v3.Project{
					systemProject("c-abcde", "p-system", map[string]string{
						secret.NeedsGlobalPrivateRegistryPullSecret: "true",
					}),
				}, nil)
			},
			setupProjectClient: func(f *fake.MockControllerInterface[*v3.Project, *v3.ProjectList]) {
				f.EXPECT().Update(gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name:        "system project has nil labels, global registry configured, label added without panic",
			cluster:     readyCluster("c-abcde", nil),
			registryURL: "my-registry.example.com",
			pullSecrets: "pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				// project with nil Labels
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return([]*v3.Project{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "p-system"},
						Spec:       v3.ProjectSpec{ClusterName: "c-abcde"},
						Status:     v3.ProjectStatus{BackingNamespace: "c-abcde-p-system"},
					},
				}, nil)
			},
			setupProjectClient: func(f *fake.MockControllerInterface[*v3.Project, *v3.ProjectList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(p *v3.Project) (*v3.Project, error) {
					assert.NotNil(t, p.Labels)
					assert.Equal(t, "true", p.Labels[secret.NeedsGlobalPrivateRegistryPullSecret])
					return p, nil
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			withSettings(t, map[settings.Setting]string{
				settings.SystemDefaultRegistry:            tt.registryURL,
				settings.SystemDefaultRegistryPullSecrets: tt.pullSecrets,
			})

			projectCache := fake.NewMockCacheInterface[*v3.Project](ctrl)
			if tt.setupProjectCache != nil {
				tt.setupProjectCache(projectCache)
			}
			projectClient := fake.NewMockControllerInterface[*v3.Project, *v3.ProjectList](ctrl)
			if tt.setupProjectClient != nil {
				tt.setupProjectClient(projectClient)
			}

			h := &handler{
				projectCache: projectCache,
				projects:     projectClient,
			}

			result, err := h.labelSystemProject("", tt.cluster)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.cluster != nil {
				assert.Equal(t, tt.cluster, result)
			}
		})
	}
}

func Test_handler_labelSourceGlobalRegistryPullSecret(t *testing.T) {
	tests := []struct {
		name              string
		setting           *v3.Setting
		registryURL       string
		pullSecrets       string
		setupSecretCache  func(*fake.MockCacheInterface[*corev1.Secret])
		setupSecretClient func(*fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList])
		wantNil           bool
		wantErr           bool
	}{
		{
			name:    "nil setting",
			setting: nil,
		},
		{
			name: "wrong setting name",
			setting: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: "some-other-setting"},
			},
		},
		{
			name: "no existing labeled secrets and no specified secrets",
			setting: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			registryURL: "",
			pullSecrets: "",
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(namespaces.System, gomock.Any()).Return([]*corev1.Secret{}, nil)
			},
		},
		{
			name: "error listing existing labeled secrets",
			setting: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(namespaces.System, gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "new secret specified, label added",
			setting: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			registryURL: "registry.com",
			pullSecrets: "new-secret",
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(namespaces.System, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get(namespaces.System, "new-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "new-secret", Namespace: namespaces.System},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "true", s.Labels[cluster.SourcePullSecretLabel])
					assert.NotEmpty(t, s.Annotations[secret.PSSIgnoreNamespacesAnnotation])
					return s, nil
				})
			},
		},
		{
			name: "existing secret removed from setting, label and annotation removed",
			setting: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			registryURL: "registry.com",
			pullSecrets: "",
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(namespaces.System, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "old-secret",
							Namespace: namespaces.System,
							Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
							Annotations: map[string]string{
								secret.PSSIgnoreNamespacesAnnotation: "cattle-system",
							},
						},
					},
				}, nil)
				f.EXPECT().Get(namespaces.System, "old-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "old-secret",
						Namespace: namespaces.System,
						Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
						Annotations: map[string]string{
							secret.PSSIgnoreNamespacesAnnotation: "cattle-system",
						},
					},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					_, hasSource := s.Labels[cluster.SourcePullSecretLabel]
					assert.False(t, hasSource)
					_, hasPSS := s.Annotations[secret.PSSIgnoreNamespacesAnnotation]
					assert.False(t, hasPSS)
					return s, nil
				})
			},
		},
		{
			name: "secret replaced",
			setting: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			registryURL: "registry.com",
			pullSecrets: "new-secret",
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(namespaces.System, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "old-secret",
							Namespace: namespaces.System,
							Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
						},
					},
				}, nil)
				f.EXPECT().Get(namespaces.System, "old-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "old-secret",
						Namespace: namespaces.System,
						Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
					},
				}, nil)
				f.EXPECT().Get(namespaces.System, "new-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "new-secret", Namespace: namespaces.System},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				// remove old
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "old-secret", s.Name)
					_, hasSource := s.Labels[cluster.SourcePullSecretLabel]
					_, hasAnno := s.Annotations[secret.PSSIgnoreNamespacesAnnotation]
					assert.False(t, hasSource)
					assert.False(t, hasAnno)
					return s, nil
				})
				// add new
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "new-secret", s.Name)
					assert.Equal(t, "true", s.Labels[cluster.SourcePullSecretLabel])
					assert.NotEmpty(t, s.Annotations[secret.PSSIgnoreNamespacesAnnotation])
					return s, nil
				})
			},
		},
		{
			name: "error getting secret during removal",
			setting: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			pullSecrets: "",
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(namespaces.System, gomock.Any()).Return([]*corev1.Secret{
					{ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: namespaces.System, Labels: map[string]string{cluster.SourcePullSecretLabel: "true"}}},
				}, nil)
				f.EXPECT().Get(namespaces.System, "secret").Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error getting secret during labeling",
			setting: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			registryURL: "registry.com",
			pullSecrets: "missing-secret",
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(namespaces.System, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get(namespaces.System, "missing-secret").Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "secret already labeled and still specified, no update needed",
			setting: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			registryURL: "registry.com",
			pullSecrets: "existing-secret",
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(namespaces.System, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "existing-secret",
							Namespace: namespaces.System,
							Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
						},
					},
				}, nil)
				// secret is in both existing and specified sets, so neither toRemove nor toLabel
				// no Get or Update should be called
			},
		},
		{
			name: "error updating secret during label removal",
			setting: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			pullSecrets: "",
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(namespaces.System, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "old-secret",
							Namespace: namespaces.System,
							Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
						},
					},
				}, nil)
				f.EXPECT().Get(namespaces.System, "old-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "old-secret",
						Namespace: namespaces.System,
						Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
					},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error updating secret during label addition",
			setting: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			registryURL: "registry.com",
			pullSecrets: "new-secret",
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(namespaces.System, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get(namespaces.System, "new-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "new-secret", Namespace: namespaces.System},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			withSettings(t, map[settings.Setting]string{
				settings.SystemDefaultRegistry:            tt.registryURL,
				settings.SystemDefaultRegistryPullSecrets: tt.pullSecrets,
			})

			secretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			if tt.setupSecretCache != nil {
				tt.setupSecretCache(secretCache)
			}
			secretClient := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			if tt.setupSecretClient != nil {
				tt.setupSecretClient(secretClient)
			}

			h := &handler{
				secretCache: secretCache,
				secrets:     secretClient,
			}

			result, err := h.labelConfiguredSourceGlobalRegistryPullSecrets("", tt.setting)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.setting != nil && !tt.wantNil {
				assert.Equal(t, tt.setting, result)
			}
		})
	}
}

func Test_handler_manageClusterSpecificPSS(t *testing.T) {
	const (
		testCluster   = "c-abcde"
		testProject   = "p-system"
		testBackingNS = testCluster + "-" + testProject

		localProject   = "p-l8l4n"
		localBackingNS = "local-" + localProject
	)

	var (
		existingData = []byte(`{"auths":{"cluster-registry.example.com":{"auth":""}}}`)
		updatedData  = []byte(`{"auths":{"cluster-registry-2.example.com":{"auth":""}}}`)
	)

	tests := []struct {
		name              string
		cluster           *v3.Cluster
		registryURL       string
		pullSecrets       string
		setupProjectCache func(*fake.MockCacheInterface[*v3.Project])
		setupSecretCache  func(*fake.MockCacheInterface[*corev1.Secret])
		setupSecretClient func(*fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList])
		wantErr           bool
	}{
		{
			name:    "nil cluster",
			cluster: nil,
		},
		{
			name:        "local cluster skips readiness check, uses global registry, PSS created in system project",
			cluster:     &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "local"}},
			registryURL: "global-registry.example.com",
			pullSecrets: "global-pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "local").Return([]*v3.Project{
					systemProject("local", localProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(localBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get(namespaces.System, "global-pull-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "global-pull-secret", Namespace: namespaces.System},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`)},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "global-pull-secret", s.Name)
					assert.Equal(t, localBackingNS, s.Namespace)
					assert.Equal(t, "true", s.Labels[cluster.CopiedPullSecretLabel])
					return s, nil
				})
			},
		},
		{
			name:        "local cluster, no registry configured, no PSS, early return",
			cluster:     &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "local"}},
			registryURL: "",
			pullSecrets: "",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "local").Return([]*v3.Project{
					systemProject("local", localProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(localBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
			},
		},
		{
			name:        "local cluster, no registry configured, stale PSS cleaned up",
			cluster:     &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "local"}},
			registryURL: "",
			pullSecrets: "",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "local").Return([]*v3.Project{
					systemProject("local", localProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(localBackingNS, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "stale-pss",
							Namespace: localBackingNS,
							Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
						},
					},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Delete(localBackingNS, "stale-pss", &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name:        "local cluster, global registry, stale PSS from old config cleaned up, new PSS created",
			cluster:     &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "local"}},
			registryURL: "global-registry.example.com",
			pullSecrets: "global-pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "local").Return([]*v3.Project{
					systemProject("local", localProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(localBackingNS, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "old-cluster-secret",
							Namespace: localBackingNS,
							Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
						},
					},
				}, nil)
				f.EXPECT().Get(namespaces.System, "global-pull-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "global-pull-secret", Namespace: namespaces.System},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`)},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Delete(localBackingNS, "old-cluster-secret", &metav1.DeleteOptions{}).Return(nil)
				f.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "global-pull-secret", s.Name)
					assert.Equal(t, localBackingNS, s.Namespace)
					assert.Equal(t, "true", s.Labels[cluster.CopiedPullSecretLabel])
					return s, nil
				})
			},
		},
		{
			name:        "no private registry configured",
			cluster:     readyCluster(testCluster, nil),
			registryURL: "",
			pullSecrets: "",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
			},
		},
		{
			name:        "provisioned cluster skipped",
			cluster:     readyCluster("c-m-abc", nil),
			registryURL: "my-registry.example.com",
			pullSecrets: "pull-secret",
		},
		{
			name: "cluster not ready",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: testCluster},
				Spec: v3.ClusterSpec{
					ImportedConfig: &v3.ImportedConfig{
						PrivateRegistryURL:         "cluster-registry.example.com",
						PrivateRegistryPullSecrets: []string{"cluster-secret"},
					},
					FleetWorkspaceName: "fleet-default",
				},
			},
		},
		{
			name: "cluster-level registry, no system project found",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return(nil, nil)
			},
			wantErr: true,
		},
		{
			name: "cluster-level registry, no existing PSS and no pull secrets to create",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL: "cluster-registry.example.com",
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
			},
		},
		{
			name: "cluster-level registry, source secret found, PSS created",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`)},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, cluster.GeneratePullSecretName("cluster-secret"), s.Name)
					assert.Equal(t, testBackingNS, s.Namespace)
					assert.Equal(t, "true", s.Labels[cluster.CopiedPullSecretLabel])
					assert.Equal(t, testProject, s.Labels["management.cattle.io/project-scoped-secret"])
					assert.Equal(t, "true", s.Labels["management.cattle.io/registry-scoped-secret"])
					assert.Equal(t, corev1.SecretTypeDockerConfigJson, s.Type)
					return s, nil
				})
			},
		},
		{
			name: "cluster-level registry, PSS already exists and is up to date, no update needed",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				existingData := []byte(`{"auths":{"cluster-registry.example.com":{"auth":""}}}`)
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      cluster.GeneratePullSecretName("cluster-secret"),
							Namespace: testBackingNS,
							Labels: map[string]string{
								cluster.CopiedPullSecretLabel: "true",
								registrySecretUIHint:          "true",
							},
						},
						Data: map[string][]byte{corev1.DockerConfigJsonKey: existingData},
					},
				}, nil)
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: existingData},
				}, nil)
			},
			// setupSecretClient: no client calls expected.
		},
		{
			name: "cluster-level registry, PSS already exists but is out of date, update needed",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      cluster.GeneratePullSecretName("cluster-secret"),
							Namespace: testBackingNS,
							Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
						},
						Data: map[string][]byte{corev1.DockerConfigJsonKey: existingData},
					},
				}, nil)
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: updatedData},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(p *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, p.Data[corev1.DockerConfigJsonKey], updatedData)
					assert.Equal(t, "true", p.Labels[registrySecretUIHint])
					return p, nil
				})
			},
		},
		{
			name: "cluster-level registry, PSS data up to date but registrySecretUIHint label absent, update triggered to add label",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      cluster.GeneratePullSecretName("cluster-secret"),
							Namespace: testBackingNS,
							// registrySecretUIHint label is intentionally absent
							Labels: map[string]string{cluster.CopiedPullSecretLabel: "true"},
						},
						Data: map[string][]byte{corev1.DockerConfigJsonKey: existingData},
					},
				}, nil)
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: existingData},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(p *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "true", p.Labels[registrySecretUIHint])
					assert.Equal(t, existingData, p.Data[corev1.DockerConfigJsonKey])
					return p, nil
				})
			},
		},
		{
			name: "cluster-level registry, PSS data up to date but registrySecretUIHint label is wrong value, update triggered to correct label",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      cluster.GeneratePullSecretName("cluster-secret"),
							Namespace: testBackingNS,
							Labels: map[string]string{
								cluster.CopiedPullSecretLabel: "true",
								registrySecretUIHint:          "false",
							},
						},
						Data: map[string][]byte{corev1.DockerConfigJsonKey: existingData},
					},
				}, nil)
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: existingData},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(p *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "true", p.Labels[registrySecretUIHint])
					assert.Equal(t, existingData, p.Data[corev1.DockerConfigJsonKey])
					return p, nil
				})
			},
		},
		{
			name: "stale PSS deleted when source secret removed from cluster spec",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL: "cluster-registry.example.com",
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "stale-secret",
							Namespace: testBackingNS,
							Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
						},
					},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Delete(testBackingNS, "stale-secret", &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name: "source secret not found, skipped",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"missing-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get("fleet-default", "missing-secret").Return(nil, errNotFound)
			},
		},
		{
			name:        "global default registry configured, PSS creation skipped after unused secret cleanup",
			cluster:     readyCluster(testCluster, nil),
			registryURL: "global-registry.example.com",
			pullSecrets: "global-pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "stale-from-old-cluster-config",
							Namespace: testBackingNS,
							Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
						},
					},
				}, nil)
				f.EXPECT().Get(namespaces.System, "global-pull-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "global-pull-secret", Namespace: namespaces.System},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`)},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Delete(testBackingNS, "stale-from-old-cluster-config", &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name: "error listing existing PSS",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error getting system project from cache",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL: "cluster-registry.example.com",
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error getting source pull secret (non-not-found error)",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"error-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get("fleet-default", "error-secret").Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error deleting stale PSS",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL: "cluster-registry.example.com",
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "stale-secret",
							Namespace: testBackingNS,
							Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
						},
					},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Delete(testBackingNS, "stale-secret", &metav1.DeleteOptions{}).Return(errDefault)
			},
			wantErr: true,
		},
		{
			name: "error creating new PSS",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`)},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Create(gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error updating existing out-of-date PSS",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      cluster.GeneratePullSecretName("cluster-secret"),
							Namespace: testBackingNS,
							Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
						},
						Data: map[string][]byte{corev1.DockerConfigJsonKey: existingData},
					},
				}, nil)
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: updatedData},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "PSS already exists without label, adopted, label and data persisted even when data matches",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				// List returns empty: the unlabeled secret is invisible to the label selector
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				// source secret in fleet-default
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: existingData},
				}, nil)
				// fetch for adoption uses generated name
				f.EXPECT().Get(testBackingNS, cluster.GeneratePullSecretName("cluster-secret")).Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cluster.GeneratePullSecretName("cluster-secret"),
						Namespace: testBackingNS,
					},
					Data: map[string][]byte{corev1.DockerConfigJsonKey: existingData},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Create(gomock.Any()).Return(nil, errAlreadyExists)
				// Update must always be called to persist the label, even when data is unchanged
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, cluster.GeneratePullSecretName("cluster-secret"), s.Name)
					assert.Equal(t, "true", s.Labels[cluster.CopiedPullSecretLabel])
					assert.Equal(t, existingData, s.Data[corev1.DockerConfigJsonKey])
					return s, nil
				})
			},
		},
		{
			name: "PSS already exists without label, adopted, data differs, label and new data persisted",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: updatedData},
				}, nil)
				f.EXPECT().Get(testBackingNS, cluster.GeneratePullSecretName("cluster-secret")).Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cluster.GeneratePullSecretName("cluster-secret"),
						Namespace: testBackingNS,
					},
					Data: map[string][]byte{corev1.DockerConfigJsonKey: existingData},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Create(gomock.Any()).Return(nil, errAlreadyExists)
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, cluster.GeneratePullSecretName("cluster-secret"), s.Name)
					assert.Equal(t, "true", s.Labels[cluster.CopiedPullSecretLabel])
					assert.Equal(t, updatedData, s.Data[corev1.DockerConfigJsonKey])
					return s, nil
				})
			},
		},
		{
			name: "PSS already exists without label, adopted, nil labels initialised",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: existingData},
				}, nil)
				// existing secret has nil labels, fetched by generated name
				f.EXPECT().Get(testBackingNS, cluster.GeneratePullSecretName("cluster-secret")).Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cluster.GeneratePullSecretName("cluster-secret"),
						Namespace: testBackingNS,
						Labels:    nil,
					},
					Data: map[string][]byte{corev1.DockerConfigJsonKey: existingData},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Create(gomock.Any()).Return(nil, errAlreadyExists)
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.NotNil(t, s.Labels)
					assert.Equal(t, "true", s.Labels[cluster.CopiedPullSecretLabel])
					return s, nil
				})
			},
		},
		{
			name: "PSS already exists without label, adopted, nil data initialised",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: existingData},
				}, nil)
				// existing secret has nil Data, fetched by generated name
				f.EXPECT().Get(testBackingNS, cluster.GeneratePullSecretName("cluster-secret")).Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cluster.GeneratePullSecretName("cluster-secret"),
						Namespace: testBackingNS,
					},
					Data: nil,
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Create(gomock.Any()).Return(nil, errAlreadyExists)
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.NotNil(t, s.Data)
					assert.Equal(t, "true", s.Labels[cluster.CopiedPullSecretLabel])
					assert.Equal(t, existingData, s.Data[corev1.DockerConfigJsonKey])
					return s, nil
				})
			},
		},
		{
			name: "error fetching existing secret during adoption",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: existingData},
				}, nil)
				f.EXPECT().Get(testBackingNS, cluster.GeneratePullSecretName("cluster-secret")).Return(nil, errDefault)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Create(gomock.Any()).Return(nil, errAlreadyExists)
			},
			wantErr: true,
		},
		{
			name: "error updating secret during adoption",
			cluster: readyCluster(testCluster, &v3.ImportedConfig{
				PrivateRegistryURL:         "cluster-registry.example.com",
				PrivateRegistryPullSecrets: []string{"cluster-secret"},
			}),
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get("fleet-default", "cluster-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-secret", Namespace: "fleet-default"},
					Type:       corev1.SecretTypeDockerConfigJson,
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: existingData},
				}, nil)
				f.EXPECT().Get(testBackingNS, cluster.GeneratePullSecretName("cluster-secret")).Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: cluster.GeneratePullSecretName("cluster-secret"), Namespace: testBackingNS},
					Data:       map[string][]byte{corev1.DockerConfigJsonKey: existingData},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Create(gomock.Any()).Return(nil, errAlreadyExists)
				f.EXPECT().Update(gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name:        "global registry with pull secrets, no existing PSS, early return",
			cluster:     readyCluster(testCluster, nil),
			registryURL: "global-registry.example.com",
			pullSecrets: "global-pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, testCluster).Return([]*v3.Project{
					systemProject(testCluster, testProject, nil),
				}, nil)
			},
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			withSettings(t, map[settings.Setting]string{
				settings.SystemDefaultRegistry:            tt.registryURL,
				settings.SystemDefaultRegistryPullSecrets: tt.pullSecrets,
			})

			projectCache := fake.NewMockCacheInterface[*v3.Project](ctrl)
			if tt.setupProjectCache != nil {
				tt.setupProjectCache(projectCache)
			}
			secretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			if tt.setupSecretCache != nil {
				tt.setupSecretCache(secretCache)
			}
			secretClient := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			if tt.setupSecretClient != nil {
				tt.setupSecretClient(secretClient)
			}

			h := &handler{
				projectCache: projectCache,
				secretCache:  secretCache,
				secrets:      secretClient,
			}

			result, err := h.manageClusterSpecificPSS("", tt.cluster)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.cluster != nil {
				assert.Equal(t, tt.cluster, result)
			}
		})
	}
}

func Test_handler_labelSourceGlobalRegistryPullSecretOnChange(t *testing.T) {
	tests := []struct {
		name              string
		secret            *corev1.Secret
		registryURL       string
		pullSecrets       string
		setupSecretClient func(*fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList])
		wantErr           bool
	}{
		{
			name:   "nil secret returns nil",
			secret: nil,
		},
		{
			name: "secret in wrong namespace skipped",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "pull-secret", Namespace: "default"},
				Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{}`)},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
		},
		{
			name: "secret with nil data skipped",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "pull-secret", Namespace: namespaces.System},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
		},
		{
			name: "secret not in active pull secrets skipped",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "other-secret", Namespace: namespaces.System},
				Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{}`)},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
		},
		{
			name: "secret already correctly labeled and annotated, no update",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pull-secret",
					Namespace: namespaces.System,
					Labels: map[string]string{
						cluster.SourcePullSecretLabel: "true",
					},
					Annotations: map[string]string{
						secret.PSSIgnoreNamespacesAnnotation: strings.Join(settings.SystemNamespacesIgnoringPullSecrets, ","),
					},
				},
				Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{}`)},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
		},
		{
			name: "secret missing source label, label and annotation added",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "pull-secret", Namespace: namespaces.System},
				Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{}`)},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "true", s.Labels[cluster.SourcePullSecretLabel])
					assert.Equal(t, strings.Join(settings.SystemNamespacesIgnoringPullSecrets, ","), s.Annotations[secret.PSSIgnoreNamespacesAnnotation])
					return s, nil
				})
			},
		},
		{
			name: "secret with empty source label, label and stale annotation corrected",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pull-secret",
					Namespace: namespaces.System,
					Labels:    map[string]string{cluster.SourcePullSecretLabel: ""},
					Annotations: map[string]string{
						secret.PSSIgnoreNamespacesAnnotation: "cattle-system",
					},
				},
				Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{}`)},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "true", s.Labels[cluster.SourcePullSecretLabel])
					assert.Equal(t, strings.Join(settings.SystemNamespacesIgnoringPullSecrets, ","), s.Annotations[secret.PSSIgnoreNamespacesAnnotation])
					return s, nil
				})
			},
		},
		{
			name: "source label explicitly false, label set to true",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pull-secret",
					Namespace: namespaces.System,
					Labels:    map[string]string{cluster.SourcePullSecretLabel: "false"},
					Annotations: map[string]string{
						secret.PSSIgnoreNamespacesAnnotation: strings.Join(settings.SystemNamespacesIgnoringPullSecrets, ","),
					},
				},
				Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{}`)},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "true", s.Labels[cluster.SourcePullSecretLabel])
					return s, nil
				})
			},
		},
		{
			name: "stale annotation value triggers annotation update",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pull-secret",
					Namespace: namespaces.System,
					Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
					Annotations: map[string]string{
						secret.PSSIgnoreNamespacesAnnotation: "cattle-system",
					},
				},
				Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{}`)},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "true", s.Labels[cluster.SourcePullSecretLabel])
					assert.Equal(t, strings.Join(settings.SystemNamespacesIgnoringPullSecrets, ","), s.Annotations[secret.PSSIgnoreNamespacesAnnotation])
					return s, nil
				})
			},
		},
		{
			name: "secret has source label but missing annotation, annotation added",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pull-secret",
					Namespace: namespaces.System,
					Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
				Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{}`)},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "true", s.Labels[cluster.SourcePullSecretLabel])
					assert.NotEmpty(t, s.Annotations[secret.PSSIgnoreNamespacesAnnotation])
					return s, nil
				})
			},
		},
		{
			name: "secret has source label but empty annotation, annotation set",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pull-secret",
					Namespace: namespaces.System,
					Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
					Annotations: map[string]string{
						secret.PSSIgnoreNamespacesAnnotation: "",
					},
				},
				Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{}`)},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "true", s.Labels[cluster.SourcePullSecretLabel])
					assert.Equal(t, strings.Join(settings.SystemNamespacesIgnoringPullSecrets, ","), s.Annotations[secret.PSSIgnoreNamespacesAnnotation])
					return s, nil
				})
			},
		},
		{
			name: "no pull secrets configured, secret skipped",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "pull-secret", Namespace: namespaces.System},
				Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{}`)},
			},
			registryURL: "",
			pullSecrets: "",
		},
		{
			name: "error on update returns error",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "pull-secret", Namespace: namespaces.System},
				Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{}`)},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
			setupSecretClient: func(f *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Update(gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			withSettings(t, map[settings.Setting]string{
				settings.SystemDefaultRegistry:            tt.registryURL,
				settings.SystemDefaultRegistryPullSecrets: tt.pullSecrets,
			})

			secretClient := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			if tt.setupSecretClient != nil {
				tt.setupSecretClient(secretClient)
			}

			h := &handler{
				secrets: secretClient,
			}

			result, err := h.labelSourceGlobalRegistryPullSecret("", tt.secret)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.secret != nil {
				assert.NotNil(t, result)
			}
		})
	}
}

func Test_handler_getSystemProjectForCluster(t *testing.T) {
	tests := []struct {
		name              string
		clusterName       string
		setupProjectCache func(*fake.MockCacheInterface[*v3.Project])
		wantProject       *v3.Project
		wantErr           bool
	}{
		{
			name:        "project found",
			clusterName: "c-abcde",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return([]*v3.Project{
					{ObjectMeta: metav1.ObjectMeta{Name: "p-system"}},
				}, nil)
			},
			wantProject: &v3.Project{ObjectMeta: metav1.ObjectMeta{Name: "p-system"}},
			wantErr:     false,
		},
		{
			name:        "no project found, error returned",
			clusterName: "c-abcde",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return(nil, nil)
			},
			wantProject: nil,
			wantErr:     true,
		},
		{
			name:        "no project found, empty slice, error returned",
			clusterName: "c-abcde",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return([]*v3.Project{}, nil)
			},
			wantProject: nil,
			wantErr:     true,
		},
		{
			name:        "error returned from indexer",
			clusterName: "c-abcde",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return(nil, errDefault)
			},
			wantProject: nil,
			wantErr:     true,
		},
		{
			name:        "multiple system projects, only returns first",
			clusterName: "c-abcde",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "c-abcde").Return([]*v3.Project{
					{ObjectMeta: metav1.ObjectMeta{Name: "p-first"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "p-second"}},
				}, nil)
			},
			wantProject: &v3.Project{ObjectMeta: metav1.ObjectMeta{Name: "p-first"}},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			projectCache := fake.NewMockCacheInterface[*v3.Project](ctrl)
			if tt.setupProjectCache != nil {
				tt.setupProjectCache(projectCache)
			}
			h := &handler{projectCache: projectCache}

			project, err := h.getSystemProjectForCluster(tt.clusterName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantProject, project)
		})
	}
}

func Test_buildPSS(t *testing.T) {
	proj := &v3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "p-system"},
	}
	data := []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`)

	pss := buildPSS(proj, "c-abcde-p-system", "my-pull-secret", data)

	assert.Equal(t, "my-pull-secret", pss.Name)
	assert.Equal(t, "c-abcde-p-system", pss.Namespace)
	assert.Equal(t, corev1.SecretTypeDockerConfigJson, pss.Type)
	assert.Equal(t, data, pss.Data[corev1.DockerConfigJsonKey])
	assert.Equal(t, "true", pss.Labels[cluster.CopiedPullSecretLabel])
	assert.Equal(t, "p-system", pss.Labels["management.cattle.io/project-scoped-secret"])
	assert.Equal(t, "true", pss.Labels["management.cattle.io/registry-scoped-secret"])
	assert.NotEmpty(t, pss.Annotations[secret.PSSIgnoreNamespacesAnnotation])
}

func Test_findV3ClustersUsingGlobalPullSecrets(t *testing.T) {
	// A cluster with no ImportedConfig URL will have isGlobalDefault=true.
	globalCluster := func(name string) *v3.Cluster {
		return &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name}}
	}
	// A cluster with an ImportedConfig URL will have isGlobalDefault=false.
	overrideCluster := func(name string) *v3.Cluster {
		return &v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: v3.ClusterSpec{
				ImportedConfig:     &v3.ImportedConfig{PrivateRegistryURL: "cluster-registry.example.com"},
				FleetWorkspaceName: "fleet-default",
			},
		}
	}

	withSettings(t, map[settings.Setting]string{
		settings.SystemDefaultRegistry:            "a-global-registry",
		settings.SystemDefaultRegistryPullSecrets: "",
	})

	tests := []struct {
		name       string
		setupCache func(*fake.MockNonNamespacedCacheInterface[*v3.Cluster])
		wantNames  []string
		wantErr    bool
	}{
		{
			name: "error listing clusters returns error",
			setupCache: func(f *fake.MockNonNamespacedCacheInterface[*v3.Cluster]) {
				f.EXPECT().List(gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "empty cluster list returns nil",
			setupCache: func(f *fake.MockNonNamespacedCacheInterface[*v3.Cluster]) {
				f.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{}, nil)
			},
			wantNames: nil,
		},
		{
			name: "clusters with cluster-level registry override are excluded",
			setupCache: func(f *fake.MockNonNamespacedCacheInterface[*v3.Cluster]) {
				f.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{
					overrideCluster("c-abcde"),
				}, nil)
			},
			wantNames: nil,
		},
		{
			name: "provisioned clusters using global default are included",
			setupCache: func(f *fake.MockNonNamespacedCacheInterface[*v3.Cluster]) {
				f.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{
					globalCluster("c-m-abc12"),
				}, nil)
			},
			wantNames: []string{"c-m-abc12"},
		},
		{
			name: "imported cluster using global default is included",
			setupCache: func(f *fake.MockNonNamespacedCacheInterface[*v3.Cluster]) {
				f.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{
					globalCluster("c-abcde"),
				}, nil)
			},
			wantNames: []string{"c-abcde"},
		},
		{
			name: "local cluster using global default is included",
			setupCache: func(f *fake.MockNonNamespacedCacheInterface[*v3.Cluster]) {
				f.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{
					globalCluster("local"),
				}, nil)
			},
			wantNames: []string{"local"},
		},
		{
			name: "mix: all clusters using global default are returned, only cluster-level overrides are excluded",
			setupCache: func(f *fake.MockNonNamespacedCacheInterface[*v3.Cluster]) {
				f.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{
					globalCluster("local"),
					globalCluster("c-abcde"),
					overrideCluster("c-fghij"), // cluster-level override → excluded
					globalCluster("c-m-abc12"), // provisioned name, global default → included
				}, nil)
			},
			wantNames: []string{"local", "c-abcde", "c-m-abc12"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			clusterCache := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
			tt.setupCache(clusterCache)

			names, err := findV3ClustersUsingGlobalPullSecrets(clusterCache)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.ElementsMatch(t, tt.wantNames, names)
		})
	}
}

func Test_handler_syncClusterOnGlobalPullSecretChange(t *testing.T) {
	importedCluster := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-abcde"}}

	tests := []struct {
		name            string
		obj             runtime.Object
		registryURL     string
		pullSecrets     string
		setupCache      func(*fake.MockNonNamespacedCacheInterface[*v3.Cluster])
		setupController func(*fake.MockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList])
		wantErr         bool
	}{
		{
			name: "nil object returns nil, nothing enqueued",
			obj:  nil,
		},
		{
			name: "non-secret object returns nil, nothing enqueued",
			obj:  &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-abcde"}},
		},
		{
			name:        "secret with nil labels returns nil, nothing enqueued",
			obj:         &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "pull-secret"}},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
		},
		{
			name: "secret with source label not true returns nil, nothing enqueued",
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "pull-secret",
					Labels: map[string]string{cluster.SourcePullSecretLabel: "false"},
				},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
		},
		{
			name: "source secret not in configured pull secrets returns nil, nothing enqueued",
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "other-secret",
					Labels: map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
		},
		{
			name: "no pull secrets configured, secret not active, nothing enqueued",
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "pull-secret",
					Labels: map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
			},
			registryURL: "",
			pullSecrets: "",
		},
		{
			name: "valid source secret triggers staggered enqueue of affected clusters",
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "pull-secret",
					Labels: map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
			setupCache: func(f *fake.MockNonNamespacedCacheInterface[*v3.Cluster]) {
				f.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{importedCluster}, nil)
			},
			setupController: func(f *fake.MockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList]) {
				f.EXPECT().EnqueueAfter("c-abcde", gomock.Any())
			},
		},
		{
			name: "valid source secret, cluster cache error propagated, nothing enqueued",
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "pull-secret",
					Labels: map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
			},
			registryURL: "registry.com",
			pullSecrets: "pull-secret",
			setupCache: func(f *fake.MockNonNamespacedCacheInterface[*v3.Cluster]) {
				f.EXPECT().List(gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			withSettings(t, map[settings.Setting]string{
				settings.SystemDefaultRegistry:            tt.registryURL,
				settings.SystemDefaultRegistryPullSecrets: tt.pullSecrets,
			})

			clusterCache := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
			if tt.setupCache != nil {
				tt.setupCache(clusterCache)
			}
			clusterController := fake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](ctrl)
			if tt.setupController != nil {
				tt.setupController(clusterController)
			}

			h := &handler{mgmtClusterCache: clusterCache, mgmtClusters: clusterController}
			keys, err := h.syncClusterOnGlobalPullSecretChange("", "", tt.obj)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			// The handler now always returns nil — cluster re-enqueue is done via EnqueueAfter.
			assert.Nil(t, keys)
		})
	}
}

func Test_handler_syncClusterOnGlobalRegistrySettingChange(t *testing.T) {
	importedCluster := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-abcde"}}
	localCluster := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "local"}}

	withSettings(t, map[settings.Setting]string{
		settings.SystemDefaultRegistry:            "a-private-registry",
		settings.SystemDefaultRegistryPullSecrets: "",
	})

	tests := []struct {
		name            string
		settingName     string
		setupCache      func(*fake.MockNonNamespacedCacheInterface[*v3.Cluster])
		setupController func(*fake.MockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList])
		wantErr         bool
	}{
		{
			name:        "unrelated setting name returns nil, nothing enqueued",
			settingName: "some-other-setting",
		},
		{
			name:        "SystemDefaultRegistryPullSecrets triggers staggered enqueue of affected clusters",
			settingName: settings.SystemDefaultRegistryPullSecrets.Name,
			setupCache: func(f *fake.MockNonNamespacedCacheInterface[*v3.Cluster]) {
				f.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{importedCluster, localCluster}, nil)
			},
			setupController: func(f *fake.MockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList]) {
				f.EXPECT().EnqueueAfter("c-abcde", gomock.Any())
				f.EXPECT().EnqueueAfter("local", gomock.Any())
			},
		},
		{
			name:        "SystemDefaultRegistry triggers staggered enqueue of affected clusters",
			settingName: settings.SystemDefaultRegistry.Name,
			setupCache: func(f *fake.MockNonNamespacedCacheInterface[*v3.Cluster]) {
				f.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{importedCluster}, nil)
			},
			setupController: func(f *fake.MockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList]) {
				f.EXPECT().EnqueueAfter("c-abcde", gomock.Any())
			},
		},
		{
			name:        "cluster cache error propagated, nothing enqueued",
			settingName: settings.SystemDefaultRegistryPullSecrets.Name,
			setupCache: func(f *fake.MockNonNamespacedCacheInterface[*v3.Cluster]) {
				f.EXPECT().List(gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			clusterCache := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
			if tt.setupCache != nil {
				tt.setupCache(clusterCache)
			}
			clusterController := fake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](ctrl)
			if tt.setupController != nil {
				tt.setupController(clusterController)
			}

			h := &handler{mgmtClusterCache: clusterCache, mgmtClusters: clusterController}
			keys, err := h.syncClusterOnGlobalRegistrySettingChange("", tt.settingName, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			// The handler now always returns nil — cluster re-enqueue is done via EnqueueAfter.
			assert.Nil(t, keys)
		})
	}
}

func Test_enqueueStaggered(t *testing.T) {
	type enqueueCall struct {
		name  string
		delay time.Duration
	}

	// clusterNames builds a slice ["c-0", "c-1", ..., "c-(n-1)"].
	clusterNames := func(n int) []string {
		names := make([]string, n)
		for i := range names {
			names[i] = fmt.Sprintf("c-%d", i)
		}
		return names
	}

	tests := []struct {
		name         string
		input        []string
		verifyDelays func(t *testing.T, calls []enqueueCall)
	}{
		{
			name:  "empty list enqueues nothing",
			input: nil,
			verifyDelays: func(t *testing.T, calls []enqueueCall) {
				assert.Empty(t, calls)
			},
		},
		{
			name:  "single cluster receives first-batch delay (< 2s + jitter ceiling)",
			input: []string{"c-abcde"},
			verifyDelays: func(t *testing.T, calls []enqueueCall) {
				assert.Len(t, calls, 1)
				assert.Equal(t, "c-abcde", calls[0].name)
				// batchIndex=0: delay = 0*clusterEnqueueDelay + jitter, jitter in [0, 750ms)
				assert.GreaterOrEqual(t, calls[0].delay, time.Duration(0))
				assert.Less(t, calls[0].delay, time.Duration(clusterEnqueueJitterCeiling)*time.Millisecond)
			},
		},
		{
			name:  "all ten clusters in first batch receive delay below 2s",
			input: clusterNames(10),
			verifyDelays: func(t *testing.T, calls []enqueueCall) {
				assert.Len(t, calls, 10)
				names := make([]string, len(calls))
				for i, c := range calls {
					names[i] = c.name
				}
				assert.ElementsMatch(t, clusterNames(10), names)
				for _, c := range calls {
					// batchIndex=0 for all: delay in [0, 750ms)
					assert.GreaterOrEqual(t, c.delay, time.Duration(0))
					assert.Less(t, c.delay, time.Duration(clusterEnqueueJitterCeiling)*time.Millisecond)
				}
			},
		},
		{
			name:  "eleventh cluster falls into second batch with delay >= 2s",
			input: clusterNames(11),
			verifyDelays: func(t *testing.T, calls []enqueueCall) {
				assert.Len(t, calls, 11)
				byName := make(map[string]time.Duration, len(calls))
				for _, c := range calls {
					byName[c.name] = c.delay
				}
				// First batch (c-0 through c-9): batchIndex=0, delay in [0, 750ms)
				for i := 0; i < 10; i++ {
					d := byName[fmt.Sprintf("c-%d", i)]
					assert.GreaterOrEqual(t, d, time.Duration(0), "c-%d should have non-negative delay", i)
					assert.Less(t, d, time.Duration(clusterEnqueueJitterCeiling)*time.Millisecond, "c-%d should be in batch 0", i)
				}
				// Second batch (c-10): batchIndex=1, delay in [2s, 2s+750ms)
				d := byName["c-10"]
				assert.GreaterOrEqual(t, d, clusterEnqueueDelay, "c-10 should be in batch 1")
				assert.Less(t, d, clusterEnqueueDelay+time.Duration(clusterEnqueueJitterCeiling)*time.Millisecond, "c-10 should be in batch 1")
			},
		},
		{
			name:  "twenty-one clusters span three batches with correct delay ranges",
			input: clusterNames(21),
			verifyDelays: func(t *testing.T, calls []enqueueCall) {
				assert.Len(t, calls, 21)
				byName := make(map[string]time.Duration, len(calls))
				for _, c := range calls {
					byName[c.name] = c.delay
				}
				jitterCeiling := time.Duration(clusterEnqueueJitterCeiling) * time.Millisecond
				// Batch 0 (c-0 to c-9): delay in [0, 750ms)
				for i := 0; i < 10; i++ {
					d := byName[fmt.Sprintf("c-%d", i)]
					assert.GreaterOrEqual(t, d, time.Duration(0), "c-%d should be in batch 0", i)
					assert.Less(t, d, jitterCeiling, "c-%d should be in batch 0", i)
				}
				// Batch 1 (c-10 to c-19): delay in [2s, 2s+750ms)
				for i := 10; i < 20; i++ {
					d := byName[fmt.Sprintf("c-%d", i)]
					assert.GreaterOrEqual(t, d, clusterEnqueueDelay, "c-%d should be in batch 1", i)
					assert.Less(t, d, clusterEnqueueDelay+jitterCeiling, "c-%d should be in batch 1", i)
				}
				// Batch 2 (c-20): delay in [4s, 4s+750ms)
				d := byName["c-20"]
				assert.GreaterOrEqual(t, d, 2*clusterEnqueueDelay, "c-20 should be in batch 2")
				assert.Less(t, d, 2*clusterEnqueueDelay+jitterCeiling, "c-20 should be in batch 2")
			},
		},
		{
			name:  "cluster names are preserved exactly as passed",
			input: []string{"local", "c-abcde", "c-m-abc12"},
			verifyDelays: func(t *testing.T, calls []enqueueCall) {
				assert.Len(t, calls, 3)
				names := make([]string, len(calls))
				for i, c := range calls {
					names[i] = c.name
				}
				assert.ElementsMatch(t, []string{"local", "c-abcde", "c-m-abc12"}, names)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			clusterController := fake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](ctrl)

			var calls []enqueueCall
			if len(tt.input) > 0 {
				clusterController.EXPECT().
					EnqueueAfter(gomock.Any(), gomock.Any()).
					Do(func(name string, delay time.Duration) {
						calls = append(calls, enqueueCall{name: name, delay: delay})
					}).
					Times(len(tt.input))
			}

			enqueueStaggered(tt.input, clusterController)

			if tt.verifyDelays != nil {
				tt.verifyDelays(t, calls)
			}
		})
	}
}

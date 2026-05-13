package privateregistry

import (
	"fmt"
	"testing"

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
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	errDefault  = fmt.Errorf("error")
	errNotFound = apierrors.NewNotFound(schema.GroupResource{}, "")
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

func withSettings(t *testing.T, registryURL, pullSecrets string) {
	t.Helper()
	origRegistry := settings.SystemDefaultRegistry.Get()
	origPullSecrets := settings.SystemDefaultRegistryPullSecrets.Get()
	t.Cleanup(func() {
		settings.SystemDefaultRegistry.Set(origRegistry)
		settings.SystemDefaultRegistryPullSecrets.Set(origPullSecrets)
	})
	settings.SystemDefaultRegistry.Set(registryURL)
	settings.SystemDefaultRegistryPullSecrets.Set(pullSecrets)
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
			name:        "local cluster skips readiness check, adds label when global registry configured",
			cluster:     &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "local"}},
			registryURL: "my-registry.example.com",
			pullSecrets: "pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().GetByIndex(clusterToSysProjIndex, "local").Return([]*v3.Project{
					systemProject("local", "p-system", nil),
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
			withSettings(t, tt.registryURL, tt.pullSecrets)

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
				project:      projectClient,
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
			withSettings(t, tt.registryURL, tt.pullSecrets)

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

			result, err := h.labelSourceGlobalRegistryPullSecret("", tt.setting)
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

func Test_handler_manageImportedAndHostedClusterPSS(t *testing.T) {
	const (
		testCluster   = "c-abcde"
		testProject   = "p-system"
		testBackingNS = testCluster + "-" + testProject
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
			name:    "local cluster skipped",
			cluster: &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "local"}},
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
					assert.Equal(t, "cluster-secret", s.Name)
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
							Name:      "cluster-secret",
							Namespace: testBackingNS,
							Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
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
							Name:      "cluster-secret",
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
							Name:      "cluster-secret",
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
			withSettings(t, tt.registryURL, tt.pullSecrets)

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

			result, err := h.manageImportedAndHostedClusterPSS("", tt.cluster)
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

package secret

import (
	"reflect"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_isSystemProject(t *testing.T) {
	tests := []struct {
		name    string
		project *v3.Project
		want    bool
	}{
		{
			name:    "nil labels",
			project: &v3.Project{},
			want:    false,
		},
		{
			name: "label set to true",
			project: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"authz.management.cattle.io/system-project": "true"},
				},
			},
			want: true,
		},
		{
			name: "label set to other value",
			project: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"authz.management.cattle.io/system-project": "false"},
				},
			},
			want: false,
		},
		{
			name: "label absent",
			project: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"other-label": "true"},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSystemProject(tt.project); got != tt.want {
				t.Errorf("isSystemProject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_usesGlobalSecrets(t *testing.T) {
	tests := []struct {
		name    string
		project *v3.Project
		want    bool
	}{
		{
			name:    "nil labels",
			project: &v3.Project{},
			want:    false,
		},
		{
			name: "label set to true",
			project: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{needsGlobalPrivateRegistryPullSecret: "true"},
				},
			},
			want: true,
		},
		{
			name: "label set to other value",
			project: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{needsGlobalPrivateRegistryPullSecret: "false"},
				},
			},
			want: false,
		},
		{
			name: "label absent",
			project: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"other-label": "true"},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usesGlobalSecrets(tt.project); got != tt.want {
				t.Errorf("usesGlobalSecrets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_namespaceHandler_getNamespacesFromGlobalPullSecret(t *testing.T) {
	systemProjectLabels := map[string]string{
		"authz.management.cattle.io/system-project": "true",
		needsGlobalPrivateRegistryPullSecret:        "true",
	}

	tests := []struct {
		name                string
		secret              *corev1.Secret
		setupProjectCache   func(*fake.MockCacheInterface[*v3.Project])
		setupNamespaceCache func(*fake.MockNonNamespacedCacheInterface[*corev1.Namespace])
		want                []*corev1.Namespace
		wantErr             bool
	}{
		{
			name:   "secret has nil labels",
			secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s"}},
			want:   nil,
		},
		{
			name: "secret has other label but not default-registry label",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"some-other-label": "true"},
				},
			},
			want: nil,
		},
		{
			name: "secret has pss label, but not global pull secret label",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{ProjectScopedSecretLabel: "p-abc"},
				},
			},
			want: nil,
		},
		{
			name: "error listing projects",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return(nil, errDefault)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "no projects need global pull secrets",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{}, nil)
			},
			want: nil,
		},
		{
			name: "project has nil labels, skipped",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{
					{ObjectMeta: metav1.ObjectMeta{Name: "p-abc", Labels: nil}},
				}, nil)
			},
			want: nil,
		},
		{
			name: "project is not a system project, skipped",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{
					{ObjectMeta: metav1.ObjectMeta{Name: "p-abc", Labels: map[string]string{needsGlobalPrivateRegistryPullSecret: "true"}}},
				}, nil)
			},
			want: nil,
		},
		{
			name: "system project found, namespaces returned",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{
					{ObjectMeta: metav1.ObjectMeta{Name: "p-system", Labels: systemProjectLabels}},
				}, nil)
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return([]*corev1.Namespace{
					{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}},
				}, nil)
			},
			want: []*corev1.Namespace{
				{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}},
			},
		},
		{
			name: "error listing namespaces for project, error joined, other projects still processed",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{
					{ObjectMeta: metav1.ObjectMeta{Name: "p-fail", Labels: systemProjectLabels}},
					{ObjectMeta: metav1.ObjectMeta{Name: "p-ok", Labels: systemProjectLabels}},
				}, nil)
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return(nil, errDefault)
				f.EXPECT().List(gomock.Any()).Return([]*corev1.Namespace{
					{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}},
				}, nil)
			},
			want: []*corev1.Namespace{
				{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}},
			},
			wantErr: true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectCache := fake.NewMockCacheInterface[*v3.Project](ctrl)
			if tt.setupProjectCache != nil {
				tt.setupProjectCache(projectCache)
			}
			namespaceCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
			if tt.setupNamespaceCache != nil {
				tt.setupNamespaceCache(namespaceCache)
			}
			n := &namespaceHandler{
				projectCache:          projectCache,
				clusterNamespaceCache: namespaceCache,
				clusterName:           "c-abc",
			}
			got, err := n.getNamespacesFromGlobalPullSecret(tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("getNamespacesFromGlobalPullSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getNamespacesFromGlobalPullSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_namespaceHandler_secretEnqueueNamespace(t *testing.T) {
	tests := []struct {
		name                string
		obj                 runtime.Object
		clusterName         string
		setupProjectCache   func(*fake.MockCacheInterface[*v3.Project])
		setupNamespaceCache func(*fake.MockNonNamespacedCacheInterface[*corev1.Namespace])
		wantKeys            []relatedresource.Key
		wantErr             bool
	}{
		{
			name:     "nil object",
			obj:      nil,
			wantKeys: nil,
		},
		{
			name:     "non-secret type",
			obj:      &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns"}},
			wantKeys: nil,
		},
		{
			name:     "secret with no labels",
			obj:      &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s"}},
			wantKeys: []relatedresource.Key{},
		},
		{
			name: "global pull secret returns system project namespace keys",
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "pull-secret",
					Labels: map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
			},
			clusterName: "c-abc",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "p-system",
							Labels: map[string]string{
								"authz.management.cattle.io/system-project": "true",
								needsGlobalPrivateRegistryPullSecret:        "true",
							},
						},
					},
				}, nil)
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return([]*corev1.Namespace{
					{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}},
				}, nil)
			},
			wantKeys: []relatedresource.Key{{Name: "cattle-system"}},
		},
		{
			name: "global pull secret returns system project namespace keys but not unrelated project namespace keys",
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "pull-secret",
					Labels: map[string]string{cluster.SourcePullSecretLabel: "true"},
				},
			},
			clusterName: "c-abc",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "p-system",
							Labels: map[string]string{
								"authz.management.cattle.io/system-project": "true",
								needsGlobalPrivateRegistryPullSecret:        "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "n-system",
							Labels: map[string]string{
								needsGlobalPrivateRegistryPullSecret: "true",
							},
						},
					},
				}, nil)
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return([]*corev1.Namespace{
					{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}},
				}, nil).Times(1) // ensure n-system is skipped and a second list call is not made.
			},
			wantKeys: []relatedresource.Key{{Name: "cattle-system"}},
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectCache := fake.NewMockCacheInterface[*v3.Project](ctrl)
			if tt.setupProjectCache != nil {
				tt.setupProjectCache(projectCache)
			}
			namespaceCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
			if tt.setupNamespaceCache != nil {
				tt.setupNamespaceCache(namespaceCache)
			}
			n := &namespaceHandler{
				projectCache:          projectCache,
				clusterNamespaceCache: namespaceCache,
				clusterName:           tt.clusterName,
			}

			keys, err := n.secretEnqueueNamespace("", "", tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("secretEnqueueNamespace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(keys, tt.wantKeys) {
				t.Errorf("secretEnqueueNamespace() = %v, want %v", keys, tt.wantKeys)
			}
		})
	}
}

func Test_namespaceHandler_onSettingEnqueueNamespace(t *testing.T) {
	systemProjectLabels := map[string]string{
		"authz.management.cattle.io/system-project": "true",
		needsGlobalPrivateRegistryPullSecret:        "true",
	}

	tests := []struct {
		name                    string
		obj                     runtime.Object
		setupProjectCache       func(*fake.MockCacheInterface[*v3.Project])
		setupNamespaceCache     func(*fake.MockNonNamespacedCacheInterface[*corev1.Namespace])
		setupSettingsController func(*fake.MockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList])
		wantKeys                []relatedresource.Key
		wantErr                 bool
	}{
		{
			name:     "nil object",
			obj:      nil,
			wantKeys: nil,
		},
		{
			name:     "non-setting type",
			obj:      &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s"}},
			wantKeys: nil,
		},
		{
			name: "wrong setting name",
			obj: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: "some-other-setting"},
			},
			wantKeys: nil,
		},
		{
			name: "correct setting (pull secrets), no projects found",
			obj: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{}, nil)
			},
			wantKeys: []relatedresource.Key{},
		},
		{
			name: "correct setting (pull secrets), non system-project skipped",
			obj: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{
					{ObjectMeta: metav1.ObjectMeta{Name: "p-user", Labels: map[string]string{needsGlobalPrivateRegistryPullSecret: "true"}}},
				}, nil)
			},
			wantKeys: []relatedresource.Key{},
		},
		{
			name: "correct setting (pull secrets), system project found",
			obj: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{
					{ObjectMeta: metav1.ObjectMeta{Name: "p-system", Labels: systemProjectLabels}},
				}, nil)
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return([]*corev1.Namespace{
					{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "cattle-fleet-system"}},
				}, nil)
			},
			wantKeys: []relatedresource.Key{
				{Name: "cattle-system"},
				{Name: "cattle-fleet-system"},
			},
		},
		{
			name: "error listing projects (pull secrets setting)",
			obj: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return(nil, errDefault)
			},
			setupSettingsController: func(f *fake.MockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList]) {
				f.EXPECT().EnqueueAfter(settings.SystemDefaultRegistryPullSecrets.Name, time.Second*2)
			},
			wantKeys: nil,
			wantErr:  true,
		},
		{
			name: "error listing namespaces, no successful namespaces (pull secrets setting)",
			obj: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{
					{ObjectMeta: metav1.ObjectMeta{Name: "p-system", Labels: systemProjectLabels}},
				}, nil)
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return(nil, errDefault)
			},
			setupSettingsController: func(f *fake.MockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList]) {
				f.EXPECT().EnqueueAfter(settings.SystemDefaultRegistryPullSecrets.Name, time.Second*2)
			},
			wantKeys: nil,
			wantErr:  true,
		},
		{
			name: "correct setting (default registry), no projects found",
			obj: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistry.Name},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{}, nil)
			},
			wantKeys: []relatedresource.Key{},
		},
		{
			name: "correct setting (default registry), non system-project skipped",
			obj: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistry.Name},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{
					{ObjectMeta: metav1.ObjectMeta{Name: "p-user", Labels: map[string]string{needsGlobalPrivateRegistryPullSecret: "true"}}},
				}, nil)
			},
			wantKeys: []relatedresource.Key{},
		},
		{
			name: "correct setting (default registry), system project found",
			obj: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistry.Name},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{
					{ObjectMeta: metav1.ObjectMeta{Name: "p-system", Labels: systemProjectLabels}},
				}, nil)
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return([]*corev1.Namespace{
					{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "cattle-fleet-system"}},
				}, nil)
			},
			wantKeys: []relatedresource.Key{
				{Name: "cattle-system"},
				{Name: "cattle-fleet-system"},
			},
		},
		{
			name: "error listing projects (default registry setting)",
			obj: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistry.Name},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return(nil, errDefault)
			},
			setupSettingsController: func(f *fake.MockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList]) {
				f.EXPECT().EnqueueAfter(settings.SystemDefaultRegistryPullSecrets.Name, time.Second*2)
			},
			wantKeys: nil,
			wantErr:  true,
		},
		{
			name: "error listing namespaces, no successful namespaces (default registry setting)",
			obj: &v3.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistry.Name},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().List("c-abc", gomock.Any()).Return([]*v3.Project{
					{ObjectMeta: metav1.ObjectMeta{Name: "p-system", Labels: systemProjectLabels}},
				}, nil)
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return(nil, errDefault)
			},
			setupSettingsController: func(f *fake.MockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList]) {
				f.EXPECT().EnqueueAfter(settings.SystemDefaultRegistryPullSecrets.Name, time.Second*2)
			},
			wantKeys: nil,
			wantErr:  true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectCache := fake.NewMockCacheInterface[*v3.Project](ctrl)
			if tt.setupProjectCache != nil {
				tt.setupProjectCache(projectCache)
			}
			namespaceCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
			if tt.setupNamespaceCache != nil {
				tt.setupNamespaceCache(namespaceCache)
			}
			settingsController := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](ctrl)
			if tt.setupSettingsController != nil {
				tt.setupSettingsController(settingsController)
			}
			n := &namespaceHandler{
				projectCache:          projectCache,
				clusterNamespaceCache: namespaceCache,
				settingsController:    settingsController,
				clusterName:           "c-abc",
			}

			keys, err := n.onSettingEnqueueNamespace("", "", tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("onSettingEnqueueNamespace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(keys, tt.wantKeys) {
				t.Errorf("onSettingEnqueueNamespace() = %v, want %v", keys, tt.wantKeys)
			}
		})
	}
}

func Test_namespaceHandler_onSystemProjectEnqueueNamespace(t *testing.T) {
	systemProjectLabels := map[string]string{
		"authz.management.cattle.io/system-project": "true",
		needsGlobalPrivateRegistryPullSecret:        "true",
	}

	tests := []struct {
		name                string
		obj                 runtime.Object
		setupNamespaceCache func(*fake.MockNonNamespacedCacheInterface[*corev1.Namespace])
		wantKeys            []relatedresource.Key
		wantErr             bool
	}{
		{
			name:     "nil object",
			obj:      nil,
			wantKeys: nil,
		},
		{
			name:     "non-project type",
			obj:      &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s"}},
			wantKeys: nil,
		},
		{
			name: "non-system project is skipped",
			obj: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "p-user",
					Labels: map[string]string{"some-label": "true"},
				},
			},
			wantKeys: nil,
		},
		{
			name: "system project without global pull secret label",
			obj: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-system",
					Labels: map[string]string{
						"authz.management.cattle.io/system-project": "true",
					},
				},
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return([]*corev1.Namespace{
					{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}},
				}, nil)
			},
			wantKeys: []relatedresource.Key{
				{Name: "cattle-system"},
			},
		},
		{
			name: "system project with global pull secret label enqueues namespaces",
			obj: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "p-system",
					Labels: systemProjectLabels,
				},
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return([]*corev1.Namespace{
					{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "cattle-fleet-system"}},
				}, nil)
			},
			wantKeys: []relatedresource.Key{
				{Name: "cattle-system"},
				{Name: "cattle-fleet-system"},
			},
		},
		{
			name: "system project with no namespaces",
			obj: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "p-system",
					Labels: systemProjectLabels,
				},
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return([]*corev1.Namespace{}, nil)
			},
			wantKeys: []relatedresource.Key{},
		},
		{
			name: "error listing namespaces",
			obj: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "p-system",
					Labels: systemProjectLabels,
				},
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return(nil, errDefault)
			},
			wantKeys: nil,
			wantErr:  true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespaceCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
			if tt.setupNamespaceCache != nil {
				tt.setupNamespaceCache(namespaceCache)
			}
			n := &namespaceHandler{
				clusterNamespaceCache: namespaceCache,
			}

			keys, err := n.onSystemProjectEnqueueNamespace("", "", tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("onSystemProjectEnqueueNamespace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(keys, tt.wantKeys) {
				t.Errorf("onSystemProjectEnqueueNamespace() = %v, want %v", keys, tt.wantKeys)
			}
		})
	}
}

func Test_namespaceHandler_getGlobalPullSecrets(t *testing.T) {
	tests := []struct {
		name                 string
		registryURL          string
		pullSecretNames      string
		setupManagementCache func(*fake.MockCacheInterface[*corev1.Secret])
		want                 []*corev1.Secret
		wantErr              bool
	}{
		{
			name:        "no registry configured",
			registryURL: "",
			want:        nil,
		},
		{
			name:            "registry configured but no pull secrets configured",
			registryURL:     "my-registry.example.com",
			pullSecretNames: "",
			want:            nil,
		},
		{
			name:            "error getting secret from management cache",
			registryURL:     "my-registry.example.com",
			pullSecretNames: "pull-secret-1",
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().Get("cattle-system", "pull-secret-1").Return(nil, errDefault)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:            "pull secret configured and found in cache",
			registryURL:     "my-registry.example.com",
			pullSecretNames: "pull-secret-1",
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().Get("cattle-system", "pull-secret-1").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pull-secret-1",
						Namespace: "cattle-system",
						Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
				}, nil)
			},
			want: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pull-secret-1",
						Namespace: "cattle-system",
						Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
				},
			},
		},
		{
			name:            "pull secret with non-dockerconfigjson type is skipped",
			registryURL:     "my-registry.example.com",
			pullSecretNames: "pull-secret-1",
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().Get("cattle-system", "pull-secret-1").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pull-secret-1",
						Namespace: "cattle-system",
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
				}, nil)
			},
			want: nil,
		},
		{
			name:            "mixed types, only .dockerconfigjson secrets are returned",
			registryURL:     "my-registry.example.com",
			pullSecretNames: "opaque-secret,valid-secret",
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().Get("cattle-system", "opaque-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "opaque-secret",
						Namespace: "cattle-system",
						Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
				}, nil)
				f.EXPECT().Get("cattle-system", "valid-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valid-secret",
						Namespace: "cattle-system",
						Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
				}, nil)
			},
			want: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valid-secret",
						Namespace: "cattle-system",
						Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
				},
			},
		},
		{
			name:            "multiple pull secrets configured, all found",
			registryURL:     "my-registry.example.com",
			pullSecretNames: "pull-secret-1,pull-secret-2",
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().Get("cattle-system", "pull-secret-1").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pull-secret-1",
						Namespace: "cattle-system",
						Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
					},
					Type: corev1.SecretTypeDockerConfigJson,
				}, nil)
				f.EXPECT().Get("cattle-system", "pull-secret-2").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pull-secret-2",
						Namespace: "cattle-system",
						Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
					},
					Type: corev1.SecretTypeDockerConfigJson,
				}, nil)
			},
			want: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pull-secret-1",
						Namespace: "cattle-system",
						Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
					},
					Type: corev1.SecretTypeDockerConfigJson,
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pull-secret-2",
						Namespace: "cattle-system",
						Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
					},
					Type: corev1.SecretTypeDockerConfigJson,
				},
			},
		},
		{
			name:            "pull secret configured but not found in cache, skipped",
			registryURL:     "my-registry.example.com",
			pullSecretNames: "missing-secret",
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().Get("cattle-system", "missing-secret").Return(nil, errNotFound)
			},
			want: nil,
		},
		{
			name:            "one secret found and one not found, only found secret returned",
			registryURL:     "my-registry.example.com",
			pullSecretNames: "exists,missing",
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().Get("cattle-system", "exists").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "exists",
						Namespace: "cattle-system",
					},
					Type: corev1.SecretTypeDockerConfigJson,
				}, nil)
				f.EXPECT().Get("cattle-system", "missing").Return(nil, errNotFound)
			},
			want: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "exists",
						Namespace: "cattle-system",
						Labels:    map[string]string{cluster.CopiedPullSecretLabel: "true"},
					},
					Type: corev1.SecretTypeDockerConfigJson,
				},
			},
		},
	}

	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the settings for the test — when provider is nil, Set modifies the default in the settings map.
			origRegistry := settings.SystemDefaultRegistry.Get()
			origPullSecrets := settings.SystemDefaultRegistryPullSecrets.Get()
			t.Cleanup(func() {
				settings.SystemDefaultRegistry.Set(origRegistry)
				settings.SystemDefaultRegistryPullSecrets.Set(origPullSecrets)
			})
			settings.SystemDefaultRegistry.Set(tt.registryURL)
			settings.SystemDefaultRegistryPullSecrets.Set(tt.pullSecretNames)

			managementSecretCacheMock := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			if tt.setupManagementCache != nil {
				tt.setupManagementCache(managementSecretCacheMock)
			}

			n := &namespaceHandler{
				managementSecretCache: managementSecretCacheMock,
			}

			got, err := n.getGlobalPullSecrets()
			if (err != nil) != tt.wantErr {
				t.Errorf("getGlobalPullSecrets() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getGlobalPullSecrets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_namespaceHandler_OnChange(t *testing.T) {
	const (
		testClusterName = "c-test"
		testProjectName = "p-test"
		testBackingNS   = "c-test-p-test"
		testNamespace   = "my-namespace"
	)

	systemProjectLabels := map[string]string{
		"authz.management.cattle.io/system-project": "true",
		needsGlobalPrivateRegistryPullSecret:        "true",
	}

	tests := []struct {
		name                  string
		namespace             *corev1.Namespace
		registryURL           string
		pullSecretNames       string
		setupProjectCache     func(*fake.MockCacheInterface[*v3.Project])
		setupManagementCache  func(*fake.MockCacheInterface[*corev1.Secret])
		setupManagementClient func(*fake.MockClientInterface[*corev1.Secret, *corev1.SecretList])
		setupSecretClient     func(*fake.MockClientInterface[*corev1.Secret, *corev1.SecretList])
		wantNil               bool
		wantErr               bool
	}{
		{
			name: "system project with global pull secrets — secrets are created",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testNamespace,
					Annotations: map[string]string{projectIDLabel: testClusterName + ":" + testProjectName},
				},
			},
			registryURL:     "my-registry.example.com",
			pullSecretNames: "global-pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get(testClusterName, testProjectName).Return(&v3.Project{
					ObjectMeta: metav1.ObjectMeta{Name: testProjectName, Labels: systemProjectLabels},
					Spec:       v3.ProjectSpec{ClusterName: testClusterName},
					Status:     v3.ProjectStatus{BackingNamespace: testBackingNS},
				}, nil)
			},
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().Get("cattle-system", "global-pull-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "global-pull-secret",
						Namespace: "cattle-system",
						Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().Get(testNamespace, "global-pull-secret", gomock.Any()).Return(nil, errNotFound)
				f.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "global-pull-secret", s.Name)
					assert.Equal(t, testNamespace, s.Namespace)
					assert.Equal(t, "true", s.Annotations[pssCopyAnnotation])
					assert.Equal(t, "true", s.Labels[cluster.CopiedPullSecretLabel])
					return s, nil
				})
				f.EXPECT().List(testNamespace, metav1.ListOptions{LabelSelector: ProjectScopedSecretLabel}).Return(&corev1.SecretList{Items: []corev1.Secret{}}, nil)
				f.EXPECT().List(testNamespace, metav1.ListOptions{LabelSelector: cluster.CopiedPullSecretLabel}).Return(&corev1.SecretList{
					Items: []corev1.Secret{
						{ObjectMeta: metav1.ObjectMeta{Name: "global-pull-secret", Namespace: testNamespace}},
					},
				}, nil)
			},
		},
		{
			name: "system project without global secrets label — global pull secrets not copied",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testNamespace,
					Annotations: map[string]string{projectIDLabel: testClusterName + ":" + testProjectName},
				},
			},
			registryURL:     "my-registry.example.com",
			pullSecretNames: "global-pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get(testClusterName, testProjectName).Return(&v3.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name: testProjectName,
						Labels: map[string]string{
							"authz.management.cattle.io/system-project": "true",
							// needsGlobalPrivateRegistryPullSecret is NOT set
						},
					},
					Spec:   v3.ProjectSpec{ClusterName: testClusterName},
					Status: v3.ProjectStatus{BackingNamespace: testBackingNS},
				}, nil)
			},
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				// getGlobalPullSecrets should NOT be called
			},
			setupSecretClient: func(f *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().List(testNamespace, metav1.ListOptions{LabelSelector: ProjectScopedSecretLabel}).Return(&corev1.SecretList{Items: []corev1.Secret{}}, nil)
				f.EXPECT().List(testNamespace, metav1.ListOptions{LabelSelector: cluster.CopiedPullSecretLabel}).Return(&corev1.SecretList{Items: []corev1.Secret{}}, nil)
			},
		},
		{
			name: "system project with both PSS and global pull secrets",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testNamespace,
					Annotations: map[string]string{projectIDLabel: testClusterName + ":" + testProjectName},
				},
			},
			registryURL:     "my-registry.example.com",
			pullSecretNames: "global-pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get(testClusterName, testProjectName).Return(&v3.Project{
					ObjectMeta: metav1.ObjectMeta{Name: testProjectName, Labels: systemProjectLabels},
					Spec:       v3.ProjectSpec{ClusterName: testClusterName},
					Status:     v3.ProjectStatus{BackingNamespace: testBackingNS},
				}, nil)
			},
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				// migrateExistingProjectScopedSecrets
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				// getProjectScopedSecretsFromNamespace
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pss-secret",
							Namespace: testBackingNS,
							Labels:    map[string]string{ProjectScopedSecretLabel: testProjectName},
						},
						Data: map[string][]byte{"key": []byte("value")},
					},
				}, nil)
				// getGlobalPullSecrets
				f.EXPECT().Get("cattle-system", "global-pull-secret").Return(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "global-pull-secret",
						Namespace: "cattle-system",
						Labels:    map[string]string{cluster.SourcePullSecretLabel: "true"},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
				}, nil)
			},
			setupSecretClient: func(f *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]) {
				// CreateOrUpdateNamespacedResource for PSS secret
				f.EXPECT().Get(testNamespace, "pss-secret", gomock.Any()).Return(nil, errNotFound)
				f.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "pss-secret", s.Name)
					assert.Equal(t, testNamespace, s.Namespace)
					return s, nil
				})
				// CreateOrUpdateNamespacedResource for global pull secret
				f.EXPECT().Get(testNamespace, "global-pull-secret", gomock.Any()).Return(nil, errNotFound)
				f.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "global-pull-secret", s.Name)
					assert.Equal(t, testNamespace, s.Namespace)
					return s, nil
				})
				// removeUndesiredProjectScopedSecrets
				f.EXPECT().List(testNamespace, metav1.ListOptions{LabelSelector: ProjectScopedSecretLabel}).Return(&corev1.SecretList{
					Items: []corev1.Secret{
						{ObjectMeta: metav1.ObjectMeta{Name: "pss-secret", Namespace: testNamespace, Annotations: map[string]string{pssCopyAnnotation: "true"}}},
					},
				}, nil)
				f.EXPECT().List(testNamespace, metav1.ListOptions{LabelSelector: cluster.CopiedPullSecretLabel}).Return(&corev1.SecretList{
					Items: []corev1.Secret{
						{ObjectMeta: metav1.ObjectMeta{Name: "global-pull-secret", Namespace: testNamespace}},
					},
				}, nil)
			},
		},
		{
			name: "stale global pull secret is removed when registry is unconfigured",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testNamespace,
					Annotations: map[string]string{projectIDLabel: testClusterName + ":" + testProjectName},
				},
			},
			registryURL:     "",
			pullSecretNames: "",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get(testClusterName, testProjectName).Return(&v3.Project{
					ObjectMeta: metav1.ObjectMeta{Name: testProjectName, Labels: systemProjectLabels},
					Spec:       v3.ProjectSpec{ClusterName: testClusterName},
					Status:     v3.ProjectStatus{BackingNamespace: testBackingNS},
				}, nil)
			},
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
			},
			setupSecretClient: func(f *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().List(testNamespace, metav1.ListOptions{LabelSelector: ProjectScopedSecretLabel}).Return(&corev1.SecretList{Items: []corev1.Secret{}}, nil)
				f.EXPECT().List(testNamespace, metav1.ListOptions{LabelSelector: cluster.CopiedPullSecretLabel}).Return(&corev1.SecretList{
					Items: []corev1.Secret{
						{ObjectMeta: metav1.ObjectMeta{Name: "stale-global-pull-secret", Namespace: testNamespace}},
					},
				}, nil)
				f.EXPECT().Delete(testNamespace, "stale-global-pull-secret", &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name: "non-system project — global pull secrets not fetched",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testNamespace,
					Annotations: map[string]string{projectIDLabel: testClusterName + ":" + testProjectName},
				},
			},
			registryURL:     "my-registry.example.com",
			pullSecretNames: "global-pull-secret",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get(testClusterName, testProjectName).Return(&v3.Project{
					ObjectMeta: metav1.ObjectMeta{Name: testProjectName},
					Spec:       v3.ProjectSpec{ClusterName: testClusterName},
					Status:     v3.ProjectStatus{BackingNamespace: testBackingNS},
				}, nil)
			},
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				f.EXPECT().List(testBackingNS, gomock.Any()).Return([]*corev1.Secret{}, nil)
				// getGlobalPullSecrets should NOT be called — project is not a system project
			},
			setupSecretClient: func(f *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().List(testNamespace, metav1.ListOptions{LabelSelector: ProjectScopedSecretLabel}).Return(&corev1.SecretList{Items: []corev1.Secret{}}, nil)
				f.EXPECT().List(testNamespace, metav1.ListOptions{LabelSelector: cluster.CopiedPullSecretLabel}).Return(&corev1.SecretList{Items: []corev1.Secret{}}, nil)
			},
		},
	}

	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origRegistry := settings.SystemDefaultRegistry.Get()
			origPullSecrets := settings.SystemDefaultRegistryPullSecrets.Get()
			t.Cleanup(func() {
				settings.SystemDefaultRegistry.Set(origRegistry)
				settings.SystemDefaultRegistryPullSecrets.Set(origPullSecrets)
			})
			settings.SystemDefaultRegistry.Set(tt.registryURL)
			settings.SystemDefaultRegistryPullSecrets.Set(tt.pullSecretNames)

			projectCache := fake.NewMockCacheInterface[*v3.Project](ctrl)
			if tt.setupProjectCache != nil {
				tt.setupProjectCache(projectCache)
			}
			managementSecretCacheMock := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			if tt.setupManagementCache != nil {
				tt.setupManagementCache(managementSecretCacheMock)
			}
			managementSecretClientMock := fake.NewMockClientInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			if tt.setupManagementClient != nil {
				tt.setupManagementClient(managementSecretClientMock)
			}
			secretClient := fake.NewMockClientInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			if tt.setupSecretClient != nil {
				tt.setupSecretClient(secretClient)
			}

			n := &namespaceHandler{
				clusterName:            testClusterName,
				projectCache:           projectCache,
				managementSecretCache:  managementSecretCacheMock,
				managementSecretClient: managementSecretClientMock,
				secretClient:           secretClient,
			}

			result, err := n.OnChange("", tt.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("OnChange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantNil {
				assert.Nil(t, result)
			}
		})
	}
}

package secret

import (
	"fmt"
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	errDefault  = fmt.Errorf("error")
	errNotFound = apierrors.NewNotFound(schema.GroupResource{}, "error")
)

func Test_namespaceHandler_getNamespacesFromSecret(t *testing.T) {
	tests := []struct {
		name                string
		secret              *corev1.Secret
		clusterName         string
		setupProjectCache   func(*fake.MockCacheInterface[*v3.Project])
		setupNamespaceCache func(*fake.MockNonNamespacedCacheInterface[*corev1.Namespace])
		want                []*corev1.Namespace
		wantErr             bool
	}{
		{
			name: "secret has no labels",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-secret",
					Labels: nil,
				},
			},
			want: nil,
		},
		{
			name: "secret has no project scoped secret label",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-secret",
					Labels: map[string]string{
						"fake-label": "fake-value",
					},
				},
			},
			want: nil,
		},
		{
			name: "project not in right cluster",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-secret",
					Labels: map[string]string{
						ProjectScopedSecretLabel: "p-abc",
					},
				},
			},
			clusterName: "c-abc",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get("c-abc", "p-abc").Return(nil, errNotFound)
			},
			want: nil,
		},
		{
			name: "error getting project",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-secret",
					Labels: map[string]string{
						ProjectScopedSecretLabel: "p-abc",
					},
				},
			},
			clusterName: "c-abc",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get("c-abc", "p-abc").Return(nil, errDefault)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "secret not in project backing namespace",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "wrong-namespace",
					Labels: map[string]string{
						ProjectScopedSecretLabel: "p-abc",
					},
				},
			},
			clusterName: "c-abc",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get("c-abc", "p-abc").Return(&v3.Project{
					Status: v3.ProjectStatus{
						BackingNamespace: "c-xyz",
					},
				}, nil)
			},
			want: nil,
		},
		{
			name: "error listing namespaces",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "c-xyz",
					Labels: map[string]string{
						ProjectScopedSecretLabel: "p-abc",
					},
				},
			},
			clusterName: "c-xyz",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get("c-xyz", "p-abc").Return(&v3.Project{
					Status: v3.ProjectStatus{
						BackingNamespace: "c-xyz",
					},
				}, nil)
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return(nil, errDefault)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "return list of namespaces",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "c-xyz",
					Labels: map[string]string{
						ProjectScopedSecretLabel: "p-abc",
					},
				},
			},
			clusterName: "c-xyz",
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get("c-xyz", "p-abc").Return(&v3.Project{
					Status: v3.ProjectStatus{
						BackingNamespace: "c-xyz",
					},
				}, nil)
			},
			setupNamespaceCache: func(f *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]) {
				f.EXPECT().List(gomock.Any()).Return([]*corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "namespace"}}}, nil)
			},
			want: []*corev1.Namespace{
				{ObjectMeta: metav1.ObjectMeta{Name: "namespace"}},
			},
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
				clusterNamespaceCache: namespaceCache,
				projectCache:          projectCache,
				clusterName:           tt.clusterName,
			}
			got, err := n.getNamespacesFromSecret(tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("namespaceHandler.getNamespacesFromSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("namespaceHandler.getNamespacesFromSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_namespaceHandler_getProjectScopedSecretsFromNamespace(t *testing.T) {
	tests := []struct {
		name                 string
		project              *v3.Project
		setupManagementCache func(*fake.MockCacheInterface[*corev1.Secret])
		wantSecrets          []*corev1.Secret
		wantErr              bool
	}{
		{
			name: "successfully retrieve project scoped secrets",
			project: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{Name: "p-test"},
				Status:     v3.ProjectStatus{BackingNamespace: "c-abc123-p-test"},
			},
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				expectedSelector, _ := labels.NewRequirement(ProjectScopedSecretLabel, selection.Equals, []string{"p-test"})
				f.EXPECT().List("c-abc123-p-test", labels.NewSelector().Add(*expectedSelector)).Return([]*corev1.Secret{
					{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "c-abc123-p-test"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: "c-abc123-p-test"}},
				}, nil)
			},
			wantSecrets: []*corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "c-abc123-p-test"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: "c-abc123-p-test"}},
			},
			wantErr: false,
		},
		{
			name: "no secrets found",
			project: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{Name: "p-test"},
				Status:     v3.ProjectStatus{BackingNamespace: "c-abc123-p-test"},
			},
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				expectedSelector, _ := labels.NewRequirement(ProjectScopedSecretLabel, selection.Equals, []string{"p-test"})
				f.EXPECT().List("c-abc123-p-test", labels.NewSelector().Add(*expectedSelector)).Return([]*corev1.Secret{}, nil)
			},
			wantSecrets: []*corev1.Secret{},
			wantErr:     false,
		},
		{
			name: "error listing secrets from cache",
			project: &v3.Project{
				ObjectMeta: metav1.ObjectMeta{Name: "p-test"},
				Status:     v3.ProjectStatus{BackingNamespace: "c-abc123-p-test"},
			},
			setupManagementCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				expectedSelector, _ := labels.NewRequirement(ProjectScopedSecretLabel, selection.Equals, []string{"p-test"})
				f.EXPECT().List("c-abc123-p-test", labels.NewSelector().Add(*expectedSelector)).Return(nil, errDefault)
			},
			wantSecrets: nil,
			wantErr:     true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			managementSecretCacheMock := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			if tt.setupManagementCache != nil {
				tt.setupManagementCache(managementSecretCacheMock)
			}

			n := &namespaceHandler{
				managementSecretCache: managementSecretCacheMock,
			}

			gotSecrets, err := n.getProjectScopedSecretsFromNamespace(tt.project)

			if (err != nil) != tt.wantErr {
				t.Errorf("namespaceHandler.getProjectScopedSecretsFromNamespace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotSecrets, tt.wantSecrets) {
				t.Errorf("namespaceHandler.getProjectScopedSecretsFromNamespace() gotSecrets = %v, want %v", gotSecrets, tt.wantSecrets)
			}
		})
	}
}

const (
	projectName = "p-testproject"
	clusterName = "c-testcluster"
	backingNS   = "test-backing-ns"
)

var (
	secretToMigrate = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s1",
			Namespace: backingNS,
			Labels:    map[string]string{normanCreatorLabel: "norman", "otherLabel": "val"},
			Annotations: map[string]string{
				oldPSSAnnotation + clusterName: "true",
				"otherAnno":                    "val",
			},
			Finalizers: []string{oldPSSFinalizer + clusterName, "otherFinalizer"},
		},
	}
	migratedSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s1",
			Namespace: backingNS,
			Labels: map[string]string{
				"otherLabel":             "val",
				ProjectScopedSecretLabel: projectName,
			},
			Annotations: map[string]string{
				"otherAnno": "val",
			},
			Finalizers: []string{"otherFinalizer"},
		},
	}

	testProject = &v3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: projectName},
		Spec:       v3.ProjectSpec{ClusterName: clusterName},
		Status:     v3.ProjectStatus{BackingNamespace: backingNS},
	}
)

func Test_namespaceHandler_migrateExistingProjectScopedSecrets(t *testing.T) {
	ctrl := gomock.NewController(t)

	validSelectorReq, _ := labels.NewRequirement(normanCreatorLabel, selection.Equals, []string{"norman"})
	validSelector := labels.NewSelector().Add(*validSelectorReq)

	tests := []struct {
		name                  string
		project               *v3.Project
		setupManagementCache  func(m *fake.MockCacheInterface[*corev1.Secret])
		setupManagementClient func(m *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList], t *testing.T, project *v3.Project)
		wantErr               bool
	}{
		{
			name:    "no secrets found with norman label",
			project: testProject,
			setupManagementCache: func(m *fake.MockCacheInterface[*corev1.Secret]) {
				m.EXPECT().List(backingNS, validSelector).Return([]*corev1.Secret{}, nil)
			},
			wantErr: false,
		},
		{
			name:    "error listing secrets from cache",
			project: testProject,
			setupManagementCache: func(m *fake.MockCacheInterface[*corev1.Secret]) {
				m.EXPECT().List(backingNS, validSelector).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name:    "one migrated secret",
			project: testProject,
			setupManagementCache: func(m *fake.MockCacheInterface[*corev1.Secret]) {
				m.EXPECT().List(backingNS, validSelector).Return([]*corev1.Secret{secretToMigrate}, nil)
			},
			setupManagementClient: func(m *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList], t *testing.T, project *v3.Project) {
				m.EXPECT().Update(migratedSecret).Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name:    "multiple migrated secrets",
			project: testProject,
			setupManagementCache: func(m *fake.MockCacheInterface[*corev1.Secret]) {
				m.EXPECT().List(backingNS, validSelector).Return([]*corev1.Secret{secretToMigrate.DeepCopy(), secretToMigrate.DeepCopy()}, nil)
			},
			setupManagementClient: func(m *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList], t *testing.T, project *v3.Project) {
				m.EXPECT().Update(migratedSecret).Return(nil, nil).Times(2)
			},
			wantErr: false,
		},
		{
			name:    "secret update fails",
			project: testProject,
			setupManagementCache: func(m *fake.MockCacheInterface[*corev1.Secret]) {
				m.EXPECT().List(backingNS, validSelector).Return([]*corev1.Secret{secretToMigrate}, nil)
			},
			setupManagementClient: func(m *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList], t *testing.T, project *v3.Project) {
				m.EXPECT().Update(migratedSecret).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name:    "multiple migrated secrets, failures not blocking",
			project: testProject,
			setupManagementCache: func(m *fake.MockCacheInterface[*corev1.Secret]) {
				m.EXPECT().List(backingNS, validSelector).Return([]*corev1.Secret{secretToMigrate.DeepCopy(), secretToMigrate.DeepCopy()}, nil)
			},
			setupManagementClient: func(m *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList], t *testing.T, project *v3.Project) {
				m.EXPECT().Update(migratedSecret).Return(nil, errDefault).Times(2)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			managementSecretCacheMock := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			managementSecretClientMock := fake.NewMockClientInterface[*corev1.Secret, *corev1.SecretList](ctrl)

			if tt.setupManagementCache != nil {
				tt.setupManagementCache(managementSecretCacheMock)
			}
			if tt.setupManagementClient != nil {
				tt.setupManagementClient(managementSecretClientMock, t, tt.project)
			}

			n := &namespaceHandler{
				managementSecretCache:  managementSecretCacheMock,
				managementSecretClient: managementSecretClientMock,
			}

			err := n.migrateExistingProjectScopedSecrets(tt.project)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_namespaceHandler_getProjectFromNamespace(t *testing.T) {
	tests := []struct {
		name              string
		namespace         *corev1.Namespace
		setupProjectCache func(*fake.MockCacheInterface[*v3.Project])
		want              *v3.Project
		wantErr           bool
	}{
		{
			name: "namespace is not part of a project",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fake-label": "fake-value",
					},
				},
			},
			want: nil,
		},
		{
			name: "projectID is malformed",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						projectIDLabel: "c-abc123-p-abc123",
					},
				},
			},
			want: nil,
		},
		{
			name: "error getting project",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						projectIDLabel: "c-abc123:p-abc123",
					},
				},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get("c-abc123", "p-abc123").Return(nil, errDefault)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "project not found",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						projectIDLabel: "c-abc123:p-abc123",
					},
				},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get("c-abc123", "p-abc123").Return(nil, errNotFound)
			},
			want: nil,
		},
		{
			name: "get a project",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						projectIDLabel: "c-abc123:p-abc123",
					},
				},
			},
			setupProjectCache: func(f *fake.MockCacheInterface[*v3.Project]) {
				f.EXPECT().Get("c-abc123", "p-abc123").Return(&v3.Project{
					Status: v3.ProjectStatus{
						BackingNamespace: "c-abc123-p-abc123",
					},
				}, nil)
			},
			want: &v3.Project{
				Status: v3.ProjectStatus{
					BackingNamespace: "c-abc123-p-abc123",
				},
			},
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectCache := fake.NewMockCacheInterface[*v3.Project](ctrl)
			if tt.setupProjectCache != nil {
				tt.setupProjectCache(projectCache)
			}
			n := &namespaceHandler{
				projectCache: projectCache,
			}
			got, err := n.getProjectFromNamespace(tt.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("namespaceHandler.getProjectScopedSecretsFromNamespace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("namespaceHandler.getProjectScopedSecretsFromNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_namespaceHandler_removeUndesiredProjectScopedSecrets(t *testing.T) {
	type args struct {
		namespace      *corev1.Namespace
		desiredSecrets sets.Set[types.NamespacedName]
	}
	tests := []struct {
		name              string
		args              args
		setupSecretClient func(*fake.MockClientInterface[*corev1.Secret, *corev1.SecretList])
		wantErr           bool
	}{
		{
			name: "error getting secrets",
			args: args{
				namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
				desiredSecrets: sets.Set[types.NamespacedName]{
					{Name: "secret1", Namespace: "ns1"}: {},
					{Name: "secret2", Namespace: "ns1"}: {},
				},
			},
			setupSecretClient: func(f *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().List("ns1", metav1.ListOptions{LabelSelector: ProjectScopedSecretLabel}).Return(&corev1.SecretList{
					Items: []corev1.Secret{},
				}, errDefault)
			},
			wantErr: true,
		},
		{
			name: "desired secrets match existing secrets, no deletion",
			args: args{
				namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
				desiredSecrets: sets.Set[types.NamespacedName]{
					{Name: "secret1", Namespace: "ns1"}: {},
					{Name: "secret2", Namespace: "ns1"}: {},
				},
			},
			setupSecretClient: func(f *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().List("ns1", metav1.ListOptions{LabelSelector: ProjectScopedSecretLabel}).Return(&corev1.SecretList{
					Items: []corev1.Secret{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "secret1",
								Namespace:   "ns1",
								Annotations: map[string]string{pssCopyAnnotation: "true"},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "secret2",
								Namespace:   "ns1",
								Annotations: map[string]string{pssCopyAnnotation: "true"},
							},
						},
					},
				}, nil)
			},
		},
		{
			name: "no undesired secrets",
			args: args{
				namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
				desiredSecrets: sets.Set[types.NamespacedName]{
					{Name: "secret1", Namespace: "ns1"}: {},
					{Name: "secret2", Namespace: "ns1"}: {},
				},
			},
			setupSecretClient: func(f *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().List("ns1", metav1.ListOptions{LabelSelector: ProjectScopedSecretLabel}).Return(&corev1.SecretList{
					Items: []corev1.Secret{},
				}, nil)
			},
		},
		{
			name: "remove undesired secrets",
			args: args{
				namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
				desiredSecrets: sets.Set[types.NamespacedName]{
					{Name: "secret1", Namespace: "ns1"}: {},
				},
			},
			setupSecretClient: func(f *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().List("ns1", metav1.ListOptions{LabelSelector: ProjectScopedSecretLabel}).Return(&corev1.SecretList{
					Items: []corev1.Secret{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "secret1",
								Namespace:   "ns1",
								Annotations: map[string]string{pssCopyAnnotation: "true"},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "secret2",
								Namespace:   "ns1",
								Annotations: map[string]string{pssCopyAnnotation: "true"},
							},
						},
					},
				}, nil)
				f.EXPECT().Delete("ns1", "secret2", &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name: "remove multiple secrets",
			args: args{
				namespace:      &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
				desiredSecrets: sets.Set[types.NamespacedName]{},
			},
			setupSecretClient: func(f *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().List("ns1", metav1.ListOptions{LabelSelector: ProjectScopedSecretLabel}).Return(&corev1.SecretList{
					Items: []corev1.Secret{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "secret1",
								Namespace:   "ns1",
								Annotations: map[string]string{pssCopyAnnotation: "true"},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "secret2",
								Namespace:   "ns1",
								Annotations: map[string]string{pssCopyAnnotation: "true"},
							},
						},
					},
				}, nil)
				f.EXPECT().Delete("ns1", "secret1", &metav1.DeleteOptions{}).Return(nil)
				f.EXPECT().Delete("ns1", "secret2", &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name: "error deleting secrets",
			args: args{
				namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
				desiredSecrets: sets.Set[types.NamespacedName]{
					{Name: "secret1", Namespace: "ns1"}: {},
				},
			},
			setupSecretClient: func(f *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]) {
				f.EXPECT().List("ns1", metav1.ListOptions{LabelSelector: ProjectScopedSecretLabel}).Return(&corev1.SecretList{
					Items: []corev1.Secret{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "secret1",
								Namespace:   "ns1",
								Annotations: map[string]string{pssCopyAnnotation: "true"},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "secret2",
								Namespace:   "ns1",
								Annotations: map[string]string{pssCopyAnnotation: "true"},
							},
						},
					},
				}, nil)
				f.EXPECT().Delete("ns1", "secret2", &metav1.DeleteOptions{}).Return(errDefault)
			},
			wantErr: true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secretClient := fake.NewMockClientInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			if tt.setupSecretClient != nil {
				tt.setupSecretClient(secretClient)
			}
			n := &namespaceHandler{
				secretClient: secretClient,
			}
			if err := n.removeUndesiredProjectScopedSecrets(tt.args.namespace, tt.args.desiredSecrets); (err != nil) != tt.wantErr {
				t.Errorf("namespaceHandler.removeUndesiredProjectScopedSecrets() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

package secret

import (
	"fmt"
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
						projectScopedSecretLabel: "p-abc",
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
						projectScopedSecretLabel: "p-abc",
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
						projectScopedSecretLabel: "p-abc",
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
						projectScopedSecretLabel: "p-abc",
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
						projectScopedSecretLabel: "p-abc",
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
		name              string
		namespace         *corev1.Namespace
		setupProjectCache func(*fake.MockCacheInterface[*v3.Project])
		setupSecretCache  func(*fake.MockCacheInterface[*corev1.Secret])
		want              []*corev1.Secret
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
			name: "error listing secrets",
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
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List("c-abc123-p-abc123", gomock.Any()).Return(nil, errDefault)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "get list of secrets",
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
			setupSecretCache: func(f *fake.MockCacheInterface[*corev1.Secret]) {
				f.EXPECT().List("c-abc123-p-abc123", gomock.Any()).Return([]*corev1.Secret{
					{ObjectMeta: metav1.ObjectMeta{Name: "secret"}},
				}, nil)
			},
			want: []*corev1.Secret{{ObjectMeta: metav1.ObjectMeta{Name: "secret"}}},
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectCache := fake.NewMockCacheInterface[*v3.Project](ctrl)
			if tt.setupProjectCache != nil {
				tt.setupProjectCache(projectCache)
			}
			secretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			if tt.setupSecretCache != nil {
				tt.setupSecretCache(secretCache)
			}
			n := &namespaceHandler{
				managementSecretCache: secretCache,
				projectCache:          projectCache,
			}
			got, err := n.getProjectScopedSecretsFromNamespace(tt.namespace)
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
				f.EXPECT().List("ns1", metav1.ListOptions{LabelSelector: projectScopedSecretLabel}).Return(&corev1.SecretList{
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
				f.EXPECT().List("ns1", metav1.ListOptions{LabelSelector: projectScopedSecretLabel}).Return(&corev1.SecretList{
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
				f.EXPECT().List("ns1", metav1.ListOptions{LabelSelector: projectScopedSecretLabel}).Return(&corev1.SecretList{
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
				f.EXPECT().List("ns1", metav1.ListOptions{LabelSelector: projectScopedSecretLabel}).Return(&corev1.SecretList{
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
				f.EXPECT().List("ns1", metav1.ListOptions{LabelSelector: projectScopedSecretLabel}).Return(&corev1.SecretList{
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
				f.EXPECT().List("ns1", metav1.ListOptions{LabelSelector: projectScopedSecretLabel}).Return(&corev1.SecretList{
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

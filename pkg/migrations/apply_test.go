package migrations

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/rancher/rancher/pkg/migrations/changes"
	"github.com/rancher/rancher/pkg/migrations/test"
)

var testSecretData = map[string][]byte{"testing": []byte("TESTSECRET")}

func TestApply(t *testing.T) {
	t.Run("unknown migration", func(t *testing.T) {
		clientset := fake.NewClientset()

		metrics, err := Apply(context.TODO(), "test-migration", NewStatusClient(clientset.CoreV1()), newFakeDynamicClient(t), changes.ApplyOptions{}, nil)
		require.ErrorContains(t, err, `unknown migration "test-migration"`)

		require.Nil(t, metrics)
	})

	t.Run("failing migration", func(t *testing.T) {
		Register(testFailingMigration{})
		t.Cleanup(func() {
			knownMigrations = nil
		})
		clientset := fake.NewClientset()

		metrics, err := Apply(context.TODO(), "test-failing-migration", NewStatusClient(clientset.CoreV1()), newFakeDynamicClient(t), changes.ApplyOptions{}, nil)

		require.ErrorContains(t, err, `calculating changes for migration "test-failing-migration": this is a failing migration`)

		require.Nil(t, metrics)
	})

	t.Run("with registered migration", func(t *testing.T) {
		Register(testMigration{})
		t.Cleanup(func() {
			knownMigrations = nil
		})

		svc := test.NewService(func(s *corev1.Service) {
			s.Spec.Ports[0].TargetPort = intstr.FromInt(8000)
		})

		t.Run("the migration is applied and resources are patched", func(t *testing.T) {
			clientset := fake.NewClientset()
			dynamicset := newFakeDynamicClient(t, svc)

			// This passes nil as a mapper because we are not creating new
			// resources in this test migration.
			metrics, err := Apply(context.TODO(), "test-migration", NewStatusClient(clientset.CoreV1()), dynamicset, changes.ApplyOptions{}, nil)
			require.NoError(t, err)

			wantSpec := corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "http-80",
						Protocol:   corev1.ProtocolTCP,
						Port:       80,
						TargetPort: intstr.FromInt(9371)},
				},
			}

			updatedSvc := loadService(t, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, dynamicset)
			require.Equal(t, wantSpec, updatedSvc.Spec)

			wantMetrics := &changes.ApplyMetrics{Patch: 1}
			if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
				t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
			}
		})

		t.Run("the migration is applied and is recorded", func(t *testing.T) {
			clientset := fake.NewClientset()
			dynamicset := newFakeDynamicClient(t, svc)
			statusClient := NewStatusClient(clientset.CoreV1())

			// This passes nil as a mapper because we are not creating new
			// resources in this test migration.
			metrics, err := Apply(context.TODO(), "test-migration", statusClient, dynamicset, changes.ApplyOptions{}, nil)
			require.NoError(t, err)

			wantStatus := &MigrationStatus{
				AppliedAt: time.Now(),
				Metrics: &changes.ApplyMetrics{
					Patch: 1,
				},
			}
			status, err := statusClient.StatusFor(context.TODO(), "test-migration")
			assert.EqualExportedValues(t, wantStatus, status)

			wantMetrics := &changes.ApplyMetrics{Patch: 1}
			if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
				t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
			}
		})
	})

	t.Run("errors applying migrations are recorded", func(t *testing.T) {
		Register(testCreateMigration{t: t})
		t.Cleanup(func() {
			knownMigrations = nil
		})

		clientset := fake.NewClientset()
		existingSecret := newSecret(types.NamespacedName{Name: "test-secret", Namespace: "cattle-secrets"}, testSecretData)
		dynamicset := newFakeDynamicClient(t, existingSecret)
		statusClient := NewStatusClient(clientset.CoreV1())
		mapper := test.NewFakeMapper()

		metrics, err := Apply(context.TODO(), "test-create-migration", statusClient, dynamicset, changes.ApplyOptions{}, mapper)
		require.ErrorContains(t, err, `secrets "test-secret" already exists`)

		wantMetrics := &changes.ApplyMetrics{Create: 1, Errors: 1}
		if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
			t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
		}

		wantStatus := &MigrationStatus{
			AppliedAt: time.Now(),
			Metrics: &changes.ApplyMetrics{
				Create: 1,
				Errors: 1,
			},
			Errors: `failed to apply Create change - creating resource: secrets "test-secret" already exists`,
		}
		status, err := statusClient.StatusFor(context.TODO(), "test-create-migration")
		assert.EqualExportedValues(t, wantStatus, status)
	})

	t.Run("with failing migration calculation", func(t *testing.T) {
		Register(testFailingMigration{})
		t.Cleanup(func() {
			knownMigrations = nil
		})
		clientset := fake.NewClientset()
		dynamicset := newFakeDynamicClient(t)
		statusClient := NewStatusClient(clientset.CoreV1())

		_, err := Apply(context.TODO(), "test-failing-migration", statusClient, dynamicset, changes.ApplyOptions{}, test.NewFakeMapper())
		require.ErrorContains(t, err, "this is a failing migration")

		status, err := statusClient.StatusFor(context.TODO(), "test-failing-migration")
		assert.EqualExportedValues(t, &MigrationStatus{Errors: "this is a failing migration"}, status)
	})

	t.Run("apply all batches of migrations", func(t *testing.T) {
		Register(example.batchedMigration{})
		t.Cleanup(func() {
			knownMigrations = nil
		})
		clientset := fake.NewClientset()
		dynamicset := newFakeDynamicClient(t)
		statusClient := NewStatusClient(clientset.CoreV1())

		_, err := Apply(context.TODO(), "test-failing-migration", statusClient, dynamicset, changes.ApplyOptions{}, test.NewFakeMapper())
		require.ErrorContains(t, err, "this is a failing migration")

		status, err := statusClient.StatusFor(context.TODO(), "test-failing-migration")
		assert.EqualExportedValues(t, &MigrationStatus{Errors: "this is a failing migration"}, status)
	})

}

func TestApplyUnappliedMigrations(t *testing.T) {
	t.Run("no migrations registered", func(t *testing.T) {
		clientset := fake.NewClientset()

		metrics, err := ApplyUnappliedMigrations(context.TODO(), NewStatusClient(clientset.CoreV1()), newFakeDynamicClient(t), changes.ApplyOptions{}, nil)
		require.NoError(t, err)

		require.Equal(t, map[string]*changes.ApplyMetrics{}, metrics)
	})

	t.Run("with registered migration", func(t *testing.T) {
		Register(testMigration{})
		t.Cleanup(func() {
			knownMigrations = nil
		})
		svc := test.NewService(func(s *corev1.Service) {
			s.Spec.Ports[0].TargetPort = intstr.FromInt(8000)
		})
		clientset := fake.NewClientset()
		dynamicset := newFakeDynamicClient(t, svc)

		metrics, err := ApplyUnappliedMigrations(context.TODO(), NewStatusClient(clientset.CoreV1()), dynamicset, changes.ApplyOptions{}, nil)
		require.NoError(t, err)

		wantMetrics := map[string]*changes.ApplyMetrics{
			testMigration{}.Name(): {Patch: 1},
		}
		if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
			t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
		}
		wantSpec := corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http-80",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(9371)},
			},
		}

		updatedSvc := loadService(t, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, dynamicset)
		assert.Equal(t, wantSpec, updatedSvc.Spec)
	})

	t.Run("with multiple registered migrations", func(t *testing.T) {
		Register(testMigration{})
		Register(testDeleteMigration{})
		t.Cleanup(func() {
			knownMigrations = nil
		})
		svc := test.NewService(func(s *corev1.Service) {
			s.Spec.Ports[0].TargetPort = intstr.FromInt(8000)
		})
		secret := newSecret(
			types.NamespacedName{Name: "test-secret", Namespace: "cattle-secrets"},
			testSecretData)
		clientset := fake.NewClientset()
		dynamicset := newFakeDynamicClient(t, svc, secret)

		metrics, err := ApplyUnappliedMigrations(context.TODO(), NewStatusClient(clientset.CoreV1()), dynamicset, changes.ApplyOptions{}, nil)
		require.NoError(t, err)

		wantMetrics := map[string]*changes.ApplyMetrics{
			"test-delete-migration": {Delete: 1},
			"test-migration":        {Patch: 1},
		}
		if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
			t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
		}
	})

	t.Run("with previously applied migration", func(t *testing.T) {
		Register(testMigration{})
		t.Cleanup(func() {
			knownMigrations = nil
		})
		svc := test.NewService(func(s *corev1.Service) {
			s.Spec.Ports[0].TargetPort = intstr.FromInt(8000)
		})

		clientset := fake.NewClientset()
		dynamicset := newFakeDynamicClient(t, svc)
		// Apply once
		_, err := ApplyUnappliedMigrations(context.TODO(), NewStatusClient(clientset.CoreV1()), dynamicset, changes.ApplyOptions{}, nil)
		require.NoError(t, err)

		// Apply twice
		metrics, err := ApplyUnappliedMigrations(context.TODO(), NewStatusClient(clientset.CoreV1()), dynamicset, changes.ApplyOptions{}, nil)
		require.NoError(t, err)

		// No metrics because no migrations applied
		wantMetrics := map[string]*changes.ApplyMetrics{}
		if diff := cmp.Diff(wantMetrics, metrics); diff != "" {
			t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
		}
	})
}

func TestNameForMigration(t *testing.T) {
	// TODO: Is this a better way of naming Migrations?
	m := testMigration{}
	want := "migrations.testMigration"
	n := NameForMigration(m)
	if n != want {
		t.Fatalf("NameForMigration() got %s, want %s", n, want)
	}

	mp := &testMigration{}
	want = "migrations.testMigration"
	n = NameForMigration(mp)
	if n != want {
		t.Fatalf("NameForMigration() got %s, want %s", n, want)
	}
}

type testMigration struct {
}

func (t testMigration) Name() string {
	return "test-migration"
}

func (t testMigration) Changes(ctx context.Context, _ changes.Interface, _ MigrationOptions) (*MigrationChanges, error) {
	return &MigrationChanges{
		Changes: []changes.ResourceChange{
			{
				Operation: changes.OperationPatch,
				Patch: &changes.PatchChange{
					ResourceRef: changes.ResourceReference{
						ObjectRef: types.NamespacedName{
							Name:      "test-svc",
							Namespace: "default",
						},
						Resource: "services",
						Version:  "v1",
					},
					Operations: []changes.PatchOperation{
						{
							Operation: "replace",
							Path:      "/spec/ports/0/targetPort",
							Value:     9371,
						},
					},
					Type: changes.PatchApplicationJSON,
				},
			},
		},
	}, nil
}

type testFailingMigration struct {
}

func (t testFailingMigration) Name() string {
	return "test-failing-migration"
}

func (t testFailingMigration) Changes(ctx context.Context, _ changes.Interface, _ MigrationOptions) (*MigrationChanges, error) {
	return nil, errors.New("this is a failing migration")
}

type testDeleteMigration struct {
}

func (t testDeleteMigration) Name() string {
	return "test-delete-migration"
}

func (t testDeleteMigration) Changes(ctx context.Context, _ changes.Interface, _ MigrationOptions) (*MigrationChanges, error) {
	secretRef := changes.ResourceReference{
		ObjectRef: types.NamespacedName{
			Name:      "test-secret",
			Namespace: "cattle-secrets",
		},
		Group:    "",
		Resource: "secrets",
		Version:  "v1",
	}

	return &MigrationChanges{
		Changes: []changes.ResourceChange{
			{
				Operation: changes.OperationDelete,
				Delete: &changes.DeleteChange{
					ResourceRef: secretRef,
				},
			},
		},
	}, nil
}

type testCreateMigration struct {
	t *testing.T
}

func (m testCreateMigration) Name() string {
	return "test-create-migration"
}

func (m testCreateMigration) Changes(ctx context.Context, _ changes.Interface, _ MigrationOptions) (*MigrationChanges, error) {
	return &MigrationChanges{
		Changes: []changes.ResourceChange{
			{
				Operation: changes.OperationCreate,
				Create: &changes.CreateChange{
					Resource: test.ToUnstructured(m.t,
						newSecret(types.NamespacedName{Name: "test-secret", Namespace: "cattle-secrets"}, testSecretData)),
				},
			},
		},
	}, nil
}

// TODO: Move this to the test package
func newFakeDynamicClient(t *testing.T, objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	testScheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(testScheme))

	return dynamicfake.NewSimpleDynamicClient(testScheme, objs...)
}

func loadService(t *testing.T, name types.NamespacedName, dynamicset *dynamicfake.FakeDynamicClient) *corev1.Service {
	raw, err := dynamicset.Resource(schema.GroupVersionResource{
		Version:  "v1",
		Resource: "services",
	}).Namespace(name.Namespace).Get(context.TODO(), name.Name, metav1.GetOptions{})
	require.NoError(t, err)

	svc := &corev1.Service{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(
		raw.UnstructuredContent(), &svc)
	require.NoError(t, err)

	return svc
}

func loadStatusConfigMap(t *testing.T, clientset *fake.Clientset) *corev1.ConfigMap {
	cm, err := clientset.CoreV1().ConfigMaps(migrationsNamespace).Get(context.TODO(), migrationsCMName, metav1.GetOptions{})
	require.NoError(t, err)

	return cm
}

func newSecret(name types.NamespacedName, data map[string][]byte, opts ...func(s *corev1.Secret)) *corev1.Secret {
	// TODO: convert data map[string]string to map[string][]byte
	s := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

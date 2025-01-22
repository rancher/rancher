package migrations

import (
	"context"
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

	"github.com/rancher/rancher/pkg/migrations/descriptive"
	"github.com/rancher/rancher/pkg/migrations/test"
)

func TestApply(t *testing.T) {
	t.Run("unknown migration", func(t *testing.T) {
		clientset := fake.NewClientset()

		// TODO metrics
		_, err := Apply(context.TODO(), "test-migration", NewStatusClient(clientset.CoreV1()), newFakeDynamicClient(t), descriptive.ApplyOptions{}, nil)
		require.ErrorContains(t, err, `unknown migration "test-migration"`)
	})

	t.Run("with registered migration", func(t *testing.T) {
		Register(testMigration{})
		t.Cleanup(func() {
			knownMigrations = nil
		})

		svc := test.NewService(func(s *corev1.Service) {
			s.Spec.Ports[0].TargetPort = intstr.FromInt(8000)
		})

		t.Run("the migration is applied and resources are created", func(t *testing.T) {
			clientset := fake.NewClientset()
			dynamicset := newFakeDynamicClient(t, svc)

			// This passes nil as a mapper because we are not creating new
			// resources in this test migration.
			// TODO metrics
			_, err := Apply(context.TODO(), "test-migration", NewStatusClient(clientset.CoreV1()), dynamicset, descriptive.ApplyOptions{}, nil)
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
			assert.Equal(t, wantSpec, updatedSvc.Spec)
		})

		t.Run("the migration is applied and is recorded", func(t *testing.T) {
			clientset := fake.NewClientset()
			dynamicset := newFakeDynamicClient(t, svc)
			statusClient := NewStatusClient(clientset.CoreV1())

			// This passes nil as a mapper because we are not creating new
			// resources in this test migration.
			// TODO metrics
			_, err := Apply(context.TODO(), "test-migration", statusClient, dynamicset, descriptive.ApplyOptions{}, nil)
			require.NoError(t, err)

			wantStatus := &MigrationStatus{AppliedAt: time.Now()}
			status, err := statusClient.StatusFor(context.TODO(), "test-migration")
			assert.EqualExportedValues(t, wantStatus, status)
		})
	})

	t.Run("when the status ConfigMap does not exist", func(t *testing.T) {
		t.Skip("TODO")
	})
}

func TestApplyUnappliedMigrations(t *testing.T) {
	t.Run("no migrations registered", func(t *testing.T) {
		clientset := fake.NewClientset()

		metrics, err := ApplyUnappliedMigrations(context.TODO(), NewStatusClient(clientset.CoreV1()), newFakeDynamicClient(t), descriptive.ApplyOptions{}, nil)
		require.NoError(t, err)

		require.Equal(t, map[string]*descriptive.ApplyMetrics{}, metrics)
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

		metrics, err := ApplyUnappliedMigrations(context.TODO(), NewStatusClient(clientset.CoreV1()), dynamicset, descriptive.ApplyOptions{}, nil)
		require.NoError(t, err)

		wantMetrics := map[string]*descriptive.ApplyMetrics{
			testMigration{}.Name(): {Patch: 1, Errors: 0},
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
		t.Skip("TODO")
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
		_, err := ApplyUnappliedMigrations(context.TODO(), NewStatusClient(clientset.CoreV1()), dynamicset, descriptive.ApplyOptions{}, nil)
		require.NoError(t, err)

		// Apply twice
		metrics, err := ApplyUnappliedMigrations(context.TODO(), NewStatusClient(clientset.CoreV1()), dynamicset, descriptive.ApplyOptions{}, nil)
		require.NoError(t, err)

		// No metrics because no migrations applied
		wantMetrics := map[string]*descriptive.ApplyMetrics{}
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

func (t testMigration) Changes(ctx context.Context, _ descriptive.Interface) ([]descriptive.ResourceChange, error) {
	return []descriptive.ResourceChange{
		{
			Operation: descriptive.OperationPatch,
			Patch: &descriptive.PatchChange{
				ResourceRef: descriptive.ResourceReference{
					ObjectRef: types.NamespacedName{
						Name:      "test-svc",
						Namespace: "default",
					},
					Resource: "services",
					Version:  "v1",
				},
				Operations: []descriptive.PatchOperation{
					{
						Operation: "replace",
						Path:      "/spec/ports/0/targetPort",
						Value:     9371,
					},
				},
				Type: descriptive.PatchApplicationJSON,
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

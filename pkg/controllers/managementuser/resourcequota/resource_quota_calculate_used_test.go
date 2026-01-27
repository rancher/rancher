package resourcequota

import (
	"fmt"
	"go.uber.org/mock/gomock"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// mockController returns a controller always returning a project with no
// quotas. This ensures that calls to `calculateProjectResourceQuota` abort
// early and without error. There is no need to dive deeper into that function
// when testing the various callers.
func mockController(t *testing.T) (*calculateLimitController, *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList]) {
	ctrl := gomock.NewController(t)
	namespaces := fake.NewMockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl)
	projects := fake.NewMockControllerInterface[*v3.Project, *v3.ProjectList](ctrl)
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	indexer.AddIndexers(map[string]cache.IndexFunc{
		nsByProjectIndex: nsByProjectID,
	})
	calculate := &calculateLimitController{
		nsIndexer:   indexer,
		projects:    projects,
		namespaces:  namespaces,
		clusterName: "n-space",
	}
	projects.EXPECT().
		Get("n-space", "p-roject", metav1.GetOptions{}).
		Return(&v3.Project{}, nil).
		AnyTimes()
	projects.EXPECT().
		Get("n-space", "p-roject1", metav1.GetOptions{}).
		Return(&v3.Project{}, nil).
		AnyTimes()
	projects.EXPECT().
		Get("n-space", "p-roject2", metav1.GetOptions{}).
		Return(&v3.Project{}, nil).
		AnyTimes()

	return calculate, namespaces
}

func TestCalculateResourceQuotaUsed(t *testing.T) {
	// namespaces in the various possible states
	ns1 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
		},
	}
	ns2 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				projectIDAnnotation: "n-space:p-roject1",
			},
		},
	}
	ns3 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				resourceQuotaProjectID: "n-space:p-roject1",
			},
		},
	}
	ns4 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				projectIDAnnotation:    "n-space:p-roject1",
				resourceQuotaProjectID: "n-space:p-roject2", // P2 != P1
			},
		},
	}
	ns5 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				projectIDAnnotation:    "n-space:p-roject1",
				resourceQuotaProjectID: "n-space:p-roject1",
			},
		},
	}

	testCases := []struct {
		name       string
		setup      func(namespaces *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList]) *corev1.Namespace
		expected   error
		expectedNs *corev1.Namespace
	}{
		{
			name:       "nil namespace",
			expected:   nil,
			expectedNs: nil,
			setup: func(namespaces *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList]) *corev1.Namespace {
				return nil
			},
		},
		{
			name:       "no project, no old project - not assigned",
			expected:   nil,
			expectedNs: nil,
			setup: func(namespaces *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList]) *corev1.Namespace {
				return ns1
			},
		},
		{
			name:       "project, no old project - just assigned",
			expected:   nil,
			expectedNs: ns5,
			setup: func(namespaces *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList]) *corev1.Namespace {
				namespaces.EXPECT().Update(ns5).Return(ns5, nil)
				return ns2
			},
		},
		{
			name:       "no project, old project - just unassigned",
			expected:   nil,
			expectedNs: ns1,
			setup: func(namespaces *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList]) *corev1.Namespace {
				namespaces.EXPECT().Update(ns1).Return(ns1, nil)
				return ns3
			},
		},
		{
			name:       "project != old project - move",
			expected:   nil,
			expectedNs: ns5,
			setup: func(namespaces *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList]) *corev1.Namespace {
				namespaces.EXPECT().Update(ns5).Return(ns5, nil)
				return ns4
			},
		},
		{
			name:       "project == old project - nothing",
			expected:   nil,
			expectedNs: nil,
			setup: func(namespaces *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList]) *corev1.Namespace {
				return ns5
			},
		},
	}

	for _, tt := range testCases {
		t.Run("calculateResourceQuotaUsed: "+tt.name, func(t *testing.T) {
			calculate, namespaces := mockController(t)

			ns := tt.setup(namespaces)
			nsOut, err := calculate.calculateResourceQuotaUsed("", ns)
			if tt.expected != nil {
				assert.Error(t, err)
			}
			assert.Equal(t, tt.expected, err)
			assert.Equal(t, tt.expectedNs, nsOut)
		})
	}
}

func TestCalculateResourceQuotaUsedProject(t *testing.T) {
	calculate, _ := mockController(t)

	t.Run("calculateResourceQuotaUsedProject, no project", func(t *testing.T) {
		obj, err := calculate.calculateResourceQuotaUsedProject("", nil)
		assert.Nil(t, obj)
		assert.NoError(t, err)
	})
	t.Run("calculateResourceQuotaUsedProject, with project", func(t *testing.T) {
		obj, err := calculate.calculateResourceQuotaUsedProject("", &v3.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "p-roject"}})
		assert.Nil(t, obj)
		assert.NoError(t, err)
	})
}

func TestCalculateProjectResourceQuota(t *testing.T) {
	testCases := []struct {
		name     string
		setup    func(ctrl *gomock.Controller, projects *fake.MockControllerInterface[*v3.Project, *v3.ProjectList])
		expected error
	}{
		{
			name:     "project not found",
			expected: nil,
			setup: func(ctrl *gomock.Controller, projects *fake.MockControllerInterface[*v3.Project, *v3.ProjectList]) {
				projects.EXPECT().
					Get("n-space", "p-roject", metav1.GetOptions{}).
					Return(nil, errors.NewNotFound(v3.Resource("project"), "p-roject"))
			},
		},
		{
			name:     "project error",
			expected: fmt.Errorf("something"),
			setup: func(ctrl *gomock.Controller, projects *fake.MockControllerInterface[*v3.Project, *v3.ProjectList]) {
				projects.EXPECT().
					Get("n-space", "p-roject", metav1.GetOptions{}).
					Return(nil, fmt.Errorf("something"))
			},
		},
		{
			name:     "project found, no quota",
			expected: nil,
			setup: func(ctrl *gomock.Controller, projects *fake.MockControllerInterface[*v3.Project, *v3.ProjectList]) {
				projects.EXPECT().
					Get("n-space", "p-roject", metav1.GetOptions{}).
					Return(&v3.Project{}, nil)
			},
		},
	}

	for _, tt := range testCases {
		t.Run("calculateProjectResourceQuota: "+tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			namespaces := fake.NewMockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl)
			projects := fake.NewMockControllerInterface[*v3.Project, *v3.ProjectList](ctrl)
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc,
				cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			indexer.AddIndexers(map[string]cache.IndexFunc{
				nsByProjectIndex: nsByProjectID,
			})
			calculate := &calculateLimitController{
				nsIndexer:   indexer,
				projects:    projects,
				namespaces:  namespaces,
				clusterName: "a-cluster",
			}
			tt.setup(ctrl, projects)

			err := calculate.calculateProjectResourceQuota("n-space:p-roject")

			if tt.expected != nil {
				assert.Error(t, err)
			}
			assert.Equal(t, tt.expected, err)
		})
	}
}

func TestSetQuotaProjectID(t *testing.T) {
	t.Run("setQuotaProjectID, no annotations", func(t *testing.T) {
		ns := corev1.Namespace{}
		setQuotaProjectID(&ns, "the-project-id")
		assert.Equal(t, corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					resourceQuotaProjectID: "the-project-id",
				},
			},
		}, ns)
	})
	t.Run("setQuotaProjectID, with annotations, no conflict", func(t *testing.T) {
		ns := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"other": "x",
				},
			},
		}
		setQuotaProjectID(&ns, "the-project-id")
		assert.Equal(t, corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"other":                "x",
					resourceQuotaProjectID: "the-project-id",
				},
			},
		}, ns)
	})
	t.Run("setQuotaProjectID, with annotations, overwrite", func(t *testing.T) {
		ns := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					resourceQuotaProjectID: "an-old-project-id",
				},
			},
		}
		setQuotaProjectID(&ns, "the-project-id")
		assert.Equal(t, corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					resourceQuotaProjectID: "the-project-id",
				},
			},
		}, ns)
	})
}

func TestDeleteQuotaProjectID(t *testing.T) {
	t.Run("deleteQuotaProjectID, no annotations, no change", func(t *testing.T) {
		ns := corev1.Namespace{}
		deleteQuotaProjectID(&ns)
		assert.Equal(t, corev1.Namespace{}, ns)
	})
	t.Run("deleteQuotaProjectID, with annotations, not present, no change", func(t *testing.T) {
		ns := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"other": "x",
				},
			},
		}
		deleteQuotaProjectID(&ns)
		assert.Equal(t, corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"other": "x",
				},
			},
		}, ns)
	})
	t.Run("deleteQuotaProjectID, with annotations, present, removed", func(t *testing.T) {
		ns := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					resourceQuotaProjectID: "an-old-project-id",
				},
			},
		}
		deleteQuotaProjectID(&ns)
		assert.Equal(t, corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
		}, ns)
	})
	t.Run("deleteQuotaProjectID, with annotations, present and other, untouched other", func(t *testing.T) {
		ns := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"other":                "x",
					resourceQuotaProjectID: "an-old-project-id",
				},
			},
		}
		deleteQuotaProjectID(&ns)
		assert.Equal(t, corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"other": "x",
				},
			},
		}, ns)
	})
}

func TestGetQuotaProjectID(t *testing.T) {
	t.Run("getQuotaProjectID, no annotations", func(t *testing.T) {
		projectID := getQuotaProjectID(&corev1.Namespace{})
		assert.Equal(t, "", projectID)
	})
	t.Run("getQuotaProjectID, with annotations, missing", func(t *testing.T) {
		projectID := getQuotaProjectID(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"other": "x",
				},
			},
		})
		assert.Equal(t, "", projectID)
	})
	t.Run("getQuotaProjectID, with annotations, present", func(t *testing.T) {
		projectID := getQuotaProjectID(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					resourceQuotaProjectID: "the-project-id",
				},
			},
		})
		assert.Equal(t, "the-project-id", projectID)
	})
}

package namespace

import (
	"testing"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHandler(t *testing.T) {
	cases := []struct {
		name string

		ns *corev1.Namespace

		expected     *corev1.Namespace
		err          string
		shouldUpdate bool
	}{
		{
			name: "with unmanaged namespaces",

			ns: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
			},

			expected: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
		{
			name: "with managed namespace with missing labels",

			ns: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						namespace.AnnotationManagedNamespace: namespace.AnnotationManagedNamespceTrue,
						"foo":                                "bar",
					},
				},
			},

			expected: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						namespace.AnnotationManagedNamespace: namespace.AnnotationManagedNamespceTrue,
						"foo":                                "bar",
					},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			shouldUpdate: true,
		},
		{
			name: "with managed namespace with missing annotations",

			ns: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						namespace.AnnotationManagedNamespace: namespace.AnnotationManagedNamespceTrue,
					},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},

			expected: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						namespace.AnnotationManagedNamespace: namespace.AnnotationManagedNamespceTrue,
						"foo":                                "bar",
					},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			shouldUpdate: true,
		},
		{
			name: "with managed namespace without missing data",

			ns: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						namespace.AnnotationManagedNamespace: namespace.AnnotationManagedNamespceTrue,
						"foo":                                "bar",
					},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},

			expected: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						namespace.AnnotationManagedNamespace: namespace.AnnotationManagedNamespceTrue,
						"foo":                                "bar",
					},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
	}

	namespace.SetMutator(&namespace.Mutator{
		Enabled: true,
		Annotations: map[string]string{
			"foo": "bar",
		},
		Labels: map[string]string{
			"foo": "bar",
		},
	})

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			namespaces := fake.NewMockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl)
			handler := &handler{
				namespaces: namespaces,
			}

			if c.shouldUpdate {
				namespaces.EXPECT().Update(c.expected).Return(c.ns, nil)
			} else {
				namespaces.EXPECT().Update(c.expected).Times(0).Return(c.ns, nil)
			}

			actual, err := handler.onChange("", c.ns)

			if c.err == "" {
				assert.Equal(t, c.expected, actual)
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, c.err)
			}
		})
	}
}

package namespace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCopy(t *testing.T) {
	cases := []struct {
		name string

		src map[string]string
		dst map[string]string

		expected map[string]string
		updated  bool
	}{
		{
			name: "copy into empty dst",

			src: map[string]string{
				"foo": "bar",
				"baz": "quz",
			},
			dst: map[string]string{},

			expected: map[string]string{
				"foo": "bar",
				"baz": "quz",
			},
			updated: true,
		},
		{
			name: "copy into non-empty map without overlap",

			src: map[string]string{
				"foo": "bar",
				"baz": "quz",
			},
			dst: map[string]string{
				"a": "A",
				"b": "B",
			},

			expected: map[string]string{
				"foo": "bar",
				"baz": "quz",
				"a":   "A",
				"b":   "B",
			},
			updated: true,
		},
		{
			name: "copy into non-empty map with overlap with update",

			src: map[string]string{
				"foo": "bar",
				"baz": "quz",
			},
			dst: map[string]string{
				"baz": "something-else",
				"a":   "A",
				"b":   "B",
			},

			expected: map[string]string{
				"foo": "bar",
				"baz": "quz",
				"a":   "A",
				"b":   "B",
			},
			updated: true,
		},
		{
			name: "copy into non-empty map with overlap without update",

			src: map[string]string{
				"foo": "bar",
				"baz": "quz",
			},
			dst: map[string]string{
				"foo": "bar",
				"baz": "quz",
				"a":   "A",
				"b":   "B",
			},

			expected: map[string]string{
				"foo": "bar",
				"baz": "quz",
				"a":   "A",
				"b":   "B",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			updated := copy(c.dst, c.src)

			assert.Equal(t, c.expected, c.dst)
			assert.Equal(t, c.updated, updated)
		})
	}
}

func TestMutator(t *testing.T) {
	cases := []struct {
		name string

		mutator Mutator
		ns      corev1.Namespace

		expected     corev1.Namespace
		shouldUpdate bool
	}{
		{
			name: "with disabled mutator",

			mutator: Mutator{
				Enabled: false,
				Annotations: map[string]string{
					"foo": "bar",
				},
				Labels: map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			name: "with no updates",

			mutator: Mutator{
				Enabled: true,
				Annotations: map[string]string{
					"foo": "bar",
				},
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			ns: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},

			expected: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
		{
			name: "with updated annotations",

			mutator: Mutator{
				Enabled: true,
				Annotations: map[string]string{
					"foo": "bar",
				},
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			ns: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},

			expected: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			shouldUpdate: true,
		},
		{
			name: "with updated labels",

			mutator: Mutator{
				Enabled: true,
				Annotations: map[string]string{
					"foo": "bar",
				},
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			ns: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
			},

			expected: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			shouldUpdate: true,
		},
		{
			name: "with updated annotations and labels",

			mutator: Mutator{
				Enabled: true,
				Annotations: map[string]string{
					"foo": "bar",
				},
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			ns: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
					Labels:      map[string]string{},
				},
			},

			expected: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			shouldUpdate: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := c.ns

			updated := c.mutator.mutate(&actual)

			assert.Equal(t, c.expected, actual)
			assert.Equal(t, c.shouldUpdate, updated)
		})
	}
}

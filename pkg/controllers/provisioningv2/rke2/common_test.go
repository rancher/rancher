package rke2

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type test struct {
	name        string
	expected    *capi.Cluster
	expectedErr error
	obj         Test
}

func (t *test) Get(_, _ string) (*capi.Cluster, error) {
	return t.expected, t.expectedErr
}

func TestFindCAPIClusterFromLabel(t *testing.T) {
	tests := []test{
		{
			name:        "nil",
			expected:    nil,
			expectedErr: errors.New("cannot find CAPI cluster for nil object"),
			obj:         nil,
		},
		{
			name:        "missing label",
			expected:    nil,
			expectedErr: errors.New("cluster.x-k8s.io/cluster-name label not present on testObject: testNamespace/testName"),
			obj: &capi.Machine{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
					Labels:    map[string]string{},
				},
				TypeMeta: v1.TypeMeta{Kind: "testObject"},
			},
		},
		{
			name:        "missing cluster",
			expected:    nil,
			expectedErr: errors.New("could not find testObject: testNamespace/testName"),
			obj: &capi.Machine{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						"cluster.x-k8s.io/cluster-name": "testCluster",
					},
				},
			},
		},
		{
			name:        "success",
			expected:    &capi.Cluster{},
			expectedErr: nil,
			obj: &capi.Machine{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "testNamespace",
					Labels: map[string]string{
						"cluster.x-k8s.io/cluster-name": "testCluster",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster, err := FindCAPIClusterFromLabel(tt.obj, &tt)
			if err == nil {
				assert.Nil(t, tt.expectedErr)
			} else if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.Fail(t, "expected err to be nil, was actually %s", err)
			}
			assert.Equal(t, tt.expected, cluster)
		})
	}
}

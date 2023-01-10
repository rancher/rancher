package rke2

import (
	"testing"

	"github.com/pkg/errors"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// Implements ClusterCache
type test struct {
	name        string
	expected    *capi.Cluster
	expectedErr error
	obj         runtime.Object
}

func (t *test) Get(_, _ string) (*capi.Cluster, error) {
	return t.expected, t.expectedErr
}

func (t *test) List(_ string, _ labels.Selector) ([]*capi.Cluster, error) {
	panic("not implemented")
}

func (t *test) AddIndexer(_ string, _ capicontrollers.ClusterIndexer) {
	panic("not implemented")
}

func (t *test) GetByIndex(_, _ string) ([]*capi.Cluster, error) {
	panic("not implemented")
}

func TestFindCAPIClusterFromLabel(t *testing.T) {
	tests := []test{
		{
			name:        "nil",
			expected:    nil,
			expectedErr: errNilObject,
			obj:         nil,
		},
		{
			name:        "missing label",
			expected:    nil,
			expectedErr: errors.New("cluster.x-k8s.io/cluster-name label not present on testObject: testNamespace/testName"),
			obj: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
					Labels:    map[string]string{},
				},
				TypeMeta: metav1.TypeMeta{Kind: "testObject"},
			},
		},
		{
			name:     "missing cluster",
			expected: nil,
			obj: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"cluster.x-k8s.io/cluster-name": "testCluster",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster, err := GetCAPIClusterFromLabel(tt.obj, &tt)
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

func TestFindOwnerCAPICluster(t *testing.T) {
	tests := []test{
		{
			name:        "nil",
			expected:    nil,
			expectedErr: errNilObject,
			obj:         nil,
		},
		{
			name:        "no owner",
			expected:    nil,
			expectedErr: ErrNoControllerMachineOwnerRef,
			obj: &rkev1.RKECluster{
				TypeMeta: metav1.TypeMeta{
					Kind: "RKECluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcluster",
					Namespace: "testnamespace",
				},
			},
		},
		{
			name:        "no controller",
			expected:    nil,
			expectedErr: ErrNoControllerMachineOwnerRef,
			obj: &rkev1.RKECluster{
				TypeMeta: metav1.TypeMeta{
					Kind: "RKECluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcluster",
					Namespace: "testnamespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "nil",
						},
					},
				},
			},
		},
		{
			name:        "owner wrong kind",
			expected:    nil,
			expectedErr: ErrNoControllerMachineOwnerRef,
			obj: &rkev1.RKECluster{
				TypeMeta: metav1.TypeMeta{
					Kind: "RKECluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcluster",
					Namespace: "testnamespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "nil",
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Controller: &[]bool{true}[0],
						},
					},
				},
			},
		},
		{
			name:        "owner wrong api version",
			expected:    nil,
			expectedErr: ErrNoControllerMachineOwnerRef,
			obj: &rkev1.RKECluster{
				TypeMeta: metav1.TypeMeta{
					Kind: "RKECluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcluster",
					Namespace: "testnamespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "Cluster",
							APIVersion: "nil",
							Controller: &[]bool{true}[0],
						},
					},
				},
			},
		},
		{
			name:        "success",
			expected:    nil,
			expectedErr: nil,
			obj: &rkev1.RKECluster{
				TypeMeta: metav1.TypeMeta{
					Kind: "RKECluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcluster",
					Namespace: "testnamespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "Cluster",
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Controller: &[]bool{true}[0],
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster, err := GetOwnerCAPICluster(tt.obj, &tt)
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

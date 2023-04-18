package provisioningcluster

import (
	"reflect"
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGetObjectRef(t *testing.T) {
	tests := []struct {
		name string
		obj  runtime.Object
		ref  *corev1.ObjectReference
	}{
		{
			name: "controlplane ref",
			obj: &rkev1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			ref: &corev1.ObjectReference{
				APIVersion: "rke.cattle.io/v1",
				Kind:       "RKEControlPlane",
				Name:       "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := getObjectRef(tt.obj)
			assert.Nil(t, err)
			assert.True(t, reflect.DeepEqual(ref, tt.ref))
		})
	}
}

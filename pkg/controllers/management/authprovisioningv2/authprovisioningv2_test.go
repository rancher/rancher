package authprovisioningv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_scopedRBACOnRemoveCondition(t *testing.T) {
	conditionFunc := scopedRBACOnRemoveCondition("auth-prov-v2-crole")
	tests := []struct {
		name string
		obj  runtime.Object
		want bool
	}{
		{
			name: "Non-protected objects are ignored",
			obj: &rbacv1.ClusterRole{ObjectMeta: v1.ObjectMeta{
				Name: "testcr",
			}},
			want: false,
		},
		{
			name: "Non-protected objects with non-matching finalizers are ignored",
			obj: &rbacv1.ClusterRole{ObjectMeta: v1.ObjectMeta{
				Name:       "testcr",
				Finalizers: []string{"wrangler.cattle.io/foobar"},
			}},
			want: false,
		},
		{
			name: "Non-protected objects with matching finalizer are handled (backwards compatibility)",
			obj: &rbacv1.ClusterRole{ObjectMeta: v1.ObjectMeta{
				Name:       "testcr",
				Finalizers: []string{"wrangler.cattle.io/auth-prov-v2-crole"},
			}},
			want: true,
		},
		{
			name: "Non-protected objects with matching finalizer are handled (backwards compatibility)",
			obj: &rbacv1.ClusterRole{ObjectMeta: v1.ObjectMeta{
				Name:       "testcr",
				Finalizers: []string{"wrangler.cattle.io/auth-prov-v2-crole"},
			}},
			want: true,
		},
		{
			name: "Protected objects are handled",
			obj: &rbacv1.ClusterRole{ObjectMeta: v1.ObjectMeta{
				Name: "testcr",
				Annotations: map[string]string{
					"cluster.cattle.io/name": "test",
				},
			}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, conditionFunc(tt.obj), "condition(%v)", tt.name)
		})
	}
}

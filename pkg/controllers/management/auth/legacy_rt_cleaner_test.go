package auth

import (
	"reflect"
	"testing"

	"github.com/rancher/rancher/pkg/api/norman/store/roletemplate"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_rtCleanUp(t *testing.T) {
	type args struct {
		key string
		obj *v3.RoleTemplate
	}
	tests := []struct {
		name    string
		args    args
		want    *v3.RoleTemplate
		wantErr bool
	}{
		{
			name: "test_1",
			args: args{
				key: "test-key",
				obj: &v3.RoleTemplate{
					ObjectMeta: v1.ObjectMeta{
						Annotations: map[string]string{
							roletemplate.RTVersion: "false",
							"lifecycle.cattle.io/create.cluster-roletemplate-sync_test": "true",
						},
						Finalizers: []string{
							"clusterscoped.controller.cattle.io/cluster-roletemplate-sync_",
							"test.cattle.io/example-finalizer",
						},
					},
				},
			},
			want: &v3.RoleTemplate{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						roletemplate.RTVersion: "true",
					},
					Finalizers: []string{
						"test.cattle.io/example-finalizer",
					},
				},
			},
		},
		{
			name: "test_2",
			args: args{
				key: "test-key",
				obj: &v3.RoleTemplate{
					ObjectMeta: v1.ObjectMeta{
						Annotations: map[string]string{
							roletemplate.RTVersion: "false",
							"lifecycle.cattle.io/create.cluster-roletemplate-sync_test-1": "true",
							"lifecycle.cattle.io/create.cluster-roletemplate-sync_test-2": "true",
							"lifecycle.cattle.io/create.cluster-roletemplate-sync_test-3": "true",
						},
						Finalizers: []string{
							"clusterscoped.controller.cattle.io/cluster-roletemplate-sync_test-1",
							"clusterscoped.controller.cattle.io/cluster-roletemplate-sync_test-2",
							"clusterscoped.controller.cattle.io/cluster-roletemplate-sync_test-3",
						},
					},
				},
			},
			want: &v3.RoleTemplate{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						roletemplate.RTVersion: "true",
					},
					Finalizers: nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rtCleanUp(tt.args.obj)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("grbCleaner.sync() = %v, want %v", got, tt.want)
			}
		})
	}
}

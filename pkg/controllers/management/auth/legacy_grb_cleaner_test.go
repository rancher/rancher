package auth

import (
	"reflect"
	"testing"

	grbstore "github.com/rancher/rancher/pkg/api/norman/store/globalrolebindings"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_cleanAnnotations(t *testing.T) {
	type args struct {
		annotations map[string]string
		prefix      string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "remove one annotation",
			args: args{
				annotations: map[string]string{
					"author":            "john.doe@example.com",
					"description":       "This is a test application",
					"creationTimestamp": "2024-07-16T00:00:00Z",
					"revision":          "1",
					"environment":       "production",
					"maintainer":        "team@example.com",
				},
				prefix: "creationTimestamp",
			},
			want: map[string]string{
				"author":      "john.doe@example.com",
				"description": "This is a test application",
				"revision":    "1",
				"environment": "production",
				"maintainer":  "team@example.com",
			},
		},
		{
			name: "remove zero annotation",
			args: args{
				annotations: map[string]string{
					"author":            "john.doe@example.com",
					"description":       "This is a test application",
					"creationTimestamp": "2024-07-16T00:00:00Z",
					"revision":          "1",
					"environment":       "production",
					"maintainer":        "team@example.com",
				},
				prefix: "no-prefix",
			},
			want: map[string]string{
				"author":            "john.doe@example.com",
				"description":       "This is a test application",
				"creationTimestamp": "2024-07-16T00:00:00Z",
				"revision":          "1",
				"environment":       "production",
				"maintainer":        "team@example.com",
			},
		},
		{
			name: "remove all annotation",
			args: args{
				annotations: map[string]string{
					"author":            "john.doe@example.com",
					"description":       "This is a test application",
					"creationTimestamp": "2024-07-16T00:00:00Z",
					"revision":          "1",
					"environment":       "production",
					"maintainer":        "team@example.com",
				},
				prefix: "",
			},
			want: map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanAnnotations(tt.args.annotations, tt.args.prefix); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("cleanAnnotations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_cleanFinalizers(t *testing.T) {
	type args struct {
		finalizers []string
		prefix     string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "remove one finalizer",
			args: args{
				finalizers: []string{
					"example.com/my-finalizer",
					"kubernetes.io/pvc-protection",
					"example.com/cleanup-resource",
					"example.com/log-deletion",
					"kubernetes.io/garbage-collector",
					"example.com/remove-from-loadbalancer",
					"example.com/delete-from-database",
				},
				prefix: "example.com/delete-from-database",
			},
			want: []string{
				"example.com/my-finalizer",
				"kubernetes.io/pvc-protection",
				"example.com/cleanup-resource",
				"example.com/log-deletion",
				"kubernetes.io/garbage-collector",
				"example.com/remove-from-loadbalancer",
			},
		},
		{
			name: "remove zero finalizer",
			args: args{
				finalizers: []string{
					"example.com/my-finalizer",
					"kubernetes.io/pvc-protection",
					"example.com/cleanup-resource",
					"example.com/log-deletion",
					"kubernetes.io/garbage-collector",
					"example.com/remove-from-loadbalancer",
					"example.com/delete-from-database",
				},
				prefix: "no-prefix",
			},
			want: []string{
				"example.com/my-finalizer",
				"kubernetes.io/pvc-protection",
				"example.com/cleanup-resource",
				"example.com/log-deletion",
				"kubernetes.io/garbage-collector",
				"example.com/remove-from-loadbalancer",
				"example.com/delete-from-database",
			},
		},
		{
			name: "remove all finalizers",
			args: args{
				finalizers: []string{
					"example.com/my-finalizer",
					"kubernetes.io/pvc-protection",
					"example.com/cleanup-resource",
					"example.com/log-deletion",
					"kubernetes.io/garbage-collector",
					"example.com/remove-from-loadbalancer",
					"example.com/delete-from-database",
				},
				prefix: "",
			},
			want: []string{},
		},
		{
			name: "remove multiple finalizers",
			args: args{
				finalizers: []string{
					"example.com/my-finalizer",
					"kubernetes.io/pvc-protection",
					"example.com/cleanup-resource",
					"example.com/log-deletion",
					"kubernetes.io/garbage-collector",
					"example.com/remove-from-loadbalancer",
					"example.com/delete-from-database",
				},
				prefix: "example",
			},
			want: []string{
				"kubernetes.io/pvc-protection",
				"kubernetes.io/garbage-collector",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanFinalizers(tt.args.finalizers, tt.args.prefix); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("cleanFinalizers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_grbCleanUp(t *testing.T) {
	type args struct {
		key string
		obj *v3.GlobalRoleBinding
	}
	tests := []struct {
		name    string
		args    args
		want    *v3.GlobalRoleBinding
		wantErr bool
	}{
		{
			name: "remove one entry for annotation and finalizer",
			args: args{
				key: "test-key",
				obj: &v3.GlobalRoleBinding{
					ObjectMeta: v1.ObjectMeta{
						Annotations: map[string]string{
							grbstore.GrbVersion:                        "false",
							"lifecycle.cattle.io/create.grb-sync_test": "true",
						},
						Finalizers: []string{
							"clusterscoped.controller.cattle.io/grb-sync_test",
							"test.cattle.io/example-finalizer",
						},
					},
				},
			},
			want: &v3.GlobalRoleBinding{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						grbstore.GrbVersion: "true",
					},
					Finalizers: []string{
						"test.cattle.io/example-finalizer",
					},
				},
			},
		},
		{
			name: "remove multiple annotation(s) and finalizer(s)",
			args: args{
				key: "test-key",
				obj: &v3.GlobalRoleBinding{
					ObjectMeta: v1.ObjectMeta{
						Annotations: map[string]string{
							grbstore.GrbVersion:                          "false",
							"lifecycle.cattle.io/create.grb-sync_test-1": "true",
							"lifecycle.cattle.io/create.grb-sync_test-2": "true",
							"lifecycle.cattle.io/create.grb-sync_test-3": "true",
						},
						Finalizers: []string{
							"clusterscoped.controller.cattle.io/grb-sync_test-1",
							"clusterscoped.controller.cattle.io/grb-sync_test-2",
							"clusterscoped.controller.cattle.io/grb-sync_test-3",
						},
					},
				},
			},
			want: &v3.GlobalRoleBinding{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						grbstore.GrbVersion: "true",
					},
					Finalizers: []string{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gbrCleanUp(tt.args.obj)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("grbCleaner.sync() = %v, want %v", got, tt.want)
			}
		})
	}
}

package roletemplates

import (
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

func Test_getCRTBByUsername(t *testing.T) {
	tests := []struct {
		name string
		obj  *v3.ClusterRoleTemplateBinding
		want []string
	}{
		{
			name: "no username",
			obj: &v3.ClusterRoleTemplateBinding{
				UserName:    "",
				ClusterName: "c-abc123",
			},
			want: []string{},
		},
		{
			name: "no clustername",
			obj: &v3.ClusterRoleTemplateBinding{
				UserName:    "test-user",
				ClusterName: "",
			},
			want: []string{},
		},
		{
			name: "returns clustername-username index",
			obj: &v3.ClusterRoleTemplateBinding{
				UserName:    "test-user",
				ClusterName: "c-abc123",
			},
			want: []string{"c-abc123-test-user"},
		},
		{
			name: "index length is capped",
			obj: &v3.ClusterRoleTemplateBinding{
				UserName:    "very-long-username-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				ClusterName: "c-abc123",
			},
			want: []string{"c-abc123-very-long-username-aaaaaaaaaaaaaaaaaaaaaaaaaaaaa-8f7f4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := getCRTBByUsername(tt.obj)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getCRTBByUsername() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPRTBByUsername(t *testing.T) {
	tests := []struct {
		name string
		obj  *v3.ProjectRoleTemplateBinding
		want []string
	}{
		{
			name: "no username",
			obj: &v3.ProjectRoleTemplateBinding{
				UserName:    "",
				ProjectName: "c-abc123:p-xyz456",
			},
			want: []string{},
		},
		{
			name: "no projectname",
			obj: &v3.ProjectRoleTemplateBinding{
				UserName:    "test-user",
				ProjectName: "",
			},
			want: []string{},
		},
		{
			name: "returns clustername-username index",
			obj: &v3.ProjectRoleTemplateBinding{
				UserName:    "test-user",
				ProjectName: "c-abc123:p-xyz456",
			},
			want: []string{"c-abc123-test-user"},
		},
		{
			name: "index length is capped",
			obj: &v3.ProjectRoleTemplateBinding{
				UserName:    "very-long-username-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				ProjectName: "c-abc123:p-xyz456",
			},
			want: []string{"c-abc123-very-long-username-aaaaaaaaaaaaaaaaaaaaaaaaaaaaa-8f7f4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := getPRTBByUsername(tt.obj)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPRTBByUsername() = %v, want %v", got, tt.want)
			}
		})
	}
}

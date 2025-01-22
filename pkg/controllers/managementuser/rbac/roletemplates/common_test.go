package roletemplates

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_isRoleTemplateExternal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		rtName  string
		getFunc func() (*v3.RoleTemplate, error)
		want    bool
		wantErr bool
	}{
		{
			name:   "error getting role template",
			rtName: "test-rt",
			getFunc: func() (*v3.RoleTemplate, error) {
				return nil, errDefault
			},
			want:    false,
			wantErr: true,
		},
		{
			name:   "role template is nil",
			rtName: "test-rt",
			getFunc: func() (*v3.RoleTemplate, error) {
				return nil, nil
			},
			want:    false,
			wantErr: true,
		},
		{
			name:   "role template not found",
			rtName: "test-rt",
			getFunc: func() (*v3.RoleTemplate, error) {
				return nil, errNotFound
			},
			want:    false,
			wantErr: false,
		},
		{
			name:   "external role template found",
			rtName: "test-rt",
			getFunc: func() (*v3.RoleTemplate, error) {
				return &v3.RoleTemplate{External: true}, nil
			},
			want:    true,
			wantErr: false,
		},
		{
			name:   "non-external role template found",
			rtName: "test-rt",
			getFunc: func() (*v3.RoleTemplate, error) {
				return &v3.RoleTemplate{External: false}, nil
			},
			want:    false,
			wantErr: false,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rtClient := fake.NewMockNonNamespacedControllerInterface[*v3.RoleTemplate, *v3.RoleTemplateList](ctrl)
			rtClient.EXPECT().Get(tt.rtName, metav1.GetOptions{}).Return(tt.getFunc())
			got, err := isRoleTemplateExternal(tt.rtName, rtClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("isRoleTemplateExternal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isRoleTemplateExternal() = %v, want %v", got, tt.want)
			}
		})
	}
}

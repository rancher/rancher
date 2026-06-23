package roletemplates

import (
	"errors"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	roletemplate = &v3.RoleTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt-1",
		},
	}

	clusterRoleTemplateBinding = &v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "crtb-1",
			Namespace: "c-1",
		},
		RoleTemplateName: "rt-1",
	}

	projectRoleTemplateBinding = &v3.ProjectRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prtb-1",
			Namespace: "p-1",
		},
		RoleTemplateName: "rt-1",
	}
)

func TestRoletemplateEnqueueCRTBs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		crtbCache func(ctrl *gomock.Controller) mgmtv3.ClusterRoleTemplateBindingCache
		obj       runtime.Object
		want      []relatedresource.Key
		wantErr   bool
	}{
		{
			name: "valid roletemplate object",
			obj:  roletemplate,
			crtbCache: func(ctrl *gomock.Controller) mgmtv3.ClusterRoleTemplateBindingCache {
				m := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
				m.EXPECT().GetByIndex(rbac.CRTBByRoleTemplateNameIndex, "rt-1").Return([]*v3.ClusterRoleTemplateBinding{clusterRoleTemplateBinding}, nil)
				return m
			},
			want: []relatedresource.Key{
				{
					Name:      "crtb-1",
					Namespace: "c-1",
				},
			},
		},
		{
			name: "crtb cache returns an error",
			obj:  roletemplate,
			crtbCache: func(ctrl *gomock.Controller) mgmtv3.ClusterRoleTemplateBindingCache {
				m := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
				m.EXPECT().GetByIndex(rbac.CRTBByRoleTemplateNameIndex, "rt-1").Return(nil, errors.New("crtb cache error"))
				return m
			},
			wantErr: true,
		},
		{
			name:    "nil object",
			obj:     nil,
			want:    nil,
			wantErr: false,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &roletemplateEnqueuer{}
			if tt.crtbCache != nil {
				r.crtbCache = tt.crtbCache(ctrl)
			}

			got, err := r.roletemplateEnqueueCRTBs("", "rt-1", tt.obj)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRoletemplateEnqueuePRTBs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		prtbCache func(ctrl *gomock.Controller) mgmtv3.ProjectRoleTemplateBindingCache
		obj       runtime.Object
		want      []relatedresource.Key
		wantErr   bool
	}{
		{
			name: "valid roletemplate object",
			obj:  roletemplate,
			prtbCache: func(ctrl *gomock.Controller) mgmtv3.ProjectRoleTemplateBindingCache {
				m := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
				m.EXPECT().GetByIndex(rbac.PRTBByRoleTemplateNameIndex, "rt-1").Return([]*v3.ProjectRoleTemplateBinding{projectRoleTemplateBinding}, nil)
				return m
			},
			want: []relatedresource.Key{
				{
					Name:      "prtb-1",
					Namespace: "p-1",
				},
			},
		},
		{
			name: "prtb cache returns an error",
			obj:  roletemplate,
			prtbCache: func(ctrl *gomock.Controller) mgmtv3.ProjectRoleTemplateBindingCache {
				m := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
				m.EXPECT().GetByIndex(rbac.PRTBByRoleTemplateNameIndex, "rt-1").Return(nil, errors.New("prtb cache error"))
				return m
			},
			wantErr: true,
		},
		{
			name:    "nil object",
			obj:     nil,
			want:    nil,
			wantErr: false,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &roletemplateEnqueuer{}
			if tt.prtbCache != nil {
				r.prtbCache = tt.prtbCache(ctrl)
			}

			got, err := r.roletemplateEnqueuePRTBs("", "rt-1", tt.obj)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClusterRoleEnqueue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		obj        runtime.Object
		lookupKeys func(owner string) ([]relatedresource.Key, error)
		want       []relatedresource.Key
		wantErr    bool
	}{
		{
			name: "valid aggregation cluster role with owner label",
			obj: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cr-1",
					Labels: map[string]string{
						rbac.ClusterRoleOwnerLabel: "rt-1",
					},
				},
				AggregationRule: &rbacv1.AggregationRule{},
			},
			lookupKeys: func(owner string) ([]relatedresource.Key, error) {
				return []relatedresource.Key{{Name: "binding-1", Namespace: "ns-1"}}, nil
			},
			want: []relatedresource.Key{{Name: "binding-1", Namespace: "ns-1"}},
		},
		{
			name: "lookupKeys returns error",
			obj: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cr-1",
					Labels: map[string]string{
						rbac.ClusterRoleOwnerLabel: "rt-1",
					},
				},
				AggregationRule: &rbacv1.AggregationRule{},
			},
			lookupKeys: func(owner string) ([]relatedresource.Key, error) {
				return nil, errors.New("lookup error")
			},
			wantErr: true,
		},
		{
			name: "cluster role without aggregation rule",
			obj: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cr-1",
					Labels: map[string]string{
						rbac.ClusterRoleOwnerLabel: "rt-1",
					},
				},
			},
			want: nil,
		},
		{
			name: "aggregation cluster role without owner label",
			obj: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cr-1",
				},
				AggregationRule: &rbacv1.AggregationRule{},
			},
			want: nil,
		},
		{
			name: "nil object",
			obj:  nil,
			want: nil,
		},
		{
			name:    "non-ClusterRole object",
			obj:     roletemplate,
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := clusterRoleEnqueue(tt.obj, tt.lookupKeys)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClusterRoleEnqueuePRTBs(t *testing.T) {
	t.Parallel()

	aggregationClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cr-1",
			Labels: map[string]string{
				rbac.ClusterRoleOwnerLabel: "rt-1",
			},
		},
		AggregationRule: &rbacv1.AggregationRule{},
	}

	tests := []struct {
		name      string
		obj       runtime.Object
		prtbCache func(ctrl *gomock.Controller) mgmtv3.ProjectRoleTemplateBindingCache
		want      []relatedresource.Key
		wantErr   bool
	}{
		{
			name: "valid aggregation cluster role returns PRTBs",
			obj:  aggregationClusterRole,
			prtbCache: func(ctrl *gomock.Controller) mgmtv3.ProjectRoleTemplateBindingCache {
				m := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
				m.EXPECT().GetByIndex(rbac.PRTBByRoleTemplateNameIndex, "rt-1").Return([]*v3.ProjectRoleTemplateBinding{projectRoleTemplateBinding}, nil)
				return m
			},
			want: []relatedresource.Key{
				{Name: "prtb-1", Namespace: "p-1"},
			},
		},
		{
			name: "prtb cache returns error",
			obj:  aggregationClusterRole,
			prtbCache: func(ctrl *gomock.Controller) mgmtv3.ProjectRoleTemplateBindingCache {
				m := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
				m.EXPECT().GetByIndex(rbac.PRTBByRoleTemplateNameIndex, "rt-1").Return(nil, errors.New("prtb cache error"))
				return m
			},
			wantErr: true,
		},
		{
			name: "nil object",
			obj:  nil,
			want: nil,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &roletemplateEnqueuer{}
			if tt.prtbCache != nil {
				r.prtbCache = tt.prtbCache(ctrl)
			}

			got, err := r.clusterRoleEnqueuePRTBs("", "", tt.obj)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClusterRoleEnqueueCRTBs(t *testing.T) {
	t.Parallel()

	aggregationClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cr-1",
			Labels: map[string]string{
				rbac.ClusterRoleOwnerLabel: "rt-1",
			},
		},
		AggregationRule: &rbacv1.AggregationRule{},
	}

	tests := []struct {
		name      string
		obj       runtime.Object
		crtbCache func(ctrl *gomock.Controller) mgmtv3.ClusterRoleTemplateBindingCache
		want      []relatedresource.Key
		wantErr   bool
	}{
		{
			name: "valid aggregation cluster role returns CRTBs",
			obj:  aggregationClusterRole,
			crtbCache: func(ctrl *gomock.Controller) mgmtv3.ClusterRoleTemplateBindingCache {
				m := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
				m.EXPECT().GetByIndex(rbac.CRTBByRoleTemplateNameIndex, "rt-1").Return([]*v3.ClusterRoleTemplateBinding{clusterRoleTemplateBinding}, nil)
				return m
			},
			want: []relatedresource.Key{
				{Name: "crtb-1", Namespace: "c-1"},
			},
		},
		{
			name: "crtb cache returns error",
			obj:  aggregationClusterRole,
			crtbCache: func(ctrl *gomock.Controller) mgmtv3.ClusterRoleTemplateBindingCache {
				m := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
				m.EXPECT().GetByIndex(rbac.CRTBByRoleTemplateNameIndex, "rt-1").Return(nil, errors.New("crtb cache error"))
				return m
			},
			wantErr: true,
		},
		{
			name: "nil object",
			obj:  nil,
			want: nil,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &roletemplateEnqueuer{}
			if tt.crtbCache != nil {
				r.crtbCache = tt.crtbCache(ctrl)
			}

			got, err := r.clusterRoleEnqueueCRTBs("", "", tt.obj)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPRTBByRoleTemplateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		prtb *v3.ProjectRoleTemplateBinding
		want []string
	}{
		{
			name: "prtb with role template name",
			prtb: &v3.ProjectRoleTemplateBinding{
				RoleTemplateName: "rt-1",
			},
			want: []string{"rt-1"},
		},
		{
			name: "prtb with empty role template name",
			prtb: &v3.ProjectRoleTemplateBinding{},
			want: []string{},
		},
		{
			name: "nil prtb",
			prtb: nil,
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := getPRTBByRoleTemplateName(tt.prtb)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetCRTBByRoleTemplateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		crtb *v3.ClusterRoleTemplateBinding
		want []string
	}{
		{
			name: "crtb with role template name",
			crtb: &v3.ClusterRoleTemplateBinding{
				RoleTemplateName: "rt-1",
			},
			want: []string{"rt-1"},
		},
		{
			name: "crtb with empty role template name",
			crtb: &v3.ClusterRoleTemplateBinding{},
			want: []string{},
		},
		{
			name: "nil crtb",
			crtb: nil,
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := getCRTBByRoleTemplateName(tt.crtb)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPRTBByClusterAndRoleTemplateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		prtb *v3.ProjectRoleTemplateBinding
		want []string
	}{
		{
			name: "prtb with cluster and role template name",
			prtb: &v3.ProjectRoleTemplateBinding{
				ProjectName:      "c-abc123:p-xyz456",
				RoleTemplateName: "rt-1",
			},
			want: []string{rbac.RoleTemplateClusterIndexKey("c-abc123", "rt-1")},
		},
		{
			name: "prtb with empty role template name",
			prtb: &v3.ProjectRoleTemplateBinding{
				ProjectName: "c-abc123:p-xyz456",
			},
			want: []string{},
		},
		{
			name: "prtb with empty project name",
			prtb: &v3.ProjectRoleTemplateBinding{
				RoleTemplateName: "rt-1",
			},
			want: []string{},
		},
		{
			name: "nil prtb",
			prtb: nil,
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := getPRTBByClusterAndRoleTemplateName(tt.prtb)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetCRTBByClusterAndRoleTemplateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		crtb *v3.ClusterRoleTemplateBinding
		want []string
	}{
		{
			name: "crtb with cluster and role template name",
			crtb: &v3.ClusterRoleTemplateBinding{
				ClusterName:      "c-abc123",
				RoleTemplateName: "rt-1",
			},
			want: []string{rbac.RoleTemplateClusterIndexKey("c-abc123", "rt-1")},
		},
		{
			name: "crtb with empty role template name",
			crtb: &v3.ClusterRoleTemplateBinding{
				ClusterName: "c-abc123",
			},
			want: []string{},
		},
		{
			name: "crtb with empty cluster name",
			crtb: &v3.ClusterRoleTemplateBinding{
				RoleTemplateName: "rt-1",
			},
			want: []string{},
		},
		{
			name: "nil crtb",
			crtb: nil,
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := getCRTBByClusterAndRoleTemplateName(tt.crtb)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

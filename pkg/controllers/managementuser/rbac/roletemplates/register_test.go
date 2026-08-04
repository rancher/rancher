package roletemplates

import (
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGetCRTBByUsername(t *testing.T) {
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

func TestGetPRTBByUsername(t *testing.T) {
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

func TestEnqueuePRTBs(t *testing.T) {
	t.Parallel()

	nonNilObj := &v3.RoleTemplate{}
	prtbInCluster := &v3.ProjectRoleTemplateBinding{
		ObjectMeta:  metav1.ObjectMeta{Name: "prtb-1", Namespace: "p-abc123"},
		ProjectName: "c-abc123:p-abc123",
	}
	prtbInCluster2 := &v3.ProjectRoleTemplateBinding{
		ObjectMeta:  metav1.ObjectMeta{Name: "prtb-2", Namespace: "p-abc123"},
		ProjectName: "c-abc123:p-abc123",
	}

	tests := []struct {
		name        string
		clusterName string
		rtName      string
		obj         runtime.Object
		prtbs       []*v3.ProjectRoleTemplateBinding
		indexErr    error
		wantKeys    []relatedresource.Key
		wantErr     bool
	}{
		{
			name:     "nil obj returns nil",
			obj:      nil,
			wantKeys: nil,
		},
		{
			name:        "GetByIndex error is propagated",
			clusterName: "c-abc123",
			rtName:      "test-rt",
			obj:         nonNilObj,
			indexErr:    errDefault,
			wantErr:     true,
		},
		{
			name:        "no PRTBs in the cluster returns nil",
			clusterName: "c-abc123",
			rtName:      "test-rt",
			obj:         nonNilObj,
			prtbs:       nil,
			wantKeys:    nil,
		},
		{
			name:        "PRTB from the cluster-scoped index is enqueued",
			clusterName: "c-abc123",
			rtName:      "test-rt",
			obj:         nonNilObj,
			prtbs:       []*v3.ProjectRoleTemplateBinding{prtbInCluster},
			wantKeys:    []relatedresource.Key{{Namespace: "p-abc123", Name: "prtb-1"}},
		},
		{
			name:        "all PRTBs from the cluster-scoped index are enqueued",
			clusterName: "c-abc123",
			rtName:      "test-rt",
			obj:         nonNilObj,
			prtbs:       []*v3.ProjectRoleTemplateBinding{prtbInCluster, prtbInCluster2},
			wantKeys:    []relatedresource.Key{{Namespace: "p-abc123", Name: "prtb-1"}, {Namespace: "p-abc123", Name: "prtb-2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			prtbCache := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
			if tt.obj != nil {
				prtbCache.EXPECT().GetByIndex(rbac.PRTBByClusterAndRoleTemplateNameIndex, rbac.RoleTemplateClusterIndexKey(tt.clusterName, tt.rtName)).Return(tt.prtbs, tt.indexErr)
			}

			enqueuer := &roleTemplateEnqueuer{
				clusterName: tt.clusterName,
				prtbCache:   prtbCache,
			}

			got, err := enqueuer.enqueuePRTBs("", tt.rtName, tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("enqueuePRTBs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.wantKeys) {
				t.Errorf("enqueuePRTBs() = %v, want %v", got, tt.wantKeys)
			}
		})
	}
}

func TestEnqueueCRTBs(t *testing.T) {
	t.Parallel()

	nonNilObj := &v3.RoleTemplate{}
	crtbInCluster := &v3.ClusterRoleTemplateBinding{
		ObjectMeta:  metav1.ObjectMeta{Name: "crtb-1", Namespace: "c-abc123"},
		ClusterName: "c-abc123",
	}
	crtbInCluster2 := &v3.ClusterRoleTemplateBinding{
		ObjectMeta:  metav1.ObjectMeta{Name: "crtb-2", Namespace: "c-abc123"},
		ClusterName: "c-abc123",
	}

	tests := []struct {
		name        string
		clusterName string
		rtName      string
		obj         runtime.Object
		crtbs       []*v3.ClusterRoleTemplateBinding
		indexErr    error
		wantKeys    []relatedresource.Key
		wantErr     bool
	}{
		{
			name:     "nil obj returns nil",
			obj:      nil,
			wantKeys: nil,
		},
		{
			name:        "GetByIndex error is propagated",
			clusterName: "c-abc123",
			rtName:      "test-rt",
			obj:         nonNilObj,
			indexErr:    errDefault,
			wantErr:     true,
		},
		{
			name:        "no CRTBs in the cluster returns nil",
			clusterName: "c-abc123",
			rtName:      "test-rt",
			obj:         nonNilObj,
			crtbs:       nil,
			wantKeys:    nil,
		},
		{
			name:        "CRTB from the cluster-scoped index is enqueued",
			clusterName: "c-abc123",
			rtName:      "test-rt",
			obj:         nonNilObj,
			crtbs:       []*v3.ClusterRoleTemplateBinding{crtbInCluster},
			wantKeys:    []relatedresource.Key{{Namespace: "c-abc123", Name: "crtb-1"}},
		},
		{
			name:        "all CRTBs from the cluster-scoped index are enqueued",
			clusterName: "c-abc123",
			rtName:      "test-rt",
			obj:         nonNilObj,
			crtbs:       []*v3.ClusterRoleTemplateBinding{crtbInCluster, crtbInCluster2},
			wantKeys:    []relatedresource.Key{{Namespace: "c-abc123", Name: "crtb-1"}, {Namespace: "c-abc123", Name: "crtb-2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			crtbCache := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
			if tt.obj != nil {
				crtbCache.EXPECT().GetByIndex(rbac.CRTBByClusterAndRoleTemplateNameIndex, rbac.RoleTemplateClusterIndexKey(tt.clusterName, tt.rtName)).Return(tt.crtbs, tt.indexErr)
			}

			enqueuer := &roleTemplateEnqueuer{
				clusterName: tt.clusterName,
				crtbCache:   crtbCache,
			}

			got, err := enqueuer.enqueueCRTBs("", tt.rtName, tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("enqueueCRTBs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.wantKeys) {
				t.Errorf("enqueueCRTBs() = %v, want %v", got, tt.wantKeys)
			}
		})
	}
}

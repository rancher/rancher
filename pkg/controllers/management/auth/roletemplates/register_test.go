package roletemplates

import (
	"errors"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	roletemplate = &v3.RoleTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rt-1",
		},
	}

	clusters = []*v3.Cluster{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "c-1",
			},
		},
	}

	project = &v3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p-1",
			Namespace: "c-1",
		},
		Spec: v3.ProjectSpec{
			ClusterName: "c-1",
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

func Test_roletemplateEnqueueCRTBs(t *testing.T) {
	t.Parallel()

	type caches struct {
		clusterCache func(ctrl *gomock.Controller) mgmtv3.ClusterCache
		crtbCache    func(ctrl *gomock.Controller) mgmtv3.ClusterRoleTemplateBindingCache
	}
	tests := []struct {
		name    string
		caches  caches
		obj     runtime.Object
		want    []relatedresource.Key
		wantErr bool
	}{
		{
			name: "valid roletemplate object",
			obj:  roletemplate,
			caches: caches{
				clusterCache: func(ctrl *gomock.Controller) mgmtv3.ClusterCache {
					m := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
					m.EXPECT().List(labels.Everything()).Return(clusters, nil)
					return m
				},
				crtbCache: func(ctrl *gomock.Controller) mgmtv3.ClusterRoleTemplateBindingCache {
					m := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
					m.EXPECT().List("c-1", labels.Everything()).Return([]*v3.ClusterRoleTemplateBinding{clusterRoleTemplateBinding}, nil)
					return m
				},
			},
			want: []relatedresource.Key{
				{
					Name:      "crtb-1",
					Namespace: "c-1",
				},
			},
		},
		{
			name: "cluster cache returns an error",
			obj:  roletemplate,
			caches: caches{
				clusterCache: func(ctrl *gomock.Controller) mgmtv3.ClusterCache {
					m := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
					m.EXPECT().List(labels.Everything()).Return(nil, errors.New("cluster cache error"))
					return m
				},
			},
			wantErr: true,
		},
		{
			name: "crtb cache returns an error",
			obj:  roletemplate,
			caches: caches{
				clusterCache: func(ctrl *gomock.Controller) mgmtv3.ClusterCache {
					m := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
					m.EXPECT().List(labels.Everything()).Return(clusters, nil)
					return m
				},
				crtbCache: func(ctrl *gomock.Controller) mgmtv3.ClusterRoleTemplateBindingCache {
					m := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
					m.EXPECT().List("c-1", labels.Everything()).Return(nil, errors.New("crtb cache error"))
					return m
				},
			},
			wantErr: true,
		},
		{
			name:    "nil object",
			obj:     nil,
			caches:  caches{},
			want:    nil,
			wantErr: false,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &roletemplateEnqueuer{}
			if tt.caches.clusterCache != nil {
				r.clusterCache = tt.caches.clusterCache(ctrl)
			}
			if tt.caches.crtbCache != nil {
				r.crtbCache = tt.caches.crtbCache(ctrl)
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

func Test_roletemplateEnqueuePRTBs(t *testing.T) {
	t.Parallel()

	type caches struct {
		clusterCache func(ctrl *gomock.Controller) mgmtv3.ClusterCache
		projectCache func(ctrl *gomock.Controller) mgmtv3.ProjectCache
		prtbCache    func(ctrl *gomock.Controller) mgmtv3.ProjectRoleTemplateBindingCache
	}
	tests := []struct {
		name    string
		caches  caches
		obj     runtime.Object
		want    []relatedresource.Key
		wantErr bool
	}{
		{
			name: "valid roletemplate object",
			obj:  roletemplate,
			caches: caches{
				clusterCache: func(ctrl *gomock.Controller) mgmtv3.ClusterCache {
					m := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
					m.EXPECT().List(labels.Everything()).Return(clusters, nil)
					return m
				},
				projectCache: func(ctrl *gomock.Controller) mgmtv3.ProjectCache {
					m := fake.NewMockCacheInterface[*v3.Project](ctrl)
					m.EXPECT().List("c-1", labels.Everything()).Return([]*v3.Project{project}, nil)
					return m
				},
				prtbCache: func(ctrl *gomock.Controller) mgmtv3.ProjectRoleTemplateBindingCache {
					m := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
					m.EXPECT().List("p-1", labels.Everything()).Return([]*v3.ProjectRoleTemplateBinding{projectRoleTemplateBinding}, nil)
					return m
				},
			},
			want: []relatedresource.Key{
				{
					Name:      "prtb-1",
					Namespace: "p-1",
				},
			},
		},
		{
			name: "cluster cache returns an error",
			obj:  roletemplate,
			caches: caches{
				clusterCache: func(ctrl *gomock.Controller) mgmtv3.ClusterCache {
					m := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
					m.EXPECT().List(labels.Everything()).Return(nil, errors.New("cluster cache error"))
					return m
				},
			},
			wantErr: true,
		},
		{
			name: "project cache returns an error",
			obj:  roletemplate,
			caches: caches{
				clusterCache: func(ctrl *gomock.Controller) mgmtv3.ClusterCache {
					m := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
					m.EXPECT().List(labels.Everything()).Return(clusters, nil)
					return m
				},
				projectCache: func(ctrl *gomock.Controller) mgmtv3.ProjectCache {
					m := fake.NewMockCacheInterface[*v3.Project](ctrl)
					m.EXPECT().List("c-1", labels.Everything()).Return(nil, errors.New("project cache error"))
					return m
				},
			},
			wantErr: true,
		},
		{
			name: "prtb cache returns an error",
			obj:  roletemplate,
			caches: caches{
				clusterCache: func(ctrl *gomock.Controller) mgmtv3.ClusterCache {
					m := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
					m.EXPECT().List(labels.Everything()).Return(clusters, nil)
					return m
				},
				projectCache: func(ctrl *gomock.Controller) mgmtv3.ProjectCache {
					m := fake.NewMockCacheInterface[*v3.Project](ctrl)
					m.EXPECT().List("c-1", labels.Everything()).Return([]*v3.Project{project}, nil)
					return m
				},
				prtbCache: func(ctrl *gomock.Controller) mgmtv3.ProjectRoleTemplateBindingCache {
					m := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
					m.EXPECT().List("p-1", labels.Everything()).Return(nil, errors.New("prtb cache error"))
					return m
				},
			},
			wantErr: true,
		},
		{
			name:    "nil object",
			obj:     nil,
			caches:  caches{},
			want:    nil,
			wantErr: false,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &roletemplateEnqueuer{}
			if tt.caches.clusterCache != nil {
				r.clusterCache = tt.caches.clusterCache(ctrl)
			}
			if tt.caches.projectCache != nil {
				r.projectCache = tt.caches.projectCache(ctrl)
			}
			if tt.caches.prtbCache != nil {
				r.prtbCache = tt.caches.prtbCache(ctrl)
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

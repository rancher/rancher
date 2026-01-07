package auth

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_enqueueAggregationResources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		obj      runtime.Object
		listFunc func() ([]*v3.ClusterRoleTemplateBinding, error)
		want     []relatedresource.Key
		wantErr  bool
	}{
		{
			name: "nil object returns no keys",
			obj:  nil,
			listFunc: func() ([]*v3.ClusterRoleTemplateBinding, error) {
				return nil, nil
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "non-feature object returns no keys",
			obj: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			listFunc: func() ([]*v3.ClusterRoleTemplateBinding, error) {
				return nil, nil
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "feature with different name returns no keys",
			obj: &v3.Feature{
				ObjectMeta: metav1.ObjectMeta{
					Name: "different-feature",
				},
			},
			listFunc: func() ([]*v3.ClusterRoleTemplateBinding, error) {
				return nil, nil
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "aggregated-roletemplates feature with list error",
			obj: &v3.Feature{
				ObjectMeta: metav1.ObjectMeta{
					Name: "aggregated-roletemplates",
				},
			},
			listFunc: func() ([]*v3.ClusterRoleTemplateBinding, error) {
				return nil, fmt.Errorf("list error")
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "aggregated-roletemplates feature with empty list",
			obj: &v3.Feature{
				ObjectMeta: metav1.ObjectMeta{
					Name: "aggregated-roletemplates",
				},
			},
			listFunc: func() ([]*v3.ClusterRoleTemplateBinding, error) {
				return []*v3.ClusterRoleTemplateBinding{}, nil
			},
			want:    []relatedresource.Key{},
			wantErr: false,
		},
		{
			name: "aggregated-roletemplates feature with single item",
			obj: &v3.Feature{
				ObjectMeta: metav1.ObjectMeta{
					Name: "aggregated-roletemplates",
				},
			},
			listFunc: func() ([]*v3.ClusterRoleTemplateBinding, error) {
				return []*v3.ClusterRoleTemplateBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "crtb-1",
							Namespace: "namespace-1",
						},
					},
				}, nil
			},
			want: []relatedresource.Key{
				{Name: "crtb-1", Namespace: "namespace-1"},
			},
			wantErr: false,
		},
		{
			name: "aggregated-roletemplates feature with multiple items",
			obj: &v3.Feature{
				ObjectMeta: metav1.ObjectMeta{
					Name: "aggregated-roletemplates",
				},
			},
			listFunc: func() ([]*v3.ClusterRoleTemplateBinding, error) {
				return []*v3.ClusterRoleTemplateBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "crtb-1",
							Namespace: "namespace-1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "crtb-2",
							Namespace: "namespace-2",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "crtb-3",
							Namespace: "namespace-1",
						},
					},
				}, nil
			},
			want: []relatedresource.Key{
				{Name: "crtb-1", Namespace: "namespace-1"},
				{Name: "crtb-2", Namespace: "namespace-2"},
				{Name: "crtb-3", Namespace: "namespace-1"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := enqueueAggregationResources(tt.obj, tt.listFunc)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_isFeatureAggregation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  runtime.Object
		want bool
	}{
		{
			name: "nil object returns false",
			obj:  nil,
			want: false,
		},
		{
			name: "non-feature object returns false",
			obj: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			want: false,
		},
		{
			name: "feature with different name returns false",
			obj: &v3.Feature{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-other-feature",
				},
			},
			want: false,
		},
		{
			name: "feature with aggregated-roletemplates name returns true",
			obj: &v3.Feature{
				ObjectMeta: metav1.ObjectMeta{
					Name: "aggregated-roletemplates",
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isFeatureAggregation(tt.obj)
			assert.Equal(t, tt.want, got)
		})
	}
}

package cluster

import (
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestVersionManagementEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	grbCache := fake.NewMockNonNamespacedCacheInterface[*mgmtv3.Setting](ctrl)
	grbCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*mgmtv3.Setting, error) {
		if name == "imported-cluster-version-management" {
			return &mgmtv3.Setting{Value: "true"}, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
	}).AnyTimes()

	type args struct {
		cluster *mgmtv3.Cluster
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "nil cluster",
			args: args{
				cluster: nil,
			},
			want: false,
		},
		{
			name: "annotation true",
			args: args{
				cluster: &mgmtv3.Cluster{
					Annotations: map[string]string{
						mgmtv3.ClusterImportedVersionManagementAnnotation: "true",
					},
				},
			},
			want: true,
		},
		{
			name: "annotation false",
			args: args{
				cluster: &mgmtv3.Cluster{
					Annotations: map[string]string{
						mgmtv3.ClusterImportedVersionManagementAnnotation: "false",
					},
				},
			},
			want: false,
		},
		{
			name: "annotation system-default with setting true",
			args: args{
				cluster: &mgmtv3.Cluster{
					Annotations: map[string]string{
						mgmtv3.ClusterImportedVersionManagementAnnotation: "system-default",
					},
				},
			},
			want: true,
		},
		{
			name: "missing annotation with setting true",
			args: args{
				cluster: &mgmtv3.Cluster{
					Annotations: map[string]string{
						"another-anno": "true",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ImportedVersionManagementEnabled(tt.args.cluster); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPaused(t *testing.T) {
	tests := []struct {
		name    string
		cluster *mgmtv3.Cluster
		want    bool
	}{
		{
			name:    "nil cluster",
			cluster: nil,
			want:    false,
		},
		{
			name:    "no annotations",
			cluster: &mgmtv3.Cluster{},
			want:    false,
		},
		{
			name:    "annotation true",
			cluster: clusterWithAnnotations(map[string]string{mgmtv3.ClusterImportedVersionManagementPausedAnnotation: "true"}),
			want:    true,
		},
		{
			name:    "annotation false",
			cluster: clusterWithAnnotations(map[string]string{mgmtv3.ClusterImportedVersionManagementPausedAnnotation: "false"}),
			want:    false,
		},
		{
			name:    "unrecognised value",
			cluster: clusterWithAnnotations(map[string]string{mgmtv3.ClusterImportedVersionManagementPausedAnnotation: "system-default"}),
			want:    false,
		},
		{
			name:    "unrelated annotation",
			cluster: clusterWithAnnotations(map[string]string{mgmtv3.ClusterImportedVersionManagementAnnotation: "true"}),
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ImportedVersionManagementPaused(tt.cluster); got != tt.want {
				t.Errorf("Paused() = %v, want %v", got, tt.want)
			}
		})
	}
}

func clusterWithAnnotations(annotations map[string]string) *mgmtv3.Cluster {
	return &mgmtv3.Cluster{
		Annotations: annotations,
	}
}

package chart_test

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

var (
	errUnimplemented = fmt.Errorf("unimplemented")
	errNotFound      = fmt.Errorf("not found")
)

const priorityClassName = "rancher-critical"

func TestGetPriorityClassNameFromRancherConfigMap(t *testing.T) {
	ctrl := gomock.NewController(t)
	configCache := fake.NewMockCacheInterface[*v1.ConfigMap](ctrl)
	configCache.EXPECT().Get(namespace.System, "set-config").Return(&v1.ConfigMap{Data: map[string]string{"priorityClassName": priorityClassName}}, nil).AnyTimes()
	configCache.EXPECT().Get(namespace.System, "empty-config").Return(&v1.ConfigMap{}, nil).AnyTimes()
	configCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("not found")).AnyTimes()

	tests := []*struct {
		name    string
		want    string
		wantErr bool
		setup   func()
	}{
		// base case config map set.
		{
			name:    "correctly set priority class name",
			want:    priorityClassName,
			wantErr: false,
			setup:   func() { settings.ConfigMapName.Set("set-config") },
		},
		// config map name is empty.
		{
			name:    "empty configMap name",
			want:    "",
			wantErr: true,
			setup:   func() { settings.ConfigMapName.Set("") },
		},
		// config map doesn't exist.
		{
			name:    "unknown config map name",
			want:    "",
			wantErr: true,
			setup:   func() { settings.ConfigMapName.Set("unknown-config-name") },
		},
		// config map exist doesn't have priority class.
		{
			name:    "empty config map",
			want:    "",
			wantErr: true,
			setup:   func() { settings.ConfigMapName.Set("empty-config") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			getter := chart.RancherConfigGetter{configCache}
			got, err := getter.GetPriorityClassName()
			if tt.wantErr {
				assert.Error(t, err, "Expected test to error.")
				return
			}
			assert.NoError(t, err, "failed to get priority class.")
			assert.Equal(t, tt.want, got, "Unexpected priorityClassName returned")
		})
	}
}

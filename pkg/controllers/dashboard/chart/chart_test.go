package chart_test

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	errUnimplemented = fmt.Errorf("unimplemented")
	errNotFound      = fmt.Errorf("not found")
)

const priorityClassName = "rancher-critical"

func TestGetPriorityClassNameFromRancherConfigMap(t *testing.T) {
	configCache := &mockCache{
		Maps: []*v1.ConfigMap{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "set-config",
					Namespace: namespace.System,
				},
				Data: map[string]string{"priorityClassName": priorityClassName},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-config",
					Namespace: namespace.System,
				},
			},
		},
	}

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

func TestGetPSPEnablementFromRancherConfigMap(t *testing.T) {
	t.Parallel()
	configCache := &mockCache{
		Maps: []*v1.ConfigMap{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "set-config",
					Namespace: namespace.System,
				},
				Data: map[string]string{"pspEnablement": "true"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-config",
					Namespace: namespace.System,
				},
			},
		},
	}

	tests := []*struct {
		name    string
		want    string
		wantErr bool
		setup   func()
	}{
		// base case config map set.
		{
			name:    "correctly set PSP enablement value",
			want:    "true",
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
		// config map exist doesn't have PSP enablement value.
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
			getter := chart.RancherConfigGetter{ConfigCache: configCache}
			got, err := getter.GetPSPEnablement()
			if tt.wantErr {
				assert.Error(t, err, "Expected test to error.")
				return
			}
			assert.NoError(t, err, "failed to get PSP enablement value.")
			assert.Equal(t, tt.want, got, "Unexpected PSP enablement value returned")
		})
	}
}

type mockCache struct {
	Maps []*v1.ConfigMap
}

func (m *mockCache) Get(namespace, name string) (*v1.ConfigMap, error) {
	for _, configMap := range m.Maps {
		if configMap.Name == name && configMap.Namespace == namespace {
			return configMap, nil
		}
	}
	return nil, errNotFound
}
func (m *mockCache) List(namespace string, selector labels.Selector) ([]*v1.ConfigMap, error) {
	return m.Maps, nil
}

func (m *mockCache) AddIndexer(indexName string, indexer corev1.ConfigMapIndexer) {}
func (m *mockCache) GetByIndex(indexName, key string) ([]*v1.ConfigMap, error) {
	return nil, errUnimplemented
}

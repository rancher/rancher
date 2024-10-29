package rancher

import (
	"errors"
	rancherversion "github.com/rancher/rancher/pkg/version"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
)

func TestVersionTombstone(t *testing.T) {
	tests := []struct {
		name        string
		expectError bool
		expected    map[string]string
		setup       func(*v1.ConfigMap, *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList], *fake.MockCacheInterface[*v1.ConfigMap])
	}{
		{
			name:        "error config map",
			expectError: true,
			setup: func(_ *v1.ConfigMap, _ *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList], cache *fake.MockCacheInterface[*v1.ConfigMap]) {
				cache.EXPECT().Get(cattleNamespace, rancherVersionTombstoneConfig).Return(nil, errors.New("error getting config map"))
			},
		},
		{
			name:        "dev version",
			expectError: false,
			setup: func(_ *v1.ConfigMap, _ *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList], cache *fake.MockCacheInterface[*v1.ConfigMap]) {
				rancherversion.Version = "dev"
				cache.EXPECT().Get(cattleNamespace, rancherVersionTombstoneConfig).Return(&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cattleNamespace,
						Name:      rancherVersionTombstoneConfig,
					},
					Data: map[string]string{},
				}, nil)
			},
		},
		{
			name:        "no last version",
			expectError: false,
			expected: map[string]string{
				rancherVersionKey: "v2.11.0",
			},
			setup: func(cm *v1.ConfigMap, controller *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList], cache *fake.MockCacheInterface[*v1.ConfigMap]) {
				rancherversion.Version = "v2.11.0"
				cache.EXPECT().Get(cattleNamespace, rancherVersionTombstoneConfig).Return(&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cattleNamespace,
						Name:      rancherVersionTombstoneConfig,
					},
					Data: map[string]string{},
				}, nil)
				controller.EXPECT().Create(gomock.Any()).DoAndReturn(func(obj *v1.ConfigMap) (*v1.ConfigMap, error) {
					*cm = *obj
					return obj, nil
				}).AnyTimes()
			},
		},
		{
			name:        "invalid version",
			expectError: false,
			expected: map[string]string{
				rancherVersionKey: "not-a-version",
			},
			setup: func(cm *v1.ConfigMap, controller *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList], cache *fake.MockCacheInterface[*v1.ConfigMap]) {
				rancherversion.Version = "not-a-version"
				cache.EXPECT().Get(cattleNamespace, rancherVersionTombstoneConfig).Return(&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cattleNamespace,
						Name:      rancherVersionTombstoneConfig,
					},
					Data: map[string]string{},
				}, nil)
				controller.EXPECT().Create(gomock.Any()).DoAndReturn(func(obj *v1.ConfigMap) (*v1.ConfigMap, error) {
					*cm = *obj
					return obj, nil
				}).AnyTimes()
			},
		},
		{
			name:        "prerelease version",
			expectError: false,
			expected: map[string]string{
				rancherVersionKey: "v2.11.0-alpha1",
			},
			setup: func(cm *v1.ConfigMap, controller *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList], cache *fake.MockCacheInterface[*v1.ConfigMap]) {
				rancherversion.Version = "v2.11.0-alpha1"
				cache.EXPECT().Get(cattleNamespace, rancherVersionTombstoneConfig).Return(&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cattleNamespace,
						Name:      rancherVersionTombstoneConfig,
					},
					Data: map[string]string{},
				}, nil)
				controller.EXPECT().Create(gomock.Any()).DoAndReturn(func(obj *v1.ConfigMap) (*v1.ConfigMap, error) {
					*cm = *obj
					return obj, nil
				}).AnyTimes()
			},
		},
		{
			name:        "last version invalid",
			expectError: false,
			expected: map[string]string{
				rancherVersionKey: "v2.11.0",
			},
			setup: func(cm *v1.ConfigMap, controller *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList], cache *fake.MockCacheInterface[*v1.ConfigMap]) {
				rancherversion.Version = "v2.11.0"
				cache.EXPECT().Get(cattleNamespace, rancherVersionTombstoneConfig).Return(&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       cattleNamespace,
						Name:            rancherVersionTombstoneConfig,
						ResourceVersion: "1",
					},
					Data: map[string]string{
						rancherVersionKey: "not-a-version",
					},
				}, nil)
				controller.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v1.ConfigMap) (*v1.ConfigMap, error) {
					*cm = *obj
					return obj, nil
				}).AnyTimes()
			},
		},
		{
			name:        "last version pre-release",
			expectError: false,
			expected: map[string]string{
				rancherVersionKey: "v2.11.0",
			},
			setup: func(cm *v1.ConfigMap, controller *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList], cache *fake.MockCacheInterface[*v1.ConfigMap]) {
				rancherversion.Version = "v2.11.0"
				cache.EXPECT().Get(cattleNamespace, rancherVersionTombstoneConfig).Return(&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       cattleNamespace,
						Name:            rancherVersionTombstoneConfig,
						ResourceVersion: "1",
					},
					Data: map[string]string{
						rancherVersionKey: "v2.11.1-alpha1",
					},
				}, nil)
				controller.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v1.ConfigMap) (*v1.ConfigMap, error) {
					*cm = *obj
					return obj, nil
				}).AnyTimes()
			},
		},
		{
			name:        "last version lesser",
			expectError: true,
			setup: func(cm *v1.ConfigMap, controller *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList], cache *fake.MockCacheInterface[*v1.ConfigMap]) {
				rancherversion.Version = "v2.10.0"
				cache.EXPECT().Get(cattleNamespace, rancherVersionTombstoneConfig).Return(&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       cattleNamespace,
						Name:            rancherVersionTombstoneConfig,
						ResourceVersion: "1",
					},
					Data: map[string]string{
						rancherVersionKey: "v2.11.0",
					},
				}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			configMapController := fake.NewMockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList](ctrl)
			configMapCache := fake.NewMockCacheInterface[*v1.ConfigMap](ctrl)
			configMapController.EXPECT().Cache().Return(configMapCache)
			cm := &v1.ConfigMap{}
			configMapController.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v1.ConfigMap) (*v1.ConfigMap, error) {
				cm = obj
				return obj, nil
			}).AnyTimes()

			tt.setup(cm, configMapController, configMapCache)
			err := versionTombstone(configMapController)
			if err == nil && tt.expectError {
				t.Errorf("versionTombstone should return error")
			} else if err != nil && !tt.expectError {
				t.Errorf("versionTombstone should not return error: %v", err)
			} else if tt.expected != nil {
				assert.True(t, reflect.DeepEqual(cm.Data, tt.expected))
			} else {
				assert.Equal(t, cm.Data[rancherVersionKey], tt.expected[rancherVersionKey])
			}
		})
	}
}

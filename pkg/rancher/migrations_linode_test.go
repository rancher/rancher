package rancher

import (
	"errors"
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rancherversion "github.com/rancher/rancher/pkg/version"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1wrangler "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// stubProvisioningV1 implements provisioningv1.Interface by returning a
// pre-set ClusterController. All other interface methods panic — they should
// never be called by the migration.
type stubProvisioningV1 struct {
	provisioningv1.Interface // embed for any uncalled methods
	clusterCtrl              provisioningv1.ClusterController
}

func (s *stubProvisioningV1) Cluster() provisioningv1.ClusterController {
	return s.clusterCtrl
}

// stubMgmtV3 implements managementv3.Interface by returning a pre-set
// NodeDriverController. All other interface methods delegate to nil (panic if
// called — they should not be reached by the migration).
type stubMgmtV3 struct {
	managementv3.Interface // embed for any uncalled methods
	nodeDriverCtrl         managementv3.NodeDriverController
}

func (s *stubMgmtV3) NodeDriver() managementv3.NodeDriverController {
	return s.nodeDriverCtrl
}

// stubCoreV1 implements corev1wrangler.Interface by returning a pre-set
// ConfigMapController.
type stubCoreV1 struct {
	corev1wrangler.Interface // embed for any uncalled methods
	configMapCtrl            corev1wrangler.ConfigMapController
}

func (s *stubCoreV1) ConfigMap() corev1wrangler.ConfigMapController {
	return s.configMapCtrl
}

func newTestWranglerCtx(
	provCtrl provisioningv1.ClusterController,
	ndCtrl managementv3.NodeDriverController,
	cmCtrl corev1wrangler.ConfigMapController,
) *wrangler.Context {
	return &wrangler.Context{
		Provisioning: &stubProvisioningV1{clusterCtrl: provCtrl},
		Mgmt:         &stubMgmtV3{nodeDriverCtrl: ndCtrl},
		Core:         &stubCoreV1{configMapCtrl: cmCtrl},
	}
}

func TestIsLinodeNodeDriverInUse(t *testing.T) {
	t.Parallel()

	linodePool := provv1.RKEMachinePool{
		NodeConfig: &corev1.ObjectReference{APIVersion: capr.DefaultMachineConfigAPIVersion, Kind: "LinodeConfig"},
	}
	otherPool := provv1.RKEMachinePool{
		NodeConfig: &corev1.ObjectReference{APIVersion: capr.DefaultMachineConfigAPIVersion, Kind: "Amazonec2Config"},
	}

	tests := []struct {
		name     string
		clusters []provv1.Cluster
		listErr  error
		want     bool
		wantErr  bool
	}{
		{
			name:    "no clusters",
			want:    false,
			wantErr: false,
		},
		{
			name: "cluster with nil RKEConfig",
			clusters: []provv1.Cluster{
				{Spec: provv1.ClusterSpec{RKEConfig: nil}},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "cluster with empty MachinePools",
			clusters: []provv1.Cluster{
				{Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{}}},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "pool with nil NodeConfig",
			clusters: []provv1.Cluster{
				{
					Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{
						MachinePools: []provv1.RKEMachinePool{{NodeConfig: nil}},
					}},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "pool with non-linode kind",
			clusters: []provv1.Cluster{
				{Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{
					MachinePools: []provv1.RKEMachinePool{otherPool},
				}}},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "pool with LinodeConfig kind",
			clusters: []provv1.Cluster{
				{Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{
					MachinePools: []provv1.RKEMachinePool{linodePool},
				}}},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "mixed pools - one is LinodeConfig",
			clusters: []provv1.Cluster{
				{Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{
					MachinePools: []provv1.RKEMachinePool{otherPool, linodePool},
				}}},
			},
			want:    true,
			wantErr: false,
		},
		{
			name:    "list error propagates",
			listErr: errors.New("api error"),
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			mockCluster := ctrlfake.NewMockControllerInterface[*provv1.Cluster, *provv1.ClusterList](ctrl)
			mockCluster.EXPECT().List("", metav1.ListOptions{}).Return(
				&provv1.ClusterList{Items: tt.clusters},
				tt.listErr,
			)

			w := &wrangler.Context{
				Provisioning: &stubProvisioningV1{clusterCtrl: mockCluster},
			}

			got, err := isLinodeNodeDriverInUse(w)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDisableUnusedLinodeNodeDriver(t *testing.T) {
	origVersion := rancherversion.Version
	t.Cleanup(func() { rancherversion.Version = origVersion })

	linodePool := provv1.RKEMachinePool{
		NodeConfig: &corev1.ObjectReference{APIVersion: capr.DefaultMachineConfigAPIVersion, Kind: "LinodeConfig"},
	}
	otherPool := provv1.RKEMachinePool{
		NodeConfig: &corev1.ObjectReference{APIVersion: capr.DefaultMachineConfigAPIVersion, Kind: "Amazonec2Config"},
	}

	newNodeDriver := func(active bool) *mgmtv3.NodeDriver {
		return &mgmtv3.NodeDriver{
			ObjectMeta: metav1.ObjectMeta{Name: linodeNodeDriverName, ResourceVersion: "1"},
			Spec:       mgmtv3.NodeDriverSpec{Active: active},
		}
	}

	newConfigMap := func(data map[string]string) *corev1.ConfigMap {
		cmData := map[string]string{}
		for k, v := range data {
			cmData[k] = v
		}

		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            disableUnusedLinodeNodeDriverConfigMap,
				Namespace:       cattleNamespace,
				ResourceVersion: "1",
			},
			Data: cmData,
		}
	}

	tests := []struct {
		name                   string
		version                string
		storedCM               *corev1.ConfigMap
		clusters               []provv1.Cluster
		clusterListErr         error
		nodeDriver             *mgmtv3.NodeDriver
		nodeDriverGetErr       error
		nodeDriverUpdateErr    error
		expectClusterList      bool
		expectNodeDriverGet    bool
		expectNodeDriverUpdate bool
		wantNodeDriverDisabled bool
		wantMarkerWrite        bool
		wantErr                string
	}{
		{
			name:            "dev build is a no-op",
			version:         "dev",
			wantMarkerWrite: false,
		},
		{
			name:            "marker already set is a complete no-op",
			version:         "v2.15.0",
			storedCM:        newConfigMap(map[string]string{unusedLinodeNodeDriverDisabledKey: "true"}),
			wantMarkerWrite: false,
		},
		{
			name:                   "not in use with no clusters disables driver and creates marker",
			version:                "v2.15.0",
			nodeDriver:             newNodeDriver(true),
			expectClusterList:      true,
			expectNodeDriverGet:    true,
			expectNodeDriverUpdate: true,
			wantNodeDriverDisabled: true,
			wantMarkerWrite:        true,
		},
		{
			name:     "non-linode cluster still disables driver and updates existing marker configmap",
			version:  "v2.15.0",
			storedCM: newConfigMap(nil),
			clusters: []provv1.Cluster{{
				Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{MachinePools: []provv1.RKEMachinePool{otherPool}}},
			}},
			nodeDriver:             newNodeDriver(true),
			expectClusterList:      true,
			expectNodeDriverGet:    true,
			expectNodeDriverUpdate: true,
			wantNodeDriverDisabled: true,
			wantMarkerWrite:        true,
		},
		{
			name:                "inactive driver only writes marker",
			version:             "v2.15.0",
			nodeDriver:          newNodeDriver(false),
			expectClusterList:   true,
			expectNodeDriverGet: true,
			wantMarkerWrite:     true,
		},
		{
			name:                "missing node driver only writes marker",
			version:             "v2.15.0",
			nodeDriverGetErr:    apierrors.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "nodedrivers"}, linodeNodeDriverName),
			expectClusterList:   true,
			expectNodeDriverGet: true,
			wantMarkerWrite:     true,
		},
		{
			name:    "linode cluster leaves driver active and still writes marker",
			version: "v2.15.0",
			clusters: []provv1.Cluster{{
				Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{MachinePools: []provv1.RKEMachinePool{linodePool}}},
			}},
			expectClusterList: true,
			wantMarkerWrite:   true,
		},
		{
			name:              "cluster list error is returned without writing marker",
			version:           "v2.15.0",
			clusterListErr:    errors.New("list failed"),
			expectClusterList: true,
			wantErr:           "isLinodeNodeDriverInUse: failed to list provisioning clusters",
		},
		{
			name:                   "nodedriver update error is returned without writing marker",
			version:                "v2.15.0",
			nodeDriver:             newNodeDriver(true),
			nodeDriverUpdateErr:    errors.New("update failed"),
			expectClusterList:      true,
			expectNodeDriverGet:    true,
			expectNodeDriverUpdate: true,
			wantNodeDriverDisabled: true,
			wantErr:                "disableUnusedLinodeNodeDriver: failed to deactivate linode NodeDriver",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			rancherversion.Version = tt.version

			ctrl := gomock.NewController(t)

			mockCM := ctrlfake.NewMockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
			mockCMCache := ctrlfake.NewMockCacheInterface[*corev1.ConfigMap](ctrl)
			mockCM.EXPECT().Cache().Return(mockCMCache).AnyTimes()

			var cachedCM *corev1.ConfigMap
			if tt.storedCM != nil {
				cachedCM = tt.storedCM.DeepCopy()
			}
			mockCMCache.EXPECT().Get(cattleNamespace, disableUnusedLinodeNodeDriverConfigMap).Return(cachedCM, nil)

			markerWrites := 0
			if tt.wantMarkerWrite {
				assertMarkerWrite := func(cm *corev1.ConfigMap) {
					t.Helper()
					markerWrites++
					require.Equal(t, disableUnusedLinodeNodeDriverConfigMap, cm.Name)
					require.Equal(t, cattleNamespace, cm.Namespace)
					require.Equal(t, "true", cm.Data[unusedLinodeNodeDriverDisabledKey])
				}

				if tt.storedCM != nil {
					mockCM.EXPECT().Update(gomock.Any()).DoAndReturn(func(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
						assertMarkerWrite(cm)
						return cm, nil
					})
				} else {
					mockCM.EXPECT().Create(gomock.Any()).DoAndReturn(func(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
						assertMarkerWrite(cm)
						return cm, nil
					})
				}
			}

			mockCluster := ctrlfake.NewMockControllerInterface[*provv1.Cluster, *provv1.ClusterList](ctrl)
			if tt.expectClusterList {
				mockCluster.EXPECT().List("", metav1.ListOptions{}).Return(
					&provv1.ClusterList{Items: tt.clusters},
					tt.clusterListErr,
				)
			}

			var nodeDriverDisabled bool
			mockND := ctrlfake.NewMockNonNamespacedControllerInterface[*mgmtv3.NodeDriver, *mgmtv3.NodeDriverList](ctrl)
			if tt.expectNodeDriverGet {
				if tt.nodeDriverGetErr != nil {
					mockND.EXPECT().Get(linodeNodeDriverName, metav1.GetOptions{}).Return(nil, tt.nodeDriverGetErr)
				} else {
					mockND.EXPECT().Get(linodeNodeDriverName, metav1.GetOptions{}).Return(tt.nodeDriver.DeepCopy(), nil)
				}
			}
			if tt.expectNodeDriverUpdate {
				mockND.EXPECT().Update(gomock.Any()).DoAndReturn(func(nd *mgmtv3.NodeDriver) (*mgmtv3.NodeDriver, error) {
					assert.False(t, nd.Spec.Active, "NodeDriver should be disabled before update")
					nodeDriverDisabled = true
					return nd, tt.nodeDriverUpdateErr
				})
			}

			w := newTestWranglerCtx(mockCluster, mockND, mockCM)

			err := disableUnusedLinodeNodeDriver(w)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, tt.wantNodeDriverDisabled, nodeDriverDisabled)
				assert.Zero(t, markerWrites)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNodeDriverDisabled, nodeDriverDisabled)
			if tt.wantMarkerWrite {
				assert.Equal(t, 1, markerWrites)
			} else {
				assert.Zero(t, markerWrites)
			}
		})
	}
}

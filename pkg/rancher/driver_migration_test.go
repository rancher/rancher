package rancher

import (
	"errors"
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	crdv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiextensions.k8s.io/v1"
	corev1wrangler "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type stubProvisioningV1ForLinode struct {
	provisioningv1.Interface
	clusterCtrl provisioningv1.ClusterController
}

func (s *stubProvisioningV1ForLinode) Cluster() provisioningv1.ClusterController {
	return s.clusterCtrl
}

type stubCoreV1ForLinode struct {
	corev1wrangler.Interface
	configMapCtrl corev1wrangler.ConfigMapController
}

func (s *stubCoreV1ForLinode) ConfigMap() corev1wrangler.ConfigMapController {
	return s.configMapCtrl
}

type stubMgmtV3ForLinode struct {
	managementv3.Interface
	nodeDriverCtrl    managementv3.NodeDriverController
	dynamicSchemaCtrl managementv3.DynamicSchemaController
}

func (s *stubMgmtV3ForLinode) NodeDriver() managementv3.NodeDriverController {
	return s.nodeDriverCtrl
}

func (s *stubMgmtV3ForLinode) DynamicSchema() managementv3.DynamicSchemaController {
	return s.dynamicSchemaCtrl
}

type stubCRDForLinode struct {
	crdv1.Interface
	crdCtrl crdv1.CustomResourceDefinitionController
}

func (s *stubCRDForLinode) CustomResourceDefinition() crdv1.CustomResourceDefinitionController {
	return s.crdCtrl
}

func newLinodeWranglerCtx(
	clusterCtrl provisioningv1.ClusterController,
	cmCtrl corev1wrangler.ConfigMapController,
	ndCtrl managementv3.NodeDriverController,
	dsCtrl managementv3.DynamicSchemaController,
	crdCtrl crdv1.CustomResourceDefinitionController,
) *wrangler.Context {
	return &wrangler.Context{
		Provisioning: &stubProvisioningV1ForLinode{clusterCtrl: clusterCtrl},
		Core:         &stubCoreV1ForLinode{configMapCtrl: cmCtrl},
		Mgmt:         &stubMgmtV3ForLinode{nodeDriverCtrl: ndCtrl, dynamicSchemaCtrl: dsCtrl},
		CRD:          &stubCRDForLinode{crdCtrl: crdCtrl},
	}
}

func linodePool() provv1.RKEMachinePool {
	return provv1.RKEMachinePool{
		NodeConfig: &corev1.ObjectReference{
			APIVersion: capr.DefaultMachineConfigAPIVersion,
			Kind:       linodeMachineConfigKind,
		},
	}
}

func otherPool() provv1.RKEMachinePool {
	return provv1.RKEMachinePool{
		NodeConfig: &corev1.ObjectReference{
			APIVersion: capr.DefaultMachineConfigAPIVersion,
			Kind:       "Amazonec2Config",
		},
	}
}

func markerCM(marked bool) *corev1.ConfigMap {
	data := map[string]string{}
	if marked {
		data[linodeDisableCheckKey] = "true"
	}
	return &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:            linodeDisableCheckConfigMap,
			Namespace:       cattleNamespace,
			ResourceVersion: "1",
		},
		Data: data,
	}
}

func markerNotFound() error {
	return apierrors.NewNotFound(
		schema.GroupResource{Group: "", Resource: "configmaps"},
		linodeDisableCheckConfigMap,
	)
}

func activeNodeDriver() *mgmtv3.NodeDriver {
	return &mgmtv3.NodeDriver{
		ObjectMeta: v1.ObjectMeta{Name: linodeDriver, ResourceVersion: "1"},
		Spec:       mgmtv3.NodeDriverSpec{Active: true},
	}
}

// parentSchemaWithField returns a minimal DynamicSchema that has a given field.
func parentSchemaWithField(name, field string) *mgmtv3.DynamicSchema {
	return &mgmtv3.DynamicSchema{
		ObjectMeta: v1.ObjectMeta{Name: name, ResourceVersion: "1"},
		Spec: mgmtv3.DynamicSchemaSpec{
			ResourceFields: map[string]mgmtv3.Field{field: {}},
		},
	}
}

// setupTeardownExpectations sets up the mock expectations for the full linode
// teardown path (deactivate NodeDriver, delete schemas, remove fields, delete
// CRDs). All operations tolerate NotFound.
func setupTeardownExpectations(
	ctrl *gomock.Controller,
	mockND *ctrlfake.MockNonNamespacedControllerInterface[*mgmtv3.NodeDriver, *mgmtv3.NodeDriverList],
	mockDS *ctrlfake.MockNonNamespacedControllerInterface[*mgmtv3.DynamicSchema, *mgmtv3.DynamicSchemaList],
	mockCRD *ctrlfake.MockNonNamespacedControllerInterface[*apiextv1.CustomResourceDefinition, *apiextv1.CustomResourceDefinitionList],
	ndActive bool,
) {
	// NodeDriver: Get + Update (if active)
	if ndActive {
		mockND.EXPECT().Get(linodeDriver, v1.GetOptions{}).Return(activeNodeDriver(), nil)
		mockND.EXPECT().Update(gomock.Any()).DoAndReturn(func(nd *mgmtv3.NodeDriver) (*mgmtv3.NodeDriver, error) {
			return nd, nil
		})
	} else {
		mockND.EXPECT().Get(linodeDriver, v1.GetOptions{}).Return(nil,
			apierrors.NewNotFound(schema.GroupResource{}, linodeDriver))
	}

	// DynamicSchema deletes (linodeconfig, linodecredentialconfig)
	mockDS.EXPECT().Delete(linodeConfigSchemaName, gomock.Any()).Return(nil)
	mockDS.EXPECT().Delete(linodeCredentialConfigSchemaName, gomock.Any()).Return(nil)

	// Parent schema field removals: Get + Update for each
	for _, pf := range []struct{ schema, field string }{
		{linodeParentNodeConfig, linodeConfigField},
		{linodeParentNodeTemplate, linodeConfigField},
		{linodeParentCredential, linodeCredentialField},
	} {
		mockDS.EXPECT().Get(pf.schema, v1.GetOptions{}).
			Return(parentSchemaWithField(pf.schema, pf.field), nil)
		mockDS.EXPECT().Update(gomock.Any()).Return(&mgmtv3.DynamicSchema{}, nil)
	}

	// CRD deletes
	for _, crdName := range []string{linodeConfigCRD, linodeMachineCRD, linodeMachineTemplateCRD} {
		mockCRD.EXPECT().Delete(crdName, gomock.Any()).Return(nil)
	}
}

func TestLinodeNodeDriverInUse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		clusters []provv1.Cluster
		listErr  error
		want     bool
		wantErr  bool
	}{
		{name: "no clusters", want: false},
		{
			name:     "cluster with nil RKEConfig",
			clusters: []provv1.Cluster{{Spec: provv1.ClusterSpec{RKEConfig: nil}}},
			want:     false,
		},
		{
			name: "cluster with empty MachinePools",
			clusters: []provv1.Cluster{
				{Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{}}},
			},
			want: false,
		},
		{
			name: "pool with nil NodeConfig",
			clusters: []provv1.Cluster{
				{Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{
					MachinePools: []provv1.RKEMachinePool{{NodeConfig: nil}},
				}}},
			},
			want: false,
		},
		{
			name: "kind matches but APIVersion empty",
			clusters: []provv1.Cluster{
				{Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{
					MachinePools: []provv1.RKEMachinePool{
						{NodeConfig: &corev1.ObjectReference{Kind: linodeMachineConfigKind}},
					},
				}}},
			},
			want: true,
		},
		{
			name: "non-linode kind",
			clusters: []provv1.Cluster{
				{Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{
					MachinePools: []provv1.RKEMachinePool{otherPool()},
				}}},
			},
			want: false,
		},
		{
			name: "LinodeConfig with correct APIVersion",
			clusters: []provv1.Cluster{
				{Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{
					MachinePools: []provv1.RKEMachinePool{linodePool()},
				}}},
			},
			want: true,
		},
		{
			name: "mixed pools",
			clusters: []provv1.Cluster{
				{Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{
					MachinePools: []provv1.RKEMachinePool{otherPool(), linodePool()},
				}}},
			},
			want: true,
		},
		{
			name:    "list error propagates",
			listErr: errors.New("api error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mockCluster := ctrlfake.NewMockControllerInterface[*provv1.Cluster, *provv1.ClusterList](ctrl)
			mockCluster.EXPECT().List("", v1.ListOptions{}).Return(
				&provv1.ClusterList{Items: tt.clusters}, tt.listErr)
			w := &wrangler.Context{
				Provisioning: &stubProvisioningV1ForLinode{clusterCtrl: mockCluster},
			}
			got, err := linodeNodeDriverInUse(w)
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
	tests := []struct {
		name            string
		storedCM        *corev1.ConfigMap // nil → NotFound
		clusters        []provv1.Cluster
		clusterListErr  error
		expectTeardown  bool
		ndActive        bool
		wantMarkerWrite bool
		wantErr         bool
	}{
		{
			name:            "marker already set – complete no-op",
			storedCM:        markerCM(true),
			wantMarkerWrite: false,
		},
		{
			name:            "not in use, active driver – full teardown, new marker CM",
			storedCM:        nil,
			expectTeardown:  true,
			ndActive:        true,
			wantMarkerWrite: true,
		},
		{
			name:            "not in use, NodeDriver NotFound – no teardown, marker written",
			storedCM:        nil,
			expectTeardown:  false,
			ndActive:        false, // causes NodeDriver.Get to return NotFound in setupTeardownExpectations
			wantMarkerWrite: true,
		},
		{
			name:            "not in use - teardown, existing marker CM updated",
			storedCM:        markerCM(false),
			expectTeardown:  true,
			ndActive:        true,
			wantMarkerWrite: true,
		},
		{
			name:     "in use – no teardown, only marker written",
			storedCM: nil,
			clusters: []provv1.Cluster{
				{Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{
					MachinePools: []provv1.RKEMachinePool{linodePool()},
				}}},
			},
			expectTeardown:  false,
			wantMarkerWrite: true,
		},
		{
			name:           "cluster list error – returns error, no marker written",
			storedCM:       nil,
			clusterListErr: errors.New("list failed"),
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// ConfigMap mock.
			mockCM := ctrlfake.NewMockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
			if tt.storedCM != nil {
				mockCM.EXPECT().
					Get(cattleNamespace, linodeDisableCheckConfigMap, v1.GetOptions{}).
					Return(tt.storedCM, nil)
			} else {
				mockCM.EXPECT().
					Get(cattleNamespace, linodeDisableCheckConfigMap, v1.GetOptions{}).
					Return(nil, markerNotFound())
			}

			markerWritten := false
			if tt.wantMarkerWrite {
				assertMarker := func(cm *corev1.ConfigMap) {
					assert.Equal(t, "true", cm.Data[linodeDisableCheckKey])
					markerWritten = true
				}
				if tt.storedCM != nil {
					mockCM.EXPECT().Update(gomock.Any()).DoAndReturn(func(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
						assertMarker(cm)
						return cm, nil
					})
				} else {
					mockCM.EXPECT().Create(gomock.Any()).DoAndReturn(func(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
						assertMarker(cm)
						return cm, nil
					})
				}
			}

			// Provisioning cluster mock (skipped when marker already present).
			markerPresent := tt.storedCM != nil && tt.storedCM.Data[linodeDisableCheckKey] == "true"
			mockCluster := ctrlfake.NewMockControllerInterface[*provv1.Cluster, *provv1.ClusterList](ctrl)
			if !markerPresent {
				mockCluster.EXPECT().List("", v1.ListOptions{}).Return(
					&provv1.ClusterList{Items: tt.clusters}, tt.clusterListErr)
			}

			// NodeDriver, DynamicSchema, and CRD mocks.
			mockND := ctrlfake.NewMockNonNamespacedControllerInterface[*mgmtv3.NodeDriver, *mgmtv3.NodeDriverList](ctrl)
			mockDS := ctrlfake.NewMockNonNamespacedControllerInterface[*mgmtv3.DynamicSchema, *mgmtv3.DynamicSchemaList](ctrl)
			mockCRD := ctrlfake.NewMockNonNamespacedControllerInterface[*apiextv1.CustomResourceDefinition, *apiextv1.CustomResourceDefinitionList](ctrl)

			if tt.expectTeardown {
				setupTeardownExpectations(ctrl, mockND, mockDS, mockCRD, tt.ndActive)
			} else if !markerPresent && tt.clusterListErr == nil {
				// Check if driver is actually in use to determine if we expect NodeDriver.Get to be called
				isLinodeInUse := false
				for _, cluster := range tt.clusters {
					if cluster.Spec.RKEConfig != nil {
						for _, pool := range cluster.Spec.RKEConfig.MachinePools {
							if pool.NodeConfig != nil && pool.NodeConfig.Kind == linodeMachineConfigKind {
								isLinodeInUse = true
								break
							}
						}
						if isLinodeInUse {
							break
						}
					}
				}
				// Only set up the expectation if the driver is not in use and NodeDriver doesn't exist
				if !isLinodeInUse && !tt.ndActive {
					mockND.EXPECT().Get(linodeDriver, v1.GetOptions{}).Return(nil,
						apierrors.NewNotFound(schema.GroupResource{}, linodeDriver))
				}
			}

			w := newLinodeWranglerCtx(mockCluster, mockCM, mockND, mockDS, mockCRD)

			err := disableUnusedLinodeNodeDriver(w)
			if tt.wantErr {
				require.Error(t, err)
				assert.False(t, markerWritten, "marker should not be written on error")
				return
			}
			require.NoError(t, err)
			if tt.wantMarkerWrite {
				assert.True(t, markerWritten)
			} else {
				assert.False(t, markerWritten)
			}
		})
	}
}

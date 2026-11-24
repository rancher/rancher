package provisioningcluster

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/lasso/pkg/dynamic"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	capictrl "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	mgmtctrl "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provctrl "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkectrl "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/condition"
	corectrl "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	wfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicclient "k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestProvisioningClusterController_reconcileConditions(t *testing.T) {
	type dummyObject struct {
		Status struct {
			Conditions []genericcondition.GenericCondition
		}
	}
	type args struct {
		conditionA                       condition.Cond
		conditionB                       condition.Cond
		objectAConditionAMessage         string
		objectAConditionAReason          string
		objectAConditionAStatus          string
		objectBConditionAMessage         string
		objectBConditionAReason          string
		objectBConditionAStatus          string
		objectAConditionBMessage         string
		objectAConditionBReason          string
		objectAConditionBStatus          string
		objectBConditionBMessage         string
		objectBConditionBReason          string
		objectBConditionBStatus          string
		expectedObjectAConditionAMessage string
		expectedObjectAConditionAReason  string
		expectedObjectAConditionAStatus  string
		expectedObjectBConditionAMessage string
		expectedObjectBConditionAReason  string
		expectedObjectBConditionAStatus  string
		expectedObjectAConditionBMessage string
		expectedObjectAConditionBReason  string
		expectedObjectAConditionBStatus  string
		expectedObjectBConditionBMessage string
		expectedObjectBConditionBReason  string
		expectedObjectBConditionBStatus  string
		expectChange                     bool
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "basic copy",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "myMessage",
				objectAConditionAReason:          "myReason",
				objectAConditionAStatus:          "myStatus",
				objectBConditionAMessage:         "",
				objectBConditionAReason:          "",
				objectBConditionAStatus:          "",
				objectAConditionBMessage:         "",
				objectAConditionBReason:          "",
				objectAConditionBStatus:          "",
				objectBConditionBMessage:         "",
				objectBConditionBReason:          "",
				objectBConditionBStatus:          "",
				expectedObjectAConditionAMessage: "myMessage",
				expectedObjectAConditionAReason:  "myReason",
				expectedObjectAConditionAStatus:  "myStatus",
				expectedObjectBConditionAMessage: "",
				expectedObjectBConditionAReason:  "",
				expectedObjectBConditionAStatus:  "",
				expectedObjectAConditionBMessage: "",
				expectedObjectAConditionBReason:  "",
				expectedObjectAConditionBStatus:  "",
				expectedObjectBConditionBMessage: "myMessage",
				expectedObjectBConditionBReason:  "myReason",
				expectedObjectBConditionBStatus:  "myStatus",
				expectChange:                     true,
			},
		},
		{
			name: "basic erase",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "",
				objectAConditionAReason:          "",
				objectAConditionAStatus:          "",
				objectBConditionAMessage:         "",
				objectBConditionAReason:          "",
				objectBConditionAStatus:          "",
				objectAConditionBMessage:         "",
				objectAConditionBReason:          "",
				objectAConditionBStatus:          "",
				objectBConditionBMessage:         "myMessage",
				objectBConditionBReason:          "myReason",
				objectBConditionBStatus:          "myStatus",
				expectedObjectAConditionAMessage: "",
				expectedObjectAConditionAReason:  "",
				expectedObjectAConditionAStatus:  "",
				expectedObjectBConditionAMessage: "",
				expectedObjectBConditionAReason:  "",
				expectedObjectBConditionAStatus:  "",
				expectedObjectAConditionBMessage: "",
				expectedObjectAConditionBReason:  "",
				expectedObjectAConditionBStatus:  "",
				expectedObjectBConditionBMessage: "",
				expectedObjectBConditionBReason:  "",
				expectedObjectBConditionBStatus:  "",
				expectChange:                     true,
			},
		},
		{
			name: "basic overwrite",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "wow",
				objectAConditionAReason:          "wowreason",
				objectAConditionAStatus:          "wowstatus",
				objectBConditionAMessage:         "",
				objectBConditionAReason:          "",
				objectBConditionAStatus:          "",
				objectAConditionBMessage:         "",
				objectAConditionBReason:          "",
				objectAConditionBStatus:          "",
				objectBConditionBMessage:         "myMessage",
				objectBConditionBReason:          "myReason",
				objectBConditionBStatus:          "myStatus",
				expectedObjectAConditionAMessage: "wow",
				expectedObjectAConditionAReason:  "wowreason",
				expectedObjectAConditionAStatus:  "wowstatus",
				expectedObjectBConditionAMessage: "",
				expectedObjectBConditionAReason:  "",
				expectedObjectBConditionAStatus:  "",
				expectedObjectAConditionBMessage: "",
				expectedObjectAConditionBReason:  "",
				expectedObjectAConditionBStatus:  "",
				expectedObjectBConditionBMessage: "wow",
				expectedObjectBConditionBReason:  "wowreason",
				expectedObjectBConditionBStatus:  "wowstatus",
				expectChange:                     true,
			},
		},
		{
			name: "copy around other conditions",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "myMessage",
				objectAConditionAReason:          "myReason",
				objectAConditionAStatus:          "myStatus",
				objectBConditionAMessage:         "donttouch",
				objectBConditionAReason:          "thiscondition",
				objectBConditionAStatus:          "becauseifyoudoitsbroken",
				objectAConditionBMessage:         "orthisone",
				objectAConditionBReason:          "andthisone",
				objectAConditionBStatus:          "alsothisone",
				objectBConditionBMessage:         "",
				objectBConditionBReason:          "",
				objectBConditionBStatus:          "",
				expectedObjectAConditionAMessage: "myMessage",
				expectedObjectAConditionAReason:  "myReason",
				expectedObjectAConditionAStatus:  "myStatus",
				expectedObjectBConditionAMessage: "donttouch",
				expectedObjectBConditionAReason:  "thiscondition",
				expectedObjectBConditionAStatus:  "becauseifyoudoitsbroken",
				expectedObjectAConditionBMessage: "orthisone",
				expectedObjectAConditionBReason:  "andthisone",
				expectedObjectAConditionBStatus:  "alsothisone",
				expectedObjectBConditionBMessage: "myMessage",
				expectedObjectBConditionBReason:  "myReason",
				expectedObjectBConditionBStatus:  "myStatus",
				expectChange:                     true,
			},
		},
		{
			name: "erase around other conditions",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "",
				objectAConditionAReason:          "",
				objectAConditionAStatus:          "",
				objectBConditionAMessage:         "donttouch",
				objectBConditionAReason:          "thiscondition",
				objectBConditionAStatus:          "becauseifyoudoitsbroken",
				objectAConditionBMessage:         "orthisone",
				objectAConditionBReason:          "andthisone",
				objectAConditionBStatus:          "alsothisone",
				objectBConditionBMessage:         "myMessage",
				objectBConditionBReason:          "myReason",
				objectBConditionBStatus:          "myStatus",
				expectedObjectAConditionAMessage: "",
				expectedObjectAConditionAReason:  "",
				expectedObjectAConditionAStatus:  "",
				expectedObjectBConditionAMessage: "donttouch",
				expectedObjectBConditionAReason:  "thiscondition",
				expectedObjectBConditionAStatus:  "becauseifyoudoitsbroken",
				expectedObjectAConditionBMessage: "orthisone",
				expectedObjectAConditionBReason:  "andthisone",
				expectedObjectAConditionBStatus:  "alsothisone",
				expectedObjectBConditionBMessage: "",
				expectedObjectBConditionBReason:  "",
				expectedObjectBConditionBStatus:  "",
				expectChange:                     true,
			},
		},
		{
			name: "overwrite around other conditions",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "wow",
				objectAConditionAReason:          "wowreason",
				objectAConditionAStatus:          "wowstatus",
				objectBConditionAMessage:         "donttouch",
				objectBConditionAReason:          "thiscondition",
				objectBConditionAStatus:          "becauseifyoudoitsbroken",
				objectAConditionBMessage:         "orthisone",
				objectAConditionBReason:          "andthisone",
				objectAConditionBStatus:          "alsothisone",
				objectBConditionBMessage:         "myMessage",
				objectBConditionBReason:          "myReason",
				objectBConditionBStatus:          "myStatus",
				expectedObjectAConditionAMessage: "wow",
				expectedObjectAConditionAReason:  "wowreason",
				expectedObjectAConditionAStatus:  "wowstatus",
				expectedObjectBConditionAMessage: "donttouch",
				expectedObjectBConditionAReason:  "thiscondition",
				expectedObjectBConditionAStatus:  "becauseifyoudoitsbroken",
				expectedObjectAConditionBMessage: "orthisone",
				expectedObjectAConditionBReason:  "andthisone",
				expectedObjectAConditionBStatus:  "alsothisone",
				expectedObjectBConditionBMessage: "wow",
				expectedObjectBConditionBReason:  "wowreason",
				expectedObjectBConditionBStatus:  "wowstatus",
				expectChange:                     true,
			},
		},
		{
			name: "no changes",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "wow",
				objectAConditionAReason:          "wowreason",
				objectAConditionAStatus:          "wowstatus",
				objectBConditionAMessage:         "donttouch",
				objectBConditionAReason:          "thiscondition",
				objectBConditionAStatus:          "becauseifyoudoitsbroken",
				objectAConditionBMessage:         "orthisone",
				objectAConditionBReason:          "andthisone",
				objectAConditionBStatus:          "alsothisone",
				objectBConditionBMessage:         "wow",
				objectBConditionBReason:          "wowreason",
				objectBConditionBStatus:          "wowstatus",
				expectedObjectAConditionAMessage: "wow",
				expectedObjectAConditionAReason:  "wowreason",
				expectedObjectAConditionAStatus:  "wowstatus",
				expectedObjectBConditionAMessage: "donttouch",
				expectedObjectBConditionAReason:  "thiscondition",
				expectedObjectBConditionAStatus:  "becauseifyoudoitsbroken",
				expectedObjectAConditionBMessage: "orthisone",
				expectedObjectAConditionBReason:  "andthisone",
				expectedObjectAConditionBStatus:  "alsothisone",
				expectedObjectBConditionBMessage: "wow",
				expectedObjectBConditionBReason:  "wowreason",
				expectedObjectBConditionBStatus:  "wowstatus",
				expectChange:                     false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests copying object A condition A to object B condition B.
			a := assert.New(t)
			var objectA = dummyObject{}
			var objectB = dummyObject{}

			if tt.args.objectAConditionAMessage != "" {
				tt.args.conditionA.Message(&objectA, tt.args.objectAConditionAMessage)
			}
			if tt.args.objectAConditionAReason != "" {
				tt.args.conditionA.Reason(&objectA, tt.args.objectAConditionAReason)
			}
			if tt.args.objectAConditionAStatus != "" {
				tt.args.conditionA.SetStatus(&objectA, tt.args.objectAConditionAStatus)
			}
			if tt.args.objectAConditionBMessage != "" {
				tt.args.conditionB.Message(&objectA, tt.args.objectAConditionBMessage)
			}
			if tt.args.objectAConditionBReason != "" {
				tt.args.conditionB.Reason(&objectA, tt.args.objectAConditionBReason)
			}
			if tt.args.objectAConditionBStatus != "" {
				tt.args.conditionB.SetStatus(&objectA, tt.args.objectAConditionBStatus)
			}
			if tt.args.objectBConditionAMessage != "" {
				tt.args.conditionA.Message(&objectB, tt.args.objectBConditionAMessage)
			}
			if tt.args.objectBConditionAReason != "" {
				tt.args.conditionA.Reason(&objectB, tt.args.objectBConditionAReason)
			}
			if tt.args.objectBConditionAStatus != "" {
				tt.args.conditionA.SetStatus(&objectB, tt.args.objectBConditionAStatus)
			}
			if tt.args.objectBConditionBMessage != "" {
				tt.args.conditionB.Message(&objectB, tt.args.objectBConditionBMessage)
			}
			if tt.args.objectBConditionBReason != "" {
				tt.args.conditionB.Reason(&objectB, tt.args.objectBConditionBReason)
			}
			if tt.args.objectBConditionBStatus != "" {
				tt.args.conditionB.SetStatus(&objectB, tt.args.objectBConditionBStatus)
			}

			a.Equal(tt.args.objectAConditionAMessage, tt.args.conditionA.GetMessage(&objectA))
			a.Equal(tt.args.objectAConditionAReason, tt.args.conditionA.GetReason(&objectA))
			a.Equal(tt.args.objectAConditionAStatus, tt.args.conditionA.GetStatus(&objectA))
			a.Equal(tt.args.objectAConditionBMessage, tt.args.conditionB.GetMessage(&objectA))
			a.Equal(tt.args.objectAConditionBReason, tt.args.conditionB.GetReason(&objectA))
			a.Equal(tt.args.objectAConditionBStatus, tt.args.conditionB.GetStatus(&objectA))

			a.Equal(tt.args.objectBConditionAMessage, tt.args.conditionA.GetMessage(&objectB))
			a.Equal(tt.args.objectBConditionAReason, tt.args.conditionA.GetReason(&objectB))
			a.Equal(tt.args.objectBConditionAStatus, tt.args.conditionA.GetStatus(&objectB))
			a.Equal(tt.args.objectBConditionBMessage, tt.args.conditionB.GetMessage(&objectB))
			a.Equal(tt.args.objectBConditionBReason, tt.args.conditionB.GetReason(&objectB))
			a.Equal(tt.args.objectBConditionBStatus, tt.args.conditionB.GetStatus(&objectB))

			a.Equal(tt.args.expectChange, reconcileCondition(&objectB, tt.args.conditionB, &objectA, tt.args.conditionA))

			a.Equal(tt.args.expectedObjectAConditionAMessage, tt.args.conditionA.GetMessage(&objectA))
			a.Equal(tt.args.expectedObjectAConditionAReason, tt.args.conditionA.GetReason(&objectA))
			a.Equal(tt.args.expectedObjectAConditionAStatus, tt.args.conditionA.GetStatus(&objectA))
			a.Equal(tt.args.expectedObjectAConditionBMessage, tt.args.conditionB.GetMessage(&objectA))
			a.Equal(tt.args.expectedObjectAConditionBReason, tt.args.conditionB.GetReason(&objectA))
			a.Equal(tt.args.expectedObjectAConditionBStatus, tt.args.conditionB.GetStatus(&objectA))

			a.Equal(tt.args.expectedObjectBConditionAMessage, tt.args.conditionA.GetMessage(&objectB))
			a.Equal(tt.args.expectedObjectBConditionAReason, tt.args.conditionA.GetReason(&objectB))
			a.Equal(tt.args.expectedObjectBConditionAStatus, tt.args.conditionA.GetStatus(&objectB))
			a.Equal(tt.args.expectedObjectBConditionBMessage, tt.args.conditionB.GetMessage(&objectB))
			a.Equal(tt.args.expectedObjectBConditionBReason, tt.args.conditionB.GetReason(&objectB))
			a.Equal(tt.args.expectedObjectBConditionBStatus, tt.args.conditionB.GetStatus(&objectB))
		})
	}

}

func TestGeneratingHandler(t *testing.T) {
	tests := []struct {
		name           string
		obj            *provv1.Cluster
		expected       []runtime.Object
		expectedStatus provv1.ClusterStatus
		expectedErr    error
	}{
		{
			name:           "no rkeconfig",
			obj:            &provv1.Cluster{},
			expected:       nil,
			expectedStatus: provv1.ClusterStatus{},
			expectedErr:    nil,
		},
		{
			name: "no cluster name",
			obj: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{},
				},
			},
			expected:       nil,
			expectedStatus: provv1.ClusterStatus{},
			expectedErr:    nil,
		},
		{
			name: "deleting",
			obj: &provv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &[]metav1.Time{metav1.Now()}[0],
				},
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{},
				},
				Status: provv1.ClusterStatus{
					ClusterName: "test-cluster",
				},
			},
			expected: nil,
			expectedStatus: provv1.ClusterStatus{
				ClusterName: "test-cluster",
			},
			expectedErr: nil,
		},
		{
			name: "k8s version",
			obj: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{},
				},
				Status: provv1.ClusterStatus{
					ClusterName: "test-cluster",
				},
			},
			expected: nil,
			expectedStatus: provv1.ClusterStatus{
				ClusterName: "test-cluster",
			},
			expectedErr: errors.New("kubernetesVersion not set on /"),
		},
		{
			name: "no finalizers",
			obj: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					KubernetesVersion: "test-version",
					RKEConfig:         &provv1.RKEConfig{},
				},
				Status: provv1.ClusterStatus{
					ClusterName: "test-cluster",
				},
			},
			expected: nil,
			expectedStatus: provv1.ClusterStatus{
				ClusterName: "test-cluster",
			},
			expectedErr: generic.ErrSkip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler{}
			var status provv1.ClusterStatus
			if tt.obj != nil {
				status = tt.obj.Status
			}
			objs, generatedStatus, err := h.OnRancherClusterChange(tt.obj, status)
			assert.Equal(t, tt.expected, objs)
			assert.Equal(t, tt.expectedStatus, generatedStatus)
			if tt.expectedErr != nil {
				assert.NotNil(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func buildSnapshotWithClusterSpec(t *testing.T, namespace, name string, spec provv1.ClusterSpec) *rkev1.ETCDSnapshot {
	t.Helper()
	inner := map[string]string{
		"provisioning-cluster-spec": func() string {
			raw, err := json.Marshal(spec)
			require.NoError(t, err)
			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			_, err = gz.Write(raw)
			require.NoError(t, err)
			require.NoError(t, gz.Close())
			return base64.StdEncoding.EncodeToString(buf.Bytes())
		}(),
	}
	metaJSON, err := json.Marshal(inner)
	require.NoError(t, err)
	return &rkev1.ETCDSnapshot{
		ObjectMeta:   metav1.ObjectMeta{Namespace: namespace, Name: name},
		SnapshotFile: rkev1.ETCDSnapshotFile{Metadata: base64.StdEncoding.EncodeToString(metaJSON)},
	}
}

func newBaseCluster(namespace, name, version string) *provv1.Cluster {
	return &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Finalizers: []string{"keep"}},
		Spec:       provv1.ClusterSpec{KubernetesVersion: version, RKEConfig: &provv1.RKEConfig{}},
		Status:     provv1.ClusterStatus{ClusterName: name},
	}
}

func buildCAPICluster(namespace, name string) *capi.Cluster {
	return &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: capi.ClusterSpec{
			ControlPlaneRef: capi.ContractVersionedObjectReference{
				Kind:     "RKEControlPlane",
				Name:     name,
				APIGroup: capr.RKEAPIGroup,
			},
		},
	}
}

func buildRKECP(namespace, name string) *rkev1.RKEControlPlane {
	return &rkev1.RKEControlPlane{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
	}
}

func buildMgmtCluster(name string) *v3.Cluster {
	return &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func Test_OnRancherClusterChange(t *testing.T) {
	type want struct {
		err error
	}

	tests := []struct {
		name   string
		input  *provv1.Cluster
		setup  func(ctrl *gomock.Controller) (*handler, *provv1.Cluster)
		expect want
	}{
		{
			name: "restore mode none does not update provisioning cluster",
			input: func() *provv1.Cluster {
				c := newBaseCluster("ns", "cluster-none", "v1.29.4+rke2r1")
				c.Spec.RKEConfig.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{
					Name: "snapshot-none", RestoreRKEConfig: rkev1.RestoreRKEConfigNone,
				}
				return c
			}(),
			setup: func(ctrl *gomock.Controller) (*handler, *provv1.Cluster) {
				provController := wfake.NewMockControllerInterface[*provv1.Cluster, *provv1.ClusterList](ctrl)
				mgmtClient := wfake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](ctrl)
				capiCache := wfake.NewMockCacheInterface[*capi.Cluster](ctrl)
				rkeControlPlaneCache := wfake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
				mgmtCache := wfake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
				etcdSnapshotCache := wfake.NewMockCacheInterface[*rkev1.ETCDSnapshot](ctrl)

				capiCache.EXPECT().Get("ns", "cluster-none").
					Return(buildCAPICluster("ns", "cluster-none"), nil).AnyTimes()
				rkeControlPlaneCache.EXPECT().Get("ns", "cluster-none").
					Return(buildRKECP("ns", "cluster-none"), nil).AnyTimes()

				mgmtCache.EXPECT().Get("cluster-none").
					Return(buildMgmtCluster("cluster-none"), nil).AnyTimes()
				mgmtClient.EXPECT().Update(gomock.Any()).
					Return(&v3.Cluster{}, nil).AnyTimes()
				mgmtClient.EXPECT().UpdateStatus(gomock.Any()).
					Return(&v3.Cluster{}, nil).AnyTimes()

				// No provisioning update expected for restore mode "none"
				provController.EXPECT().Update(gomock.Any()).Times(0)

				h := &handler{
					clusterController: provController,
					mgmtClusterClient: mgmtClient,
					mgmtClusterCache:  mgmtCache,
					capiClusters:      capiCache,
					rkeControlPlane:   rkeControlPlaneCache,
					etcdSnapshotCache: etcdSnapshotCache,
				}
				return h, nil
			},
			expect: want{err: nil},
		},
		{
			name: "restore mode kubernetesVersion updates version and returns ErrSkip",
			input: func() *provv1.Cluster {
				c := newBaseCluster("ns", "cluster-kversion", "v1.29.4+rke2r1")
				c.Spec.RKEConfig.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{
					Name: "snap-kv", RestoreRKEConfig: rkev1.RestoreRKEConfigKubernetesVersion,
				}
				return c
			}(),
			setup: func(ctrl *gomock.Controller) (*handler, *provv1.Cluster) {
				provController := wfake.NewMockControllerInterface[*provv1.Cluster, *provv1.ClusterList](ctrl)
				mgmtClient := wfake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](ctrl)
				capiCache := wfake.NewMockCacheInterface[*capi.Cluster](ctrl)
				rkeControlPlaneCache := wfake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
				mgmtCache := wfake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
				etcdSnapshotCache := wfake.NewMockCacheInterface[*rkev1.ETCDSnapshot](ctrl)

				capiCache.EXPECT().Get("ns", "cluster-kversion").
					Return(buildCAPICluster("ns", "cluster-kversion"), nil).AnyTimes()
				rkeControlPlaneCache.EXPECT().Get("ns", "cluster-kversion").
					Return(buildRKECP("ns", "cluster-kversion"), nil).AnyTimes()

				mgmtCache.EXPECT().Get("cluster-kversion").
					Return(buildMgmtCluster("cluster-kversion"), nil).AnyTimes()
				mgmtClient.EXPECT().Update(gomock.Any()).
					Return(&v3.Cluster{}, nil).AnyTimes()
				mgmtClient.EXPECT().UpdateStatus(gomock.Any()).
					Return(&v3.Cluster{}, nil).AnyTimes()

				snap := buildSnapshotWithClusterSpec(t, "ns", "snap-kv",
					provv1.ClusterSpec{KubernetesVersion: "v1.29.3+rke2r1"},
				)
				etcdSnapshotCache.EXPECT().Get("ns", "snap-kv").
					Return(snap, nil).AnyTimes()

				provController.EXPECT().Update(gomock.Any()).
					DoAndReturn(func(updated *provv1.Cluster) (*provv1.Cluster, error) {
						require.Equal(t, "v1.29.3+rke2r1", updated.Spec.KubernetesVersion)
						return updated, nil
					}).Times(1)

				h := &handler{
					clusterController: provController,
					mgmtClusterClient: mgmtClient,
					mgmtClusterCache:  mgmtCache,
					capiClusters:      capiCache,
					rkeControlPlane:   rkeControlPlaneCache,
					etcdSnapshotCache: etcdSnapshotCache,
				}
				return h, nil
			},
			expect: want{err: generic.ErrSkip},
		},

		{
			name: "restore mode all updates selected spec fields and returns ErrSkip",
			input: func() *provv1.Cluster {
				c := newBaseCluster("ns", "cluster-all-ok", "v1.29.4+rke2r1")
				c.Spec.RKEConfig.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{
					Name:             "snap-all-ok",
					RestoreRKEConfig: rkev1.RestoreRKEConfigAll,
				}
				return c
			}(),
			setup: func(ctrl *gomock.Controller) (*handler, *provv1.Cluster) {
				provController := wfake.NewMockControllerInterface[*provv1.Cluster, *provv1.ClusterList](ctrl)
				mgmtClient := wfake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](ctrl)
				capiCache := wfake.NewMockCacheInterface[*capi.Cluster](ctrl)
				rkeControlPlaneCache := wfake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
				mgmtCache := wfake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
				etcdSnapshotCache := wfake.NewMockCacheInterface[*rkev1.ETCDSnapshot](ctrl)

				// getRKEControlPlaneForCluster path
				capiCache.EXPECT().
					Get("ns", "cluster-all-ok").
					Return(buildCAPICluster("ns", "cluster-all-ok"), nil).
					AnyTimes()
				rkeControlPlaneCache.EXPECT().
					Get("ns", "cluster-all-ok").
					Return(buildRKECP("ns", "cluster-all-ok"), nil).
					AnyTimes()

				// retrieveMgmtClusterFromCache path
				mgmtCache.EXPECT().
					Get("cluster-all-ok").
					Return(buildMgmtCluster("cluster-all-ok"), nil).
					AnyTimes()
				mgmtClient.EXPECT().
					Update(gomock.Any()).
					Return(&v3.Cluster{}, nil).
					AnyTimes()
				mgmtClient.EXPECT().
					UpdateStatus(gomock.Any()).
					Return(&v3.Cluster{}, nil).
					AnyTimes()

				// Snapshot contains spec changes that should be reconciled (e.g., AdditionalManifest)
				snap := buildSnapshotWithClusterSpec(t, "ns", "snap-all-ok",
					provv1.ClusterSpec{
						KubernetesVersion: "v1.29.4+rke2r1", // version can be the same; we'll assert another field changed
						RKEConfig: &provv1.RKEConfig{
							ClusterConfiguration: rkev1.ClusterConfiguration{
								AdditionalManifest: "kind: ConfigMap\nmetadata:\n  name: injected\n",
							},
						},
					},
				)
				etcdSnapshotCache.EXPECT().
					Get("ns", "snap-all-ok").
					Return(snap, nil).
					AnyTimes()

				// Expect provisioning Update with the reconciled field
				provController.EXPECT().
					Update(gomock.Any()).
					DoAndReturn(func(updated *provv1.Cluster) (*provv1.Cluster, error) {
						require.NotNil(t, updated.Spec.RKEConfig)
						require.Equal(t,
							"kind: ConfigMap\nmetadata:\n  name: injected\n",
							updated.Spec.RKEConfig.AdditionalManifest,
						)
						return updated, nil
					}).
					Times(1)

				h := &handler{
					clusterController: provController,
					mgmtClusterClient: mgmtClient,
					mgmtClusterCache:  mgmtCache,
					capiClusters:      capiCache,
					rkeControlPlane:   rkeControlPlaneCache,
					etcdSnapshotCache: etcdSnapshotCache,
				}
				return h, nil
			},
			expect: want{err: generic.ErrSkip},
		},
		{
			name: "snapshot lookup error returns error and does not update provisioning cluster",
			input: func() *provv1.Cluster {
				c := newBaseCluster("ns", "cluster-fail", "v1.29.4+rke2r1")
				c.Spec.RKEConfig.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{
					Name: "snap-missing", RestoreRKEConfig: rkev1.RestoreRKEConfigAll,
				}
				return c
			}(),
			setup: func(ctrl *gomock.Controller) (*handler, *provv1.Cluster) {
				provController := wfake.NewMockControllerInterface[*provv1.Cluster, *provv1.ClusterList](ctrl)
				mgmtClient := wfake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](ctrl)
				capiCache := wfake.NewMockCacheInterface[*capi.Cluster](ctrl)
				rkeControlPlaneCache := wfake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
				mgmtCache := wfake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
				etcdSnapshotCache := wfake.NewMockCacheInterface[*rkev1.ETCDSnapshot](ctrl)

				capiCache.EXPECT().Get("ns", "cluster-fail").
					Return(buildCAPICluster("ns", "cluster-fail"), nil).AnyTimes()
				rkeControlPlaneCache.EXPECT().Get("ns", "cluster-fail").
					Return(buildRKECP("ns", "cluster-fail"), nil).AnyTimes()

				mgmtCache.EXPECT().Get("cluster-fail").
					Return(buildMgmtCluster("cluster-fail"), nil).AnyTimes()
				mgmtClient.EXPECT().Update(gomock.Any()).
					Return(&v3.Cluster{}, nil).AnyTimes()
				mgmtClient.EXPECT().UpdateStatus(gomock.Any()).
					Return(&v3.Cluster{}, nil).AnyTimes()

				etcdSnapshotCache.EXPECT().Get("ns", "snap-missing").
					Return(nil, errors.New("not found")).AnyTimes()

				provController.EXPECT().Update(gomock.Any()).Times(0)

				h := &handler{
					clusterController: provController,
					mgmtClusterClient: mgmtClient,
					mgmtClusterCache:  mgmtCache,
					capiClusters:      capiCache,
					rkeControlPlane:   rkeControlPlaneCache,
					etcdSnapshotCache: etcdSnapshotCache,
				}
				return h, nil
			},
			expect: want{err: errors.New("non-nil")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)

			h, _ := tc.setup(ctrl)
			_, _, err := h.OnRancherClusterChange(tc.input, tc.input.Status)

			if tc.expect.err != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestReconcileClusterSpecEtcdRestore(t *testing.T) {
	tests := []struct {
		name           string
		current        *provv1.Cluster
		desired        provv1.ClusterSpec
		expectedChange bool
		assertState    func(t *testing.T, cluster *provv1.Cluster)
	}{
		{
			name: "no changes",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{},
				},
			},
			desired: provv1.ClusterSpec{
				RKEConfig: &provv1.RKEConfig{},
			},
			expectedChange: false,
			assertState:    func(t *testing.T, c *provv1.Cluster) {},
		},
		{
			name: "networking pointers differ but values are semantically equal",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							Networking: &rkev1.Networking{},
						},
					},
				},
			},
			desired: provv1.ClusterSpec{
				RKEConfig: &provv1.RKEConfig{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						Networking: &rkev1.Networking{},
					},
				},
			},
			expectedChange: false,
			assertState:    func(t *testing.T, c *provv1.Cluster) {},
		},
		{
			name: "networking pointers differ but values are semantically equal (with values)",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							Networking: &rkev1.Networking{
								StackPreference: "ipv4",
							},
						},
					},
				},
			},
			desired: provv1.ClusterSpec{
				RKEConfig: &provv1.RKEConfig{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						Networking: &rkev1.Networking{
							StackPreference: "ipv4",
						},
					},
				},
			},
			expectedChange: false,
			assertState:    func(t *testing.T, c *provv1.Cluster) {},
		},
		{
			name: "update MachineGlobalConfig",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							MachineGlobalConfig: rkev1.GenericMap{
								Data: map[string]any{
									"debug": "false",
								},
							},
						},
					},
				},
			},
			desired: provv1.ClusterSpec{
				RKEConfig: &provv1.RKEConfig{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						MachineGlobalConfig: rkev1.GenericMap{
							Data: map[string]any{
								"debug": "true",
							},
						},
					},
				},
			},
			expectedChange: true,
			assertState: func(t *testing.T, c *provv1.Cluster) {
				require.NotNil(t, c.Spec.RKEConfig, "RKEConfig should not be nil")
				require.NotNil(t, c.Spec.RKEConfig.MachineGlobalConfig.Data, "Data should not be nil")
				assert.Equal(t, "true", c.Spec.RKEConfig.MachineGlobalConfig.Data["debug"])
			},
		},
		{
			name: "update MachineSelectorConfig",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{},
				},
			},
			desired: provv1.ClusterSpec{
				RKEConfig: &provv1.RKEConfig{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						MachineSelectorConfig: []rkev1.RKESystemConfig{
							{
								Config: rkev1.GenericMap{
									Data: map[string]any{"profile": "default"},
								},
							},
						},
					},
				},
			},
			expectedChange: true,
			assertState: func(t *testing.T, c *provv1.Cluster) {
				require.Len(t, c.Spec.RKEConfig.MachineSelectorConfig, 1, "should have exactly 1 selector")
				assert.Equal(t, "default", c.Spec.RKEConfig.MachineSelectorConfig[0].Config.Data["profile"])
			},
		},
		{
			name: "update ChartValues",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							ChartValues: rkev1.GenericMap{
								Data: map[string]any{
									"rke2-canal": map[string]any{"mtu": 1500},
								},
							},
						},
					},
				},
			},
			desired: provv1.ClusterSpec{
				RKEConfig: &provv1.RKEConfig{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						ChartValues: rkev1.GenericMap{
							Data: map[string]any{
								"rke2-canal": map[string]any{"mtu": 1450},
							},
						},
					},
				},
			},
			expectedChange: true,
			assertState: func(t *testing.T, c *provv1.Cluster) {
				require.NotNil(t, c.Spec.RKEConfig, "RKEConfig should not be nil")
				require.NotNil(t, c.Spec.RKEConfig.ChartValues.Data, "ChartValues Data should not be nil")
				val := c.Spec.RKEConfig.ChartValues.Data["rke2-canal"].(map[string]any)
				assert.Equal(t, 1450, val["mtu"])
			},
		},
		{
			name: "update Registries",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{},
				},
			},
			desired: provv1.ClusterSpec{
				RKEConfig: &provv1.RKEConfig{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						Registries: &rkev1.Registry{
							Mirrors: map[string]rkev1.Mirror{
								"docker.io": {Endpoints: []string{"https://mirror.local"}},
							},
						},
					},
				},
			},
			expectedChange: true,
			assertState: func(t *testing.T, c *provv1.Cluster) {
				assert.NotNil(t, c.Spec.RKEConfig.Registries)
				assert.Contains(t, c.Spec.RKEConfig.Registries.Mirrors, "docker.io")
			},
		},
		{
			name: "update UpgradeStrategy",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							UpgradeStrategy: rkev1.ClusterUpgradeStrategy{
								WorkerConcurrency: "1",
							},
						},
					},
				},
			},
			desired: provv1.ClusterSpec{
				RKEConfig: &provv1.RKEConfig{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						UpgradeStrategy: rkev1.ClusterUpgradeStrategy{
							WorkerConcurrency: "2",
						},
					},
				},
			},
			expectedChange: true,
			assertState: func(t *testing.T, c *provv1.Cluster) {
				assert.Equal(t, "2", c.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)
			},
		},
		{
			name: "update AdditionalManifest",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							AdditionalManifest: "old",
						},
					},
				},
			},
			desired: provv1.ClusterSpec{
				RKEConfig: &provv1.RKEConfig{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						AdditionalManifest: "new",
					},
				},
			},
			expectedChange: true,
			assertState: func(t *testing.T, c *provv1.Cluster) {
				assert.Equal(t, "new", c.Spec.RKEConfig.AdditionalManifest)
			},
		},
		{
			name: "update Networking content",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							Networking: &rkev1.Networking{StackPreference: "ipv4"},
						},
					},
				},
			},
			desired: provv1.ClusterSpec{
				RKEConfig: &provv1.RKEConfig{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						Networking: &rkev1.Networking{StackPreference: "ipv6"},
					},
				},
			},
			expectedChange: true,
			assertState: func(t *testing.T, c *provv1.Cluster) {
				assert.Equal(t, "ipv6", string(c.Spec.RKEConfig.Networking.StackPreference))
			},
		},
		{
			name: "update KubernetesVersion",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					KubernetesVersion: "v1.25.0",
					RKEConfig:         &provv1.RKEConfig{},
				},
			},
			desired: provv1.ClusterSpec{
				KubernetesVersion: "v1.26.0",
				RKEConfig:         &provv1.RKEConfig{},
			},
			expectedChange: true,
			assertState: func(t *testing.T, c *provv1.Cluster) {
				assert.Equal(t, "v1.26.0", c.Spec.KubernetesVersion)
			},
		},
		{
			name: "update DefaultPodSecurityAdmissionConfigurationTemplateName",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					DefaultPodSecurityAdmissionConfigurationTemplateName: "privileged",
					RKEConfig: &provv1.RKEConfig{},
				},
			},
			desired: provv1.ClusterSpec{
				DefaultPodSecurityAdmissionConfigurationTemplateName: "restricted",
				RKEConfig: &provv1.RKEConfig{},
			},
			expectedChange: true,
			assertState: func(t *testing.T, c *provv1.Cluster) {
				assert.Equal(t, "restricted", c.Spec.DefaultPodSecurityAdmissionConfigurationTemplateName)
			},
		},
		{
			name: "update ClusterAgentDeploymentCustomization",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{},
				},
			},
			desired: provv1.ClusterSpec{
				ClusterAgentDeploymentCustomization: &provv1.AgentDeploymentCustomization{
					AppendTolerations: []corev1.Toleration{{Key: "foo", Operator: "Exists"}},
				},
				RKEConfig: &provv1.RKEConfig{},
			},
			expectedChange: true,
			assertState: func(t *testing.T, c *provv1.Cluster) {
				assert.NotNil(t, c.Spec.ClusterAgentDeploymentCustomization)
				assert.Equal(t, "foo", c.Spec.ClusterAgentDeploymentCustomization.AppendTolerations[0].Key)
			},
		},
		{
			name: "update FleetAgentDeploymentCustomization",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{},
				},
			},
			desired: provv1.ClusterSpec{
				FleetAgentDeploymentCustomization: &provv1.AgentDeploymentCustomization{
					AppendTolerations: []corev1.Toleration{{Key: "bar", Operator: "Equal"}},
				},
				RKEConfig: &provv1.RKEConfig{},
			},
			expectedChange: true,
			assertState: func(t *testing.T, c *provv1.Cluster) {
				assert.NotNil(t, c.Spec.FleetAgentDeploymentCustomization)
				assert.Equal(t, "bar", c.Spec.FleetAgentDeploymentCustomization.AppendTolerations[0].Key)
			},
		},
		{
			name: "update WebhookDeploymentCustomization",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{},
				},
			},
			desired: provv1.ClusterSpec{
				WebhookDeploymentCustomization: &provv1.WebhookDeploymentCustomization{
					AppendTolerations: []corev1.Toleration{{Key: "webhook-node", Operator: "Exists"}},
				},
				RKEConfig: &provv1.RKEConfig{},
			},
			expectedChange: true,
			assertState: func(t *testing.T, c *provv1.Cluster) {
				assert.NotNil(t, c.Spec.WebhookDeploymentCustomization)
				assert.Equal(t, "webhook-node", c.Spec.WebhookDeploymentCustomization.AppendTolerations[0].Key)
			},
		},
		{
			name: "no change when WebhookDeploymentCustomization is unchanged",
			current: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					WebhookDeploymentCustomization: &provv1.WebhookDeploymentCustomization{
						AppendTolerations: []corev1.Toleration{{Key: "webhook-node", Operator: "Exists"}},
					},
					RKEConfig: &provv1.RKEConfig{},
				},
			},
			desired: provv1.ClusterSpec{
				WebhookDeploymentCustomization: &provv1.WebhookDeploymentCustomization{
					AppendTolerations: []corev1.Toleration{{Key: "webhook-node", Operator: "Exists"}},
				},
				RKEConfig: &provv1.RKEConfig{},
			},
			expectedChange: false,
			assertState:    func(t *testing.T, c *provv1.Cluster) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reconcileClusterSpecEtcdRestore(tt.current, tt.desired)
			assert.Equal(t, tt.expectedChange, got, "reconcileClusterSpecEtcdRestore return value mismatch")
			if tt.expectedChange {
				tt.assertState(t, tt.current)
			}
		})
	}
}

func TestAutoscaledMachineDeploymentReplicas(t *testing.T) {
	tests := []struct {
		name                         string
		min, max                     *int32
		quantity                     int32
		scaledReplicas, wantReplicas int64
		disableAutoscaling           bool
	}{
		{name: "stale pool quantity after scale up", min: ptr.To[int32](1), max: ptr.To[int32](5), quantity: 2, scaledReplicas: 3, wantReplicas: 3},
		{name: "stale pool quantity after scale to zero", min: ptr.To[int32](0), max: ptr.To[int32](5), quantity: 2, scaledReplicas: 0, wantReplicas: 0},
		{name: "fixed size pool", quantity: 2, scaledReplicas: 3, wantReplicas: 2},
		{name: "fixed size pool at zero", quantity: 0, scaledReplicas: 3, wantReplicas: 0},
		{name: "disable autoscaling", min: ptr.To[int32](1), max: ptr.To[int32](5), quantity: 2, scaledReplicas: 3, wantReplicas: 2, disableAutoscaling: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := &provv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "ns", Finalizers: []string{"test"}},
				Status:     provv1.ClusterStatus{ClusterName: "c-m-test"},
				Spec: provv1.ClusterSpec{
					KubernetesVersion: "v1.35.1+rke2r1",
					RKEConfig: &provv1.RKEConfig{
						MachinePools: []provv1.RKEMachinePool{{
							Name: "workers", WorkerRole: true, Quantity: &tt.quantity,
							AutoscalingMinSize: tt.min, AutoscalingMaxSize: tt.max,
							NodeConfig: &corev1.ObjectReference{APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2", Kind: "ExternalMachineTemplate", Name: "workers"},
						}},
					},
				},
			}
			reconcile, client, deployments := registerReplicaTestController(t)
			_, err := reconcile("ns/cluster", cluster)
			require.NoError(t, err)
			resource := client.Resource(capi.GroupVersion.WithResource("machinedeployments")).Namespace(cluster.Namespace)
			live, err := resource.Get(t.Context(), "cluster-workers", metav1.GetOptions{})
			require.NoError(t, err)
			initial, exists, err := unstructured.NestedInt64(live.Object, "spec", "replicas")
			require.NoError(t, err)
			require.True(t, exists)
			assert.Equal(t, int64(tt.quantity), initial, "initial MachineDeployment replicas")

			// The autoscaler writes the live replica count while the Cluster still has the old quantity.
			require.NoError(t, unstructured.SetNestedField(live.Object, tt.scaledReplicas, "spec", "replicas"))
			live, err = resource.Update(t.Context(), live, metav1.UpdateOptions{})
			require.NoError(t, err)
			var cached capi.MachineDeployment
			require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(live.Object, &cached))
			require.NoError(t, deployments.GetIndexer().Add(&cached))

			// Change an unrelated field so reconciliation must still update the deployment.
			pool := &cluster.Spec.RKEConfig.MachinePools[0]
			pool.Paused = true
			if tt.disableAutoscaling {
				pool.AutoscalingMinSize, pool.AutoscalingMaxSize = nil, nil
			}
			_, err = reconcile("ns/cluster", cluster)
			require.NoError(t, err)
			live, err = resource.Get(t.Context(), live.GetName(), metav1.GetOptions{})
			require.NoError(t, err)
			paused, _, err := unstructured.NestedBool(live.Object, "spec", "paused")
			require.NoError(t, err)
			assert.True(t, paused)
			replicas, exists, err := unstructured.NestedInt64(live.Object, "spec", "replicas")
			require.NoError(t, err)
			require.True(t, exists)
			assert.Equal(t, tt.wantReplicas, replicas, "reconciled MachineDeployment replicas")
		})
	}
}

func registerReplicaTestController(t *testing.T) (generic.Handler, *dynamicfake.FakeDynamicClient, cache.SharedIndexInformer) {
	t.Helper()
	ctrl := gomock.NewController(t)
	provisioning := wfake.NewMockControllerInterface[*provv1.Cluster, *provv1.ClusterList](ctrl)
	provisioningCache := wfake.NewMockCacheInterface[*provv1.Cluster](ctrl)
	provisioningCache.EXPECT().AddIndexer(byNodeInfra, gomock.Any())
	provisioning.EXPECT().Cache().Return(provisioningCache).AnyTimes()
	provisioning.EXPECT().GroupVersionKind().Return(provv1.SchemeGroupVersion.WithKind("Cluster"))
	provisioning.EXPECT().Informer().Return(cache.NewSharedIndexInformer(nil, &provv1.Cluster{}, 0, cache.Indexers{})).AnyTimes()
	provisioning.EXPECT().OnChange(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	provisioning.EXPECT().OnRemove(gomock.Any(), gomock.Any(), gomock.Any())
	provisioning.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(c *provv1.Cluster) (*provv1.Cluster, error) { return c, nil }).AnyTimes()
	var reconcile generic.Handler
	provisioning.EXPECT().AddGenericHandler(gomock.Any(), "rke-cluster", gomock.Any()).
		Do(func(_ context.Context, _ string, handler generic.Handler) { reconcile = handler })
	provisioning.EXPECT().AddGenericHandler(gomock.Any(), gomock.Not("rke-cluster"), gomock.Any()).AnyTimes()

	scheme := runtime.NewScheme()
	require.NoError(t, capi.AddToScheme(scheme))
	require.NoError(t, rkev1.AddToScheme(scheme))
	require.NoError(t, planv1alpha1.AddToScheme(scheme))
	client := dynamicfake.NewSimpleDynamicClient(scheme)
	resource := func(name, kind string) metav1.APIResource {
		return metav1.APIResource{Name: name, Kind: kind, Namespaced: true}
	}
	discovery := &discoveryfake.FakeDiscovery{Fake: &clienttesting.Fake{}}
	discovery.Resources = []*metav1.APIResourceList{
		{GroupVersion: capi.GroupVersion.String(), APIResources: []metav1.APIResource{
			resource("clusters", "Cluster"), resource("machinedeployments", "MachineDeployment"), resource("machinehealthchecks", "MachineHealthCheck"),
		}},
		{GroupVersion: rkev1.SchemeGroupVersion.String(), APIResources: []metav1.APIResource{
			resource("rkeclusters", "RKECluster"), resource("rkecontrolplanes", "RKEControlPlane"), resource("rkebootstraptemplates", "RKEBootstrapTemplate"),
		}},
		{GroupVersion: planv1alpha1.SchemeGroupVersion.String(), APIResources: []metav1.APIResource{resource("beacons", "Beacon")}},
	}
	applier := apply.New(discovery, func(gvr schema.GroupVersionResource) (dynamicclient.NamespaceableResourceInterface, error) {
		return client.Resource(gvr), nil
	})
	factory, err := controller.NewSharedControllerFactoryFromConfig(&rest.Config{Host: "http://localhost"}, wrangler.Scheme)
	require.NoError(t, err)
	capiControllers := capictrl.New(factory)
	Register(t.Context(), &wrangler.CAPIContext{
		CAPI: capiControllers,
		Context: &wrangler.Context{
			Core:         corectrl.New(factory),
			Mgmt:         mgmtctrl.New(factory),
			RKE:          rkectrl.New(factory),
			Provisioning: replicaTestProvisioning{cluster: provisioning},
			Dynamic:      dynamic.New(discovery),
			Apply:        applier,
		},
	})
	require.NotNil(t, reconcile)
	return reconcile, client, capiControllers.MachineDeployment().Informer()
}

type replicaTestProvisioning struct {
	provctrl.Interface
	cluster provctrl.ClusterController
}

func (p replicaTestProvisioning) Cluster() provctrl.ClusterController {
	return p.cluster
}

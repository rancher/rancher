package planner

import (
	"strconv"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func Test_shouldRotateEntry(t *testing.T) {
	tests := []struct {
		name               string
		services           []string
		rotateWorker       bool
		rotateControlPlane bool
		rotateETCD         bool
	}{
		{name: "rke2-server", services: []string{"rke2-server"}, rotateWorker: true, rotateControlPlane: true, rotateETCD: true},
		{name: "k3s-server", services: []string{"k3s-server"}, rotateWorker: true, rotateControlPlane: true, rotateETCD: true},
		{name: "api-server", services: []string{"api-server"}, rotateWorker: true, rotateControlPlane: true, rotateETCD: false},
		{name: "kubelet", services: []string{"kubelet"}, rotateWorker: true, rotateControlPlane: true, rotateETCD: true},
		{name: "kube-proxy", services: []string{"kube-proxy"}, rotateWorker: true, rotateControlPlane: true, rotateETCD: false},
		{name: "auth-proxy", services: []string{"auth-proxy"}, rotateWorker: true, rotateControlPlane: true, rotateETCD: false},
		{name: "controller-manager", services: []string{"controller-manager"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: false},
		{name: "scheduler", services: []string{"scheduler"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: false},
		{name: "rke2-controller", services: []string{"rke2-controller"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: false},
		{name: "k3s-controller", services: []string{"k3s-controller"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: false},
		{name: "admin", services: []string{"admin"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: false},
		{name: "cloud-controller", services: []string{"cloud-controller"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: false},
		{name: "etcd", services: []string{"etcd"}, rotateWorker: false, rotateControlPlane: false, rotateETCD: true},
		{name: "many", services: []string{"etcd", "cloud-controller"}, rotateWorker: false, rotateControlPlane: true, rotateETCD: true},
		{name: "none", services: []string{}, rotateWorker: true, rotateControlPlane: true, rotateETCD: true},
	}

	workerRoleEntry := &planEntry{Metadata: &plan.Metadata{Labels: map[string]string{capr.WorkerRoleLabel: "true"}}}
	controlPlaneRoleEntry := &planEntry{Metadata: &plan.Metadata{Labels: map[string]string{capr.ControlPlaneRoleLabel: "true"}}}
	etcdRoleEntry := &planEntry{Metadata: &plan.Metadata{Labels: map[string]string{capr.EtcdRoleLabel: "true"}}}
	allRoleEntry := &planEntry{Metadata: &plan.Metadata{Labels: map[string]string{capr.WorkerRoleLabel: "true", capr.ControlPlaneRoleLabel: "true", capr.EtcdRoleLabel: "true"}}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.rotateWorker, shouldRotateEntry(&rkev1.RotateCertificates{Services: tt.services}, workerRoleEntry))
			assert.Equal(t, tt.rotateControlPlane, shouldRotateEntry(&rkev1.RotateCertificates{Services: tt.services}, controlPlaneRoleEntry))
			assert.Equal(t, tt.rotateETCD, shouldRotateEntry(&rkev1.RotateCertificates{Services: tt.services}, etcdRoleEntry))
			assert.True(t, shouldRotateEntry(&rkev1.RotateCertificates{Services: tt.services}, allRoleEntry))
		})
	}
}

func Test_certificateRotationOrderedEntriesPlan(t *testing.T) {
	tests := []struct {
		name                             string
		clusterPlan                      *plan.Plan
		expectedNumberOfCollectedEntries int
		expectedOrder                    []string
	}{
		{
			name: "one all-in-one",
			clusterPlan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"node1": {ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
				},
				Metadata: map[string]*plan.Metadata{
					"node1": {Labels: map[string]string{capr.WorkerRoleLabel: "true", capr.ControlPlaneRoleLabel: "true", capr.EtcdRoleLabel: "true"}},
				},
			},
			expectedNumberOfCollectedEntries: 1,
			expectedOrder: []string{
				"node1",
			},
		},
		{
			name: "various dedicated roles",
			clusterPlan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"node1": {ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
					"node2": {ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
					"node3": {ObjectMeta: metav1.ObjectMeta{Name: "node3"}},
				},
				Metadata: map[string]*plan.Metadata{
					"node1": {Labels: map[string]string{capr.WorkerRoleLabel: "true"}},
					"node2": {Labels: map[string]string{capr.ControlPlaneRoleLabel: "true"}},
					"node3": {Labels: map[string]string{capr.EtcdRoleLabel: "true"}},
				},
			},
			expectedNumberOfCollectedEntries: 3,
			expectedOrder: []string{
				"node3",
				"node2",
				"node1",
			},
		},
		{
			name: "combined control and dedicated worker roles",
			clusterPlan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"node1": {ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
					"node2": {ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
					"node3": {ObjectMeta: metav1.ObjectMeta{Name: "node3"}},
					"node4": {ObjectMeta: metav1.ObjectMeta{Name: "node4"}},
					"node5": {ObjectMeta: metav1.ObjectMeta{Name: "node5"}},
					"node6": {ObjectMeta: metav1.ObjectMeta{Name: "node6"}},
				},
				Metadata: map[string]*plan.Metadata{
					"node1": {Labels: map[string]string{capr.ControlPlaneRoleLabel: "true", capr.EtcdRoleLabel: "true"}},
					"node2": {Labels: map[string]string{capr.WorkerRoleLabel: "true"}},
					"node3": {Labels: map[string]string{capr.ControlPlaneRoleLabel: "true", capr.EtcdRoleLabel: "true"}},
					"node4": {Labels: map[string]string{capr.WorkerRoleLabel: "true"}},
					"node5": {Labels: map[string]string{capr.ControlPlaneRoleLabel: "true", capr.EtcdRoleLabel: "true"}},
					"node6": {Labels: map[string]string{capr.WorkerRoleLabel: "true"}},
				},
			},
			expectedNumberOfCollectedEntries: 6,
			expectedOrder: []string{
				"node1",
				"node3",
				"node5",
				"node2",
				"node4",
				"node6",
			},
		},
		{
			name: "etcd-worker and dedicated control and worker roles",
			clusterPlan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"node1": {ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
					"node2": {ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
					"node3": {ObjectMeta: metav1.ObjectMeta{Name: "node3"}},
					"node4": {ObjectMeta: metav1.ObjectMeta{Name: "node4"}},
					"node5": {ObjectMeta: metav1.ObjectMeta{Name: "node5"}},
					"node6": {ObjectMeta: metav1.ObjectMeta{Name: "node6"}},
					"node7": {ObjectMeta: metav1.ObjectMeta{Name: "node7"}},
				},
				Metadata: map[string]*plan.Metadata{
					"node1": {Labels: map[string]string{capr.WorkerRoleLabel: "true", capr.EtcdRoleLabel: "true"}},
					"node2": {Labels: map[string]string{capr.WorkerRoleLabel: "true"}},
					"node3": {Labels: map[string]string{capr.WorkerRoleLabel: "true", capr.EtcdRoleLabel: "true"}},
					"node4": {Labels: map[string]string{capr.ControlPlaneRoleLabel: "true"}},
					"node5": {Labels: map[string]string{capr.WorkerRoleLabel: "true"}},
					"node6": {Labels: map[string]string{capr.WorkerRoleLabel: "true", capr.EtcdRoleLabel: "true"}},
					"node7": {Labels: map[string]string{capr.EtcdRoleLabel: "true"}},
				},
			},
			expectedNumberOfCollectedEntries: 7,
			expectedOrder: []string{
				"node1",
				"node3",
				"node6",
				"node7",
				"node4",
				"node2",
				"node5",
			},
		},
		{
			name: "control-worker and dedicated etcd and worker roles",
			clusterPlan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"node1": {ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
					"node2": {ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
					"node3": {ObjectMeta: metav1.ObjectMeta{Name: "node3"}},
					"node4": {ObjectMeta: metav1.ObjectMeta{Name: "node4"}},
					"node5": {ObjectMeta: metav1.ObjectMeta{Name: "node5"}},
					"node6": {ObjectMeta: metav1.ObjectMeta{Name: "node6"}},
					"node7": {ObjectMeta: metav1.ObjectMeta{Name: "node7"}},
				},
				Metadata: map[string]*plan.Metadata{
					"node1": {Labels: map[string]string{capr.WorkerRoleLabel: "true", capr.ControlPlaneRoleLabel: "true"}},
					"node2": {Labels: map[string]string{capr.WorkerRoleLabel: "true"}},
					"node3": {Labels: map[string]string{capr.WorkerRoleLabel: "true", capr.ControlPlaneRoleLabel: "true"}},
					"node4": {Labels: map[string]string{capr.EtcdRoleLabel: "true"}},
					"node5": {Labels: map[string]string{capr.WorkerRoleLabel: "true"}},
					"node6": {Labels: map[string]string{capr.WorkerRoleLabel: "true"}},
					"node7": {Labels: map[string]string{capr.WorkerRoleLabel: "true", capr.ControlPlaneRoleLabel: "true"}},
				},
			},
			expectedNumberOfCollectedEntries: 7,
			expectedOrder: []string{
				"node4",
				"node1",
				"node3",
				"node7",
				"node2",
				"node5",
				"node6",
			},
		},
		{
			name: "traditional architecture with a no role node",
			clusterPlan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"node1": {ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
					"node2": {ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
					"node3": {ObjectMeta: metav1.ObjectMeta{Name: "node3"}},
					"node4": {ObjectMeta: metav1.ObjectMeta{Name: "node4"}},
					"node5": {ObjectMeta: metav1.ObjectMeta{Name: "node5"}},
					"node6": {ObjectMeta: metav1.ObjectMeta{Name: "node6"}},
					"node7": {ObjectMeta: metav1.ObjectMeta{Name: "node7"}},
				},
				Metadata: map[string]*plan.Metadata{
					"node1": {Labels: map[string]string{capr.EtcdRoleLabel: "true", capr.ControlPlaneRoleLabel: "true"}},
					"node2": {Labels: map[string]string{capr.EtcdRoleLabel: "true", capr.ControlPlaneRoleLabel: "true"}},
					"node3": {Labels: map[string]string{capr.EtcdRoleLabel: "true", capr.ControlPlaneRoleLabel: "true"}},
					"node4": {Labels: map[string]string{capr.WorkerRoleLabel: "true"}},
					"node5": {Labels: map[string]string{capr.WorkerRoleLabel: "true"}},
					"node6": {Labels: map[string]string{capr.WorkerRoleLabel: "true"}},
					"node7": {Labels: map[string]string{}},
				},
			},
			expectedNumberOfCollectedEntries: 6,
			expectedOrder: []string{
				"node1",
				"node2",
				"node3",
				"node4",
				"node5",
				"node6",
			},
		},
	}

	for _, tt := range tests {
		t.Run((tt.name), func(t *testing.T) {
			collected := collectOrderedCertificateRotationEntries(tt.clusterPlan)
			assert.Equal(t, tt.expectedNumberOfCollectedEntries, len(collected))
			for i, n := range tt.expectedOrder {
				assert.Equal(t, n, collected[i].Machine.Name)
			}
		})
	}
}

func Test_rotateCertificatesPlan(t *testing.T) {
	type expected struct {
		otiIndex   int
		oti        *plan.OneTimeInstruction
		otiCount   int
		joinServer string
	}

	genericSetup := func(mp *mockPlanner) {
		mp.clusterRegistrationTokenCache.EXPECT().GetByIndex(ClusterRegToken, "somecluster").Return([]*v3.ClusterRegistrationToken{{Status: v3.ClusterRegistrationTokenStatus{Token: "lol"}}}, nil)
		mp.managementClusters.EXPECT().Get("somecluster").Return(&v3.Cluster{}, nil)
	}

	tests := []struct {
		name                string
		version             string
		setup               func(mp *mockPlanner)
		joinServer          string
		entryIsControlPlane bool
		machineGlobalConfig *rkev1.GenericMap
		expected            expected
		rotateCertificates  *rkev1.RotateCertificates
	}{
		{
			name:                "test KCM cert regeneration removal instruction contains K3s",
			version:             "v1.25.7+k3s1",
			entryIsControlPlane: true,
			joinServer:          "my-magic-joinserver",
			setup:               genericSetup,
			expected: expected{
				otiIndex: 2,
				oti: &[]plan.OneTimeInstruction{idempotentInstruction(
					createTestControlPlane("v1.25.7+k3s1"),
					"certificate-rotation/rm-kcm-cert",
					strconv.FormatInt(int64(0), 10),
					"rm",
					[]string{
						"-f",
						"/var/lib/rancher/k3s/server/tls/kube-controller-manager/kube-controller-manager.crt",
					},
					[]string{},
				)}[0],
				otiCount:   8,
				joinServer: "my-magic-joinserver",
			},
		},
		{
			name:                "test KCM cert regeneration removal instruction contains RKE2",
			version:             "v1.25.7+rke2r1",
			entryIsControlPlane: true,
			joinServer:          "my-magic-joinserver",
			setup:               genericSetup,
			rotateCertificates: &rkev1.RotateCertificates{
				Generation: 244,
			},
			expected: expected{
				otiIndex: 2,
				oti: &[]plan.OneTimeInstruction{idempotentInstruction(
					createTestControlPlane("v1.25.7+rke2r1"),
					"certificate-rotation/rm-kcm-cert",
					strconv.FormatInt(int64(244), 10),
					"rm",
					[]string{
						"-f",
						"/var/lib/rancher/rke2/server/tls/kube-controller-manager/kube-controller-manager.crt",
					},
					[]string{},
				)}[0],
				otiCount:   11, // the extra removal instructions are for removing the static pod manifests for RKE2
				joinServer: "my-magic-joinserver",
			},
		},
		{
			name:                "test KS cert regeneration removal instruction contains K3s",
			version:             "v1.25.7+k3s1",
			entryIsControlPlane: true,
			joinServer:          "my-magic-joinserver",
			setup:               genericSetup,
			expected: expected{
				otiIndex: 4,
				oti: &[]plan.OneTimeInstruction{idempotentInstruction(
					createTestControlPlane("v1.25.7+k3s1"),
					"certificate-rotation/rm-ks-cert",
					strconv.FormatInt(int64(0), 10),
					"rm",
					[]string{
						"-f",
						"/var/lib/rancher/k3s/server/tls/kube-scheduler/kube-scheduler.crt",
					},
					[]string{},
				)}[0],
				otiCount:   8,
				joinServer: "my-magic-joinserver",
			},
		},
		{
			name:                "test KS cert regeneration removal instruction contains RKE2",
			version:             "v1.25.7+rke2r1",
			entryIsControlPlane: true,
			joinServer:          "my-magic-joinserver",
			setup:               genericSetup,
			expected: expected{
				otiIndex: 5,
				oti: &[]plan.OneTimeInstruction{idempotentInstruction(
					createTestControlPlane("v1.25.7+rke2r1"),
					"certificate-rotation/rm-ks-cert",
					strconv.FormatInt(int64(0), 10),
					"rm",
					[]string{
						"-f",
						"/var/lib/rancher/rke2/server/tls/kube-scheduler/kube-scheduler.crt",
					},
					[]string{},
				)}[0],
				otiCount:   11, // the extra removal instructions are for removing the static pod manifests for RKE2
				joinServer: "my-magic-joinserver",
			},
		},
		{
			name:                "test rke2 worker-only instruction",
			version:             "v1.25.7+rke2r1",
			entryIsControlPlane: false,
			joinServer:          "my-magic-joinserver",
			expected: expected{
				otiIndex: 1,
				oti: &[]plan.OneTimeInstruction{idempotentRestartInstructions(
					createTestControlPlane("v1.25.7+rke2r1"),
					"certificate-rotation/restart",
					strconv.FormatInt(int64(0), 10),
					capr.GetRuntimeAgentUnit("v1.25.7+rke2r1"))[1]}[0],
				otiCount:   2,
				joinServer: "",
			},
		},
		{
			name:                "test k3s worker-only instruction",
			version:             "v1.25.7+k3s1",
			entryIsControlPlane: false,
			joinServer:          "my-magic-joinserver",
			expected: expected{
				otiIndex: 1,
				oti: &[]plan.OneTimeInstruction{idempotentRestartInstructions(
					createTestControlPlane("v1.25.7+k3s1"),
					"certificate-rotation/restart",
					strconv.FormatInt(int64(0), 10),
					capr.GetRuntimeAgentUnit("v1.25.7+k3s1"))[1]}[0],
				otiCount:   2,
				joinServer: "",
			},
		},
		{
			name:                "test K3s kcm custom kube-controller-manager cert-dir instruction",
			version:             "v1.25.7+k3s1",
			entryIsControlPlane: true,
			joinServer:          "my-magic-joinserver",
			setup:               genericSetup,
			machineGlobalConfig: &rkev1.GenericMap{
				Data: map[string]interface{}{
					KubeControllerManagerArg: []any{"cert-dir=/mycustomdir"},
				},
			},
			expected: expected{
				otiIndex: 2,
				oti: &[]plan.OneTimeInstruction{idempotentInstruction(
					createTestControlPlane("v1.25.7+k3s1"),
					"certificate-rotation/rm-kcm-cert",
					strconv.FormatInt(int64(0), 10),
					"rm",
					[]string{
						"-f",
						"/mycustomdir/kube-controller-manager.crt",
					},
					[]string{},
				)}[0],
				otiCount:   8,
				joinServer: "my-magic-joinserver",
			},
		},
	}

	for _, tt := range tests {
		// copy test case for persistence in parallel runs
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mockPlanner := newMockPlanner(t, InfoFunctions{
				SystemAgentImage:      func() string { return "system-agent" },
				ImageResolver:         image.ResolveWithControlPlane,
				GetBootstrapManifests: func(plane *rkev1.RKEControlPlane) ([]plan.File, error) { return nil, nil },
			})
			if tt.setup != nil {
				tt.setup(mockPlanner)
			}
			controlPlane := createTestControlPlane(tt.version)
			if tt.machineGlobalConfig != nil {
				controlPlane.Spec.MachineGlobalConfig = *tt.machineGlobalConfig
			}
			controlPlane.Spec.ManagementClusterName = "somecluster"
			controlPlane.Spec.RotateCertificates = &rkev1.RotateCertificates{}
			if tt.rotateCertificates != nil {
				controlPlane.Spec.RotateCertificates = tt.rotateCertificates
			} else {
				controlPlane.Spec.RotateCertificates = &rkev1.RotateCertificates{}
			}
			entry := createTestPlanEntry(capr.DefaultMachineOS)
			if tt.entryIsControlPlane {
				entry.Machine.Labels[capr.ControlPlaneRoleLabel] = "true"
				entry.Metadata.Labels[capr.ControlPlaneRoleLabel] = "true"
			} else {
				// worker nodes ignore passed in join server and rely only on annotation
				if entry.Metadata.Annotations == nil {
					entry.Metadata.Annotations = map[string]string{}
				}
				entry.Metadata.Annotations[capr.JoinedToAnnotation] = tt.expected.joinServer
			}

			ts := plan.Secret{
				ServerToken: "lol",
			}

			np, joined, err := mockPlanner.planner.rotateCertificatesPlan(controlPlane, ts, controlPlane.Spec.RotateCertificates, entry, tt.joinServer)
			if tt.expected.oti != nil {
				assert.Equal(t, *tt.expected.oti, np.Instructions[tt.expected.otiIndex])
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected.joinServer, joined)
			assert.Equal(t, tt.expected.otiCount, len(np.Instructions))
		})
	}
}

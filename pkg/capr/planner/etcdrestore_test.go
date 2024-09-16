package planner

import (
	"testing"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestForceDeleteAllDeletingEtcdMachines(t *testing.T) {
	t.Parallel()

	timeNow := metav1.NewTime(time.Now())
	setup := func(t *testing.T, mp *mockPlanner) {
		mp.machines.EXPECT().Update(gomock.Any()).DoAndReturn(func(machine *capi.Machine) (*capi.Machine, error) {
			assert.Equal(t, machine.Annotations[capi.ExcludeNodeDrainingAnnotation], "true")
			return machine, nil
		}).AnyTimes()
		mp.rkeBootstrapCache.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string) (*rkev1.RKEBootstrap, error) {
			return &rkev1.RKEBootstrap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}, nil
		}).AnyTimes()
		mp.rkeBootstrap.EXPECT().Update(gomock.Any()).DoAndReturn(func(bootstrap *rkev1.RKEBootstrap) (*rkev1.RKEBootstrap, error) {
			assert.Equal(t, bootstrap.Annotations[capr.ForceRemoveEtcdAnnotation], "true")
			return bootstrap, nil
		}).AnyTimes()
	}

	tests := []struct {
		name         string
		controlPlane *rkev1.RKEControlPlane
		plan         *plan.Plan
		expected     int
		setup        func(t *testing.T, mp *mockPlanner)
	}{
		{
			name:         "no machines",
			controlPlane: &rkev1.RKEControlPlane{},
			plan:         &plan.Plan{},
			expected:     0,
			setup:        nil,
		},
		{
			name:         "no etcd machines",
			controlPlane: &rkev1.RKEControlPlane{},
			plan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"a": {},
				},
				Metadata: map[string]*plan.Metadata{
					"a": {
						Labels: map[string]string{
							capr.WorkerRoleLabel: "true",
						},
					},
				},
			},
			expected: 0,
			setup:    nil,
		},
		{
			name:         "non-deleting etcd machine",
			controlPlane: &rkev1.RKEControlPlane{},
			plan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"a": {},
				},
				Metadata: map[string]*plan.Metadata{
					"a": {
						Labels: map[string]string{
							capr.EtcdRoleLabel: "true",
						},
					},
				},
			},
			expected: 0,
			setup:    nil,
		},
		{
			name:         "nil bootstrap ref",
			controlPlane: &rkev1.RKEControlPlane{},
			plan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"a": {
						Spec: capi.MachineSpec{
							Bootstrap: capi.Bootstrap{
								ConfigRef: nil,
							},
						},
					},
				},
				Metadata: map[string]*plan.Metadata{
					"a": {
						Labels: map[string]string{
							capr.EtcdRoleLabel: "true",
						},
					},
				},
			},
			expected: 0,
			setup:    nil,
		},
		{
			name:         "non rke.cattle.io APIVersion",
			controlPlane: &rkev1.RKEControlPlane{},
			plan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"a": {
						Spec: capi.MachineSpec{
							Bootstrap: capi.Bootstrap{
								ConfigRef: &corev1.ObjectReference{
									APIVersion: "rancher.testing.io",
								},
							},
						},
					},
				},
				Metadata: map[string]*plan.Metadata{
					"a": {
						Labels: map[string]string{
							capr.EtcdRoleLabel: "true",
						},
					},
				},
			},
			expected: 0,
			setup:    nil,
		},
		{
			name:         "nil annotations",
			controlPlane: &rkev1.RKEControlPlane{},
			plan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"a": {
						ObjectMeta: metav1.ObjectMeta{
							DeletionTimestamp: &timeNow,
						},
						Spec: capi.MachineSpec{
							Bootstrap: capi.Bootstrap{
								ConfigRef: &corev1.ObjectReference{
									APIVersion: "rke.cattle.io",
								},
							},
						},
					},
				},
				Metadata: map[string]*plan.Metadata{
					"a": {
						Labels: map[string]string{
							capr.EtcdRoleLabel: "true",
						},
					},
				},
			},
			expected: 1,
			setup:    setup,
		},
		{
			name:         "exclude draining false",
			controlPlane: &rkev1.RKEControlPlane{},
			plan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"a": {
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								capi.ExcludeNodeDrainingAnnotation: "false",
							},
							DeletionTimestamp: &timeNow,
						},
						Spec: capi.MachineSpec{
							Bootstrap: capi.Bootstrap{
								ConfigRef: &corev1.ObjectReference{
									APIVersion: "rke.cattle.io",
								},
							},
						},
					},
				},
				Metadata: map[string]*plan.Metadata{
					"a": {
						Labels: map[string]string{
							capr.EtcdRoleLabel: "true",
						},
					},
				},
			},
			expected: 1,
			setup:    setup,
		},
		{
			name:         "three deleting etcd",
			controlPlane: &rkev1.RKEControlPlane{},
			plan: &plan.Plan{
				Machines: map[string]*capi.Machine{
					"a": {
						ObjectMeta: metav1.ObjectMeta{
							DeletionTimestamp: &timeNow,
						},
						Spec: capi.MachineSpec{
							Bootstrap: capi.Bootstrap{
								ConfigRef: &corev1.ObjectReference{
									APIVersion: "rke.cattle.io",
								},
							},
						},
					},
					"b": {
						ObjectMeta: metav1.ObjectMeta{
							DeletionTimestamp: &timeNow,
						},
						Spec: capi.MachineSpec{
							Bootstrap: capi.Bootstrap{
								ConfigRef: &corev1.ObjectReference{
									APIVersion: "rke.cattle.io",
								},
							},
						},
					},
					"c": {
						ObjectMeta: metav1.ObjectMeta{
							DeletionTimestamp: &timeNow,
						},
						Spec: capi.MachineSpec{
							Bootstrap: capi.Bootstrap{
								ConfigRef: &corev1.ObjectReference{
									APIVersion: "rke.cattle.io",
								},
							},
						},
					},
				},
				Metadata: map[string]*plan.Metadata{
					"a": {
						Labels: map[string]string{
							capr.EtcdRoleLabel: "true",
						},
					},
					"b": {
						Labels: map[string]string{
							capr.EtcdRoleLabel: "true",
						},
					},
					"c": {
						Labels: map[string]string{
							capr.EtcdRoleLabel: "true",
						},
					},
				},
			},
			expected: 3,
			setup:    setup,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mp := newMockPlanner(t, InfoFunctions{
				SystemAgentImage: func() string { return "system-agent" },
				ImageResolver:    image.ResolveWithControlPlane,
			})
			if tt.setup != nil {
				tt.setup(t, mp)
			}
			l, err := mp.planner.forceDeleteAllDeletingEtcdMachines(tt.controlPlane, tt.plan)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, l)
		})
	}
}

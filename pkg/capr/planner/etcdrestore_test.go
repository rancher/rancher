package planner

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
								ConfigRef: capi.ContractVersionedObjectReference{},
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
								ConfigRef: capi.ContractVersionedObjectReference{
									Kind:     "",
									Name:     "",
									APIGroup: "rancher.testing.io",
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
								ConfigRef: capi.ContractVersionedObjectReference{
									Kind:     "",
									Name:     "",
									APIGroup: "rke.cattle.io",
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
								ConfigRef: capi.ContractVersionedObjectReference{
									Kind:     "",
									Name:     "",
									APIGroup: "rke.cattle.io",
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
								ConfigRef: capi.ContractVersionedObjectReference{
									Kind:     "",
									Name:     "",
									APIGroup: "rke.cattle.io",
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
								ConfigRef: capi.ContractVersionedObjectReference{
									Kind:     "",
									Name:     "",
									APIGroup: "rke.cattle.io",
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
								ConfigRef: capi.ContractVersionedObjectReference{
									Kind:     "",
									Name:     "",
									APIGroup: "rke.cattle.io",
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

func TestCreateEtcdTombstoneIfPresentInstructionRunsOnShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not found")
	}
	cp := &rkev1.RKEControlPlane{Spec: rkev1.RKEControlPlaneSpec{KubernetesVersion: "v1.30.0+rke2r1"}}
	run := func(dir string) error {
		instr := generateCreateEtcdTombstoneIfPresentInstruction(cp)
		args := append([]string{}, instr.Args...)
		args[len(args)-1] = dir
		return exec.Command(instr.Command, args...).Run()
	}

	t.Run("missing directory is skipped", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "server/db/etcd")
		require.NoError(t, run(dir))
		_, err := os.Stat(dir)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("existing directory gets tombstone and keeps its contents", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "sentinel"), []byte("keep"), 0600))
		require.NoError(t, run(dir))
		_, err := os.Stat(filepath.Join(dir, "tombstone"))
		assert.NoError(t, err)
		content, err := os.ReadFile(filepath.Join(dir, "sentinel"))
		require.NoError(t, err)
		assert.Equal(t, "keep", string(content))
	})

	t.Run("rerun on existing directory succeeds", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, run(dir))
		require.NoError(t, run(dir))
	})

	t.Run("unwritable existing directory fails", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory permissions")
		}
		dir := t.TempDir()
		require.NoError(t, os.Chmod(dir, 0500))
		t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
		assert.Error(t, run(dir))
	})
}

func TestCreateEtcdTombstoneInstructionPaths(t *testing.T) {
	tests := []struct {
		name     string
		cp       *rkev1.RKEControlPlane
		expected string
	}{
		{
			name:     "rke2",
			cp:       &rkev1.RKEControlPlane{Spec: rkev1.RKEControlPlaneSpec{KubernetesVersion: "v1.30.0+rke2r1"}},
			expected: "/var/lib/rancher/rke2/server/db/etcd",
		},
		{
			name:     "k3s",
			cp:       &rkev1.RKEControlPlane{Spec: rkev1.RKEControlPlaneSpec{KubernetesVersion: "v1.30.0+k3s1"}},
			expected: "/var/lib/rancher/k3s/server/db/etcd",
		},
		{
			name: "custom data directory",
			cp: &rkev1.RKEControlPlane{Spec: rkev1.RKEControlPlaneSpec{
				KubernetesVersion: "v1.30.0+rke2r1",
				ClusterConfiguration: rkev1.ClusterConfiguration{
					DataDirectories: rkev1.DataDirectories{K8sDistro: "/opt/custom"},
				},
			}},
			expected: "/opt/custom/server/db/etcd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instr := generateCreateEtcdTombstoneIfPresentInstruction(tt.cp)
			require.NotEmpty(t, instr.Args)
			assert.Equal(t, tt.expected, instr.Args[len(instr.Args)-1])
		})
	}
}

func TestRunEtcdRestoreServiceStopTombstoneTarget(t *testing.T) {
	t.Parallel()

	const ns = "fleet-default"
	cp := &rkev1.RKEControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: ns},
		Spec: rkev1.RKEControlPlaneSpec{
			KubernetesVersion: "v1.30.0+rke2r1",
			UnmanagedConfig:   true,
		},
	}
	machine := func(name string, labels map[string]string, annotations map[string]string) (*capi.Machine, *plan.Metadata) {
		return &capi.Machine{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
			Spec: capi.MachineSpec{Bootstrap: capi.Bootstrap{ConfigRef: capi.ContractVersionedObjectReference{
				Kind:     capr.RKEBootstrapKind,
				Name:     name,
				APIGroup: "rke.cattle.io",
			}}},
			Status: capi.MachineStatus{Conditions: []metav1.Condition{{Type: capi.InfrastructureReadyCondition, Status: metav1.ConditionTrue}}},
		}, &plan.Metadata{
			Labels:      labels,
			Annotations: annotations,
		}
	}
	targetMachine, targetMeta := machine("target", map[string]string{capr.EtcdRoleLabel: "true", capr.InitNodeLabel: "true"}, map[string]string{capr.JoinURLAnnotation: "https://target:9345"})
	otherMachine, otherMeta := machine("other", map[string]string{capr.EtcdRoleLabel: "true"}, map[string]string{})
	cpMachine, cpMeta := machine("cp", map[string]string{capr.ControlPlaneRoleLabel: "true"}, map[string]string{})

	clusterPlan := &plan.Plan{
		Machines: map[string]*capi.Machine{"target": targetMachine, "other": otherMachine, "cp": cpMachine},
		Nodes: map[string]*plan.Node{
			"target": {},
			"other":  {},
			"cp":     {},
		},
		Metadata: map[string]*plan.Metadata{"target": targetMeta, "other": otherMeta, "cp": cpMeta},
	}

	mp := newMockPlanner(t, InfoFunctions{
		SystemAgentImage: func() string { return "system-agent" },
		ImageResolver:    image.ResolveWithControlPlane,
	})
	mp.planner.equalities = equality.Semantic.Copy()

	written := map[string]plan.NodePlan{}
	mp.secretClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Type:       capr.SecretTypeMachinePlan,
			Data:       map[string][]byte{},
		}, nil
	}).AnyTimes()
	mp.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		var np plan.NodePlan
		require.NoError(t, json.Unmarshal(secret.Data["plan"], &np))
		written[secret.Name] = np
		return secret, nil
	}).AnyTimes()

	snapshot := &rkev1.ETCDSnapshot{
		SnapshotFile: rkev1.ETCDSnapshotFile{S3: &rkev1.ETCDSnapshotS3{}},
	}
	err := mp.planner.runEtcdRestoreServiceStop(cp, &capi.Cluster{}, snapshot, plan.Secret{}, clusterPlan)
	require.True(t, IsErrWaiting(err), "expected waiting error, got %v", err)

	findTombstone := func(name string) *plan.OneTimeInstruction {
		np, ok := written[capr.PlanSecretFromBootstrapName(name)]
		require.True(t, ok, "no stop plan written for %s", name)
		for i := range np.Instructions {
			if np.Instructions[i].Name == "create-etcd-tombstone" {
				return &np.Instructions[i]
			}
		}
		return nil
	}

	target := findTombstone("target")
	require.NotNil(t, target)
	assert.Equal(t, "sh", target.Command)
	assert.Contains(t, target.Args, `[ ! -d "$1" ] || touch "$1/tombstone"`)

	other := findTombstone("other")
	require.NotNil(t, other)
	assert.Equal(t, "touch", other.Command)
	assert.Equal(t, []string{"/var/lib/rancher/rke2/server/db/etcd/tombstone"}, other.Args)

	assert.Nil(t, findTombstone("cp"))
}

func TestRunEtcdRestoreServiceStopInitMarks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		deleting    bool
		expectedErr string
	}{
		{
			name:        "duplicate live init marks are rejected before stop plans",
			expectedErr: "more than one init node existed, cannot run etcd snapshot restore",
		},
		{
			name:        "deleting init mark is force removed before validation",
			deleting:    true,
			expectedErr: "waiting for 1 etcd machines to delete",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const ns = "fleet-default"
			cp := &rkev1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "c",
					Namespace: ns,
					Labels:    map[string]string{capr.InitNodeMachineIDLabel: "e"},
				},
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.30.0+rke2r1",
					UnmanagedConfig:   true,
				},
			}
			machine := func(name string) *capi.Machine {
				return &capi.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
						Name:      name,
						Labels:    map[string]string{capr.MachineIDLabel: name},
					},
					Spec: capi.MachineSpec{Bootstrap: capi.Bootstrap{ConfigRef: capi.ContractVersionedObjectReference{
						Kind:     capr.RKEBootstrapKind,
						Name:     name,
						APIGroup: capr.RKEAPIGroup,
					}}},
					Status: capi.MachineStatus{Conditions: []metav1.Condition{{Type: capi.InfrastructureReadyCondition, Status: metav1.ConditionTrue}}},
				}
			}
			otherName := "f"
			if tt.deleting {
				otherName = "d"
			}
			other := machine(otherName)
			if tt.deleting {
				now := metav1.Now()
				other.DeletionTimestamp = &now
			}
			clusterPlan := &plan.Plan{
				Machines: map[string]*capi.Machine{"e": machine("e"), otherName: other},
				Nodes:    map[string]*plan.Node{"e": {}, otherName: {}},
				Metadata: map[string]*plan.Metadata{},
			}
			for name := range clusterPlan.Machines {
				clusterPlan.Metadata[name] = &plan.Metadata{
					Labels: map[string]string{
						capr.EtcdRoleLabel:  "true",
						capr.InitNodeLabel:  "true",
						capr.MachineIDLabel: name,
					},
					Annotations: map[string]string{capr.JoinURLAnnotation: "https://" + name + ":9345"},
				}
			}

			mp := newMockPlanner(t, InfoFunctions{
				SystemAgentImage: func() string { return "system-agent" },
				ImageResolver:    image.ResolveWithControlPlane,
			})
			mp.planner.equalities = equality.Semantic.Copy()
			planUpdates := 0
			mp.secretClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
				return &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
					Type:       capr.SecretTypeMachinePlan,
					Data:       map[string][]byte{},
				}, nil
			}).AnyTimes()
			mp.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
				planUpdates++
				return secret, nil
			}).AnyTimes()

			var forcedBootstrap *rkev1.RKEBootstrap
			if tt.deleting {
				mp.machines.EXPECT().Update(gomock.Any()).DoAndReturn(func(machine *capi.Machine) (*capi.Machine, error) {
					assert.Equal(t, otherName, machine.Name)
					assert.Equal(t, "true", machine.Annotations[capi.ExcludeNodeDrainingAnnotation])
					assert.Equal(t, "true", machine.Annotations[capi.ExcludeWaitForNodeVolumeDetachAnnotation])
					return machine, nil
				})
				mp.rkeBootstrapCache.EXPECT().Get(ns, otherName).Return(&rkev1.RKEBootstrap{
					ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: otherName},
				}, nil)
				mp.rkeBootstrap.EXPECT().Update(gomock.Any()).DoAndReturn(func(bootstrap *rkev1.RKEBootstrap) (*rkev1.RKEBootstrap, error) {
					forcedBootstrap = bootstrap.DeepCopy()
					return bootstrap, nil
				})
			} else {
				mp.rkeBootstrap.EXPECT().Update(gomock.Any()).Times(0)
			}

			snapshot := &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{S3: &rkev1.ETCDSnapshotS3{}},
			}
			err := mp.planner.runEtcdRestoreServiceStop(cp, &capi.Cluster{}, snapshot, plan.Secret{}, clusterPlan)
			assert.EqualError(t, err, tt.expectedErr)
			assert.Zero(t, planUpdates, "no stop plans should be delivered")
			if tt.deleting {
				assert.True(t, IsErrWaiting(err))
				require.NotNil(t, forcedBootstrap)
				assert.Equal(t, otherName, forcedBootstrap.Name)
				assert.Equal(t, "true", forcedBootstrap.Annotations[capr.ForceRemoveEtcdAnnotation])
			} else {
				assert.False(t, IsErrWaiting(err))
				assert.Nil(t, forcedBootstrap)
			}
		})
	}
}

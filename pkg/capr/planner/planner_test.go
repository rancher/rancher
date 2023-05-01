package planner

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestPlanner_addInstruction(t *testing.T) {
	type args struct {
		version         string
		expectedVersion string
		os              string
		command         string
		scriptName      string
		envs            []string
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "Checking Linux Instructions",
			args: args{
				version:         "v1.21.5+rke2r2",
				expectedVersion: "v1.21.5-rke2r2",
				os:              "linux",
				command:         "sh",
				scriptName:      "run.sh",
				envs:            []string{"INSTALL_RKE2_EXEC"},
			},
		},
		{
			name: "Checking Windows Instructions",
			args: args{
				version:         "v1.21.5+rke2r2",
				expectedVersion: "v1.21.5-rke2r2",
				os:              "windows",
				command:         "powershell.exe",
				scriptName:      "run.ps1",
				envs:            []string{"$env:RESTART_STAMP", "$env:INSTALL_RKE2_EXEC"},
			},
		},
		{
			name: "Checking K3s Instructions",
			args: args{
				version:         "v1.21.5+k3s2",
				expectedVersion: "v1.21.5-k3s2",
				os:              "linux",
				command:         "sh",
				scriptName:      "run.sh",
				envs:            []string{"INSTALL_K3S_EXEC"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			a := assert.New(t)
			var planner Planner
			controlPlane := createTestControlPlane(tt.args.version)
			entry := createTestPlanEntry(tt.args.os)
			planner.retrievalFunctions.SystemAgentImage = func() string { return "system-agent" }
			planner.retrievalFunctions.ImageResolver = image.ResolveWithControlPlane
			// act
			p, err := planner.addInstallInstructionWithRestartStamp(plan.NodePlan{}, controlPlane, entry)

			// assert
			a.Nil(err)
			a.NotNil(p)
			a.Equal(entry.Metadata.Labels[capr.CattleOSLabel], tt.args.os)
			a.NotZero(len(p.Instructions))
			instruction := p.Instructions[0]
			a.Contains(instruction.Command, tt.args.command)
			a.Contains(instruction.Image, tt.args.expectedVersion)
			a.Contains(instruction.Args, tt.args.scriptName)
			for _, e := range tt.args.envs {
				a.True(findEnv(instruction.Env, e), "couldn't find %s in environment", e)
			}
		})
	}
}

func createTestControlPlane(version string) *rkev1.RKEControlPlane {
	return &rkev1.RKEControlPlane{
		Spec: rkev1.RKEControlPlaneSpec{
			KubernetesVersion: version,
		},
	}
}

func createTestPlanEntry(os string) *planEntry {
	return &planEntry{
		Machine: &capi.Machine{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					capr.ControlPlaneRoleLabel: "false",
					capr.EtcdRoleLabel:         "false",
					capr.WorkerRoleLabel:       "true",
				},
			},
			Spec: capi.MachineSpec{},
			Status: capi.MachineStatus{
				NodeInfo: &v1.NodeSystemInfo{
					OperatingSystem: os,
				},
			},
		},
		Metadata: &plan.Metadata{
			Labels: map[string]string{
				capr.CattleOSLabel:         os,
				capr.ControlPlaneRoleLabel: "false",
				capr.EtcdRoleLabel:         "false",
				capr.WorkerRoleLabel:       "true",
			},
		},
	}
}

func createTestPlanEntryWithoutRoles(os string) *planEntry {
	entry := createTestPlanEntry(os)
	entry.Metadata.Labels = map[string]string{
		capr.CattleOSLabel: os,
	}
	return entry
}

func findEnv(s []string, v string) bool {
	for _, item := range s {
		if strings.Contains(item, v) {
			return true
		}
	}
	return false
}

func Test_IsWindows(t *testing.T) {
	a := assert.New(t)
	data := map[string]bool{
		"windows": true,
		"linux":   false,
		"":        false,
	}
	for k, v := range data {
		a.Equal(v, windows(&planEntry{
			Metadata: &plan.Metadata{
				Labels: map[string]string{
					capr.CattleOSLabel: k,
				},
			},
		}))
	}
}

func Test_notWindows(t *testing.T) {
	type args struct {
		entry    *planEntry
		expected bool
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "Checking that linux isn't windows",
			args: args{
				entry:    createTestPlanEntry("linux"),
				expected: true,
			},
		},
		{
			name: "Checking that windows is windows",
			args: args{
				entry:    createTestPlanEntry("windows"),
				expected: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			a := assert.New(t)

			// act
			result := roleNot(windows)(tt.args.entry)

			// assert
			a.Equal(result, tt.args.expected)
		})
	}
}

func Test_anyRoleWithoutWindows(t *testing.T) {
	type args struct {
		entry    *planEntry
		expected bool
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "Should return linux node with roles",
			args: args{
				entry:    createTestPlanEntry("linux"),
				expected: true,
			},
		},
		{
			name: "Shouldn't return windows node.",
			args: args{
				entry:    createTestPlanEntry("windows"),
				expected: false,
			},
		},
		{
			name: "Shouldn't return node without any roles.",
			args: args{
				entry:    createTestPlanEntryWithoutRoles("linux"),
				expected: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			a := assert.New(t)

			// act
			result := anyRoleWithoutWindows(tt.args.entry)

			// assert
			a.Equal(result, tt.args.expected)
		})
	}
}

func TestPlanner_getLowestMachineKubeletVersion(t *testing.T) {
	type args struct {
		versions       []string
		expectedLowest string
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "Check lowest RKE2 version within minor release",
			args: args{
				versions: []string{
					"v1.25.5+rke2r1",
					"v1.25.6+rke2r1",
					"v1.25.7+rke2r1",
				},
				expectedLowest: "v1.25.5+rke2r1",
			},
		},
		{
			name: "Check lowest K3s version within minor release",
			args: args{
				versions: []string{
					"v1.25.5+k3s1",
					"v1.25.6+k3s1",
					"v1.25.7+k3s1",
				},
				expectedLowest: "v1.25.5+k3s1",
			},
		},
		{
			name: "Check lowest RKE2 version across any change in release",
			args: args{
				versions: []string{
					"v1.25.4+rke2r1",
					"v2.21.6+rke2r1",
					"v1.26.7+rke2r1",
				},
				expectedLowest: "v1.25.4+rke2r1",
			},
		},
		{
			name: "Check lowest K3s version across any change in release",
			args: args{
				versions: []string{
					"v1.25.4+k3s1",
					"v2.21.6+k3s1",
					"v1.26.7+k3s1",
				},
				expectedLowest: "v1.25.4+k3s1",
			},
		},
		{
			name: "Check lowest version across mixed K3s/RKE2 cluster",
			args: args{
				versions: []string{
					"v1.25.4+k3s1",
					"v2.21.6+k3s1",
					"v1.26.7+k3s1",
					"v1.21.5+rke2r1",
				},
				expectedLowest: "v1.21.5+rke2r1",
			},
		},
		{
			name: "Check lowest K3s version with RCs",
			args: args{
				versions: []string{
					"v1.21.4+k3s1",
					"v1.21.3-rc1+k3s1",
					"v1.23.7+k3s1",
				},
				expectedLowest: "v1.21.3-rc1+k3s1",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := assert.New(t)
			var plan = &plan.Plan{
				Machines: map[string]*capi.Machine{},
			}
			rand.Seed(time.Now().UnixNano())
			versions := test.args.versions
			// Shuffle the versions to really test the function.
			rand.Shuffle(len(versions), func(i, j int) { versions[i], versions[j] = versions[j], versions[i] })
			for i, v := range versions {
				plan.Machines[fmt.Sprintf("machine%d", i)] = &capi.Machine{
					Status: capi.MachineStatus{
						NodeInfo: &v1.NodeSystemInfo{
							KubeletVersion: v,
						},
					},
				}
			}
			lowestV := getLowestMachineKubeletVersion(plan)
			if len(test.args.versions) > 0 {
				a.NotNil(lowestV)
				expectedLowest, err := semver.NewVersion(test.args.expectedLowest)
				if a.NoError(err) {
					a.Equal(lowestV.String(), expectedLowest.String())
				}
			} else {
				a.Nil(lowestV)
			}
		})
	}
}

func Test_getInstallerImage(t *testing.T) {
	tests := []struct {
		name         string
		expected     string
		controlPlane *rkev1.RKEControlPlane
	}{
		{
			name:     "default",
			expected: "rancher/system-agent-installer-rke2:v1.25.7-rke2r1",
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.25.7+rke2r1",
				},
			},
		},
		{
			name:     "cluster private registry - machine global",
			expected: "test.rancher.io/rancher/system-agent-installer-rke2:v1.25.7-rke2r1",
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
						MachineGlobalConfig: rkev1.GenericMap{
							Data: map[string]any{
								"system-default-registry": "test.rancher.io",
							},
						},
					},
					KubernetesVersion: "v1.25.7+rke2r1",
				},
			},
		},
		{
			name:     "cluster private registry - machine selector",
			expected: "test.rancher.io/rancher/system-agent-installer-rke2:v1.25.7-rke2r1",
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
						MachineSelectorConfig: []rkev1.RKESystemConfig{
							{
								Config: rkev1.GenericMap{
									Data: map[string]any{
										"system-default-registry": "test.rancher.io",
									},
								},
							},
						},
					},
					KubernetesVersion: "v1.25.7+rke2r1",
				},
			},
		},
		{
			name:     "cluster private registry - prefer machine global",
			expected: "test.rancher.io/rancher/system-agent-installer-rke2:v1.25.7-rke2r1",
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
						MachineGlobalConfig: rkev1.GenericMap{
							Data: map[string]any{
								"system-default-registry": "test.rancher.io",
							},
						},
						MachineSelectorConfig: []rkev1.RKESystemConfig{
							{
								Config: rkev1.GenericMap{
									Data: map[string]any{
										"system-default-registry": "test2.rancher.io",
									},
								},
							},
						},
					},
					KubernetesVersion: "v1.25.7+rke2r1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var planner Planner
			planner.retrievalFunctions.ImageResolver = image.ResolveWithControlPlane
			planner.retrievalFunctions.SystemAgentImage = func() string { return "rancher/system-agent-installer-" }

			assert.Equal(t, tt.expected, planner.getInstallerImage(tt.controlPlane))
		})
	}
}

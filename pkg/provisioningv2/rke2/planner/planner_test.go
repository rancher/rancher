package planner

import (
	"strings"
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/stretchr/testify/assert"
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

			// act
			p, err := planner.addInstallInstructionWithRestartStamp(plan.NodePlan{}, controlPlane, entry)

			// assert
			a.Nil(err)
			a.NotNil(p)
			a.Equal(entry.Metadata.Labels[rke2.CattleOSLabel], tt.args.os)
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
					rke2.ControlPlaneRoleLabel: "false",
					rke2.EtcdRoleLabel:         "false",
					rke2.WorkerRoleLabel:       "true",
				},
			},
			Spec:   capi.MachineSpec{},
			Status: capi.MachineStatus{},
		},
		Metadata: &plan.Metadata{
			Labels: map[string]string{
				rke2.CattleOSLabel:         os,
				rke2.ControlPlaneRoleLabel: "false",
				rke2.EtcdRoleLabel:         "false",
				rke2.WorkerRoleLabel:       "true",
			},
		},
	}
}

func findEnv(s []string, v string) bool {
	for _, item := range s {
		if strings.Contains(item, v) {
			return true
		}
	}
	return false
}

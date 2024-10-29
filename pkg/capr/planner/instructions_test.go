package planner

import (
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/stretchr/testify/assert"
)

func TestPlanner_generateInstallInstruction(t *testing.T) {
	type args struct {
		version         string
		expectedVersion string
		os              string
		command         string
		scriptName      string
		envs            []string
		expectedEnvsLen int
		image           string
		expectedImage   string
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "Checking Empty Linux Instructions",
			args: args{
				version:         "v1.21.5+rke2r2",
				expectedVersion: "v1.21.5-rke2r2",
				os:              "linux",
				command:         "sh",
				scriptName:      "run.sh",
				envs:            []string{},
				expectedEnvsLen: 2,
				image:           "my/custom-image-",
				expectedImage:   "my/custom-image-rke2:v1.21.5-rke2r2",
			},
		},
		{
			name: "Checking Empty Windows Instructions",
			args: args{
				version:         "v1.21.5+rke2r2",
				expectedVersion: "v1.21.5-rke2r2",
				os:              "windows",
				command:         "powershell.exe",
				scriptName:      "run.ps1",
				envs:            []string{},
				expectedEnvsLen: 3,
				image:           "my/custom-image-",
				expectedImage:   "my/custom-image-rke2:v1.21.5-rke2r2",
			},
		},
		{
			name: "Checking Linux Instructions",
			args: args{
				version:         "v1.21.5+rke2r2",
				expectedVersion: "v1.21.5-rke2r2",
				os:              "linux",
				command:         "sh",
				scriptName:      "run.sh",
				envs:            []string{"HTTP_PROXY", "HTTPS_PROXY", "INSTALL_RKE2_EXEC"},
				expectedEnvsLen: 4,
				image:           "my/custom-image-",
				expectedImage:   "my/custom-image-rke2:v1.21.5-rke2r2",
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
				envs:            []string{"HTTP_PROXY", "HTTPS_PROXY", "INSTALL_RKE2_EXEC"},
				expectedEnvsLen: 5,
				image:           "my/custom-image-",
				expectedImage:   "my/custom-image-rke2:v1.21.5-rke2r2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			a := assert.New(t)
			controlPlane := createTestControlPlane(tt.args.version)
			if len(tt.args.envs) != 0 {
				controlPlane.Spec.AgentEnvVars = []v1.EnvVar{{Name: "HTTP_PROXY", Value: "0.0.0.0"}, {Name: "HTTPS_PROXY", Value: "0.0.0.0"}}
			}
			entry := createTestPlanEntry(tt.args.os)
			var planner Planner
			planner.retrievalFunctions.SystemAgentImage = func() string { return tt.args.image }
			planner.retrievalFunctions.ImageResolver = image.ResolveWithControlPlane
			// act
			p := planner.generateInstallInstruction(controlPlane, entry, []string{})

			// assert
			a.NotNil(p)
			a.Contains(p.Command, tt.args.command)
			a.Contains(p.Args, tt.args.scriptName)
			a.Equal(p.Image, tt.args.expectedImage)
			a.Equal(tt.args.expectedEnvsLen, len(p.Env))
			for _, e := range tt.args.envs {
				a.True(findEnvName(p.Env, e), "couldn't find %s in environment", e)
			}
		})
	}
}

func TestPlanner_addInstallInstructionWithRestartStamp(t *testing.T) {
	type args struct {
		version         string
		expectedVersion string
		os              string
		command         string
		scriptName      string
		envs            []string
		image           string
		expectedImage   string
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "Checking Linux Plan restart stamp",
			args: args{
				version:         "v1.21.5+rke2r2",
				expectedVersion: "v1.21.5-rke2r2",
				os:              "linux",
				command:         "sh",
				scriptName:      "run.sh",
				envs:            []string{"RESTART_STAMP"},
				image:           "my/custom-image-",
				expectedImage:   "my/custom-image-rke2:v1.21.5-rke2r2",
			},
		},
		{
			name: "Checking Windows plan restart stamp",
			args: args{
				version:         "v1.21.5+rke2r2",
				expectedVersion: "v1.21.5-rke2r2",
				os:              "windows",
				command:         "powershell.exe",
				scriptName:      "run.ps1",
				envs:            []string{"WINS_RESTART_STAMP"},
				image:           "my/custom-image-",
				expectedImage:   "my/custom-image-rke2:v1.21.5-rke2r2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			a := assert.New(t)
			var planner Planner
			planner.retrievalFunctions.SystemAgentImage = func() string { return tt.args.image }
			planner.retrievalFunctions.ImageResolver = image.ResolveWithControlPlane
			controlPlane := createTestControlPlane(tt.args.version)
			entry := createTestPlanEntry(tt.args.os)

			// act
			p, err := planner.addInstallInstructionWithRestartStamp(plan.NodePlan{}, controlPlane, entry)

			// assert
			a.Nil(err)
			a.NotNil(p)
			a.Equal(entry.Metadata.Labels[capr.CattleOSLabel], tt.args.os)
			a.NotZero(len(p.Instructions))
			instruction := p.Instructions[0]
			a.Contains(instruction.Image, tt.args.expectedVersion)
			a.Equal(instruction.Image, tt.args.expectedImage)
			a.Contains(instruction.Command, tt.args.command)
			a.Contains(instruction.Image, tt.args.expectedVersion)
			a.Contains(instruction.Args, tt.args.scriptName)
			a.GreaterOrEqual(len(instruction.Env), 1)
			for _, e := range tt.args.envs {
				a.True(findEnvName(instruction.Env, e), "couldn't find %s in environment", e)
			}
		})
	}
}

func TestPlanner_generateInstallInstructionWithSkipStart(t *testing.T) {
	type args struct {
		version         string
		expectedVersion string
		os              string
		command         string
		scriptName      string
		envs            []string
		image           string
		expectedImage   string
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "Checking Linux Plan skip restart",
			args: args{
				version:         "v1.21.5+rke2r2",
				expectedVersion: "v1.21.5-rke2r2",
				os:              "linux",
				command:         "sh",
				scriptName:      "run.sh",
				envs:            []string{"INSTALL_RKE2_SKIP_START"},
				image:           "my/custom-image-",
				expectedImage:   "my/custom-image-rke2:v1.21.5-rke2r2",
			},
		},
		{
			name: "Checking Windows plan skip restart",
			args: args{
				version:         "v1.21.5+rke2r2",
				expectedVersion: "v1.21.5-rke2r2",
				os:              "windows",
				command:         "powershell.exe",
				scriptName:      "run.ps1",
				envs:            []string{"INSTALL_RKE2_SKIP_START"},
				image:           "my/custom-image-",
				expectedImage:   "my/custom-image-rke2:v1.21.5-rke2r2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			a := assert.New(t)
			var planner Planner
			planner.retrievalFunctions.SystemAgentImage = func() string { return tt.args.image }
			planner.retrievalFunctions.ImageResolver = image.ResolveWithControlPlane
			controlPlane := createTestControlPlane(tt.args.version)
			entry := createTestPlanEntry(tt.args.os)

			// act
			p := planner.generateInstallInstructionWithSkipStart(controlPlane, entry)

			// assert
			a.NotNil(p)
			a.Equal(entry.Metadata.Labels[capr.CattleOSLabel], tt.args.os)
			a.Contains(p.Image, tt.args.expectedVersion)
			a.Equal(p.Image, tt.args.expectedImage)
			a.Contains(p.Command, tt.args.command)
			a.Contains(p.Image, tt.args.expectedVersion)
			a.Contains(p.Args, tt.args.scriptName)
			a.GreaterOrEqual(len(p.Env), 1)
			for _, e := range tt.args.envs {
				a.True(findEnvName(p.Env, e), "couldn't find %s in environment", e)
			}
		})
	}
}

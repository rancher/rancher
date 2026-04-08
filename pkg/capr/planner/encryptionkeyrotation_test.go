package planner

import (
	"strings"
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func Test_rotateEncryptionKeys_resetsStateWhenSpecCleared(t *testing.T) {
	planner := newTestEncryptionKeyRotationPlanner()
	status := rkev1.RKEControlPlaneStatus{
		RotateEncryptionKeys:       &rkev1.RotateEncryptionKeys{Generation: 1},
		RotateEncryptionKeysPhase:  rkev1.RotateEncryptionKeysPhaseRotate,
		RotateEncryptionKeysLeader: "server-1",
	}

	controlPlane := newTestEncryptionKeyRotationControlPlane(nil, status)
	updatedStatus, err := planner.rotateEncryptionKeys(controlPlane, status, plan.Secret{}, &plan.Plan{})

	assert.True(t, IsErrWaiting(err))
	assert.Nil(t, updatedStatus.RotateEncryptionKeys)
	assert.Empty(t, updatedStatus.RotateEncryptionKeysPhase)
	assert.Empty(t, updatedStatus.RotateEncryptionKeysLeader)
}

func Test_rotateEncryptionKeys_startsNewGeneration(t *testing.T) {
	planner := newTestEncryptionKeyRotationPlanner()
	controlPlane := newTestEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 2}, rkev1.RKEControlPlaneStatus{
		Initialization: rkev1.RKEControlPlaneInitializationStatus{
			ControlPlaneInitialized: ptr.To(true),
		},
	})
	clusterPlan := newTestEncryptionKeyRotationPlan(
		newTestEncryptionKeyRotationMachine("server-1", true, true, true, true, "https://server-1:9345"),
	)

	updatedStatus, err := planner.rotateEncryptionKeys(controlPlane, controlPlane.Status, plan.Secret{}, clusterPlan)

	assert.True(t, IsErrWaiting(err))
	assert.Equal(t, int64(2), updatedStatus.RotateEncryptionKeys.Generation)
	assert.Equal(t, rkev1.RotateEncryptionKeysPhaseRotate, updatedStatus.RotateEncryptionKeysPhase)
}

func Test_rotateEncryptionKeys_noopWhenDoneForSameGeneration(t *testing.T) {
	planner := newTestEncryptionKeyRotationPlanner()
	status := rkev1.RKEControlPlaneStatus{
		RotateEncryptionKeys:      &rkev1.RotateEncryptionKeys{Generation: 1},
		RotateEncryptionKeysPhase: rkev1.RotateEncryptionKeysPhaseDone,
	}
	controlPlane := newTestEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 1}, status)

	updatedStatus, err := planner.rotateEncryptionKeys(controlPlane, status, plan.Secret{}, &plan.Plan{})

	assert.NoError(t, err)
	assert.Equal(t, status, updatedStatus)
}

func Test_encryptionKeyRotationFindLeader(t *testing.T) {
	testCases := []struct {
		name          string
		status        rkev1.RKEControlPlaneStatus
		clusterPlan   *plan.Plan
		initNodeName  string
		expectedName  string
		expectedError string
	}{
		{
			name: "existing stored leader reused when still valid",
			status: rkev1.RKEControlPlaneStatus{
				RotateEncryptionKeysPhase:  rkev1.RotateEncryptionKeysPhasePostRotateRestart,
				RotateEncryptionKeysLeader: "server-2",
			},
			clusterPlan: newTestEncryptionKeyRotationPlan(
				newTestEncryptionKeyRotationMachine("server-1", true, true, true, true, "https://server-1:9345"),
				newTestEncryptionKeyRotationMachine("server-2", true, true, false, true, "https://server-2:9345"),
			),
			initNodeName: "server-1",
			expectedName: "server-2",
		},
		{
			name: "init node selected when valid",
			status: rkev1.RKEControlPlaneStatus{
				RotateEncryptionKeysPhase: rkev1.RotateEncryptionKeysPhaseRotate,
			},
			clusterPlan: newTestEncryptionKeyRotationPlan(
				newTestEncryptionKeyRotationMachine("server-1", true, true, true, true, "https://server-1:9345"),
				newTestEncryptionKeyRotationMachine("server-2", true, true, false, true, "https://server-2:9345"),
			),
			initNodeName: "server-1",
			expectedName: "server-1",
		},
		{
			name: "fallback to first suitable control plane when init node is not suitable",
			status: rkev1.RKEControlPlaneStatus{
				RotateEncryptionKeysPhase: rkev1.RotateEncryptionKeysPhaseRotate,
			},
			clusterPlan: newTestEncryptionKeyRotationPlan(
				newTestEncryptionKeyRotationMachine("server-1", true, false, true, true, "https://server-1:9345"),
				newTestEncryptionKeyRotationMachine("server-2", true, true, false, true, "https://server-2:9345"),
				newTestEncryptionKeyRotationMachine("server-3", true, true, false, true, "https://server-3:9345"),
			),
			initNodeName: "server-1",
			expectedName: "server-2",
		},
		{
			name: "error when no suitable control plane leader exists",
			status: rkev1.RKEControlPlaneStatus{
				RotateEncryptionKeysPhase: rkev1.RotateEncryptionKeysPhaseRotate,
			},
			clusterPlan: newTestEncryptionKeyRotationPlan(
				newTestEncryptionKeyRotationMachine("server-1", true, false, true, true, "https://server-1:9345"),
			),
			initNodeName:  "server-1",
			expectedError: "no suitable control plane nodes for encryption key rotation",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			planner := newTestEncryptionKeyRotationPlanner()
			initNode := newTestPlanEntryFromPlan(testCase.clusterPlan, testCase.initNodeName)

			leader, err := planner.encryptionKeyRotationFindLeader(testCase.status, testCase.clusterPlan, initNode)
			if testCase.expectedError != "" {
				assert.EqualError(t, err, testCase.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedName, leader.Machine.Name)
		})
	}
}

func Test_encryptionKeyRotationRestartTargetsForCluster(t *testing.T) {
	controlPlane := newTestEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 1}, rkev1.RKEControlPlaneStatus{
		RotateEncryptionKeys:       &rkev1.RotateEncryptionKeys{Generation: 1},
		RotateEncryptionKeysPhase:  rkev1.RotateEncryptionKeysPhasePostRotateRestart,
		RotateEncryptionKeysLeader: "control-plane-leader",
	})

	t.Run("etcd-only init node split from control-plane convergence entries", func(t *testing.T) {
		clusterPlan := newTestEncryptionKeyRotationPlan(
			newTestEncryptionKeyRotationMachine("etcd-init", true, false, true, true, "https://etcd-init:9345"),
			newTestEncryptionKeyRotationMachine("control-plane-leader", true, true, false, true, "https://control-plane-leader:9345"),
			newTestEncryptionKeyRotationMachine("etcd-follower", true, false, false, true, "https://etcd-follower:9345"),
			newTestEncryptionKeyRotationMachine("control-plane-follower", true, true, false, true, "https://control-plane-follower:9345"),
		)

		targets := encryptionKeyRotationRestartTargetsForCluster(
			controlPlane,
			clusterPlan,
			newTestPlanEntryFromPlan(clusterPlan, "control-plane-leader"),
			newTestPlanEntryFromPlan(clusterPlan, "etcd-init"),
		)

		assert.Equal(t, 4, targets.count())
		assert.Len(t, targets.etcdOnly, 2)
		assert.Equal(t, "etcd-init", targets.etcdOnly[0].Machine.Name)
		assert.Equal(t, "etcd-follower", targets.etcdOnly[1].Machine.Name)
		assert.Len(t, targets.controlPlane, 2)
		assert.Equal(t, "control-plane-leader", targets.controlPlane[0].Machine.Name)
		assert.Equal(t, "control-plane-follower", targets.controlPlane[1].Machine.Name)
	})

	t.Run("control plane init node stays in control-plane ordering", func(t *testing.T) {
		clusterPlan := newTestEncryptionKeyRotationPlan(
			newTestEncryptionKeyRotationMachine("control-plane-init", true, true, true, true, "https://control-plane-init:9345"),
			newTestEncryptionKeyRotationMachine("control-plane-leader", true, true, false, true, "https://control-plane-leader:9345"),
			newTestEncryptionKeyRotationMachine("control-plane-follower", true, true, false, true, "https://control-plane-follower:9345"),
		)

		targets := encryptionKeyRotationRestartTargetsForCluster(
			controlPlane,
			clusterPlan,
			newTestPlanEntryFromPlan(clusterPlan, "control-plane-leader"),
			newTestPlanEntryFromPlan(clusterPlan, "control-plane-init"),
		)

		assert.Equal(t, 3, targets.count())
		assert.Empty(t, targets.etcdOnly)
		assert.Len(t, targets.controlPlane, 3)
		assert.Equal(t, "control-plane-init", targets.controlPlane[0].Machine.Name)
		assert.Equal(t, "control-plane-leader", targets.controlPlane[1].Machine.Name)
		assert.Equal(t, "control-plane-follower", targets.controlPlane[2].Machine.Name)
	})
}

func Test_encryptionKeyRotationSecretsEncryptInstruction(t *testing.T) {
	controlPlane := newTestEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 3}, rkev1.RKEControlPlaneStatus{
		RotateEncryptionKeys:      &rkev1.RotateEncryptionKeys{Generation: 3},
		RotateEncryptionKeysPhase: rkev1.RotateEncryptionKeysPhaseRotate,
	})

	instruction, err := encryptionKeyRotationSecretsEncryptInstruction(controlPlane)
	commandArguments := strings.Join(instruction.Args, " ")

	assert.NoError(t, err)
	assert.Equal(t, "/bin/sh", instruction.Command)
	assert.True(t, instruction.SaveOutput)
	assert.Contains(t, commandArguments, "secrets-encrypt")
	assert.Contains(t, commandArguments, encryptionKeyRotationCommandRotateKeys)
	assert.Contains(t, commandArguments, "2>&1")
	assert.NotContains(t, commandArguments, "prepare")
	assert.NotContains(t, commandArguments, "reencrypt")
}

func Test_encryptionKeyRotationStatusFromOutput(t *testing.T) {
	entry := &planEntry{
		Machine: &capi.Machine{ObjectMeta: metav1.ObjectMeta{Name: "server-1"}},
	}

	testCases := []struct {
		name       string
		output     string
		expected   encryptionKeyRotationRuntimeStatus
		expectWait bool
	}{
		{
			name: "parses start stage",
			output: "Current Rotation Stage: start\n" +
				"Server Encryption Hashes: hash mismatch\n",
			expected: encryptionKeyRotationRuntimeStatus{
				Stage:       encryptionKeyRotationStageStart,
				HashesMatch: false,
			},
		},
		{
			name: "parses finished stage with matching hashes",
			output: "Current Rotation Stage: reencrypt_finished\n" +
				"Server Encryption Hashes: All hashes match\n",
			expected: encryptionKeyRotationRuntimeStatus{
				Stage:       encryptionKeyRotationStageReencryptFinished,
				HashesMatch: true,
			},
		},
		{
			name:       "malformed output waits",
			output:     "Current Rotation Stage\n",
			expectWait: true,
		},
		{
			name: "unknown stage handled safely",
			output: "Current Rotation Stage: mystery\n" +
				"Server Encryption Hashes: hash mismatch\n",
			expected: encryptionKeyRotationRuntimeStatus{
				Stage:       "mystery",
				HashesMatch: false,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			status, err := encryptionKeyRotationStatusFromOutput(entry, testCase.output)
			if testCase.expectWait {
				assert.True(t, IsErrWaiting(err))
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, status)
		})
	}
}

func Test_encryptionKeyRotationRotateKeysReconcile(t *testing.T) {
	testCases := []struct {
		name         string
		periodicOut  string
		savedOutput  string
		planFailed   bool
		expectStage  string
		expectWait   bool
		expectFailed bool
	}{
		{
			name: "complete status returned",
			periodicOut: "Current Rotation Stage: reencrypt_finished\n" +
				"Server Encryption Hashes: All hashes match\n",
			expectStage: encryptionKeyRotationStageReencryptFinished,
		},
		{
			name: "incomplete status returned for requeue",
			periodicOut: "Current Rotation Stage: start\n" +
				"Server Encryption Hashes: hash mismatch\n",
			expectStage: encryptionKeyRotationStageStart,
		},
		{
			name: "timeout output falls back to periodic status",
			periodicOut: "Current Rotation Stage: reencrypt_finished\n" +
				"Server Encryption Hashes: All hashes match\n",
			savedOutput: "level=fatal msg=\"Put \\\"https://127.0.0.1:6443/v1-k3s/encrypt/config\\\": context deadline exceeded (Client.Timeout exceeded while awaiting headers): " + encryptionKeyRotationRotateKeysTimeoutMessage + "\"\n",
			planFailed:  true,
			expectStage: encryptionKeyRotationStageReencryptFinished,
		},
		{
			name:        "timeout output waits for periodic status when not yet available",
			savedOutput: "level=fatal msg=\"Put \\\"https://127.0.0.1:6443/v1-k3s/encrypt/config\\\": context deadline exceeded (Client.Timeout exceeded while awaiting headers): " + encryptionKeyRotationRotateKeysTimeoutMessage + "\"\n",
			planFailed:  true,
			expectWait:  true,
		},
		{
			name:         "plan failure marks status failed",
			planFailed:   true,
			expectFailed: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			planner := newTestEncryptionKeyRotationPlanner()
			controlPlane := newTestEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 1}, rkev1.RKEControlPlaneStatus{
				RotateEncryptionKeys:      &rkev1.RotateEncryptionKeys{Generation: 1},
				RotateEncryptionKeysPhase: rkev1.RotateEncryptionKeysPhaseRotate,
			})
			leaderEntry := newTestPlanEntry(newTestEncryptionKeyRotationMachine("server-1", true, true, true, true, "https://server-1:9345"))
			nodePlan, joinedServer, err := planner.encryptionKeyRotationRotateKeysPlan(controlPlane, plan.Secret{}, "https://server-1:9345", leaderEntry)
			assert.NoError(t, err)
			assert.Empty(t, joinedServer)

			failedOutput := map[string][]byte{}
			if testCase.savedOutput != "" {
				failedOutput[nodePlan.Instructions[0].Name] = []byte(testCase.savedOutput)
			}
			leaderEntry.Plan = &plan.Node{
				Plan:         nodePlan,
				FailedOutput: failedOutput,
				Failed:       testCase.planFailed,
				InSync:       true,
				Healthy:      true,
				PeriodicOutput: map[string]plan.PeriodicInstructionOutput{
					encryptionKeyRotationSecretsEncryptStatusCommand: {
						Stdout: []byte(testCase.periodicOut),
					},
				},
			}

			rotationStatus, updatedStatus, err := planner.encryptionKeyRotationRotateKeysReconcile(controlPlane, controlPlane.Status, plan.Secret{}, "https://server-1:9345", leaderEntry)
			if testCase.expectWait {
				assert.True(t, IsErrWaiting(err))
				assert.Equal(t, controlPlane.Status.RotateEncryptionKeysPhase, updatedStatus.RotateEncryptionKeysPhase)
				return
			}
			if testCase.expectFailed {
				assert.Error(t, err)
				assert.Equal(t, rkev1.RotateEncryptionKeysPhaseFailed, updatedStatus.RotateEncryptionKeysPhase)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectStage, rotationStatus.Stage)
			assert.Equal(t, controlPlane.Status.RotateEncryptionKeysPhase, updatedStatus.RotateEncryptionKeysPhase)
		})
	}
}

func Test_rotateEncryptionKeys_marksDoneWhenSingleServerHasFinishedStatus(t *testing.T) {
	mockPlanner := newMockPlanner(t, InfoFunctions{})
	mockPlanner.planner.store.equalities = testPlannerEqualities()

	controlPlane := newOwnedEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 1}, rkev1.RKEControlPlaneStatus{
		RotateEncryptionKeys:       &rkev1.RotateEncryptionKeys{Generation: 1},
		RotateEncryptionKeysPhase:  rkev1.RotateEncryptionKeysPhaseRotate,
		RotateEncryptionKeysLeader: "server-1",
		Initialization: rkev1.RKEControlPlaneInitializationStatus{
			ControlPlaneInitialized: ptr.To(true),
		},
	})
	clusterPlan := newTestEncryptionKeyRotationPlan(
		newTestEncryptionKeyRotationMachine("server-1", true, true, true, true, "https://server-1:9345"),
	)
	leader := newTestPlanEntryFromPlan(clusterPlan, "server-1")
	nodePlan, _, err := mockPlanner.planner.encryptionKeyRotationRotateKeysPlan(controlPlane, plan.Secret{}, "https://server-1:9345", leader)
	assert.NoError(t, err)
	clusterPlan.Nodes["server-1"] = &plan.Node{
		Plan:    nodePlan,
		InSync:  true,
		Healthy: true,
		PeriodicOutput: map[string]plan.PeriodicInstructionOutput{
			encryptionKeyRotationSecretsEncryptStatusCommand: {
				Stdout: []byte("Current Rotation Stage: reencrypt_finished\nServer Encryption Hashes: All hashes match\n"),
			},
		},
	}

	mockPlanner.capiClusters.EXPECT().Get(controlPlane.Namespace, "capi-cluster").Return(pausedCluster(controlPlane.Namespace, true), nil)
	mockPlanner.capiClusters.EXPECT().Get(controlPlane.Namespace, "capi-cluster").Return(pausedCluster(controlPlane.Namespace, false), nil)

	updatedStatus, err := mockPlanner.planner.rotateEncryptionKeys(controlPlane, controlPlane.Status, plan.Secret{}, clusterPlan)

	assert.True(t, IsErrWaiting(err))
	assert.Equal(t, rkev1.RotateEncryptionKeysPhaseDone, updatedStatus.RotateEncryptionKeysPhase)
	assert.Empty(t, updatedStatus.RotateEncryptionKeysLeader)
}

func Test_rotateEncryptionKeys_movesToRestartPhaseWhenEtcdOnlyFollowersExist(t *testing.T) {
	mockPlanner := newMockPlanner(t, InfoFunctions{})
	mockPlanner.planner.store.equalities = testPlannerEqualities()

	controlPlane := newOwnedEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 1}, rkev1.RKEControlPlaneStatus{
		RotateEncryptionKeys:       &rkev1.RotateEncryptionKeys{Generation: 1},
		RotateEncryptionKeysPhase:  rkev1.RotateEncryptionKeysPhaseRotate,
		RotateEncryptionKeysLeader: "control-plane-leader",
		Initialization: rkev1.RKEControlPlaneInitializationStatus{
			ControlPlaneInitialized: ptr.To(true),
		},
	})
	clusterPlan := newTestEncryptionKeyRotationPlan(
		newTestEncryptionKeyRotationMachine("etcd-init", true, false, true, true, "https://etcd-init:9345"),
		newTestEncryptionKeyRotationMachine("control-plane-leader", true, true, false, true, "https://control-plane-leader:9345"),
		newTestEncryptionKeyRotationMachine("etcd-follower", true, false, false, true, "https://etcd-follower:9345"),
	)
	leader := newTestPlanEntryFromPlan(clusterPlan, "control-plane-leader")
	nodePlan, _, err := mockPlanner.planner.encryptionKeyRotationRotateKeysPlan(controlPlane, plan.Secret{}, "https://etcd-init:9345", leader)
	assert.NoError(t, err)
	clusterPlan.Nodes["control-plane-leader"] = &plan.Node{
		Plan:    nodePlan,
		InSync:  true,
		Healthy: true,
		PeriodicOutput: map[string]plan.PeriodicInstructionOutput{
			encryptionKeyRotationSecretsEncryptStatusCommand: {
				Stdout: []byte("Current Rotation Stage: reencrypt_finished\nServer Encryption Hashes: All hashes match\n"),
			},
		},
	}

	mockPlanner.capiClusters.EXPECT().Get(controlPlane.Namespace, "capi-cluster").Return(pausedCluster(controlPlane.Namespace, true), nil)

	updatedStatus, err := mockPlanner.planner.rotateEncryptionKeys(controlPlane, controlPlane.Status, plan.Secret{}, clusterPlan)

	assert.True(t, IsErrWaiting(err))
	assert.Equal(t, rkev1.RotateEncryptionKeysPhasePostRotateRestart, updatedStatus.RotateEncryptionKeysPhase)
	assert.Equal(t, "control-plane-leader", updatedStatus.RotateEncryptionKeysLeader)
}

func Test_rotateEncryptionKeys_requeuesWhileLeaderStatusIncomplete(t *testing.T) {
	mockPlanner := newMockPlanner(t, InfoFunctions{})
	mockPlanner.planner.store.equalities = testPlannerEqualities()

	controlPlane := newOwnedEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 1}, rkev1.RKEControlPlaneStatus{
		RotateEncryptionKeys:       &rkev1.RotateEncryptionKeys{Generation: 1},
		RotateEncryptionKeysPhase:  rkev1.RotateEncryptionKeysPhaseRotate,
		RotateEncryptionKeysLeader: "server-1",
		Initialization: rkev1.RKEControlPlaneInitializationStatus{
			ControlPlaneInitialized: ptr.To(true),
		},
	})
	clusterPlan := newTestEncryptionKeyRotationPlan(
		newTestEncryptionKeyRotationMachine("server-1", true, true, true, true, "https://server-1:9345"),
	)
	leader := newTestPlanEntryFromPlan(clusterPlan, "server-1")
	nodePlan, _, err := mockPlanner.planner.encryptionKeyRotationRotateKeysPlan(controlPlane, plan.Secret{}, "https://server-1:9345", leader)
	assert.NoError(t, err)
	clusterPlan.Nodes["server-1"] = &plan.Node{
		Plan:    nodePlan,
		InSync:  true,
		Healthy: true,
		PeriodicOutput: map[string]plan.PeriodicInstructionOutput{
			encryptionKeyRotationSecretsEncryptStatusCommand: {
				Stdout: []byte("Current Rotation Stage: start\nServer Encryption Hashes: hash mismatch\n"),
			},
		},
	}

	mockPlanner.capiClusters.EXPECT().Get(controlPlane.Namespace, "capi-cluster").Return(pausedCluster(controlPlane.Namespace, true), nil)

	updatedStatus, err := mockPlanner.planner.rotateEncryptionKeys(controlPlane, controlPlane.Status, plan.Secret{}, clusterPlan)

	assert.True(t, IsErrWaiting(err))
	assert.Equal(t, rkev1.RotateEncryptionKeysPhaseRotate, updatedStatus.RotateEncryptionKeysPhase)
	assert.Equal(t, "server-1", updatedStatus.RotateEncryptionKeysLeader)
}

func Test_rotateEncryptionKeys_marksFailedWhenLeaderPlanFails(t *testing.T) {
	mockPlanner := newMockPlanner(t, InfoFunctions{})
	mockPlanner.planner.store.equalities = testPlannerEqualities()

	controlPlane := newOwnedEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 1}, rkev1.RKEControlPlaneStatus{
		RotateEncryptionKeys:       &rkev1.RotateEncryptionKeys{Generation: 1},
		RotateEncryptionKeysPhase:  rkev1.RotateEncryptionKeysPhaseRotate,
		RotateEncryptionKeysLeader: "server-1",
		Initialization: rkev1.RKEControlPlaneInitializationStatus{
			ControlPlaneInitialized: ptr.To(true),
		},
	})
	clusterPlan := newTestEncryptionKeyRotationPlan(
		newTestEncryptionKeyRotationMachine("server-1", true, true, true, true, "https://server-1:9345"),
	)
	leader := newTestPlanEntryFromPlan(clusterPlan, "server-1")
	nodePlan, _, err := mockPlanner.planner.encryptionKeyRotationRotateKeysPlan(controlPlane, plan.Secret{}, "https://server-1:9345", leader)
	assert.NoError(t, err)
	clusterPlan.Nodes["server-1"] = &plan.Node{
		Plan:   nodePlan,
		Failed: true,
		InSync: true,
	}

	mockPlanner.capiClusters.EXPECT().Get(controlPlane.Namespace, "capi-cluster").Return(pausedCluster(controlPlane.Namespace, true), nil)
	mockPlanner.capiClusters.EXPECT().Get(controlPlane.Namespace, "capi-cluster").Return(pausedCluster(controlPlane.Namespace, false), nil)

	updatedStatus, err := mockPlanner.planner.rotateEncryptionKeys(controlPlane, controlPlane.Status, plan.Secret{}, clusterPlan)

	assert.Error(t, err)
	assert.Equal(t, rkev1.RotateEncryptionKeysPhaseFailed, updatedStatus.RotateEncryptionKeysPhase)
	assert.Empty(t, updatedStatus.RotateEncryptionKeysLeader)
}

func Test_rotateEncryptionKeys_requeuesWhenLeaderRotateKeysTimesOut(t *testing.T) {
	mockPlanner := newMockPlanner(t, InfoFunctions{})
	mockPlanner.planner.store.equalities = testPlannerEqualities()

	controlPlane := newOwnedEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 1}, rkev1.RKEControlPlaneStatus{
		RotateEncryptionKeys:       &rkev1.RotateEncryptionKeys{Generation: 1},
		RotateEncryptionKeysPhase:  rkev1.RotateEncryptionKeysPhaseRotate,
		RotateEncryptionKeysLeader: "server-1",
		Initialization: rkev1.RKEControlPlaneInitializationStatus{
			ControlPlaneInitialized: ptr.To(true),
		},
	})
	clusterPlan := newTestEncryptionKeyRotationPlan(
		newTestEncryptionKeyRotationMachine("server-1", true, true, true, true, "https://server-1:9345"),
	)
	leader := newTestPlanEntryFromPlan(clusterPlan, "server-1")
	nodePlan, _, err := mockPlanner.planner.encryptionKeyRotationRotateKeysPlan(controlPlane, plan.Secret{}, "https://server-1:9345", leader)
	assert.NoError(t, err)
	clusterPlan.Nodes["server-1"] = &plan.Node{
		Plan:   nodePlan,
		Failed: true,
		InSync: true,
		FailedOutput: map[string][]byte{
			nodePlan.Instructions[0].Name: []byte("level=fatal msg=\"Put \\\"https://127.0.0.1:6443/v1-k3s/encrypt/config\\\": context deadline exceeded (Client.Timeout exceeded while awaiting headers): " + encryptionKeyRotationRotateKeysTimeoutMessage + "\"\n"),
		},
		PeriodicOutput: map[string]plan.PeriodicInstructionOutput{
			encryptionKeyRotationSecretsEncryptStatusCommand: {
				Stdout: []byte("Current Rotation Stage: start\nServer Encryption Hashes: hash mismatch\n"),
			},
		},
	}

	mockPlanner.capiClusters.EXPECT().Get(controlPlane.Namespace, "capi-cluster").Return(pausedCluster(controlPlane.Namespace, true), nil)

	updatedStatus, err := mockPlanner.planner.rotateEncryptionKeys(controlPlane, controlPlane.Status, plan.Secret{}, clusterPlan)

	assert.True(t, IsErrWaiting(err))
	assert.Equal(t, rkev1.RotateEncryptionKeysPhaseRotate, updatedStatus.RotateEncryptionKeysPhase)
	assert.Equal(t, "server-1", updatedStatus.RotateEncryptionKeysLeader)
}

func Test_rotateEncryptionKeys_marksFailedWhenNoSuitableLeaderExists(t *testing.T) {
	mockPlanner := newMockPlanner(t, InfoFunctions{})
	mockPlanner.planner.store.equalities = testPlannerEqualities()

	controlPlane := newOwnedEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 1}, rkev1.RKEControlPlaneStatus{
		RotateEncryptionKeys:      &rkev1.RotateEncryptionKeys{Generation: 1},
		RotateEncryptionKeysPhase: rkev1.RotateEncryptionKeysPhaseRotate,
		Initialization: rkev1.RKEControlPlaneInitializationStatus{
			ControlPlaneInitialized: ptr.To(true),
		},
	})
	clusterPlan := newTestEncryptionKeyRotationPlan(
		newTestEncryptionKeyRotationMachine("etcd-init", true, false, true, true, "https://etcd-init:9345"),
	)

	mockPlanner.capiClusters.EXPECT().Get(controlPlane.Namespace, "capi-cluster").Return(pausedCluster(controlPlane.Namespace, false), nil)

	updatedStatus, err := mockPlanner.planner.rotateEncryptionKeys(controlPlane, controlPlane.Status, plan.Secret{}, clusterPlan)

	assert.Error(t, err)
	assert.Equal(t, rkev1.RotateEncryptionKeysPhaseFailed, updatedStatus.RotateEncryptionKeysPhase)
	assert.Empty(t, updatedStatus.RotateEncryptionKeysLeader)
}

func Test_rotateEncryptionKeys_marksFailedWhenVersionUnsupported(t *testing.T) {
	mockPlanner := newMockPlanner(t, InfoFunctions{})
	mockPlanner.planner.store.equalities = testPlannerEqualities()

	controlPlane := newOwnedEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 1}, rkev1.RKEControlPlaneStatus{
		Initialization: rkev1.RKEControlPlaneInitializationStatus{
			ControlPlaneInitialized: ptr.To(true),
		},
	})
	controlPlane.Spec.KubernetesVersion = "v1.29.9+rke2r1"

	mockPlanner.capiClusters.EXPECT().Get(controlPlane.Namespace, "capi-cluster").Return(pausedCluster(controlPlane.Namespace, false), nil)

	updatedStatus, err := mockPlanner.planner.rotateEncryptionKeys(controlPlane, controlPlane.Status, plan.Secret{}, &plan.Plan{})

	assert.True(t, IsErrWaiting(err))
	assert.Equal(t, rkev1.RotateEncryptionKeysPhaseFailed, updatedStatus.RotateEncryptionKeysPhase)
	assert.Empty(t, updatedStatus.RotateEncryptionKeysLeader)
}

func Test_encryptionKeyRotationRotateKeysReconcile_isIdempotentAcrossRepeatedCalls(t *testing.T) {
	planner := newTestEncryptionKeyRotationPlanner()
	controlPlane := newTestEncryptionKeyRotationControlPlane(&rkev1.RotateEncryptionKeys{Generation: 1}, rkev1.RKEControlPlaneStatus{
		RotateEncryptionKeys:      &rkev1.RotateEncryptionKeys{Generation: 1},
		RotateEncryptionKeysPhase: rkev1.RotateEncryptionKeysPhaseRotate,
	})
	leaderEntry := newTestPlanEntry(newTestEncryptionKeyRotationMachine("server-1", true, true, true, true, "https://server-1:9345"))
	nodePlan, _, err := planner.encryptionKeyRotationRotateKeysPlan(controlPlane, plan.Secret{}, "https://server-1:9345", leaderEntry)
	assert.NoError(t, err)
	leaderEntry.Plan = &plan.Node{
		Plan:    nodePlan,
		InSync:  true,
		Healthy: true,
		PeriodicOutput: map[string]plan.PeriodicInstructionOutput{
			encryptionKeyRotationSecretsEncryptStatusCommand: {
				Stdout: []byte("Current Rotation Stage: start\nServer Encryption Hashes: hash mismatch\n"),
			},
		},
	}

	firstStatus, updatedStatus, firstErr := planner.encryptionKeyRotationRotateKeysReconcile(controlPlane, controlPlane.Status, plan.Secret{}, "https://server-1:9345", leaderEntry)
	secondStatus, secondUpdatedStatus, secondErr := planner.encryptionKeyRotationRotateKeysReconcile(controlPlane, controlPlane.Status, plan.Secret{}, "https://server-1:9345", leaderEntry)

	assert.NoError(t, firstErr)
	assert.NoError(t, secondErr)
	assert.Equal(t, firstStatus, secondStatus)
	assert.Equal(t, updatedStatus, secondUpdatedStatus)
	assert.Equal(t, encryptionKeyRotationStageStart, firstStatus.Stage)
}

func newTestEncryptionKeyRotationPlanner() *Planner {
	return &Planner{
		store: &PlanStore{
			equalities: testPlannerEqualities(),
		},
	}
}

func testPlannerEqualities() conversion.Equalities {
	equalities := equality.Semantic.Copy()
	_ = equalities.AddFunc(func(a, b plan.File) bool {
		return a.Content == b.Content && a.Path == b.Path && a.Permissions == b.Permissions && a.Dynamic == b.Dynamic && a.Minor == b.Minor
	})
	return equalities
}

func newTestEncryptionKeyRotationControlPlane(rotate *rkev1.RotateEncryptionKeys, status rkev1.RKEControlPlaneStatus) *rkev1.RKEControlPlane {
	controlPlane := &rkev1.RKEControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-control-plane",
			Namespace: "fleet-default",
		},
		Spec: rkev1.RKEControlPlaneSpec{
			ClusterName:          "test-cluster",
			KubernetesVersion:    "v1.30.6+rke2r1",
			RotateEncryptionKeys: rotate,
			UnmanagedConfig:      true,
		},
		Status: status,
	}
	capr.Ready.True(controlPlane)
	return controlPlane
}

func newOwnedEncryptionKeyRotationControlPlane(rotate *rkev1.RotateEncryptionKeys, status rkev1.RKEControlPlaneStatus) *rkev1.RKEControlPlane {
	controlPlane := newTestEncryptionKeyRotationControlPlane(rotate, status)
	controlPlane.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: capi.GroupVersion.String(),
			Kind:       "Cluster",
			Name:       "capi-cluster",
			Controller: ptr.To(true),
		},
	}
	return controlPlane
}

func newTestEncryptionKeyRotationMachine(name string, etcd, controlPlane, initNode, ready bool, joinURL string) *planEntry {
	labels := map[string]string{}
	if etcd {
		labels[capr.EtcdRoleLabel] = "true"
	}
	if controlPlane {
		labels[capr.ControlPlaneRoleLabel] = "true"
	}
	if initNode {
		labels[capr.InitNodeLabel] = "true"
	}

	machine := &capi.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "fleet-default",
		},
		Status: capi.MachineStatus{
			NodeRef: capi.MachineNodeReference{Name: name},
		},
	}
	metadata := &plan.Metadata{
		Labels:      labels,
		Annotations: map[string]string{capr.JoinURLAnnotation: joinURL},
	}

	if ready {
		capr.SetCAPIResourceCondition(machine, metav1.Condition{
			Type:               string(capr.Ready),
			Status:             metav1.ConditionTrue,
			Reason:             "Test",
			LastTransitionTime: metav1.Now(),
		})
		capr.SetCAPIResourceCondition(machine, metav1.Condition{
			Type:               capi.InfrastructureReadyCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "Test",
			LastTransitionTime: metav1.Now(),
		})
	}

	return &planEntry{
		Machine:  machine,
		Plan:     &plan.Node{},
		Metadata: metadata,
	}
}

func newTestPlanEntry(entry *planEntry) *planEntry {
	return &planEntry{
		Machine:  entry.Machine.DeepCopy(),
		Plan:     copyNode(entry.Plan),
		Metadata: &plan.Metadata{Labels: copyStringMap(entry.Metadata.Labels), Annotations: copyStringMap(entry.Metadata.Annotations)},
	}
}

func newTestEncryptionKeyRotationPlan(entries ...*planEntry) *plan.Plan {
	clusterPlan := &plan.Plan{
		Machines: map[string]*capi.Machine{},
		Nodes:    map[string]*plan.Node{},
		Metadata: map[string]*plan.Metadata{},
	}

	for _, entry := range entries {
		clusterPlan.Machines[entry.Machine.Name] = entry.Machine.DeepCopy()
		clusterPlan.Nodes[entry.Machine.Name] = copyNode(entry.Plan)
		clusterPlan.Metadata[entry.Machine.Name] = &plan.Metadata{
			Labels:      copyStringMap(entry.Metadata.Labels),
			Annotations: copyStringMap(entry.Metadata.Annotations),
		}
	}

	return clusterPlan
}

func newTestPlanEntryFromPlan(clusterPlan *plan.Plan, machineName string) *planEntry {
	return &planEntry{
		Machine:  clusterPlan.Machines[machineName],
		Plan:     clusterPlan.Nodes[machineName],
		Metadata: clusterPlan.Metadata[machineName],
	}
}

func pausedCluster(namespace string, paused bool) *capi.Cluster {
	return &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "capi-cluster",
			Namespace: namespace,
		},
		Spec: capi.ClusterSpec{
			Paused: ptr.To(paused),
		},
	}
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}

	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}

	return out
}

func copyNode(in *plan.Node) *plan.Node {
	if in == nil {
		return nil
	}

	out := *in
	if in.Output != nil {
		out.Output = make(map[string][]byte, len(in.Output))
		for key, value := range in.Output {
			out.Output[key] = append([]byte(nil), value...)
		}
	}
	if in.FailedOutput != nil {
		out.FailedOutput = make(map[string][]byte, len(in.FailedOutput))
		for key, value := range in.FailedOutput {
			out.FailedOutput[key] = append([]byte(nil), value...)
		}
	}
	if in.PeriodicOutput != nil {
		out.PeriodicOutput = make(map[string]plan.PeriodicInstructionOutput, len(in.PeriodicOutput))
		for key, value := range in.PeriodicOutput {
			value.Stdout = append([]byte(nil), value.Stdout...)
			value.Stderr = append([]byte(nil), value.Stderr...)
			out.PeriodicOutput[key] = value
		}
	}
	if in.ProbeStatus != nil {
		out.ProbeStatus = make(map[string]plan.ProbeStatus, len(in.ProbeStatus))
		for key, value := range in.ProbeStatus {
			out.ProbeStatus[key] = value
		}
	}

	return &out
}

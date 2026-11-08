package planner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/moby/locker"
	"github.com/rancher/channelserver/pkg/model"
	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestHasEtcdVoter(t *testing.T) {
	newCandidate := func() (*rkev1.RKEControlPlane, *planEntry) {
		controlPlane := &rkev1.RKEControlPlane{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns"},
			Spec:       rkev1.RKEControlPlaneSpec{ManagementClusterName: "management"},
		}
		replacement := &capi.Machine{ObjectMeta: metav1.ObjectMeta{Name: "replacement", UID: types.UID("replacement-uid")}, Status: capi.MachineStatus{Conditions: []metav1.Condition{{Type: capi.InfrastructureReadyCondition, Status: metav1.ConditionTrue}}}}
		replacementMeta := &plan.Metadata{Labels: map[string]string{capr.EtcdRoleLabel: "true"}, Annotations: map[string]string{capr.JoinedToAnnotation: "https://old:9345"}}
		return controlPlane, &planEntry{Machine: replacement, Metadata: replacementMeta}
	}

	tests := []struct {
		name      string
		node      *apisv3.Node
		listErr   error
		wantVoter bool
		clearUID  bool
	}{
		{name: "positive status label mapping", node: voterNode("replacement-uid", true), wantVoter: true},
		{name: "wrong uid", node: voterNode("another-uid", true)},
		{name: "metadata label does not map", node: &apisv3.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{capr.MachineUIDLabel: "replacement-uid"}}}},
		{name: "missing condition", node: voterNode("replacement-uid", false)},
		{name: "false condition", node: voterNodeWithStatus("replacement-uid", corev1.ConditionFalse)},
		{name: "unknown condition", node: voterNodeWithStatus("replacement-uid", corev1.ConditionUnknown)},
		{name: "cache error", listErr: errors.New("cache unavailable")},
		{name: "empty identity", node: voterNode("replacement-uid", true), clearUID: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := newMockPlanner(t, InfoFunctions{})
			controlPlane, candidate := newCandidate()
			if tt.clearUID {
				candidate.Machine.UID = ""
			}
			if !tt.clearUID {
				if tt.listErr != nil {
					mp.managementNodes.EXPECT().List("management", labels.Everything()).Return(nil, tt.listErr)
				} else {
					mp.managementNodes.EXPECT().List("management", labels.Everything()).Return([]*apisv3.Node{tt.node}, nil)
				}
			}
			require.Equal(t, tt.wantVoter, mp.planner.hasEtcdVoter(controlPlane, candidate))
		})
	}
}

func TestProtectedInitHandoffSelectsOnlyJoinedVoter(t *testing.T) {
	mp := newMockPlanner(t, InfoFunctions{})
	controlPlane := &rkev1.RKEControlPlane{Spec: rkev1.RKEControlPlaneSpec{ManagementClusterName: "management"}}
	newMachine := func(name, uid string) *capi.Machine {
		return &capi.Machine{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", UID: types.UID(uid)},
			Spec: capi.MachineSpec{Bootstrap: capi.Bootstrap{ConfigRef: capi.ContractVersionedObjectReference{
				APIGroup: capr.RKEAPIGroup,
				Kind:     capr.RKEBootstrapKind,
				Name:     name,
			}}},
			Status: capi.MachineStatus{Conditions: []metav1.Condition{{Type: capi.InfrastructureReadyCondition, Status: metav1.ConditionTrue}}},
		}
	}
	old := newMachine("old", "old-uid")
	old.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	first := newMachine("a-earlier", "uid-a")
	emptyJoinedTo := newMachine("b-empty", "uid-b")
	joined := newMachine("c-joined", "uid-c")
	oldMetadata := &plan.Metadata{Labels: map[string]string{capr.EtcdRoleLabel: "true", capr.InitNodeLabel: "true"}, Annotations: map[string]string{capr.JoinURLAnnotation: "https://old:9345"}}
	firstMetadata := &plan.Metadata{Labels: map[string]string{capr.EtcdRoleLabel: "true"}, Annotations: map[string]string{capr.JoinURLAnnotation: "https://a:9345", capr.JoinedToAnnotation: "https://other:9345"}}
	emptyMetadata := &plan.Metadata{Labels: map[string]string{capr.EtcdRoleLabel: "true"}, Annotations: map[string]string{capr.JoinURLAnnotation: "https://b:9345", capr.JoinedToAnnotation: ""}}
	joinedMetadata := &plan.Metadata{Labels: map[string]string{capr.EtcdRoleLabel: "true"}, Annotations: map[string]string{capr.JoinURLAnnotation: "https://c:9345", capr.JoinedToAnnotation: "https://old:9345"}}
	clusterPlan := &plan.Plan{
		Machines: map[string]*capi.Machine{"old": old, "a-earlier": first, "b-empty": emptyJoinedTo, "c-joined": joined},
		Metadata: map[string]*plan.Metadata{"old": oldMetadata, "a-earlier": firstMetadata, "b-empty": emptyMetadata, "c-joined": joinedMetadata},
	}
	secrets, _ := configureRecordedPlanSecretWrites(mp, clusterPlan, "", 0)
	mp.managementNodes.EXPECT().List("management", labels.Everything()).Return([]*apisv3.Node{
		voterNode("uid-a", true),
		voterNode("uid-b", true),
		voterNode("uid-c", true),
	}, nil).Times(1)
	_, err := mp.planner.fullReconcile(controlPlane, rkev1.RKEControlPlaneStatus{}, &capi.Cluster{}, plan.Secret{}, clusterPlan, false)
	require.ErrorIs(t, err, generic.ErrSkip)
	require.Equal(t, "true", clusterPlan.Metadata["c-joined"].Labels[capr.InitNodeLabel])
	require.Empty(t, clusterPlan.Metadata["old"].Labels[capr.InitNodeLabel])
	require.Equal(t, "true", secrets[capr.PlanSecretFromBootstrapName("c-joined")].Labels[capr.InitNodeLabel])
	require.NotContains(t, clusterPlan.Metadata["a-earlier"].Labels, capr.InitNodeLabel)
	require.NotContains(t, clusterPlan.Metadata["b-empty"].Labels, capr.InitNodeLabel)
}

func TestProtectedHandoffDoesNotOverrideFixedInitSelection(t *testing.T) {
	controlPlane := &rkev1.RKEControlPlane{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{capr.InitNodeMachineIDLabel: "fixed"}}}
	old := &capi.Machine{ObjectMeta: metav1.ObjectMeta{Name: "old", DeletionTimestamp: &metav1.Time{Time: time.Now()}}}
	clusterPlan := &plan.Plan{
		Machines: map[string]*capi.Machine{"old": old},
		Metadata: map[string]*plan.Metadata{"old": {Labels: map[string]string{capr.EtcdRoleLabel: "true", capr.InitNodeLabel: "true"}, Annotations: map[string]string{capr.JoinURLAnnotation: "https://old:9345"}}},
	}
	require.Nil(t, protectedInitTarget(controlPlane, clusterPlan))
}

func voterNode(uid string, include bool) *apisv3.Node {
	if !include {
		return &apisv3.Node{Status: apisv3.NodeStatus{NodeLabels: map[string]string{capr.MachineUIDLabel: uid}}}
	}
	return voterNodeWithStatus(uid, corev1.ConditionTrue)
}

func voterNodeWithStatus(uid string, status corev1.ConditionStatus) *apisv3.Node {
	return &apisv3.Node{Status: apisv3.NodeStatus{
		NodeLabels:         map[string]string{capr.MachineUIDLabel: uid},
		InternalNodeStatus: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: "EtcdIsVoter", Status: status}}},
	}}
}

func TestFreshPromotionRequiresOwnJoinURL(t *testing.T) {
	mp, controlPlane, clusterPlan := protectedHandoffFixture(t)
	delete(clusterPlan.Metadata["b-qualified"].Annotations, capr.JoinURLAnnotation)
	secrets, _ := configureRecordedPlanSecretWrites(mp, clusterPlan, "", 0)
	mp.managementNodes.EXPECT().List("management", labels.Everything()).Return([]*apisv3.Node{voterNode("b-qualified-uid", true)}, nil).AnyTimes()
	mp.rkeControlPlanes.EXPECT().EnqueueAfter("ns", "cluster", 5*time.Second).Times(1)
	status := rkev1.RKEControlPlaneStatus{}
	capr.Bootstrapped.True(&status)

	// Joined-to and voter evidence alone do not make the replacement usable as an init node.
	_, err := mp.planner.fullReconcile(controlPlane, status, &capi.Cluster{}, plan.Secret{}, clusterPlan, false)
	require.Error(t, err)
	require.NotErrorIs(t, err, generic.ErrSkip)
	require.Empty(t, clusterPlan.Metadata["b-qualified"].Labels[capr.InitNodeLabel])
	require.Equal(t, "true", clusterPlan.Metadata["old"].Labels[capr.InitNodeLabel])
	require.Empty(t, secrets[capr.PlanSecretFromBootstrapName("b-qualified")].Labels[capr.InitNodeLabel])

	// Once the replacement has its own join URL, the same observed voter evidence promotes it.
	clusterPlan.Metadata["b-qualified"].Annotations[capr.JoinURLAnnotation] = "https://b:9345"
	_, err = mp.planner.fullReconcile(controlPlane, status, &capi.Cluster{}, plan.Secret{}, clusterPlan, false)
	require.ErrorIs(t, err, generic.ErrSkip)
	require.Equal(t, "true", clusterPlan.Metadata["b-qualified"].Labels[capr.InitNodeLabel])
	require.Empty(t, clusterPlan.Metadata["old"].Labels[capr.InitNodeLabel])
}

func TestPersistedReplacementBeatsEarlierQualifiedCandidate(t *testing.T) {
	mp, controlPlane, clusterPlan := protectedHandoffFixture(t)
	clusterPlan.Metadata["b-qualified"].Labels[capr.InitNodeLabel] = "true"
	secrets, _ := configureRecordedPlanSecretWrites(mp, clusterPlan, "", 0)
	// a-other sorts first and also passes the fresh gate, so only the persisted-first rule can pick B.
	mp.managementNodes.EXPECT().List("management", labels.Everything()).Return([]*apisv3.Node{voterNode("a-other-uid", true), voterNode("b-qualified-uid", true)}, nil).AnyTimes()
	status := rkev1.RKEControlPlaneStatus{}
	capr.Bootstrapped.True(&status)

	_, err := mp.planner.fullReconcile(controlPlane, status, &capi.Cluster{}, plan.Secret{}, clusterPlan, false)
	require.ErrorIs(t, err, generic.ErrSkip)
	require.Equal(t, "true", clusterPlan.Metadata["b-qualified"].Labels[capr.InitNodeLabel])
	require.NotContains(t, clusterPlan.Metadata["a-other"].Labels, capr.InitNodeLabel)
	require.Empty(t, clusterPlan.Metadata["old"].Labels[capr.InitNodeLabel])
	require.Equal(t, "true", secrets[capr.PlanSecretFromBootstrapName("b-qualified")].Labels[capr.InitNodeLabel])
	require.NotContains(t, secrets[capr.PlanSecretFromBootstrapName("a-other")].Labels, capr.InitNodeLabel)
}

func TestInterruptedMoveFinishesWithoutVoterObservation(t *testing.T) {
	mp, controlPlane, clusterPlan := protectedHandoffFixture(t)
	clusterPlan.Metadata["b-qualified"].Labels[capr.InitNodeLabel] = "true"
	secrets, _ := configureRecordedPlanSecretWrites(mp, clusterPlan, "", 0)
	// The cache has no voter observation for the replacement. The saved init mark is the decision.
	mp.managementNodes.EXPECT().List("management", labels.Everything()).Times(0)
	status := rkev1.RKEControlPlaneStatus{}
	capr.Bootstrapped.True(&status)

	_, err := mp.planner.fullReconcile(controlPlane, status, &capi.Cluster{}, plan.Secret{}, clusterPlan, false)
	require.ErrorIs(t, err, generic.ErrSkip)
	require.Empty(t, clusterPlan.Metadata["old"].Labels[capr.InitNodeLabel])
	require.Equal(t, "true", clusterPlan.Metadata["b-qualified"].Labels[capr.InitNodeLabel])
	require.NotContains(t, secrets[capr.PlanSecretFromBootstrapName("old")].Labels, capr.InitNodeLabel)
}

func TestProcessRejectsConflictingInitMarks(t *testing.T) {
	// Mismatched B: marked, but its persisted joined-to is not A's URL.
	mismatchB := func(secrets map[string]*corev1.Secret) {
		secrets[capr.PlanSecretFromBootstrapName("b-qualified")].Annotations[capr.JoinedToAnnotation] = "https://other:9345"
	}
	requireRefused := func(t *testing.T, mp *mockPlanner, controlPlane *rkev1.RKEControlPlane, secrets map[string]*corev1.Secret, updates *[]string) {
		t.Helper()
		_, err := mp.planner.Process(controlPlane, controlPlane.Status)
		require.Truef(t, IsErrWaiting(err), "unexpected error: %v", err)
		require.ErrorContains(t, err, "multiple init nodes found")
		require.Empty(t, *updates, "a conflicting init mark must refuse before any Secret write")
		require.Equal(t, "true", secrets[capr.PlanSecretFromBootstrapName("old")].Labels[capr.InitNodeLabel])
		require.Equal(t, "true", secrets[capr.PlanSecretFromBootstrapName("b-qualified")].Labels[capr.InitNodeLabel])
	}

	t.Run("replacement not joined to the old init node", func(t *testing.T) {
		mp, controlPlane, secrets, updates, _, _ := processProtectedHandoff(t, "b-qualified")
		mismatchB(secrets)
		mp.rkeControlPlanes.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		requireRefused(t, mp, controlPlane, secrets, updates)
	})

	t.Run("qualifying voter beside the mismatched init mark", func(t *testing.T) {
		mp, controlPlane, secrets, updates, _, _ := processProtectedHandoff(t, "b-qualified")
		mismatchB(secrets)
		// a-other is a fresh voter joined to A, so it would qualify if the unrelated B mark were not there.
		nodes := fake.NewMockCacheInterface[*apisv3.Node](gomock.NewController(t))
		nodes.EXPECT().List("management", labels.Everything()).Return([]*apisv3.Node{voterNode("a-other-uid", true)}, nil).AnyTimes()
		mp.planner.managementNodes = nodes
		mp.rkeControlPlanes.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		requireRefused(t, mp, controlPlane, secrets, updates)
		require.NotContains(t, secrets[capr.PlanSecretFromBootstrapName("a-other")].Labels, capr.InitNodeLabel, "a-other must not receive the init mark")
	})

	t.Run("failed replacement candidate still carries an init mark", func(t *testing.T) {
		mp, controlPlane, secrets, updates, _, clusterPlan := processProtectedHandoff(t, "b-qualified")
		mismatchB(secrets)
		clusterPlan.Machines["b-qualified"].Status.Phase = string(capi.MachinePhaseFailed)
		mp.rkeControlPlanes.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		requireRefused(t, mp, controlPlane, secrets, updates)
	})
}

func TestSharedElectionPrefersCurrentInitOwnJoinURL(t *testing.T) {
	mp, _, clusterPlan := protectedHandoffFixture(t)
	delete(clusterPlan.Metadata["old"].Labels, capr.InitNodeLabel)
	clusterPlan.Metadata["b-qualified"].Labels[capr.InitNodeLabel] = "true"
	clusterPlan.Metadata["b-qualified"].Annotations[capr.JoinURLAnnotation] = "https://new:9345"
	joinURL, err := mp.planner.electInitNode(&rkev1.RKEControlPlane{}, clusterPlan, true)
	require.NoError(t, err)
	require.Equal(t, "https://new:9345", joinURL)
}

func TestPartialHandoffResumesMarkedReplacementAfterOldWriteFailure(t *testing.T) {
	mp, controlPlane, clusterPlan := protectedHandoffFixture(t)
	clusterPlan.Metadata["b-qualified"].Labels[capr.InitNodeLabel] = "true"
	secrets, _ := configureRecordedPlanSecretWrites(mp, clusterPlan, capr.PlanSecretFromBootstrapName("old"), 1)
	require.ErrorContains(t, reconcileProtectedHandoff(mp, controlPlane, clusterPlan), "injected "+capr.PlanSecretFromBootstrapName("old")+" write failure")
	require.Equal(t, "true", clusterPlan.Metadata["b-qualified"].Labels[capr.InitNodeLabel])
	require.Equal(t, "true", clusterPlan.Metadata["old"].Labels[capr.InitNodeLabel])
	require.Equal(t, "true", secrets[capr.PlanSecretFromBootstrapName("b-qualified")].Labels[capr.InitNodeLabel])
	require.Equal(t, "true", secrets[capr.PlanSecretFromBootstrapName("old")].Labels[capr.InitNodeLabel])
	require.ErrorIs(t, reconcileProtectedHandoff(mp, controlPlane, clusterPlan), generic.ErrSkip)
	require.Equal(t, 1, len(collect(clusterPlan, isInitNode)))
	require.Equal(t, "b-qualified", collect(clusterPlan, isInitNode)[0].Machine.Name)
	require.Equal(t, "true", secrets[capr.PlanSecretFromBootstrapName("b-qualified")].Labels[capr.InitNodeLabel])
	require.NotContains(t, secrets[capr.PlanSecretFromBootstrapName("old")].Labels, capr.InitNodeLabel)
}

func protectedHandoffFixture(t *testing.T) (*mockPlanner, *rkev1.RKEControlPlane, *plan.Plan) {
	mp := newMockPlanner(t, InfoFunctions{GetBootstrapManifests: func(*rkev1.RKEControlPlane) ([]plan.File, error) { return nil, nil }})
	controlPlane := &rkev1.RKEControlPlane{ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "ns"}, Spec: rkev1.RKEControlPlaneSpec{ManagementClusterName: "management"}}
	clusterPlan := &plan.Plan{Machines: map[string]*capi.Machine{}, Metadata: map[string]*plan.Metadata{}}
	// collect sorts by name, so a-other precedes b-qualified. This lets tests verify
	// that the saved init choice wins over an earlier candidate that also qualifies.
	for _, name := range []string{"old", "a-other", "b-qualified"} {
		machine := &capi.Machine{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", UID: types.UID(name + "-uid")},
			Spec: capi.MachineSpec{Bootstrap: capi.Bootstrap{ConfigRef: capi.ContractVersionedObjectReference{
				APIGroup: capr.RKEAPIGroup,
				Kind:     capr.RKEBootstrapKind,
				Name:     name,
			}}},
			Status: capi.MachineStatus{Conditions: []metav1.Condition{{
				Type:   capi.InfrastructureReadyCondition,
				Status: metav1.ConditionTrue,
			}}},
		}
		metadata := &plan.Metadata{
			Labels: map[string]string{capr.EtcdRoleLabel: "true"},
			Annotations: map[string]string{
				capr.JoinedToAnnotation: "https://old:9345",
			},
		}
		if name == "old" {
			machine.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			metadata.Labels[capr.InitNodeLabel] = "true"
			metadata.Annotations[capr.JoinURLAnnotation] = "https://old:9345"
		}
		if name == "a-other" {
			metadata.Annotations[capr.JoinURLAnnotation] = "https://a:9345"
		}
		if name == "b-qualified" {
			metadata.Annotations[capr.JoinURLAnnotation] = "https://b:9345"
		}
		clusterPlan.Machines[name], clusterPlan.Metadata[name] = machine, metadata
	}
	return mp, controlPlane, clusterPlan
}

func TestProcessRechecksProtectedHandoffWhileWaiting(t *testing.T) {
	mp, controlPlane, secrets, updates, _, _ := processProtectedHandoff(t)
	// Every pass re-checks the hand-off on the timer, so the planner waits without polling.
	var enqueues int
	mp.rkeControlPlanes.EXPECT().EnqueueAfter("ns", "cluster", 5*time.Second).Do(func(string, string, time.Duration) { enqueues++ }).AnyTimes()

	_, err := mp.planner.Process(controlPlane, controlPlane.Status)
	require.Truef(t, IsErrWaiting(err), "unexpected error: %v", err)
	require.Contains(t, err.Error(), "configuring etcd node(s) a-other,b-qualified")
	require.Contains(t, err.Error(), "; waiting for the replacement to join the existing etcd cluster through old")
	require.Equal(t, 1, enqueues, "the first pass schedules exactly one re-check")

	// Each pass the agent applies what was written. Once etcd is ready, the control-plane plan is written and then the hand-off wait is reached.
	for pass := 0; pass < 6 && !strings.HasPrefix(err.Error(), "waiting for the replacement to join"); pass++ {
		before := enqueues
		markWrittenPlansApplied(secrets)
		_, err = mp.planner.Process(controlPlane, controlPlane.Status)
		require.Truef(t, IsErrWaiting(err), "unexpected error on pass %d: %v", pass, err)
		require.Equal(t, before+1, enqueues, "pass %d must schedule exactly one re-check", pass)
	}
	require.True(t, strings.HasPrefix(err.Error(), "waiting for the replacement to join the existing etcd cluster through old"), "final pass must be the bare hand-off wait: %v", err)
	require.Contains(t, *updates, capr.PlanSecretFromBootstrapName("control-plane"), "the control-plane plan is delivered before the wait")
	require.NotContains(t, *updates, capr.PlanSecretFromBootstrapName("old"), "no label write before the replacement qualifies")
	require.Equal(t, "true", secrets[capr.PlanSecretFromBootstrapName("old")].Labels[capr.InitNodeLabel])
	require.NotContains(t, secrets[capr.PlanSecretFromBootstrapName("b-qualified")].Labels, capr.InitNodeLabel)
}

func TestMoveInitNodeMarkStopsWhenReplacementSaveFails(t *testing.T) {
	mp, _, clusterPlan := protectedHandoffFixture(t)
	oldSecret, replacementSecret := capr.PlanSecretFromBootstrapName("old"), capr.PlanSecretFromBootstrapName("b-qualified")
	secrets, updates := configureRecordedPlanSecretWrites(mp, clusterPlan, replacementSecret, 1)
	oldEntry, replacement := planEntryFor(clusterPlan, "old"), planEntryFor(clusterPlan, "b-qualified")

	err := mp.planner.moveInitNodeMark(oldEntry, replacement)
	require.ErrorContains(t, err, "injected")
	require.NotErrorIs(t, err, generic.ErrSkip)
	require.Equal(t, []string{replacementSecret}, *updates, "old mark must not be touched after the replacement save fails")
	require.Equal(t, "true", secrets[oldSecret].Labels[capr.InitNodeLabel])
	require.NotContains(t, secrets[replacementSecret].Labels, capr.InitNodeLabel)
	require.NotEqual(t, "true", replacement.Metadata.Labels[capr.InitNodeLabel], "failed local mutation must be rolled back")

	require.ErrorIs(t, mp.planner.moveInitNodeMark(oldEntry, replacement), generic.ErrSkip)
	require.Equal(t, []string{replacementSecret, replacementSecret, oldSecret}, *updates)
	require.Equal(t, "true", secrets[replacementSecret].Labels[capr.InitNodeLabel])
	require.NotContains(t, secrets[oldSecret].Labels, capr.InitNodeLabel)
}

func TestMoveInitNodeMarkResumesAfterOldClearFails(t *testing.T) {
	mp, controlPlane, clusterPlan := protectedHandoffFixture(t)
	oldSecret, replacementSecret := capr.PlanSecretFromBootstrapName("old"), capr.PlanSecretFromBootstrapName("b-qualified")
	aSecret := capr.PlanSecretFromBootstrapName("a-other")
	secrets, updates := configureRecordedPlanSecretWrites(mp, clusterPlan, oldSecret, 1)
	listCalls := 0
	mp.managementNodes.EXPECT().List("management", labels.Everything()).DoAndReturn(func(_ string, _ labels.Selector) ([]*apisv3.Node, error) {
		listCalls++
		return []*apisv3.Node{voterNode("b-qualified-uid", true)}, nil
	}).AnyTimes()
	status := rkev1.RKEControlPlaneStatus{}
	capr.Bootstrapped.True(&status)

	// Initial selection: B's save succeeds (ErrSkip), then A's real clear error is returned unmasked.
	_, err := mp.planner.fullReconcile(controlPlane, status, &capi.Cluster{}, plan.Secret{}, clusterPlan, false)
	require.ErrorContains(t, err, "injected")
	require.NotErrorIs(t, err, generic.ErrSkip)
	require.Equal(t, []string{replacementSecret, oldSecret}, *updates)
	require.Equal(t, "true", secrets[replacementSecret].Labels[capr.InitNodeLabel])
	require.Equal(t, "true", secrets[oldSecret].Labels[capr.InitNodeLabel])

	// A new reconcile starts from persisted metadata and must resume without a voter lookup.
	reloadPlanMetadataFromSecrets(clusterPlan, secrets)
	listCallsBeforeRepair := listCalls
	require.ErrorIs(t, reconcileProtectedHandoff(mp, controlPlane, clusterPlan), generic.ErrSkip)
	require.Equal(t, listCallsBeforeRepair, listCalls)
	require.Equal(t, []string{replacementSecret, oldSecret, oldSecret}, *updates)
	require.Equal(t, "true", secrets[replacementSecret].Labels[capr.InitNodeLabel])
	require.NotContains(t, secrets[oldSecret].Labels, capr.InitNodeLabel)
	require.NotContains(t, secrets[aSecret].Labels, capr.InitNodeLabel)
}

func TestMoveInitNodeMarkReplayIsNoOp(t *testing.T) {
	mp, controlPlane, clusterPlan := protectedHandoffFixture(t)
	oldSecret, replacementSecret := capr.PlanSecretFromBootstrapName("old"), capr.PlanSecretFromBootstrapName("b-qualified")
	secrets, updates := configureRecordedPlanSecretWrites(mp, clusterPlan, "", 0)

	require.ErrorIs(t, mp.planner.moveInitNodeMark(planEntryFor(clusterPlan, "old"), planEntryFor(clusterPlan, "b-qualified")), generic.ErrSkip)
	require.Equal(t, []string{replacementSecret, oldSecret}, *updates)

	reloadPlanMetadataFromSecrets(clusterPlan, secrets)
	require.NoError(t, mp.planner.moveInitNodeMark(planEntryFor(clusterPlan, "old"), planEntryFor(clusterPlan, "b-qualified")))
	require.Nil(t, protectedInitTarget(controlPlane, clusterPlan))
	require.Equal(t, []string{replacementSecret, oldSecret}, *updates)
}

func TestMoveInitNodeMarkIgnoresStaleObservationAfterCompletion(t *testing.T) {
	mp, controlPlane, clusterPlan := protectedHandoffFixture(t)
	oldSecret, replacementSecret := capr.PlanSecretFromBootstrapName("old"), capr.PlanSecretFromBootstrapName("b-qualified")
	secrets, updates := configureRecordedPlanSecretWrites(mp, clusterPlan, "", 0)

	require.ErrorIs(t, mp.planner.moveInitNodeMark(planEntryFor(clusterPlan, "old"), planEntryFor(clusterPlan, "b-qualified")), generic.ErrSkip)
	require.Equal(t, []string{replacementSecret, oldSecret}, *updates)

	// The persisted Secrets are complete, but the old observation still carries A's mark.
	clusterPlan.Metadata["old"].Labels[capr.InitNodeLabel] = "true"
	require.ErrorIs(t, mp.planner.moveInitNodeMark(planEntryFor(clusterPlan, "old"), planEntryFor(clusterPlan, "b-qualified")), generic.ErrSkip)
	require.Equal(t, []string{replacementSecret, oldSecret}, *updates, "stale observation must not write B or A again")
	require.Equal(t, "true", secrets[replacementSecret].Labels[capr.InitNodeLabel])
	require.NotContains(t, secrets[oldSecret].Labels, capr.InitNodeLabel)

	reloadPlanMetadataFromSecrets(clusterPlan, secrets)
	require.NoError(t, mp.planner.moveInitNodeMark(planEntryFor(clusterPlan, "old"), planEntryFor(clusterPlan, "b-qualified")))
	require.Nil(t, protectedInitTarget(controlPlane, clusterPlan))
	require.Equal(t, []string{replacementSecret, oldSecret}, *updates)
}

// reconcileProtectedHandoff runs the real fullReconcile path that finishes a persisted init-mark move.
func reconcileProtectedHandoff(mp *mockPlanner, controlPlane *rkev1.RKEControlPlane, clusterPlan *plan.Plan) error {
	status := rkev1.RKEControlPlaneStatus{}
	capr.Bootstrapped.True(&status)
	_, err := mp.planner.fullReconcile(controlPlane, status, &capi.Cluster{}, plan.Secret{}, clusterPlan, false)
	return err
}

// configureRecordedPlanSecretWrites records every Secret update attempt in order. The first failCount attempts
// for failSecret return an error without changing the persisted copy.
func configureRecordedPlanSecretWrites(mp *mockPlanner, clusterPlan *plan.Plan, failSecret string, failCount int) (map[string]*corev1.Secret, *[]string) {
	secrets := map[string]*corev1.Secret{}
	for name, metadata := range clusterPlan.Metadata {
		secretName := capr.PlanSecretFromBootstrapName(name)
		secrets[secretName] = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        secretName,
				Namespace:   "ns",
				Labels:      maps.Clone(metadata.Labels),
				Annotations: maps.Clone(metadata.Annotations),
			},
			Type: capr.SecretTypeMachinePlan,
			Data: map[string][]byte{"plan": []byte("{}")},
		}
	}
	updates := []string{}
	mp.secretClient.EXPECT().Get("ns", gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
		return secrets[name].DeepCopy(), nil
	}).AnyTimes()
	mp.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		updates = append(updates, secret.Name)
		if secret.Name == failSecret && failCount > 0 {
			failCount--
			return nil, errors.New("injected " + failSecret + " write failure")
		}
		secrets[secret.Name] = secret.DeepCopy()
		return secret.DeepCopy(), nil
	}).AnyTimes()
	return secrets, &updates
}

// reloadPlanMetadataFromSecrets replaces observed plan metadata with the persisted Secret labels and annotations.
func reloadPlanMetadataFromSecrets(clusterPlan *plan.Plan, secrets map[string]*corev1.Secret) {
	for name, metadata := range clusterPlan.Metadata {
		persisted := secrets[capr.PlanSecretFromBootstrapName(name)]
		metadata.Labels = maps.Clone(persisted.Labels)
		metadata.Annotations = maps.Clone(persisted.Annotations)
	}
}

func planEntryFor(clusterPlan *plan.Plan, name string) *planEntry {
	return &planEntry{Machine: clusterPlan.Machines[name], Metadata: clusterPlan.Metadata[name]}
}

// markWrittenPlansApplied records every persisted plan as applied, as the agent does once it has run it.
func markWrittenPlansApplied(secrets map[string]*corev1.Secret) {
	for _, secret := range secrets {
		if len(secret.Data["plan"]) > 0 {
			secret.Data["appliedPlan"] = append([]byte{}, secret.Data["plan"]...)
		}
	}
}

// joinPlanSecrets names the plan Secrets of the etcd nodes that the hand-off delivers before the wait.
func joinPlanSecrets() []string {
	return []string{capr.PlanSecretFromBootstrapName("a-other"), capr.PlanSecretFromBootstrapName("b-qualified")}
}

// decodeJoinConfig returns the k3s/RKE2 config that a persisted plan Secret delivers.
func decodeJoinConfig(t *testing.T, secret *corev1.Secret) map[string]interface{} {
	t.Helper()
	var nodePlan plan.NodePlan
	require.NoError(t, json.Unmarshal(secret.Data["plan"], &nodePlan))
	for _, file := range nodePlan.Files {
		if file.Path == "/etc/rancher/k3s/config.yaml.d/50-rancher.yaml" {
			content, err := base64.StdEncoding.DecodeString(file.Content)
			require.NoError(t, err)
			config := map[string]interface{}{}
			require.NoError(t, json.Unmarshal(content, &config))
			return config
		}
	}
	paths := []string{}
	for _, file := range nodePlan.Files {
		paths = append(paths, file.Path)
	}
	require.FailNowf(t, "join config file was not delivered", "plan files: %v", paths)
	return nil
}

// requireInterruptedMoveFinished checks that a saved replacement mark finishes the move: only A's mark is cleared
// and no operation plan is delivered.
func requireInterruptedMoveFinished(t *testing.T, err error, updates *[]string) {
	t.Helper()
	require.ErrorIs(t, err, generic.ErrSkip)
	require.Equal(t, []string{capr.PlanSecretFromBootstrapName("old")}, *updates, "only A's mark is cleared; no operation plan")
}

func TestFindInitNodeForOperationDefersDuringProtectedHandoff(t *testing.T) {
	mp, controlPlane, clusterPlan := protectedHandoffFixture(t)
	require.NotNil(t, protectedInitTarget(controlPlane, clusterPlan))

	found, joinServer, initNode, err := mp.planner.findInitNodeForOperation(controlPlane, clusterPlan)
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, joinServer)
	require.Nil(t, initNode)
}

func TestFindInitNodeForOperationDelegatesWithoutProtectedTarget(t *testing.T) {
	mp, controlPlane, clusterPlan := protectedHandoffFixture(t)
	clusterPlan.Machines["old"].DeletionTimestamp = nil
	require.Nil(t, protectedInitTarget(controlPlane, clusterPlan))

	wantFound, wantJoin, wantNode, wantErr := mp.planner.findInitNode(controlPlane, clusterPlan)
	gotFound, gotJoin, gotNode, gotErr := mp.planner.findInitNodeForOperation(controlPlane, clusterPlan)
	require.Equal(t, wantErr, gotErr)
	require.Equal(t, wantFound, gotFound)
	require.Equal(t, wantJoin, gotJoin)
	require.Equal(t, wantNode, gotNode)
	require.True(t, gotFound)
	require.NotNil(t, gotNode)
	require.Equal(t, "old", gotNode.Machine.Name)
}

func planEqualities() conversion.Equalities {
	equalities := equality.Semantic.Copy()
	_ = equalities.AddFunc(func(a, b plan.File) bool {
		return a.Content == b.Content && a.Path == b.Path && a.Permissions == b.Permissions && a.Dynamic == b.Dynamic && a.Minor == b.Minor
	})
	return equalities
}

// protectedHandoffWithoutPlanSecrets builds the hand-off cluster without registering plan Secret writes, so a test can choose how they fail.
func protectedHandoffWithoutPlanSecrets(t *testing.T, initMarked ...string) (*mockPlanner, *rkev1.RKEControlPlane, *plan.Plan, *capi.Cluster) {
	mp, controlPlane, clusterPlan := protectedHandoffFixture(t)
	for _, name := range initMarked {
		clusterPlan.Metadata[name].Labels[capr.InitNodeLabel] = "true"
	}
	mp.planner.locker = *locker.New()
	// The mock planner skips New's equalities; without them, plan.File would compare DrainHashFunc and never match.
	mp.planner.equalities = planEqualities()
	mp.planner.store.equalities = mp.planner.equalities
	mp.planner.retrievalFunctions.ReleaseData = func(context.Context, *rkev1.RKEControlPlane) *model.Release {
		return &model.Release{FeatureVersions: map[string]string{"encryption-key-rotation": "2.0.0"}}
	}
	mp.planner.retrievalFunctions.SystemAgentImage = func() string { return "system-agent" }
	mp.planner.retrievalFunctions.ImageResolver = image.ResolveWithControlPlane
	controlPlane.Spec.KubernetesVersion = "v1.28.4+k3s1"
	// The state Secret supplies the server and agent tokens that managed join config renders.
	mp.secretCache.EXPECT().Get("ns", "cluster-rke-state").Return(&corev1.Secret{Type: capr.SecretTypeClusterState, Data: map[string][]byte{"serverToken": []byte("server-token"), "agentToken": []byte("agent-token")}}, nil).AnyTimes()
	// Each pass renders the cluster-agent manifest for the "management" cluster, so it needs the same token and cluster stubs as the certificate-rotation tests.
	mp.clusterRegistrationTokenCache.EXPECT().GetByIndex(ClusterRegToken, "management").Return([]*apisv3.ClusterRegistrationToken{{ObjectMeta: metav1.ObjectMeta{Namespace: "management", Name: "default-token"}, Status: apisv3.ClusterRegistrationTokenStatus{TokenSecretName: "crt-token-default-token"}}}, nil).AnyTimes()
	mp.secretCache.EXPECT().Get("management", "crt-token-default-token").Return(&corev1.Secret{Data: map[string][]byte{"token": []byte("lol")}}, nil).AnyTimes()
	mp.managementClusters.EXPECT().Get("management").Return(&apisv3.Cluster{}, nil).AnyTimes()
	controlPlane.OwnerReferences = []metav1.OwnerReference{{APIVersion: capi.GroupVersion.String(), Kind: "Cluster", Name: "cluster"}}
	controlPlane.Status.Initialization.ControlPlaneInitialized = ptr.To(true)
	controlPlane.Status.AgentConnected = true
	capr.Bootstrapped.True(&controlPlane.Status)

	// Control-plane and worker members keep clusterIsSane true; they are not part of the hand-off.
	for name, role := range map[string]string{"control-plane": capr.ControlPlaneRoleLabel, "worker": capr.WorkerRoleLabel} {
		clusterPlan.Machines[name] = &capi.Machine{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", UID: types.UID(name + "-uid")},
			Spec: capi.MachineSpec{Bootstrap: capi.Bootstrap{ConfigRef: capi.ContractVersionedObjectReference{
				APIGroup: capr.RKEAPIGroup, Kind: capr.RKEBootstrapKind, Name: name,
			}}},
			Status: capi.MachineStatus{Conditions: []metav1.Condition{{Type: capi.InfrastructureReadyCondition, Status: metav1.ConditionTrue}}},
		}
		clusterPlan.Metadata[name] = &plan.Metadata{Labels: map[string]string{role: "true"}}
		if name == "control-plane" {
			clusterPlan.Metadata[name].Annotations = map[string]string{capr.JoinURLAnnotation: "https://control-plane:9345"}
		}
	}

	machines := []*capi.Machine{}
	for _, machine := range clusterPlan.Machines {
		machine.Labels = map[string]string{capi.ClusterNameLabel: "cluster"}
		machines = append(machines, machine)
	}
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "ns"},
		Status:     capi.ClusterStatus{Initialization: capi.ClusterInitializationStatus{InfrastructureProvisioned: ptr.To(true)}},
	}
	mp.capiClusters.EXPECT().Get("ns", "cluster").Return(cluster, nil).AnyTimes()
	mp.machinesCache.EXPECT().List("ns", gomock.Any()).Return(machines, nil).AnyTimes()
	// Machine Reconciled conditions are status-only writes; the cache reflects them on the next pass, so write them back to the plan's machines.
	mp.machines.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(m *capi.Machine) (*capi.Machine, error) {
		if existing := clusterPlan.Machines[m.Name]; existing != nil {
			existing.Status = *m.Status.DeepCopy()
		}
		return m, nil
	}).AnyTimes()
	// No replacement has a voter observation yet, so the hand-off keeps waiting.
	mp.managementNodes.EXPECT().List("management", labels.Everything()).Return(nil, nil).AnyTimes()
	return mp, controlPlane, clusterPlan, cluster
}

// processProtectedHandoff builds a Process-ready cluster around the protected hand-off plan. Plan Secrets are
// served from the planner's mocked client and every Secret update is recorded in order. initMarked names extra
// machines that already carry the init mark, which is how an interrupted move is simulated. Callers may edit the
// returned Secrets or the returned plan's Machines before calling Process.
func processProtectedHandoff(t *testing.T, initMarked ...string) (*mockPlanner, *rkev1.RKEControlPlane, map[string]*corev1.Secret, *[]string, *capi.Cluster, *plan.Plan) {
	mp, controlPlane, clusterPlan, cluster := protectedHandoffWithoutPlanSecrets(t, initMarked...)
	secrets, updates := configureRecordedPlanSecretWrites(mp, clusterPlan, "", 0)
	// Replacement candidates have no plan yet, so the etcd tier writes their join plans directly.
	for _, secret := range secrets {
		delete(secret.Data, "plan")
	}
	return mp, controlPlane, secrets, updates, cluster, clusterPlan
}

func TestProcessProtectedInitOperationDefersPendingSnapshot(t *testing.T) {
	mp, controlPlane, _, updates, _, _ := processProtectedHandoff(t)
	snapshot := &rkev1.ETCDSnapshotCreate{Generation: 1}
	controlPlane.Spec.ETCDSnapshotCreate = snapshot
	controlPlane.Status.ETCDSnapshotCreate = snapshot.DeepCopy()
	controlPlane.Status.ETCDSnapshotCreatePhase = rkev1.ETCDSnapshotPhaseStarted
	mp.rkeControlPlanes.EXPECT().EnqueueAfter("ns", "cluster", 5*time.Second)

	status, err := mp.planner.Process(controlPlane, controlPlane.Status)
	require.Error(t, err)
	require.True(t, IsErrWaiting(err))
	require.Contains(t, err.Error(), "configuring etcd node(s) a-other,b-qualified")
	require.Equal(t, rkev1.ETCDSnapshotPhaseStarted, status.ETCDSnapshotCreatePhase, "pending snapshot request must be kept")
	require.Equal(t, snapshot, controlPlane.Spec.ETCDSnapshotCreate)
	require.ElementsMatch(t, joinPlanSecrets(), *updates, "only the etcd join plans may be delivered during the hand-off")
}

func TestProcessProtectedInitOperationDefersPendingCertificateRotation(t *testing.T) {
	mp, controlPlane, _, updates, _, _ := processProtectedHandoff(t)
	rotate := &rkev1.RotateCertificates{Generation: 1}
	controlPlane.Spec.RotateCertificates = rotate
	mp.rkeControlPlanes.EXPECT().EnqueueAfter("ns", "cluster", 5*time.Second)

	_, err := mp.planner.Process(controlPlane, controlPlane.Status)
	require.Error(t, err)
	require.True(t, IsErrWaiting(err))
	require.Contains(t, err.Error(), "configuring etcd node(s) a-other,b-qualified")
	require.Equal(t, rotate, controlPlane.Spec.RotateCertificates, "pending certificate rotation request must be kept")
	require.ElementsMatch(t, joinPlanSecrets(), *updates, "only the etcd join plans may be delivered during the hand-off")
}

func TestProcessProtectedInitOperationEncryptionReadyNewRequestDefersAtLookup(t *testing.T) {
	mp, controlPlane, _, updates, _, _ := processProtectedHandoff(t)
	capr.Ready.True(&controlPlane.Status)
	request := &rkev1.RotateEncryptionKeys{Generation: 1}
	controlPlane.Spec.RotateEncryptionKeys = request
	mp.rkeControlPlanes.EXPECT().EnqueueAfter("ns", "cluster", 5*time.Second)

	status, err := mp.planner.Process(controlPlane, controlPlane.Status)
	require.Error(t, err)
	require.True(t, IsErrWaiting(err))
	require.Contains(t, err.Error(), "configuring etcd node(s) a-other,b-qualified")
	require.Equal(t, request, controlPlane.Spec.RotateEncryptionKeys, "new encryption-key request must be kept")
	require.Empty(t, status.RotateEncryptionKeysPhase, "new encryption-key request must not start during the hand-off")
	require.ElementsMatch(t, joinPlanSecrets(), *updates, "only the etcd join plans may be delivered during the hand-off")
}

func TestProcessProtectedInitOperationPausedCertificateRotationWaitsThenUnpauseReachesHandoff(t *testing.T) {
	mp, controlPlane, _, updates, cluster, _ := processProtectedHandoff(t)
	rotate := &rkev1.RotateCertificates{Generation: 1}
	controlPlane.Spec.RotateCertificates = rotate
	// A paused CAPI cluster must not be unpaused or re-paused by the deferred rotation: the mock fails on any Update.
	cluster.Spec.Paused = ptr.To(true)

	_, err := mp.planner.Process(controlPlane, controlPlane.Status)
	require.Error(t, err)
	require.Contains(t, err.Error(), "paused")
	require.Equal(t, rotate, controlPlane.Spec.RotateCertificates, "paused certificate rotation request must be kept")
	require.Empty(t, *updates, "no certificate plan may be delivered while paused")

	cluster.Spec.Paused = ptr.To(false)
	mp.rkeControlPlanes.EXPECT().EnqueueAfter("ns", "cluster", 5*time.Second)
	_, err = mp.planner.Process(controlPlane, controlPlane.Status)
	require.Error(t, err)
	require.True(t, IsErrWaiting(err))
	require.Contains(t, err.Error(), "configuring etcd node(s) a-other,b-qualified")
	require.Equal(t, rotate, controlPlane.Spec.RotateCertificates, "certificate rotation request must still be kept after unpause")
	require.ElementsMatch(t, joinPlanSecrets(), *updates, "only the etcd join plans may be delivered during the hand-off after unpause")
}

func TestProcessProtectedHandoffWaitingReplacementSavesJoinPlanAndKeepsMark(t *testing.T) {
	mp, controlPlane, secrets, _, _, _ := processProtectedHandoff(t)
	mp.rkeControlPlanes.EXPECT().EnqueueAfter("ns", "cluster", 5*time.Second)

	_, err := mp.planner.Process(controlPlane, controlPlane.Status)
	require.True(t, IsErrWaiting(err))
	require.Contains(t, err.Error(), "configuring etcd node(s) a-other,b-qualified")
	require.Contains(t, err.Error(), "; waiting for the replacement to join the existing etcd cluster through old")
	require.Contains(t, err.Error(), "a restore may be required")

	stored := secrets[capr.PlanSecretFromBootstrapName("b-qualified")]
	require.Equal(t, "https://old:9345", stored.Annotations[capr.JoinedToAnnotation])
	config := decodeJoinConfig(t, stored)
	require.Equal(t, "https://old:9345", config["server"])
	require.NotContains(t, config, "cluster-init")
	require.NotContains(t, stored.Labels, capr.InitNodeLabel)
	require.Equal(t, "true", secrets[capr.PlanSecretFromBootstrapName("old")].Labels[capr.InitNodeLabel])
}

func TestProcessProtectedHandoffReturnsPlanWriteErrorUnchanged(t *testing.T) {
	mp, controlPlane, clusterPlan, _ := protectedHandoffWithoutPlanSecrets(t)
	// The first candidate write fails with a plain error that the hand-off must return unchanged.
	failedSecret := capr.PlanSecretFromBootstrapName("a-other")
	secrets, updates := configureRecordedPlanSecretWrites(mp, clusterPlan, failedSecret, 1)
	// Candidates without plans take the direct-write path in reconcile, so the first candidate write reaches the hand-off wrapper.
	for _, candidate := range []string{"a-other", "b-qualified"} {
		delete(secrets[capr.PlanSecretFromBootstrapName(candidate)].Data, "plan")
	}
	mp.rkeControlPlanes.EXPECT().EnqueueAfter("ns", "cluster", 5*time.Second)

	_, err := mp.planner.Process(controlPlane, controlPlane.Status)
	require.EqualError(t, err, "injected "+failedSecret+" write failure", "store errors must reach the caller unchanged")
	require.False(t, IsErrWaiting(err))
	require.NotContains(t, err.Error(), "waiting for the replacement to join")
	require.Equal(t, []string{failedSecret}, *updates, "the failed write must be the only write attempted")
	require.Equal(t, "true", secrets[capr.PlanSecretFromBootstrapName("old")].Labels[capr.InitNodeLabel], "old must keep its init mark")
	require.NotContains(t, secrets[capr.PlanSecretFromBootstrapName("b-qualified")].Labels, capr.InitNodeLabel, "candidate must not gain the init mark")
}

func TestProcessProtectedHandoffWithoutDeletingInitNodeHasNoHandoffText(t *testing.T) {
	mp, controlPlane, secrets, _, _, clusterPlan := processProtectedHandoff(t)
	// Nothing is deleting, so there is no protected target and no hand-off text may appear. The init mark is
	// cleared on old so election can move on. Process stops at the bootstrap tier because this fixture cannot
	// report a healthy init node, so the etcd tier is not reached.
	clusterPlan.Machines["old"].DeletionTimestamp = nil
	delete(clusterPlan.Metadata["old"].Labels, capr.InitNodeLabel)
	delete(secrets[capr.PlanSecretFromBootstrapName("old")].Labels, capr.InitNodeLabel)
	secrets[capr.PlanSecretFromBootstrapName("old")].Data["plan"] = []byte("{}")
	mp.rkeControlPlanes.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	_, err := mp.planner.Process(controlPlane, controlPlane.Status)
	for pass := 0; pass < 6 && err != nil && !IsErrWaiting(err); pass++ {
		markWrittenPlansApplied(secrets)
		_, err = mp.planner.Process(controlPlane, controlPlane.Status)
	}
	require.Truef(t, IsErrWaiting(err), "unexpected error: %v", err)
	require.Contains(t, err.Error(), "configuring bootstrap node(s)")
	require.NotContains(t, err.Error(), "waiting for the replacement to join")
}

func TestProcessProtectedInitOperationSnapshotInterruptedMoveFinishes(t *testing.T) {
	mp, controlPlane, _, updates, _, _ := processProtectedHandoff(t, "b-qualified")
	snapshot := &rkev1.ETCDSnapshotCreate{Generation: 1}
	controlPlane.Spec.ETCDSnapshotCreate = snapshot
	controlPlane.Status.ETCDSnapshotCreate = snapshot.DeepCopy()
	controlPlane.Status.ETCDSnapshotCreatePhase = rkev1.ETCDSnapshotPhaseStarted
	mp.rkeControlPlanes.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	status, err := mp.planner.Process(controlPlane, controlPlane.Status)
	requireInterruptedMoveFinished(t, err, updates)
	require.Equal(t, rkev1.ETCDSnapshotPhaseStarted, status.ETCDSnapshotCreatePhase, "pending snapshot request must be kept")
	require.Equal(t, snapshot, controlPlane.Spec.ETCDSnapshotCreate)
}

func TestProcessProtectedInitOperationCertificateRotationInterruptedMoveFinishes(t *testing.T) {
	mp, controlPlane, _, updates, _, _ := processProtectedHandoff(t, "b-qualified")
	rotate := &rkev1.RotateCertificates{Generation: 1}
	controlPlane.Spec.RotateCertificates = rotate
	mp.rkeControlPlanes.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	_, err := mp.planner.Process(controlPlane, controlPlane.Status)
	requireInterruptedMoveFinished(t, err, updates)
	require.Equal(t, rotate, controlPlane.Spec.RotateCertificates, "pending certificate rotation request must be kept")
}

func TestProcessProtectedInitOperationEncryptionReadyNewRequestInterruptedMoveFinishes(t *testing.T) {
	mp, controlPlane, _, updates, _, _ := processProtectedHandoff(t, "b-qualified")
	capr.Ready.True(&controlPlane.Status)
	request := &rkev1.RotateEncryptionKeys{Generation: 1}
	controlPlane.Spec.RotateEncryptionKeys = request
	mp.rkeControlPlanes.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	status, err := mp.planner.Process(controlPlane, controlPlane.Status)
	requireInterruptedMoveFinished(t, err, updates)
	require.Empty(t, status.RotateEncryptionKeysPhase, "new encryption-key request must not start during the hand-off")
	require.Equal(t, request, controlPlane.Spec.RotateEncryptionKeys, "new encryption-key request must be kept")
}

func TestProcessProtectedInitOperationPausedCertificateRotationInterruptedMoveFinishesAfterUnpause(t *testing.T) {
	mp, controlPlane, _, updates, cluster, _ := processProtectedHandoff(t, "b-qualified")
	rotate := &rkev1.RotateCertificates{Generation: 1}
	controlPlane.Spec.RotateCertificates = rotate
	cluster.Spec.Paused = ptr.To(true)

	_, err := mp.planner.Process(controlPlane, controlPlane.Status)
	require.ErrorContains(t, err, "paused")
	require.Empty(t, *updates, "no label move while paused")
	require.Equal(t, rotate, controlPlane.Spec.RotateCertificates, "paused certificate rotation request must be kept")

	cluster.Spec.Paused = ptr.To(false)
	mp.rkeControlPlanes.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	_, err = mp.planner.Process(controlPlane, controlPlane.Status)
	requireInterruptedMoveFinished(t, err, updates)
	require.Equal(t, rotate, controlPlane.Spec.RotateCertificates, "certificate rotation request must still be kept after unpause")
}

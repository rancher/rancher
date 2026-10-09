package etcdsnapshotrestore

import (
	"encoding/json"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

// Every node of two topologies is walked through the steps of a restore that assign it a plan, using
// the plans the controller builds, to pin two properties of the plans one restore hands a node:
//
//   - No two consecutive steps hand it the same plan. The Store only runs a plan afresh when its
//     content or its writer changes, and every step of one operation is written by the same writer,
//     so a step whose plan matched the one before it would be reported on that plan's outcome
//     instead of being run.
//   - No idempotent instruction is repeated by a later step under the same identifier, value and
//     command. The idempotent script runs a command only if it has no record of having run it at the
//     agent's current attempt number, which is 1 for every pending plan, so the later step would
//     find the earlier one's record and skip its command. Shutdown clears the records at the start
//     of each restore, so only repeats within one restore matter.
func TestConsecutiveStepsAssignDistinctPlans(t *testing.T) {
	t.Parallel()

	topologies := map[string][]*corev1.Secret{
		"single node": {
			makePlanSecret("init", "node-init", map[string]string{
				capr.EtcdRoleLabel: "true", capr.ControlPlaneRoleLabel: "true", capr.WorkerRoleLabel: "true", capr.InitNodeLabel: "true",
			}),
		},
		"split roles": {
			makePlanSecret("init", "node-init", map[string]string{capr.EtcdRoleLabel: "true", capr.InitNodeLabel: "true"}),
			makePlanSecret("etcd-2", "node-etcd-2", map[string]string{capr.EtcdRoleLabel: "true"}),
			makePlanSecret("cp-1", "node-cp-1", map[string]string{capr.ControlPlaneRoleLabel: "true"}),
			makePlanSecret("worker-1", "node-worker-1", map[string]string{capr.WorkerRoleLabel: "true"}),
		},
	}

	for name, nodes := range topologies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, node := range nodes {
				node.Annotations = map[string]string{}
			}
			initSecret := nodes[0]
			adapter := defaultAdapter()
			adapter.leader = initSecret
			s := newTestScope(adapter, "restore-uid")
			s.ownerKey = "restore-owner"

			// Each step's plans by node name; a node a step leaves alone is absent.
			type step struct {
				name  opv1alpha1.ETCDSnapshotRestoreStep
				plans map[string][]byte
			}
			build := func(build func(*corev1.Secret) (*planapi.Plan, error), filter func(*corev1.Secret) bool) map[string][]byte {
				plans := map[string][]byte{}
				for _, node := range nodes {
					if !filter(node) {
						continue
					}
					p, err := build(node)
					require.NoError(t, err)
					data, err := json.Marshal(p)
					require.NoError(t, err)
					plans[node.Name] = data
				}
				return plans
			}
			all := func(*corev1.Secret) bool { return true }
			only := func(want *corev1.Secret) func(*corev1.Secret) bool {
				return func(node *corev1.Secret) bool { return node.Name == want.Name }
			}
			isEtcd := func(node *corev1.Secret) bool { return node.Labels[capr.EtcdRoleLabel] == "true" }
			serverURL := adapter.GetServerURL(initSecret)

			steps := []step{
				{opv1alpha1.ETCDSnapshotRestoreStepPreflight, build(func(n *corev1.Secret) (*planapi.Plan, error) { return buildPreflightPlan(s, n) }, isEtcd)},
				{opv1alpha1.ETCDSnapshotRestoreStepShutdown, build(func(n *corev1.Secret) (*planapi.Plan, error) { return buildShutdownPlan(s, n) }, all)},
				{opv1alpha1.ETCDSnapshotRestoreStepRestore, build(func(n *corev1.Secret) (*planapi.Plan, error) { return buildRestorePlan(s, n, nil, "snapshot") }, only(initSecret))},
				{opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup, podCleanupPlans(t, s, nodes)},
				{opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster, build(func(n *corev1.Secret) (*planapi.Plan, error) {
					return buildRestartPlan(s, n, initSecret, serverURL, s.restartIdempotencyValue(true), true)
				}, all)},
				{opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup, build(func(n *corev1.Secret) (*planapi.Plan, error) {
					p, _, err := buildPostRestoreNodeCleanupPlan(s, n, nodes)
					return p, err
				}, only(initSecret))},
				{opv1alpha1.ETCDSnapshotRestoreStepRestartCluster, build(func(n *corev1.Secret) (*planapi.Plan, error) {
					return buildRestartPlan(s, n, initSecret, serverURL, s.restartIdempotencyValue(false), false)
				}, all)},
			}

			for _, node := range nodes {
				var previous []byte
				var previousStep opv1alpha1.ETCDSnapshotRestoreStep
				recorded := map[[3]string]opv1alpha1.ETCDSnapshotRestoreStep{}
				for _, step := range steps {
					current, ok := step.plans[node.Name]
					if !ok || string(current) == "null" {
						continue
					}
					assert.NotEqual(t, string(previous), string(current),
						"node %s is assigned the same plan in step %s as in step %s before it", node.Name, step.name, previousStep)
					previous, previousStep = current, step.name

					for _, record := range idempotencyRecords(t, current, adapter.provisioningDir) {
						if earlier, ok := recorded[record]; ok {
							assert.Failf(t, "idempotent command repeated", "node %s: step %s repeats step %s's idempotent command %v, so it would be skipped",
								node.Name, step.name, earlier, record)
						}
						recorded[record] = step.name
					}
				}
			}
		})
	}
}

// podCleanupPlans runs the pod cleanup step over nodes, completing each plan it assigns so it moves
// on to the next, and returns the plans it assigned by node name. The step builds its plans inline,
// so running it is the only way to see them.
func podCleanupPlans(t *testing.T, s *scope, nodes []*corev1.Secret) map[string][]byte {
	t.Helper()

	items := make([]*corev1.Secret, len(nodes))
	for i, node := range nodes {
		items[i] = node.DeepCopy()
	}
	adapter := s.adapter.(*stubAdapter)
	leader := adapter.leader

	plans := map[string][]byte{}
	for range nodes {
		secrets := &fakePlanSecrets{items: items}
		h := &handler{secrets: secrets, store: planapi.NewStore(secrets)}
		got, err := h.reconcilePostRestorePodCleanup(s, opv1alpha1.ETCDSnapshotRestoreStatus{Step: opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup})
		require.NoError(t, err)
		require.NotEqual(t, opv1alpha1.OperationPhaseFailed, got.Phase, "pod cleanup failed: %s", opv1alpha1.FailedCondition.GetMessage(&got))
		if len(secrets.updates) == 0 {
			break
		}

		written := secrets.updates[0]
		plans[written.Name] = written.Data[planapi.PlanDataKey]
		written.Data[planapi.PlanStateKey] = []byte(planapi.PlanStateSucceeded)
		written.Data["probe-statuses"] = []byte(`{"x":{"healthy":true}}`)
		written.Annotations[planapi.PlanProbesPassedAnnotation] = "applied"
		for i, item := range items {
			if item.Name == written.Name {
				items[i] = written
			}
		}
		if written.Name == leader.Name {
			adapter.leader = written
		}
	}
	adapter.leader = leader

	require.NotEmpty(t, plans, "pod cleanup must assign a plan")
	return plans
}

// idempotencyRecords returns the identifier, value and command under which the idempotent script
// records each idempotent instruction of the serialized plan as run.
func idempotencyRecords(t *testing.T, data []byte, provisioningDir string) [][3]string {
	t.Helper()

	var p planapi.Plan
	require.NoError(t, json.Unmarshal(data, &p))

	var records [][3]string
	for _, instruction := range p.OneTimeInstructions {
		args := instruction.Args
		if len(args) < 5 || args[1] != ops.IdempotentActionScriptPath(provisioningDir) {
			continue
		}
		records = append(records, [3]string{args[2], args[3], args[4]})
	}
	return records
}

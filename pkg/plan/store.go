package plan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
)

// a plan can be:
// waiting to be applied (pending)
// in progress
// passed but waiting for probes
// passed and probes passed
// failing (but not yet failed)
// failed and hit max failures
//

// PlanStatus represents the current status of a plan.
// This is used to determine if the agent should continue to apply the plan, or if it has reached a terminal state.
//
// The following states indicate we should wait with a message:
// - Pending,
// - InProgress
// - Applied && !(ProbesPassed)
// - Failing
//
// The following states are terminal:
// - Applied && ProbesPassed
// - Failed
type PlanStatus struct {
	// Secret is the machine-plan secret containing the plan.
	Secret *corev1.Secret

	// Pending is true if the plan is waiting to be applied.
	Pending bool

	// InProgress is true if the plan is currently being applied.
	InProgress bool

	// Applied is true if the plan has been successfully applied.
	Applied bool

	// ProbesPassed is true if the plan has passed all probes.
	ProbesPassed bool

	// Failing is true if the plan is currently failing but has not yet reached the failure threshold.
	Failing bool

	// Failed is true if the plan has failed to be applied.
	Failed bool

	// Paused is true if the agent is holding the plan because of PlanPausedAnnotation. Only reported by
	// agents which support plan-state.
	Paused bool

	// Canceled is true if the agent has recorded the plan as canceled because of PlanCanceledAnnotation.
	// Cancellation is terminal: the agent will not act again until new plan content is assigned. Only
	// reported by agents which support plan-state.
	Canceled bool
}

// Success returns true if the plan has been successfully applied and all probes have passed.
func (p *PlanStatus) Success() bool {
	return p.Applied && p.ProbesPassed
}

// Failure returns true if the plan has failed to be applied, or was canceled before it could be.
// Either way, the plan will never complete without new content being assigned.
func (p *PlanStatus) Failure() bool {
	return p.Failed || p.Canceled
}

// Waiting returns true if the plan is in a transient state.
func (p *PlanStatus) Waiting() bool {
	switch {
	case p.Canceled:
		return false
	case p.Pending:
		return true
	case p.InProgress:
		return true
	case p.Paused:
		return true
	case p.Applied && !p.ProbesPassed:
		return true
	case p.Applied && p.ProbesPassed:
		return false
	case p.Failing && !p.Failed:
		return true
	case p.Failed:
		return false
	}
	return false
}

func (p *PlanStatus) String() string {
	switch {
	case p.Canceled:
		return "plan canceled"
	case p.Pending:
		return "waiting for plan to be picked up"
	case p.InProgress:
		return "waiting for plan to be applied"
	case p.Paused:
		return "plan paused"
	case p.Applied && !p.ProbesPassed:
		return "waiting for probes"
	case p.Applied && p.ProbesPassed:
		return "plan successfully applied"
	case p.Failing && !p.Failed:
		return "waiting for plan to succeed or reach failure limit"
	case p.Failed:
		return "plan failed to be applied"
	}
	return ""
}

// Message aggregates a slice of PlanStatus structures into a single, human-readable status string.
//
// Nodes are bucketed by their active operational phase according to a strict priority hierarchy:
//  1. failing plan (Failing == true, Failed == false)
//  2. waiting for plan to be picked up (Pending == true)
//  3. waiting for plan applied (InProgress == true)
//  4. plan paused (Paused == true)
//  5. waiting for probes (Applied == true, ProbesPassed == false)
//
// Nodes that do not fit into these buckets (e.g., fully successfully applied, strictly failed or canceled) are ignored.
// Within each bucket, node names are sorted lexicographically to guarantee deterministic outputs.
//
// Output string patterns adapt dynamically based on the node count per bucket:
//   - 1 node: "bucket_text for X"
//   - 2 nodes: "bucket_text for X & 1 other node"
//   - 3+ nodes: "bucket_text for X & N other nodes"
//
// If multiple statuses are present across the cluster, their resulting summary strings are joined
// with a comma and space, ordered by the phase priority listed above. Returns an empty string if
// results is empty or if no active statuses match the tracked progress buckets.
//
// Message length is unlimited but has a bounded size of 1124, 112 for all plan states, plus 253*4 for node names, and
// 15 bytes for "& N other nodes", plus however many digits N contains, though in practice it will be difficult to reach
// over 5 digits.
//
// As the result of this function may vary wildly between reconciliations, its intended purpose is to be used solely by
// handlers that have a fixed enqueue period to prevent thrashing. For example, a simple plan application for 100 nodes
// can cause at worst 100 status updates if each machine-plan state transition retriggers the handler if the message is
// used in a condition.
func Message(results []PlanStatus) string {
	if len(results) == 0 {
		return ""
	}

	// Group node names by their current message bucket
	buckets := make(map[string][]string)

	for _, res := range results {
		if res.Secret == nil {
			continue
		}
		name := res.Secret.Name
		if res.Secret.Labels != nil {
			machineName := res.Secret.Labels[planv1alpha1.MachineLifecycleNameLabel]
			if machineName != "" {
				name = machineName
			}
		}

		// Order of evaluation sets the bucket for each node
		if res.Canceled {
			continue
		} else if res.Failing && !res.Failed {
			buckets["failing plan"] = append(buckets["failing plan"], name)
		} else if res.Pending {
			buckets["waiting for plan to be picked up"] = append(buckets["waiting for plan to be picked up"], name)
		} else if res.InProgress {
			buckets["waiting for plan applied"] = append(buckets["waiting for plan applied"], name)
		} else if res.Paused {
			buckets["plan paused"] = append(buckets["plan paused"], name)
		} else if res.Applied && !res.ProbesPassed {
			buckets["waiting for probes"] = append(buckets["waiting for probes"], name)
		}
	}

	if len(buckets) == 0 {
		return ""
	}

	// Helper to determine priority ranking (lower number = higher priority)
	getPriority := func(bucket string) int {
		switch bucket {
		case "failing plan":
			return 1
		case "waiting for plan to be picked up":
			return 2
		case "waiting for plan applied":
			return 3
		case "plan paused":
			return 4
		case "waiting for probes":
			return 5
		default:
			return 6
		}
	}

	type msgGroup struct {
		text     string
		priority int
	}
	var groups []msgGroup

	// Format each bucket independently
	for bucket, nodes := range buckets {
		if len(nodes) == 0 {
			continue
		}

		// Sort node names lexicographically within their own bucket
		sort.Strings(nodes)

		var formatted string
		count := len(nodes)

		if count == 1 {
			formatted = fmt.Sprintf("%s for %s", bucket, nodes[0])
		} else if count == 2 {
			formatted = fmt.Sprintf("%s for %s & 1 other node", bucket, nodes[0])
		} else {
			formatted = fmt.Sprintf("%s for %s & %d other nodes", bucket, nodes[0], count-1)
		}

		groups = append(groups, msgGroup{
			text:     formatted,
			priority: getPriority(bucket),
		})
	}

	// Sort the message groups: primarily by priority, secondarily lexicographically
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].priority != groups[j].priority {
			return groups[i].priority < groups[j].priority
		}
		return groups[i].text < groups[j].text
	})

	// Flatten sorted groups into the final comma-separated string
	var finalMsgs []string
	for _, g := range groups {
		finalMsgs = append(finalMsgs, g.text)
	}

	return strings.Join(finalMsgs, ", ")
}

type Store struct {
	secrets corecontrollers.SecretClient
}

func NewStore(secrets corecontrollers.SecretClient) *Store {
	return &Store{
		secrets: secrets,
	}
}

// ParseProbeStatuses parses the probe statuses from the secret.
// Returns a map of the probe name to ProbeStatus and a boolean indicating if all probes are healthy.
// If the probeStatuses is empty returns an error.
// probeStatuses is a JSON-encoded map of the probe name to ProbeStatus.
func ParseProbeStatuses(probeStatuses []byte) (*map[string]ProbeStatus, bool, error) {
	healthy := true
	if len(probeStatuses) == 0 {
		return nil, false, fmt.Errorf("probe status length was 0")
	}
	probeStatusMap := map[string]ProbeStatus{}
	if err := json.Unmarshal(probeStatuses, &probeStatusMap); err != nil {
		return nil, false, err
	}
	for _, status := range probeStatusMap {
		if !status.Healthy {
			healthy = false
		}
	}
	return &probeStatusMap, healthy, nil
}

// PlanHash returns the SHA256 hash of the plan.
// Any byte slice can be hashed, but the hash is only useful for comparison.
// Valid usages are:
// - to compare the hash of a plan to the hash of the applied plan
// - to compare the hash of a plan to the hash of the failed plan
// - to generate idempotent instructions
func PlanHash(plan []byte) string {
	result := sha256.Sum256(plan)
	return hex.EncodeToString(result[:])
}

// CancelPlan asks the agent to abort the plan currently assigned to secret, by setting
// PlanCanceledAnnotation. It reports whether the annotation had to be written, and returns the
// secret to go on using — the updated one when it wrote, the one passed in otherwise — so a caller
// can assign it back unconditionally.
//
// Cancellation is terminal for the plan: clearing the annotation does not resume it, and the agent
// will not act again until new plan content is assigned. It is also idempotent here, so a caller
// which cannot tell whether a previous attempt landed can simply call it again.
func (s *Store) CancelPlan(secret *corev1.Secret) (bool, *corev1.Secret, error) {
	if secret == nil {
		return false, nil, nil
	}

	if secret.Annotations[PlanCanceledAnnotation] == "true" {
		return false, secret, nil
	}

	updated := secret.DeepCopy()
	if updated.Annotations == nil {
		updated.Annotations = map[string]string{}
	}
	updated.Annotations[PlanCanceledAnnotation] = "true"

	updated, err := s.secrets.Update(updated)
	if err != nil {
		return false, secret, err
	}

	return true, updated, nil
}

// AssignPlan assigns the plan to the secret.
// Returns a PlanStatus indicating the current state of the plan.
// This function is based off the CAPR assignAndCheckPlan function and will supersede it in the future once its CAPI dependency is unraveled.
//
// New plan content is written with plan-state set to pending, after which an agent which supports
// plan-state drives the state itself, and that state is what the returned PlanStatus is built from.
// An agent which predates plan-state never moves it off pending, so for pending (or a secret
// assigned before plan-state existed) the status falls back to comparing the applied plan and the
// failed checksum, as it always has.
//
// maxFailures is the number of attempts to make at the plan, and failureThreshold the number of
// failures after which it is reported as Failed; -1 means unlimited for either. In the plan-state
// flow the agent stops at the first failure and leaves the retry to the orchestrator, so the retries
// are made here: see retryFailedPlan.
func (s *Store) AssignPlan(secret *corev1.Secret, plan *Plan, maxFailures, failureThreshold int) (*PlanStatus, error) {
	data, err := json.Marshal(&plan)
	if err != nil {
		return nil, err
	}

	secret = secret.DeepCopy()
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}

	if !bytes.Equal(secret.Data[PlanDataKey], data) {
		markPending(secret)
		delete(secret.Annotations, PlanCanceledAnnotation)

		secret.Data[PlanDataKey] = data
		if maxFailures > 0 || maxFailures == -1 {
			secret.Data[maxFailuresKey] = []byte(strconv.Itoa(maxFailures))
		} else {
			delete(secret.Data, maxFailuresKey)
		}

		if failureThreshold > 0 || failureThreshold == -1 {
			secret.Data[failureThresholdKey] = []byte(strconv.Itoa(failureThreshold))
		} else {
			delete(secret.Data, failureThresholdKey)
		}

		secret, err = s.secrets.Update(secret)
		if err != nil {
			return nil, err
		}

		// Nothing the secret holds about earlier plans says anything about this one yet.
		return &PlanStatus{Secret: secret, Pending: true}, nil
	}

	switch PlanState(secret.Data[PlanStateKey]) {
	case PlanStateInProgress:
		failing, _, err := failureStatus(secret)
		if err != nil {
			return nil, err
		}
		// A retry of an earlier failed attempt is still reported as failing, so the plan's message
		// shows that it has failed before rather than reading as a first attempt.
		return &PlanStatus{Secret: secret, InProgress: true, Failing: failing}, nil
	case PlanStateSucceeded:
		probesPassed, err := probesPassed(secret)
		if err != nil {
			return nil, err
		}
		return &PlanStatus{Secret: secret, Applied: true, ProbesPassed: probesPassed}, nil
	case PlanStateFailed:
		return s.retryFailedPlan(secret)
	case PlanStateCanceled:
		return &PlanStatus{Secret: secret, Canceled: true}, nil
	case PlanStatePaused:
		return &PlanStatus{Secret: secret, Paused: true}, nil
	default:
		return checksumPlanStatus(secret)
	}
}

const (
	// maxFailuresKey and failureThresholdKey are the secret data keys AssignPlan writes the plan's
	// failure limits to.
	maxFailuresKey      = "max-failures"
	failureThresholdKey = "failure-threshold"

	// The secret data keys the agent records the plan's outcome under.
	probeStatusesKey  = "probe-statuses"
	failedChecksumKey = "failed-checksum"
	failureCountKey   = "failure-count"
	lastApplyTimeKey  = "last-apply-time"
	appliedPlanKey    = "appliedPlan"

	// failedPlanRetryCooldown is how long a failed plan is left before it is retried. It matches the
	// cooldown agents which predate plan-state apply between their own retries, so a plan which fails
	// immediately does not burn through its attempts in the time it takes to reconcile a few times.
	failedPlanRetryCooldown = 30 * time.Second
)

// now is replaced in tests.
var now = time.Now

// markPending resets secret for the agent to run its plan from the start, discarding everything
// recorded about the previous run's probes so that this run is judged on its own.
func markPending(secret *corev1.Secret) {
	delete(secret.Data, probeStatusesKey)
	secret.Annotations[PlanLastUpdatedAnnotation] = now().UTC().Format(time.RFC3339)
	secret.Annotations[PlanProbesPassedAnnotation] = ""
	secret.Data[PlanStateKey] = []byte(PlanStatePending)
}

// retryFailedPlan handles a plan the agent has recorded as failed. The agent never retries a plan
// itself in the plan-state flow, so while the plan has attempts left this sets plan-state back to
// pending for the agent to run it again, once failedPlanRetryCooldown has passed since the last
// attempt.
//
// Note that the agent starts a pending plan at attempt 1, so CATTLE_AGENT_ATTEMPT_NUMBER does not
// advance across these retries. A plan built from IdempotentActionScript instructions should be
// assigned with a single attempt, as a retry would find the instruction already recorded as run.
func (s *Store) retryFailedPlan(secret *corev1.Secret) (*PlanStatus, error) {
	result := &PlanStatus{Secret: secret}

	failing, failed, err := failureStatus(secret)
	if err != nil {
		return nil, err
	}
	if !failing && !failed {
		// plan-state is authoritative about the plan having failed, even if the failure count does
		// not show it. Count it as the one failure it must have been.
		failed, err = failureThresholdReached(secret, 1)
		if err != nil {
			return nil, err
		}
		failing = !failed
	}
	if failed {
		result.Failed = true
		return result, nil
	}
	result.Failing = true

	if secret.Annotations[PlanCanceledAnnotation] == "true" {
		// The plan was asked to stop. Retrying it would only have the agent record the cancellation.
		return &PlanStatus{Secret: secret, Canceled: true}, nil
	}

	attempts, err := failureCount(secret)
	if err != nil {
		return nil, err
	}
	maxFailures := -1
	if raw := secret.Data[maxFailuresKey]; len(raw) > 0 {
		maxFailures, err = strconv.Atoi(string(raw))
		if err != nil {
			return nil, err
		}
	}
	if maxFailures != -1 && attempts >= maxFailures {
		// Out of attempts but short of the failure threshold, which the threshold being unlimited
		// allows. Agents which predate plan-state stop retrying here too, and keep reporting the plan
		// as failing.
		return result, nil
	}

	if lastApply, err := time.Parse(time.UnixDate, string(secret.Data[lastApplyTimeKey])); err == nil && now().Before(lastApply.Add(failedPlanRetryCooldown)) {
		return result, nil
	}

	secret = secret.DeepCopy()
	markPending(secret)
	secret, err = s.secrets.Update(secret)
	if err != nil {
		return nil, err
	}
	result.Secret = secret
	result.Pending = true

	return result, nil
}

// checksumPlanStatus builds the status of a plan from the applied plan and the failed checksum.
// This is how the status of plans run by agents which predate plan-state is told, and of plans
// which are still pending, which agents that support plan-state have yet to pick up.
func checksumPlanStatus(secret *corev1.Secret) (*PlanStatus, error) {
	result := &PlanStatus{Secret: secret}

	var err error
	result.ProbesPassed, err = probesPassed(secret)
	if err != nil {
		return nil, err
	}

	result.Failing, result.Failed, err = failureStatus(secret)
	if err != nil {
		return nil, err
	}

	result.Applied = bytes.Equal(secret.Data[PlanDataKey], secret.Data[appliedPlanKey])

	if !result.Applied && !result.Failed {
		if PlanState(secret.Data[PlanStateKey]) == PlanStatePending && !result.Failing {
			// Nothing has been recorded against the plan yet, so as far as can be told the agent has
			// not picked it up.
			result.Pending = true
		} else {
			result.InProgress = true
		}
	}

	return result, nil
}

// probesPassed reports whether the probes have passed for the plan currently assigned to secret.
func probesPassed(secret *corev1.Secret) (bool, error) {
	if secret.Annotations[PlanProbesPassedAnnotation] == "" {
		return false, nil
	}
	probes := secret.Data[probeStatusesKey]
	if len(probes) == 0 {
		return false, nil
	}
	_, healthy, err := ParseProbeStatuses(probes)
	if err != nil {
		return false, err
	}
	return healthy, nil
}

// failureCount returns the number of times the plan currently assigned to secret has failed. The
// failure count carries over from earlier plans, so it is only taken to describe this one when the
// failed checksum is this plan's.
func failureCount(secret *corev1.Secret) (int, error) {
	rawFailureCount := secret.Data[failureCountKey]
	if len(rawFailureCount) == 0 || PlanHash(secret.Data[PlanDataKey]) != string(secret.Data[failedChecksumKey]) {
		return 0, nil
	}
	return strconv.Atoi(string(rawFailureCount))
}

// failureStatus reports whether the plan currently assigned to secret has failed but not yet reached
// its failure threshold (failing), or has reached it (failed).
func failureStatus(secret *corev1.Secret) (failing, failed bool, err error) {
	count, err := failureCount(secret)
	if err != nil || count <= 0 {
		return false, false, err
	}
	failed, err = failureThresholdReached(secret, count)
	if err != nil {
		return false, false, err
	}
	return !failed, failed, nil
}

// failureThresholdReached reports whether count failures reach the failure threshold of the plan
// currently assigned to secret. The threshold is set by AssignPlan; if it is not set, then it
// essentially defaults to 1, and any failure causes the plan to be marked as failed.
func failureThresholdReached(secret *corev1.Secret, count int) (bool, error) {
	rawFailureThreshold := secret.Data[failureThresholdKey]
	if len(rawFailureThreshold) == 0 {
		return true, nil
	}
	failureThreshold, err := strconv.Atoi(string(rawFailureThreshold))
	if err != nil {
		return false, err
	}
	// The plan hasn't actually failed to be applied if we haven't passed the failure threshold or
	// failure threshold is set to -1.
	return failureThreshold != -1 && count >= failureThreshold, nil
}

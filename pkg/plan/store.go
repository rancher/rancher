package plan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
)

// PlanStatus is the status of the plan assigned to a machine-plan secret, as the agent records it
// under plan-state.
//
// A plan the caller is waiting on is in one of the active states (see PlanState.IsActive), has
// succeeded but not yet passed its probes, or has failed with a retry still to come. Every other
// plan is done with: it succeeded and passed its probes (Success), or it will never complete without
// new content being assigned (Failure).
type PlanStatus struct {
	// Secret is the machine-plan secret containing the plan.
	Secret *corev1.Secret

	// State is the plan-state the agent recorded for the plan.
	State PlanState

	// ProbesPassed is true if the plan's probes have passed since it was assigned. It is only
	// meaningful once the plan has succeeded.
	ProbesPassed bool

	// Retrying is true if the plan has failed before, and is being or is about to be attempted again.
	Retrying bool
}

// Success returns true if the plan has been successfully applied and all probes have passed.
func (p *PlanStatus) Success() bool {
	return p.State == PlanStateSucceeded && p.ProbesPassed
}

// Failure returns true if the plan will never complete without new content being assigned: it
// failed with no retry to come, was canceled, or is in a state this build does not know, which the
// agent treats as terminal too.
func (p *PlanStatus) Failure() bool {
	switch {
	case p.State.IsActive(), p.State == PlanStateSucceeded:
		return false
	case p.State == PlanStateFailed:
		return !p.Retrying
	default:
		return true
	}
}

// Waiting returns true if the plan is in a transient state.
func (p *PlanStatus) Waiting() bool {
	return !p.Success() && !p.Failure()
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

// PlanWriterAnnotation names whoever last wrote the plan assigned to a machine-plan secret, or asked
// for it to be canceled: the key the writer holds the cluster's beacon under. Every write the Store
// makes to the plan, or to the requests the agent acts on, records it.
//
// It serves two purposes. It is the fencing token the machine-plan webhook checks such a write
// against the beacon's current holder, which is what stops a writer that has lost the beacon from
// reaching into the work of the one that holds it now (see BeaconRefForSecret). And it is what
// tells one assignment of a plan from the next: AssignPlan runs a plan afresh when its writer
// changes, even if its content does not.
const PlanWriterAnnotation = "plan.cattle.io/writer"

// PlanAttemptAnnotation records which attempt at the assigned plan the agent is on: 1 when the
// plan is assigned, incremented each time a failed plan is reset to pending to be retried. It is
// owned by the Store, which is what retries the plan in the plan-state flow, and so what counts the
// failures toward max-failures and failure-threshold.
//
// The agent's own failure-count cannot be used for this. It is incremented on every failure and
// reset only on success, whatever plan the failures before it belonged to, so it carries the
// failures of earlier assignments into this one.
const PlanAttemptAnnotation = "plan.cattle.io/attempt"

const (
	// maxFailuresKey and failureThresholdKey are the secret data keys AssignPlan writes the plan's
	// failure limits to.
	maxFailuresKey      = "max-failures"
	failureThresholdKey = "failure-threshold"

	// The secret data keys the agent records the plan's outcome under.
	probeStatusesKey = "probe-statuses"
	lastApplyTimeKey = "last-apply-time"

	// failedPlanRetryCooldown is how long a failed plan is left before it is retried, so a plan which
	// fails immediately does not burn through its attempts in the time it takes to reconcile a few
	// times.
	failedPlanRetryCooldown = 30 * time.Second
)

// now is replaced in tests.
var now = time.Now

// CancelPlan asks the agent to abort the plan currently assigned to secret, by setting
// PlanCanceledAnnotation on behalf of writer (see PlanWriterAnnotation). It reports whether the
// annotation had to be written, and returns the secret to go on using — the updated one when it
// wrote, the one passed in otherwise — so a caller can assign it back unconditionally.
//
// Cancellation is terminal for the plan: clearing the annotation does not resume it, and the agent
// will not act again until new plan content is assigned. It is also idempotent here, so a caller
// which cannot tell whether a previous attempt landed can simply call it again.
func (s *Store) CancelPlan(secret *corev1.Secret, writer string) (bool, *corev1.Secret, error) {
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
	setWriter(updated, writer)

	updated, err := s.secrets.Update(updated)
	if err != nil {
		return false, secret, err
	}

	return true, updated, nil
}

// AssignPlan assigns the plan to the secret on behalf of writer, the key the caller holds the
// cluster's beacon under (see PlanWriterAnnotation), and returns the status of the plan the secret
// then holds.
//
// The plan is assigned afresh, with plan-state set to pending, when its content differs from the
// plan on the secret or when writer differs from whoever assigned that one. So a plan whose content
// is identical to the previous assignment's is still run again when it is assigned by somebody else,
// such as the next operation to run on the cluster; and calling AssignPlan again with the same plan
// and writer, as every reconcile does, leaves it alone and reports on it.
//
// maxFailures is the number of attempts to make at the plan, and failureThreshold the number of
// failures after which it is reported as failed; -1 means unlimited for either. The agent stops at
// the first failure and leaves the retry to the orchestrator, so the retries are made, and counted,
// here: see PlanAttemptAnnotation.
//
// Note that the agent starts a pending plan at attempt 1, so CATTLE_AGENT_ATTEMPT_NUMBER does not
// advance across these retries. A plan built from IdempotentActionScript instructions should be
// assigned with a single attempt, as a retry would find the instruction already recorded as run.
func (s *Store) AssignPlan(secret *corev1.Secret, plan *Plan, writer string, maxFailures, failureThreshold int) (*PlanStatus, error) {
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

	if !bytes.Equal(secret.Data[PlanDataKey], data) || secret.Annotations[PlanWriterAnnotation] != writer {
		markPending(secret, writer, 1)
		delete(secret.Annotations, PlanCanceledAnnotation)
		// The agent scopes a resume checkpoint to the plan's checksum, which an identical plan
		// assigned afresh shares with the one before it. A checkpoint left suspended by that one would
		// otherwise have this one resume part-way through. It is cleared with an empty value rather
		// than deleted, as the agent clears it.
		secret.Data[PlanCheckpointKey] = []byte{}

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
		return &PlanStatus{Secret: secret, State: PlanStatePending}, nil
	}

	status, err := Status(secret)
	if err != nil {
		return nil, err
	}
	if status.State != PlanStateFailed || !status.Retrying {
		return status, nil
	}
	return s.retryFailedPlan(status, writer)
}

// Status returns the status of the plan assigned to secret, without acting on it: unlike
// AssignPlan, it never retries a failed plan.
func Status(secret *corev1.Secret) (*PlanStatus, error) {
	result := &PlanStatus{Secret: secret, State: PlanState(secret.Data[PlanStateKey])}

	attempt, err := planAttempt(secret)
	if err != nil {
		return nil, err
	}

	switch result.State {
	case PlanStatePending, PlanStateInProgress:
		// A retry of an earlier failed attempt is still reported as retrying, so the plan's message
		// shows that it has failed before rather than reading as a first attempt.
		result.Retrying = attempt > 1
	case PlanStateSucceeded:
		result.ProbesPassed, err = probesPassed(secret)
		if err != nil {
			return nil, err
		}
	case PlanStateFailed:
		result.Retrying, err = retryPermitted(secret, attempt)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// retryFailedPlan sets a failed plan which has attempts left back to pending for the agent to run it
// again, once failedPlanRetryCooldown has passed since the last attempt.
func (s *Store) retryFailedPlan(status *PlanStatus, writer string) (*PlanStatus, error) {
	secret := status.Secret

	now := now()

	lastApply, err := time.Parse(time.UnixDate, string(secret.Data[lastApplyTimeKey]))

	// if now is before last apply there is clearly an issue with timezones so attempt the retry immediately.
	if err == nil && !now.Before(lastApply) && now.Before(lastApply.Add(failedPlanRetryCooldown)) {
		return status, nil
	}

	attempt, err := planAttempt(secret)
	if err != nil {
		return nil, err
	}

	secret = secret.DeepCopy()
	markPending(secret, writer, attempt+1)
	secret, err = s.secrets.Update(secret)
	if err != nil {
		return nil, err
	}

	return &PlanStatus{Secret: secret, State: PlanStatePending, Retrying: true}, nil
}

// retryPermitted reports whether a plan which failed on the given attempt is to be attempted again:
// it has neither reached its failure threshold nor run out of attempts, and has not been asked to
// stop. The failure-threshold is set by AssignPlan; if it is not set, then it essentially defaults
// to 1, and any failure causes the plan to be marked as failed. max-failures, if not set, does not
// limit the attempts.
func retryPermitted(secret *corev1.Secret, attempt int) (bool, error) {
	if secret.Annotations[PlanCanceledAnnotation] == "true" {
		// Retrying it would only have the agent record the cancellation.
		return false, nil
	}

	failureThreshold, err := failureLimit(secret, failureThresholdKey, 1)
	if err != nil {
		return false, err
	}
	if failureThreshold != -1 && attempt >= failureThreshold {
		return false, nil
	}

	maxFailures, err := failureLimit(secret, maxFailuresKey, -1)
	if err != nil {
		return false, err
	}
	return maxFailures == -1 || attempt < maxFailures, nil
}

// failureLimit returns the failure limit stored under key, or def if it is not set.
func failureLimit(secret *corev1.Secret, key string, def int) (int, error) {
	raw := secret.Data[key]
	if len(raw) == 0 {
		return def, nil
	}
	limit, err := strconv.Atoi(string(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", key, raw, err)
	}
	return limit, nil
}

// markPending resets secret for the agent to run its plan from the start as the given attempt on
// behalf of writer, discarding everything recorded about the previous run's probes so that this run
// is judged on its own.
func markPending(secret *corev1.Secret, writer string, attempt int) {
	delete(secret.Data, probeStatusesKey)
	secret.Annotations[PlanLastUpdatedAnnotation] = now().UTC().Format(time.RFC3339)
	secret.Annotations[PlanProbesPassedAnnotation] = ""
	secret.Annotations[PlanAttemptAnnotation] = strconv.Itoa(attempt)
	setWriter(secret, writer)
	secret.Data[PlanStateKey] = []byte(PlanStatePending)
}

// setWriter records writer as PlanWriterAnnotation on secret, whose annotations must be non-nil.
func setWriter(secret *corev1.Secret, writer string) {
	if writer == "" {
		delete(secret.Annotations, PlanWriterAnnotation)
		return
	}
	secret.Annotations[PlanWriterAnnotation] = writer
}

// planAttempt returns which attempt at the assigned plan the agent is on; see PlanAttemptAnnotation.
// A plan assigned before the annotation existed has only been attempted once as far as the Store
// knows.
func planAttempt(secret *corev1.Secret) (int, error) {
	raw, ok := secret.Annotations[PlanAttemptAnnotation]
	if !ok || raw == "" {
		return 1, nil
	}
	attempt, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", PlanAttemptAnnotation, raw, err)
	}
	return attempt, nil
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

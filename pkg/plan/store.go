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
}

// Success returns true if the plan has been successfully applied and all probes have passed.
func (p *PlanStatus) Success() bool {
	return p.Applied && p.ProbesPassed
}

// Failure returns true if the plan has failed to be applied.
func (p *PlanStatus) Failure() bool {
	return p.Failed
}

// Wait returns true if the plan is in a transient state, and a message indicating why.
func (p *PlanStatus) Wait() (bool, string) {
	switch {
	case p.Pending:
		return true, "waiting for plan to be picked up"
	case p.InProgress:
		return true, "waiting for plan to be applied"
	case p.Applied && !p.ProbesPassed:
		return true, "waiting for probes"
	case p.Applied && p.ProbesPassed:
		return false, "plan successfully applied"
	case p.Failing && !p.Failed:
		return true, "waiting for plan to succeed or reach failure limit"
	case p.Failed:
		return false, "plan failed to be applied"
	}
	return false, ""
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

// AssignPlan assigns the plan to the secret.
// Returns a PlanStatus indicating the current state of the plan.
// This function is based off the CAPR assignAndCheckPlan function and will supersede it in the future once its CAPI dependency is unraveled.
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

	result := &PlanStatus{}

	if !bytes.Equal(secret.Data["plan"], data) {
		result.Pending = true
		delete(secret.Data, "probe-statuses")
		secret.Annotations[PlanLastUpdatedAnnotation] = time.Now().UTC().Format(time.RFC3339)
		secret.Annotations[PlanProbesPassedAnnotation] = ""

		secret.Data["plan"] = data
		if maxFailures > 0 || maxFailures == -1 {
			secret.Data["max-failures"] = []byte(strconv.Itoa(maxFailures))
		} else {
			delete(secret.Data, "max-failures")
		}

		if failureThreshold > 0 || failureThreshold == -1 {
			secret.Data["failure-threshold"] = []byte(strconv.Itoa(failureThreshold))
		} else {
			delete(secret.Data, "failure-threshold")
		}

		secret, err = s.secrets.Update(secret)
		if err != nil {
			return nil, err
		}
	} else {
		result.Pending = false
		result.InProgress = true
	}

	probes := secret.Data["probe-statuses"]
	if probesPassed, ok := secret.Annotations[PlanProbesPassedAnnotation]; ok && probesPassed != "" {
		if len(probes) > 0 {
			_, healthy, err := ParseProbeStatuses(probes)
			if err != nil {
				return nil, err
			}
			result.ProbesPassed = healthy
		}
	}

	planData := secret.Data["plan"]
	failedChecksum := string(secret.Data["failed-checksum"])
	failureCount := secret.Data["failure-count"]

	if len(failureCount) > 0 && PlanHash(planData) == failedChecksum {
		failureCount, err := strconv.Atoi(string(failureCount))
		if err != nil {
			return nil, err
		}
		if failureCount > 0 {
			result.Failed = true
			// The failure-threshold is set by Rancher when the plan is updated. If it is not set, then it essentially
			// defaults to 1, and any failure causes the plan to be marked as failed
			rawFailureThreshold := secret.Data["failure-threshold"]
			if len(rawFailureThreshold) > 0 {
				failureThreshold, err := strconv.Atoi(string(rawFailureThreshold))
				if err != nil {
					return nil, err
				}
				if failureCount < failureThreshold || failureThreshold == -1 {
					// the plan hasn't actually failed to be applied because we haven't passed the failure threshold or failure threshold is set to -1.
					result.Failed = false
					result.Failing = true
				}
			}
		}
	}

	appliedPlanData := secret.Data["appliedPlan"]

	if bytes.Equal(planData, appliedPlanData) {
		result.Applied = true
	}

	if result.Applied || result.Failed {
		result.InProgress = false
	}

	return result, nil
}

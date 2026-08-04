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
}

// Success returns true if the plan has been successfully applied and all probes have passed.
func (p *PlanStatus) Success() bool {
	return p.Applied && p.ProbesPassed
}

// Failure returns true if the plan has failed to be applied.
func (p *PlanStatus) Failure() bool {
	return p.Failed
}

// Waiting returns true if the plan is in a transient state.
func (p *PlanStatus) Waiting() bool {
	switch {
	case p.Pending:
		return true
	case p.InProgress:
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
	case p.Pending:
		return "waiting for plan to be picked up"
	case p.InProgress:
		return "waiting for plan to be applied"
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
//  4. waiting for probes (Applied == true, ProbesPassed == false)
//
// Nodes that do not fit into these buckets (e.g., fully successfully applied or strictly failed) are ignored.
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
		if res.Failing && !res.Failed {
			buckets["failing plan"] = append(buckets["failing plan"], name)
		} else if res.Pending {
			buckets["waiting for plan to be picked up"] = append(buckets["waiting for plan to be picked up"], name)
		} else if res.InProgress {
			buckets["waiting for plan applied"] = append(buckets["waiting for plan applied"], name)
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
		case "waiting for probes":
			return 4
		default:
			return 5
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

	result := &PlanStatus{
		Secret: secret,
	}

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
		result.Secret = secret
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

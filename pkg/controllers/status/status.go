package status

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Status field summary stati
const (
	SummaryInProgress  = "InProgress"
	SummaryCompleted   = "Completed"
	SummaryError       = "Error"
	SummaryTerminating = "Terminating"
)

func RemoveConditions(conditions []v1.Condition, toRemove map[string]struct{}) []v1.Condition {
	filtered := []v1.Condition{}
	for _, c := range conditions {
		// Skip over conditions found in toRemove.
		if _, ok := toRemove[c.Reason]; ok {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered
}

func HasAllOf(conditions []v1.Condition, toHave map[string]struct{}) bool {
	// collect conditions for quick lookup
	has := map[string]struct{}{}
	for _, c := range conditions {
		has[c.Reason] = struct{}{}
	}
	// check that all required conditions are present
	for key, _ := range toHave {
		if _, ok := has[key]; !ok {
			return false
		}
	}
	return true
}

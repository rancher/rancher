package planner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rancher/rancher/pkg/capr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func atMostThree(names []string) string {
	sort.Strings(names)
	if len(names) > 3 {
		return fmt.Sprintf("%s and %d more", strings.Join(names[:3], ","), len(names)-3)
	}
	return strings.Join(names, ",")
}

func detailedMessage(machines []string, messages map[string][]string) string {
	if len(machines) != 1 {
		return ""
	}
	message := messages[machines[0]]
	if len(message) != 0 {
		return fmt.Sprintf(": %s", strings.Join(message, ", "))
	}
	return ""
}

// removeReconciledCondition removes the condition "Reconciled" from a CAPI machine object so that messages are not
// duplicated during summarization. The condition is removed from both v1beta2 (status.conditions) and deprecated
// v1beta1 (status.deprecated.v1beta1.conditions) because the wrangler summarizer reads from the v1beta1 conditions
// for CAPI v1beta2 resources.
func removeReconciledCondition(machine *capi.Machine) *capi.Machine {
	if machine == nil {
		return machine
	}

	reconciledType := string(capr.Reconciled)

	// Check if the Reconciled condition exists in either location.
	var foundInV1Beta2, foundInV1Beta1 bool
	for _, c := range machine.Status.Conditions {
		if c.Type == reconciledType {
			foundInV1Beta2 = true
			break
		}
	}
	for _, c := range machine.GetV1Beta1Conditions() {
		if string(c.Type) == reconciledType {
			foundInV1Beta1 = true
			break
		}
	}

	if !foundInV1Beta2 && !foundInV1Beta1 {
		return machine
	}

	machine = machine.DeepCopy()

	if foundInV1Beta2 {
		conds := make([]metav1.Condition, 0, len(machine.Status.Conditions))
		for _, c := range machine.Status.Conditions {
			if c.Type != reconciledType {
				conds = append(conds, c)
			}
		}
		machine.SetConditions(conds)
	}

	if foundInV1Beta1 {
		v1beta1Conds := machine.GetV1Beta1Conditions()
		filtered := make(capi.Conditions, 0, len(v1beta1Conds))
		for _, c := range v1beta1Conds {
			if string(c.Type) != reconciledType {
				filtered = append(filtered, c)
			}
		}
		machine.SetV1Beta1Conditions(filtered)
	}

	return machine
}

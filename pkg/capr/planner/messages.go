package planner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rancher/rancher/pkg/capr"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
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
// duplicated during summarization.
func removeReconciledCondition(machine *capi.Machine) *capi.Machine {
	if machine == nil || len(machine.Status.Conditions) == 0 {
		return machine
	}

	conds := make([]capi.Condition, 0, len(machine.Status.Conditions))
	for _, c := range machine.Status.Conditions {
		if string(c.Type) != string(capr.Reconciled) {
			conds = append(conds, c)
		}
	}

	if len(conds) == len(machine.Status.Conditions) {
		return machine
	}

	machine = machine.DeepCopy()
	machine.SetConditions(conds)
	return machine
}

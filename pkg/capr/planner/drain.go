package planner

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rancher/norman/types/convert"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/wrangler/v2/pkg/kv"
)

func getRestartStamp(plan *plan.NodePlan) string {
	for _, instr := range plan.Instructions {
		for _, env := range instr.Env {
			k, v := kv.Split(env, "=")
			if k == "RESTART_STAMP" ||
				k == "$env:RESTART_STAMP" {
				return v
			}
		}
	}
	return ""
}

// shouldDrain determines whether the node should be drained based on the plans provided. If the oldPlan doesn't exist,
// then it will not be drained, otherwise, it compares the restart stamps to determine whether the engine will be restarted
func shouldDrain(oldPlan *plan.NodePlan, newPlan plan.NodePlan) bool {
	if oldPlan == nil {
		return false
	}
	return getRestartStamp(oldPlan) != getRestartStamp(&newPlan)
}

func optionsToString(options rkev1.DrainOptions, disable bool) (string, error) {
	if disable {
		options.Enabled = false
	}
	// convert to map first for consistent ordering before creating the json string
	opts, err := convert.EncodeToMap(options)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(opts)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (p *Planner) drain(oldPlan *plan.NodePlan, newPlan plan.NodePlan, entry *planEntry, clusterPlan *plan.Plan, options rkev1.DrainOptions) (bool, error) {
	if entry == nil || entry.Metadata == nil || entry.Metadata.Annotations == nil || entry.Machine == nil || entry.Machine.Status.NodeRef == nil {
		return true, nil
	}

	// Short circuit if there is nothing to do, don't set annotations and move on
	if (!options.Enabled || len(clusterPlan.Machines) == 1) &&
		len(options.PreDrainHooks) == 0 &&
		len(options.PostDrainHooks) == 0 {
		return true, nil
	}

	if !shouldDrain(oldPlan, newPlan) {
		return true, nil
	}

	// Don't drain a single node cluster, but still run the hooks
	optionString, err := optionsToString(options, len(clusterPlan.Machines) == 1)
	if err != nil {
		return false, err
	}

	if entry.Metadata.Annotations[capr.DrainAnnotation] != optionString {
		entry.Metadata.Annotations[capr.DrainAnnotation] = optionString
		return false, p.store.updatePlanSecretLabelsAndAnnotations(entry)
	}

	if err := checkForDrainError(entry, "draining"); err != nil {
		// This is the only place true and an error is returned to indicate that draining is ongoing, but there is an error.
		return true, err
	}

	return entry.Metadata.Annotations[capr.DrainDoneAnnotation] == optionString, nil
}

func (p *Planner) undrain(entry *planEntry) (bool, error) {
	if entry.Metadata.Annotations[capr.DrainAnnotation] != "" &&
		entry.Metadata.Annotations[capr.DrainAnnotation] != entry.Metadata.Annotations[capr.UnCordonAnnotation] {
		entry.Metadata.Annotations[capr.UnCordonAnnotation] = entry.Metadata.Annotations[capr.DrainAnnotation]
		return false, p.store.updatePlanSecretLabelsAndAnnotations(entry)
	}

	if err := checkForDrainError(entry, "undraining"); err != nil {
		// This is the only place true and an error is returned to indicate that undraining is ongoing, but there is an error.
		return true, err
	}

	// The annotations will be removed when undrain is done
	return entry.Metadata.Annotations[capr.UnCordonAnnotation] == "", nil
}

// checkForDrainError checks if there is an error in the drain annotations. It ignores errors that contain "error trying to reach service"
// because these indicate that the cluster is not reachable because the node is restarting or the cattle-cluster-agent is getting rescheduled.
// These errors are expected during draining and it is not necessary to alert the user about them.
func checkForDrainError(entry *planEntry, drainStep string) error {
	if errStr := entry.Metadata.Annotations[capr.DrainErrorAnnotation]; errStr != "" && !strings.Contains(errStr, "error trying to reach service") {
		return fmt.Errorf("error %s machine %s: %s", drainStep, entry.Machine.Name, errStr)
	}
	return nil
}

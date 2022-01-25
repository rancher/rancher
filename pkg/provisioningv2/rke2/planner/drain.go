package planner

import (
	"encoding/json"

	"github.com/rancher/norman/types/convert"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/wrangler/pkg/kv"
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

	if entry.Metadata.Annotations[rke2.DrainAnnotation] != optionString {
		entry.Metadata.Annotations[rke2.DrainAnnotation] = optionString
		return false, p.store.updatePlanSecretLabelsAndAnnotations(entry)
	}

	return entry.Metadata.Annotations[rke2.DrainDoneAnnotation] == optionString, nil
}

func (p *Planner) undrain(entry *planEntry) (bool, error) {
	if entry.Metadata.Annotations[rke2.DrainAnnotation] != "" &&
		entry.Metadata.Annotations[rke2.DrainAnnotation] != entry.Metadata.Annotations[rke2.UnCordonAnnotation] {
		entry.Metadata.Annotations[rke2.UnCordonAnnotation] = entry.Metadata.Annotations[rke2.DrainAnnotation]
		return false, p.store.updatePlanSecretLabelsAndAnnotations(entry)
	}

	return entry.Metadata.Annotations[rke2.UnCordonAnnotation] == "", nil
}

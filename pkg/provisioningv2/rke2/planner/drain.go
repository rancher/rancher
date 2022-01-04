package planner

import (
	"encoding/json"

	"github.com/rancher/norman/types/convert"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/wrangler/pkg/kv"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	PreDrainAnnotation  = "rke.cattle.io/pre-drain"
	PostDrainAnnotation = "rke.cattle.io/post-drain"
	DrainAnnotation     = "rke.cattle.io/drain-options"
	UnCordonAnnotation  = "rke.cattle.io/uncordon"
	DrainDoneAnnotation = "rke.cattle.io/drain-done"
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

func (p *Planner) drain(oldPlan *plan.NodePlan, newPlan plan.NodePlan, machine *capi.Machine, clusterPlan *plan.Plan, options rkev1.DrainOptions) (bool, error) {
	if machine == nil || machine.Status.NodeRef == nil {
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

	if machine.Annotations[DrainAnnotation] != optionString {
		return p.setMachineAnnotation(machine, DrainAnnotation, optionString)
	}

	return machine.Annotations[DrainDoneAnnotation] == optionString, nil
}

func (p *Planner) setMachineAnnotation(machine *capi.Machine, key, value string) (bool, error) {
	machine = machine.DeepCopy()
	if machine.Annotations == nil {
		machine.Annotations = map[string]string{}
	}
	machine.Annotations[key] = value
	_, err := p.machines.Update(machine)
	return false, err
}

func (p *Planner) undrain(machine *capi.Machine) (bool, error) {
	if machine.Annotations[DrainAnnotation] != "" &&
		machine.Annotations[DrainAnnotation] != machine.Annotations[UnCordonAnnotation] {
		return p.setMachineAnnotation(machine, UnCordonAnnotation, machine.Annotations[DrainAnnotation])
	}

	return machine.Annotations[UnCordonAnnotation] == "", nil
}

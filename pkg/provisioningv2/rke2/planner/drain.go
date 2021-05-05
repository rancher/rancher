package planner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/rancher/norman/types/convert"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

const (
	DrainAnnotation     = "rke.cattle.io/drain-options"
	UnCordonAnnotation  = "rke.cattle.io/uncordon"
	DrainDoneAnnotation = "rke.cattle.io/drain-done"
)

func DrainHash(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])[:12]
}

func (p *Planner) drain(machine *capi.Machine, clusterPlan *plan.Plan, options rkev1.DrainOptions) (bool, error) {
	// We never drain a single node cluster
	if !options.Enabled || len(clusterPlan.Machines) == 1 {
		return true, nil
	}

	// convert to map first for consistent ordering before creating the json string
	opts, err := convert.EncodeToMap(options)
	if err != nil {
		return false, err
	}

	data, err := json.Marshal(opts)
	if err != nil {
		return false, err
	}

	hash := DrainHash(data)
	if machine.Annotations[DrainDoneAnnotation] == hash {
		return true, nil
	}

	if machine.Annotations[DrainAnnotation] != string(data) {
		machine = machine.DeepCopy()
		if machine.Annotations == nil {
			machine.Annotations = map[string]string{}
		}
		machine.Annotations[DrainAnnotation] = string(data)
		_, err := p.machines.Update(machine)
		return false, err
	}

	return false, nil
}

func (p *Planner) undrain(machine *capi.Machine) (bool, error) {
	if machine.Annotations[DrainAnnotation] != "" {
		machine = machine.DeepCopy()
		delete(machine.Annotations, DrainAnnotation)
		delete(machine.Annotations, DrainDoneAnnotation)
		machine.Annotations[UnCordonAnnotation] = "true"
		_, err := p.machines.Update(machine)
		return false, err
	}

	return machine.Annotations[UnCordonAnnotation] == "", nil
}

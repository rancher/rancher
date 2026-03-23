package chart

import (
	"encoding/json"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

// WebhookHelmValues translates a WebhookDeploymentCustomization into a Helm values map
// suitable for passing to the rancher-webhook chart. The keys correspond directly to
// the values defined in the chart's values.yaml.
// Returns nil, nil when customization is nil.
func WebhookHelmValues(wdc *v3.WebhookDeploymentCustomization) (map[string]interface{}, error) {
	if wdc == nil {
		return nil, nil
	}

	values := map[string]interface{}{}

	if wdc.ReplicaCount != nil {
		values["replicaCount"] = *wdc.ReplicaCount
	}

	if len(wdc.AppendTolerations) > 0 {
		v, err := marshalToInterface(wdc.AppendTolerations)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal webhook tolerations: %w", err)
		}
		values["tolerations"] = v
	}

	if wdc.OverrideAffinity != nil {
		v, err := marshalToInterface(wdc.OverrideAffinity)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal webhook affinity: %w", err)
		}
		values["affinity"] = v
	}

	if wdc.OverrideResourceRequirements != nil {
		v, err := marshalToInterface(wdc.OverrideResourceRequirements)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal webhook resources: %w", err)
		}
		values["resources"] = v
	}

	if wdc.PodDisruptionBudget != nil {
		pdb := map[string]interface{}{
			"enabled": true,
		}
		if wdc.PodDisruptionBudget.MinAvailable != "" {
			pdb["minAvailable"] = wdc.PodDisruptionBudget.MinAvailable
		}
		if wdc.PodDisruptionBudget.MaxUnavailable != "" {
			pdb["maxUnavailable"] = wdc.PodDisruptionBudget.MaxUnavailable
		}
		values["podDisruptionBudget"] = pdb
	}

	return values, nil
}

// marshalToInterface round-trips a value through JSON encoding to produce a
// map[string]interface{} or []interface{} representation suitable for Helm values.
func marshalToInterface(v interface{}) (interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err = json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

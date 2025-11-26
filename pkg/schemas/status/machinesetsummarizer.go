package status

import (
	"fmt"

	"github.com/rancher/wrangler/v3/pkg/data"
	"github.com/rancher/wrangler/v3/pkg/summary"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func init() {
	// Prepend the MachineSet summarizer to run before other summarizers.
	// This ensures we can set appropriate states for MachineSets before
	// the default summarizers run (which would set "unavailable" for Ready=False).
	summary.Summarizers = append([]summary.Summarizer{checkMachineSet}, summary.Summarizers...)
}

// checkMachineSet is a custom summarizer for cluster.x-k8s.io MachineSets.
// It handles the following cases:
// - MachineSet scaled down to 0: shows "Scaled down" (not error/transitioning)
// - MachineSet scaling up: shows "Scaling up" (transitioning)
// - MachineSet scaling down: shows "Scaling down" (transitioning)
// - MachineSet with all replicas ready: shows "active" (not transitioning)
func checkMachineSet(obj data.Object, conditions []summary.Condition, s summary.Summary) summary.Summary {
	if s.State != "" {
		return s
	}

	// Only handle MachineSets from cluster.x-k8s.io
	ustr := &unstructured.Unstructured{Object: obj}
	gvk := ustr.GroupVersionKind()
	if gvk.Group != "cluster.x-k8s.io" || gvk.Kind != "MachineSet" {
		return s
	}

	specReplicas, specFound, _ := unstructured.NestedInt64(obj, "spec", "replicas")
	statusReplicas, _, _ := unstructured.NestedInt64(obj, "status", "replicas")
	availableReplicas, _, _ := unstructured.NestedInt64(obj, "status", "availableReplicas")

	// Get desired replicas - use spec.replicas if set, otherwise default to 1
	var desiredReplicas int64 = 1
	if specFound {
		desiredReplicas = specReplicas
	}

	// Case 1: Scaled down to 0 replicas
	if desiredReplicas == 0 && statusReplicas == 0 {
		s.State = "Scaled down"
		s.Transitioning = false
		s.Error = false
		return s
	}

	// Case 2: Scaling up (desired > current)
	if desiredReplicas > statusReplicas {
		s.State = "Scaling up"
		s.Transitioning = true
		s.Message = append(s.Message, scalingMessage(statusReplicas, desiredReplicas))
		return s
	}

	// Case 3: Scaling down (desired < current)
	if desiredReplicas < statusReplicas {
		s.State = "Scaling down"
		s.Transitioning = true
		s.Message = append(s.Message, scalingMessage(statusReplicas, desiredReplicas))
		return s
	}

	// Case 4: Replicas match but not all are available yet (waiting for machines to become ready)
	if desiredReplicas > 0 && availableReplicas < desiredReplicas {
		s.State = "Updating"
		s.Transitioning = true
		return s
	}

	// Case 5: All replicas are ready and available
	if desiredReplicas > 0 && availableReplicas >= desiredReplicas {
		s.State = "active"
		s.Transitioning = false
		return s
	}

	return s
}

func scalingMessage(current, desired int64) string {
	return fmt.Sprintf("%d of %d replicas", current, desired)
}

package planner

import (
	"sort"

	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type planEntry struct {
	Machine  *capi.Machine
	Plan     *plan.Node
	Metadata *plan.Metadata
}

type roleFilter func(*planEntry) bool

func collectAndValidateAnnotationValue(p *plan.Plan, validation roleFilter, annotation, value string) bool {
	for machineName, machine := range p.Machines {
		entry := &planEntry{
			Machine:  machine,
			Plan:     p.Nodes[machineName],
			Metadata: p.Metadata[machineName],
		}
		if !validation(entry) {
			continue
		}
		if entry.Metadata.Annotations[annotation] == value {
			return true
		}
	}
	return false
}

func collect(p *plan.Plan, include roleFilter) (result []*planEntry) {
	for machineName, machine := range p.Machines {
		entry := &planEntry{
			Machine:  machine,
			Plan:     p.Nodes[machineName],
			Metadata: p.Metadata[machineName],
		}
		if !include(entry) {
			continue
		}
		result = append(result, entry)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Machine.Name < result[j].Machine.Name
	})

	return result
}

func isEtcd(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Labels[capr.EtcdRoleLabel] == "true"
}

func isInitNode(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Labels[capr.InitNodeLabel] == "true"
}

func isInitNodeOrDeleting(entry *planEntry) bool {
	return isInitNode(entry) || isDeleting(entry)
}

func IsEtcdOnlyInitNode(entry *planEntry) bool {
	return isInitNode(entry) && IsOnlyEtcd(entry)
}

func isNotInitNodeOrIsDeleting(entry *planEntry) bool {
	return !isInitNode(entry) || isDeleting(entry)
}

func isDeleting(entry *planEntry) bool {
	return entry.Machine.DeletionTimestamp != nil
}

func isNotDeleting(entry *planEntry) bool {
	return !isDeleting(entry)
}

func isNotDeletingAndControlPlaneOrInitNode(entry *planEntry) bool {
	return !isDeleting(entry) && (isControlPlane(entry) || isInitNode(entry))
}

// isFailed returns true if the provided entry machine.status.phase is failed
func isFailed(entry *planEntry) bool {
	return entry.Machine.Status.Phase == string(capi.MachinePhaseFailed)
}

// canBeInitNode returns true if the provided entry is an etcd node, is not deleting, is not failed, and has its infrastructure ready
// We should wait for the infrastructure condition to be marked as ready because we need the IP address(es) set prior to bootstrapping the node.
func canBeInitNode(entry *planEntry) bool {
	return isEtcd(entry) && !isDeleting(entry) && !isFailed(entry) && capr.InfrastructureReady.IsTrue(entry.Machine)
}

func isControlPlane(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Labels[capr.ControlPlaneRoleLabel] == "true"
}

func isControlPlaneAndNotInitNode(entry *planEntry) bool {
	return isControlPlane(entry) && !isInitNode(entry)
}

func isControlPlaneEtcd(entry *planEntry) bool {
	return isControlPlane(entry) || isEtcd(entry)
}

func IsOnlyEtcd(entry *planEntry) bool {
	return isEtcd(entry) && !isControlPlane(entry)
}

func isOnlyControlPlane(entry *planEntry) bool {
	return !isEtcd(entry) && isControlPlane(entry)
}

func isWorker(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Labels[capr.WorkerRoleLabel] == "true"
}

func noRole(entry *planEntry) bool {
	return !isEtcd(entry) && !isControlPlane(entry) && !isWorker(entry)
}

func anyRole(entry *planEntry) bool {
	return !noRole(entry)
}

func anyRoleWithoutWindows(entry *planEntry) bool {
	return !noRole(entry) && !windows(entry)
}

func isOnlyWorker(entry *planEntry) bool {
	return !isEtcd(entry) && !isControlPlane(entry) && isWorker(entry)
}

func windows(entry *planEntry) bool {
	if entry == nil || entry.Metadata == nil {
		return false
	}
	if val, ok := entry.Metadata.Labels[capr.CattleOSLabel]; ok {
		return val == capr.WindowsMachineOS
	}
	return false
}

func anyPlanDataExists(entry *planEntry) bool {
	if entry.Plan != nil {
		return entry.Plan.PlanDataExists
	}
	return false
}

func validJoinURL(plan *plan.Plan, joinURL string) bool {
	return collectAndValidateAnnotationValue(plan, isNotDeletingAndControlPlaneOrInitNode, capr.JoinURLAnnotation, joinURL)
}

func hasJoinURL(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Annotations[capr.JoinURLAnnotation] != ""
}

func hasJoinedTo(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Annotations[capr.JoinedToAnnotation] != ""
}

func roleAnd(r1, r2 roleFilter) roleFilter {
	return func(entry *planEntry) bool {
		return r1(entry) && r2(entry)
	}
}

func roleOr(r1, r2 roleFilter) roleFilter {
	return func(entry *planEntry) bool {
		return r1(entry) || r2(entry)
	}
}

func roleNot(r1 roleFilter) roleFilter {
	return func(entry *planEntry) bool {
		return !r1(entry)
	}
}

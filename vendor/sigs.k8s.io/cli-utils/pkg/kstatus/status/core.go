// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"fmt"
	"math"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GetConditionsFn defines the signature for functions to compute the
// status of a built-in resource.
type GetConditionsFn func(*unstructured.Unstructured) (*Result, error)

// legacyTypes defines the mapping from GroupKind to a function that can
// compute the status for the given resource.
var legacyTypes = map[string]GetConditionsFn{
	"Service":                    serviceConditions,
	"Pod":                        podConditions,
	"Secret":                     alwaysReady,
	"PersistentVolumeClaim":      pvcConditions,
	"apps/StatefulSet":           stsConditions,
	"apps/DaemonSet":             daemonsetConditions,
	"extensions/DaemonSet":       daemonsetConditions,
	"apps/Deployment":            deploymentConditions,
	"extensions/Deployment":      deploymentConditions,
	"apps/ReplicaSet":            replicasetConditions,
	"extensions/ReplicaSet":      replicasetConditions,
	"policy/PodDisruptionBudget": pdbConditions,
	"batch/CronJob":              alwaysReady,
	"ConfigMap":                  alwaysReady,
	"batch/Job":                  jobConditions,
	"apiextensions.k8s.io/CustomResourceDefinition": crdConditions,
}

const (
	tooFewReady     = "LessReady"
	tooFewAvailable = "LessAvailable"
	tooFewUpdated   = "LessUpdated"
	tooFewReplicas  = "LessReplicas"
	extraPods       = "ExtraPods"

	onDeleteUpdateStrategy = "OnDelete"

	// How long a pod can be unscheduled before it is reported as
	// unschedulable.
	scheduleWindow = 15 * time.Second
)

// GetLegacyConditionsFn returns a function that can compute the status for the
// given resource, or nil if the resource type is not known.
func GetLegacyConditionsFn(u *unstructured.Unstructured) GetConditionsFn {
	gvk := u.GroupVersionKind()
	g := gvk.Group
	k := gvk.Kind
	key := g + "/" + k
	if g == "" {
		key = k
	}
	return legacyTypes[key]
}

// alwaysReady Used for resources that are always ready
func alwaysReady(u *unstructured.Unstructured) (*Result, error) {
	return &Result{
		Status:     CurrentStatus,
		Message:    "Resource is always ready",
		Conditions: []Condition{},
	}, nil
}

// stsConditions return standardized Conditions for Statefulset
//
// StatefulSet does define the .status.conditions property, but the controller never
// actually sets any Conditions. Thus, status must be computed only based on the other
// properties under .status. We don't have any way to find out if a reconcile for a
// StatefulSet has failed.
func stsConditions(u *unstructured.Unstructured) (*Result, error) {
	obj := u.UnstructuredContent()

	// updateStrategy==ondelete is a user managed statefulset.
	updateStrategy := GetStringField(obj, ".spec.updateStrategy.type", "")
	if updateStrategy == onDeleteUpdateStrategy {
		return &Result{
			Status:     CurrentStatus,
			Message:    "StatefulSet is using the ondelete update strategy",
			Conditions: []Condition{},
		}, nil
	}

	// Replicas
	specReplicas := GetIntField(obj, ".spec.replicas", 1)
	readyReplicas := GetIntField(obj, ".status.readyReplicas", 0)
	currentReplicas := GetIntField(obj, ".status.currentReplicas", 0)
	updatedReplicas := GetIntField(obj, ".status.updatedReplicas", 0)
	statusReplicas := GetIntField(obj, ".status.replicas", 0)
	partition := GetIntField(obj, ".spec.updateStrategy.rollingUpdate.partition", -1)

	if specReplicas > statusReplicas {
		message := fmt.Sprintf("Replicas: %d/%d", statusReplicas, specReplicas)
		return newInProgressStatus(tooFewReplicas, message), nil
	}

	if specReplicas > readyReplicas {
		message := fmt.Sprintf("Ready: %d/%d", readyReplicas, specReplicas)
		return newInProgressStatus(tooFewReady, message), nil
	}

	if statusReplicas > specReplicas {
		message := fmt.Sprintf("Pending termination: %d", statusReplicas-specReplicas)
		return newInProgressStatus(extraPods, message), nil
	}

	// https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#partitions
	if partition != -1 {
		if updatedReplicas < (specReplicas - partition) {
			message := fmt.Sprintf("updated: %d/%d", updatedReplicas, specReplicas-partition)
			return newInProgressStatus("PartitionRollout", message), nil
		}
		// Partition case All ok
		return &Result{
			Status:     CurrentStatus,
			Message:    fmt.Sprintf("Partition rollout complete. updated: %d", updatedReplicas),
			Conditions: []Condition{},
		}, nil
	}

	if specReplicas > currentReplicas {
		message := fmt.Sprintf("current: %d/%d", currentReplicas, specReplicas)
		return newInProgressStatus("LessCurrent", message), nil
	}

	// Revision
	currentRevision := GetStringField(obj, ".status.currentRevision", "")
	updatedRevision := GetStringField(obj, ".status.updateRevision", "")
	if currentRevision != updatedRevision {
		message := "Waiting for updated revision to match current"
		return newInProgressStatus("RevisionMismatch", message), nil
	}

	// All ok
	return &Result{
		Status:     CurrentStatus,
		Message:    fmt.Sprintf("All replicas scheduled as expected. Replicas: %d", statusReplicas),
		Conditions: []Condition{},
	}, nil
}

// deploymentConditions return standardized Conditions for Deployment.
//
// For Deployments, we look at .status.conditions as well as the other properties
// under .status. Status will be Failed if the progress deadline has been exceeded.
func deploymentConditions(u *unstructured.Unstructured) (*Result, error) {
	obj := u.UnstructuredContent()

	progressing := false

	// Check if progressDeadlineSeconds is set. If not, the controller will not set
	// the `Progressing` condition, so it will always consider a deployment to be
	// progressing. The use of math.MaxInt32 is due to special handling in the
	// controller:
	// https://github.com/kubernetes/kubernetes/blob/a3ccea9d8743f2ff82e41b6c2af6dc2c41dc7b10/pkg/controller/deployment/util/deployment_util.go#L886
	progressDeadline := GetIntField(obj, ".spec.progressDeadlineSeconds", math.MaxInt32)
	if progressDeadline == math.MaxInt32 {
		progressing = true
	}

	available := false

	objc, err := GetObjectWithConditions(obj)
	if err != nil {
		return nil, err
	}

	for _, c := range objc.Status.Conditions {
		switch c.Type {
		case "Progressing": //appsv1.DeploymentProgressing:
			// https://github.com/kubernetes/kubernetes/blob/a3ccea9d8743f2ff82e41b6c2af6dc2c41dc7b10/pkg/controller/deployment/progress.go#L52
			if c.Reason == "ProgressDeadlineExceeded" {
				return &Result{
					Status:     FailedStatus,
					Message:    "Progress deadline exceeded",
					Conditions: []Condition{{ConditionStalled, corev1.ConditionTrue, c.Reason, c.Message}},
				}, nil
			}
			if c.Status == corev1.ConditionTrue && c.Reason == "NewReplicaSetAvailable" {
				progressing = true
			}
		case "Available": //appsv1.DeploymentAvailable:
			if c.Status == corev1.ConditionTrue {
				available = true
			}
		}
	}

	// replicas
	specReplicas := GetIntField(obj, ".spec.replicas", 1) // Controller uses 1 as default if not specified.
	statusReplicas := GetIntField(obj, ".status.replicas", 0)
	updatedReplicas := GetIntField(obj, ".status.updatedReplicas", 0)
	readyReplicas := GetIntField(obj, ".status.readyReplicas", 0)
	availableReplicas := GetIntField(obj, ".status.availableReplicas", 0)

	// TODO spec.replicas zero case ??

	if specReplicas > statusReplicas {
		message := fmt.Sprintf("replicas: %d/%d", statusReplicas, specReplicas)
		return newInProgressStatus(tooFewReplicas, message), nil
	}

	if specReplicas > updatedReplicas {
		message := fmt.Sprintf("Updated: %d/%d", updatedReplicas, specReplicas)
		return newInProgressStatus(tooFewUpdated, message), nil
	}

	if statusReplicas > specReplicas {
		message := fmt.Sprintf("Pending termination: %d", statusReplicas-specReplicas)
		return newInProgressStatus(extraPods, message), nil
	}

	if updatedReplicas > availableReplicas {
		message := fmt.Sprintf("Available: %d/%d", availableReplicas, updatedReplicas)
		return newInProgressStatus(tooFewAvailable, message), nil
	}

	if specReplicas > readyReplicas {
		message := fmt.Sprintf("Ready: %d/%d", readyReplicas, specReplicas)
		return newInProgressStatus(tooFewReady, message), nil
	}

	// check conditions
	if !progressing {
		message := "ReplicaSet not Available"
		return newInProgressStatus("ReplicaSetNotAvailable", message), nil
	}
	if !available {
		message := "Deployment not Available"
		return newInProgressStatus("DeploymentNotAvailable", message), nil
	}
	// All ok
	return &Result{
		Status:     CurrentStatus,
		Message:    fmt.Sprintf("Deployment is available. Replicas: %d", statusReplicas),
		Conditions: []Condition{},
	}, nil
}

// replicasetConditions return standardized Conditions for Replicaset
func replicasetConditions(u *unstructured.Unstructured) (*Result, error) {
	obj := u.UnstructuredContent()

	// Conditions
	objc, err := GetObjectWithConditions(obj)
	if err != nil {
		return nil, err
	}

	for _, c := range objc.Status.Conditions {
		// https://github.com/kubernetes/kubernetes/blob/a3ccea9d8743f2ff82e41b6c2af6dc2c41dc7b10/pkg/controller/replicaset/replica_set_utils.go
		if c.Type == "ReplicaFailure" && c.Status == corev1.ConditionTrue {
			message := "Replica Failure condition. Check Pods"
			return newInProgressStatus("ReplicaFailure", message), nil
		}
	}

	// Replicas
	specReplicas := GetIntField(obj, ".spec.replicas", 1) // Controller uses 1 as default if not specified.
	statusReplicas := GetIntField(obj, ".status.replicas", 0)
	readyReplicas := GetIntField(obj, ".status.readyReplicas", 0)
	availableReplicas := GetIntField(obj, ".status.availableReplicas", 0)
	fullyLabelledReplicas := GetIntField(obj, ".status.fullyLabeledReplicas", 0)

	if specReplicas > fullyLabelledReplicas {
		message := fmt.Sprintf("Labelled: %d/%d", fullyLabelledReplicas, specReplicas)
		return newInProgressStatus("LessLabelled", message), nil
	}

	if specReplicas > availableReplicas {
		message := fmt.Sprintf("Available: %d/%d", availableReplicas, specReplicas)
		return newInProgressStatus(tooFewAvailable, message), nil
	}

	if specReplicas > readyReplicas {
		message := fmt.Sprintf("Ready: %d/%d", readyReplicas, specReplicas)
		return newInProgressStatus(tooFewReady, message), nil
	}

	if statusReplicas > specReplicas {
		message := fmt.Sprintf("Pending termination: %d", statusReplicas-specReplicas)
		return newInProgressStatus(extraPods, message), nil
	}
	// All ok
	return &Result{
		Status:     CurrentStatus,
		Message:    fmt.Sprintf("ReplicaSet is available. Replicas: %d", statusReplicas),
		Conditions: []Condition{},
	}, nil
}

// daemonsetConditions return standardized Conditions for DaemonSet
func daemonsetConditions(u *unstructured.Unstructured) (*Result, error) {
	obj := u.UnstructuredContent()

	// replicas
	desiredNumberScheduled := GetIntField(obj, ".status.desiredNumberScheduled", -1)
	currentNumberScheduled := GetIntField(obj, ".status.currentNumberScheduled", 0)
	updatedNumberScheduled := GetIntField(obj, ".status.updatedNumberScheduled", 0)
	numberAvailable := GetIntField(obj, ".status.numberAvailable", 0)
	numberReady := GetIntField(obj, ".status.numberReady", 0)

	if desiredNumberScheduled == -1 {
		message := "Missing .status.desiredNumberScheduled"
		return newInProgressStatus("NoDesiredNumber", message), nil
	}

	if desiredNumberScheduled > currentNumberScheduled {
		message := fmt.Sprintf("Current: %d/%d", currentNumberScheduled, desiredNumberScheduled)
		return newInProgressStatus("LessCurrent", message), nil
	}

	if desiredNumberScheduled > updatedNumberScheduled {
		message := fmt.Sprintf("Updated: %d/%d", updatedNumberScheduled, desiredNumberScheduled)
		return newInProgressStatus(tooFewUpdated, message), nil
	}

	if desiredNumberScheduled > numberAvailable {
		message := fmt.Sprintf("Available: %d/%d", numberAvailable, desiredNumberScheduled)
		return newInProgressStatus(tooFewAvailable, message), nil
	}

	if desiredNumberScheduled > numberReady {
		message := fmt.Sprintf("Ready: %d/%d", numberReady, desiredNumberScheduled)
		return newInProgressStatus(tooFewReady, message), nil
	}

	// All ok
	return &Result{
		Status:     CurrentStatus,
		Message:    fmt.Sprintf("All replicas scheduled as expected. Replicas: %d", desiredNumberScheduled),
		Conditions: []Condition{},
	}, nil
}

// pvcConditions return standardized Conditions for PVC
func pvcConditions(u *unstructured.Unstructured) (*Result, error) {
	obj := u.UnstructuredContent()

	phase := GetStringField(obj, ".status.phase", "unknown")
	if phase != "Bound" { // corev1.ClaimBound
		message := fmt.Sprintf("PVC is not Bound. phase: %s", phase)
		return newInProgressStatus("NotBound", message), nil
	}
	// All ok
	return &Result{
		Status:     CurrentStatus,
		Message:    "PVC is Bound",
		Conditions: []Condition{},
	}, nil
}

// podConditions return standardized Conditions for Pod
func podConditions(u *unstructured.Unstructured) (*Result, error) {
	obj := u.UnstructuredContent()
	objc, err := GetObjectWithConditions(obj)
	if err != nil {
		return nil, err
	}
	phase := GetStringField(obj, ".status.phase", "")

	switch phase {
	case "Succeeded":
		return &Result{
			Status:     CurrentStatus,
			Message:    "Pod has completed successfully",
			Conditions: []Condition{},
		}, nil
	case "Failed":
		return &Result{
			Status:     CurrentStatus,
			Message:    "Pod has completed, but not successfully",
			Conditions: []Condition{},
		}, nil
	case "Running":
		if hasConditionWithStatus(objc.Status.Conditions, "Ready", corev1.ConditionTrue) {
			return &Result{
				Status:     CurrentStatus,
				Message:    "Pod is Ready",
				Conditions: []Condition{},
			}, nil
		}

		containerNames, isCrashLooping, err := getCrashLoopingContainers(obj)
		if err != nil {
			return nil, err
		}
		if isCrashLooping {
			return newFailedStatus("ContainerCrashLooping",
				fmt.Sprintf("Containers in CrashLoop state: %s", strings.Join(containerNames, ","))), nil
		}

		return newInProgressStatus("PodRunningNotReady", "Pod is running but is not Ready"), nil
	case "Pending":
		c, found := getConditionWithStatus(objc.Status.Conditions, "PodScheduled", corev1.ConditionFalse)
		if found && c.Reason == "Unschedulable" {
			if time.Now().Add(-scheduleWindow).Before(u.GetCreationTimestamp().Time) {
				// We give the pod 15 seconds to be scheduled before we report it
				// as unschedulable.
				return newInProgressStatus("PodNotScheduled", "Pod has not been scheduled"), nil
			}
			return newFailedStatus("PodUnschedulable", "Pod could not be scheduled"), nil
		}
		return newInProgressStatus("PodPending", "Pod is in the Pending phase"), nil
	default:
		// If the controller hasn't observed the pod yet, there is no phase. We consider this as it
		// still being in progress.
		if phase == "" {
			return newInProgressStatus("PodNotObserved", "Pod phase not available"), nil
		}
		return nil, fmt.Errorf("unknown phase %s", phase)
	}
}

func getCrashLoopingContainers(obj map[string]interface{}) ([]string, bool, error) {
	var containerNames []string
	css, found, err := unstructured.NestedSlice(obj, "status", "containerStatuses")
	if !found || err != nil {
		return containerNames, found, err
	}
	for _, item := range css {
		cs := item.(map[string]interface{})
		n, found := cs["name"]
		if !found {
			continue
		}
		name := n.(string)
		s, found := cs["state"]
		if !found {
			continue
		}
		state := s.(map[string]interface{})

		ws, found := state["waiting"]
		if !found {
			continue
		}
		waitingState := ws.(map[string]interface{})

		r, found := waitingState["reason"]
		if !found {
			continue
		}
		reason := r.(string)
		if reason == "CrashLoopBackOff" {
			containerNames = append(containerNames, name)
		}
	}
	if len(containerNames) > 0 {
		return containerNames, true, nil
	}
	return containerNames, false, nil
}

// pdbConditions computes the status for PodDisruptionBudgets. A PDB
// is currently considered Current if the disruption controller has
// observed the latest version of the PDB resource and has computed
// the AllowedDisruptions. PDBs do have ObservedGeneration in the
// Status object, so if this function gets called we know that
// the controller has observed the latest changes.
// The disruption controller does not set any conditions if
// computing the AllowedDisruptions fails (and there are many ways
// it can fail), but there is PR against OSS Kubernetes to address
// this: https://github.com/kubernetes/kubernetes/pull/86929
func pdbConditions(_ *unstructured.Unstructured) (*Result, error) {
	// All ok
	return &Result{
		Status:     CurrentStatus,
		Message:    "AllowedDisruptions has been computed.",
		Conditions: []Condition{},
	}, nil
}

// jobConditions return standardized Conditions for Job
//
// A job will have the InProgress status until it starts running. Then it will have the Current
// status while the job is running and after it has been completed successfully. It
// will have the Failed status if it the job has failed.
func jobConditions(u *unstructured.Unstructured) (*Result, error) {
	obj := u.UnstructuredContent()

	parallelism := GetIntField(obj, ".spec.parallelism", 1)
	completions := GetIntField(obj, ".spec.completions", parallelism)
	succeeded := GetIntField(obj, ".status.succeeded", 0)
	active := GetIntField(obj, ".status.active", 0)
	failed := GetIntField(obj, ".status.failed", 0)
	starttime := GetStringField(obj, ".status.startTime", "")

	// Conditions
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/job/utils.go#L24
	objc, err := GetObjectWithConditions(obj)
	if err != nil {
		return nil, err
	}
	for _, c := range objc.Status.Conditions {
		switch c.Type {
		case "Complete":
			if c.Status == corev1.ConditionTrue {
				message := fmt.Sprintf("Job Completed. succeeded: %d/%d", succeeded, completions)
				return &Result{
					Status:     CurrentStatus,
					Message:    message,
					Conditions: []Condition{},
				}, nil
			}
		case "Failed":
			if c.Status == corev1.ConditionTrue {
				return newFailedStatus("JobFailed",
					fmt.Sprintf("Job Failed. failed: %d/%d", failed, completions)), nil
			}
		}
	}

	// replicas
	if starttime == "" {
		message := "Job not started"
		return newInProgressStatus("JobNotStarted", message), nil
	}
	return &Result{
		Status:     CurrentStatus,
		Message:    fmt.Sprintf("Job in progress. success:%d, active: %d, failed: %d", succeeded, active, failed),
		Conditions: []Condition{},
	}, nil
}

// serviceConditions return standardized Conditions for Service
func serviceConditions(u *unstructured.Unstructured) (*Result, error) {
	obj := u.UnstructuredContent()

	specType := GetStringField(obj, ".spec.type", "ClusterIP")
	specClusterIP := GetStringField(obj, ".spec.clusterIP", "")

	if specType == "LoadBalancer" {
		if specClusterIP == "" {
			message := "ClusterIP not set. Service type: LoadBalancer"
			return newInProgressStatus("NoIPAssigned", message), nil
		}
	}

	return &Result{
		Status:     CurrentStatus,
		Message:    "Service is ready",
		Conditions: []Condition{},
	}, nil
}

func crdConditions(u *unstructured.Unstructured) (*Result, error) {
	obj := u.UnstructuredContent()

	objc, err := GetObjectWithConditions(obj)
	if err != nil {
		return nil, err
	}

	for _, c := range objc.Status.Conditions {
		if c.Type == "NamesAccepted" && c.Status == corev1.ConditionFalse {
			return newFailedStatus(c.Reason, c.Message), nil
		}
		if c.Type == "Established" {
			if c.Status == corev1.ConditionFalse && c.Reason != "Installing" {
				return newFailedStatus(c.Reason, c.Message), nil
			}
			if c.Status == corev1.ConditionTrue {
				return &Result{
					Status:     CurrentStatus,
					Message:    "CRD is established",
					Conditions: []Condition{},
				}, nil
			}
		}
	}
	return newInProgressStatus("Installing", "Install in progress"), nil
}

package summary

import (
	"strings"
	"time"

	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/kv"
)

var (
	// True ==
	// False == error
	// Unknown == transitioning
	TransitioningUnknown = map[string]string{
		"Active":                      "activating",
		"AddonDeploy":                 "provisioning",
		"AgentDeployed":               "provisioning",
		"BackingNamespaceCreated":     "configuring",
		"Built":                       "building",
		"CertsGenerated":              "provisioning",
		"ConfigOK":                    "configuring",
		"Created":                     "creating",
		"CreatorMadeOwner":            "configuring",
		"DefaultNamespaceAssigned":    "configuring",
		"DefaultNetworkPolicyCreated": "configuring",
		"DefaultProjectCreated":       "configuring",
		"DockerProvisioned":           "provisioning",
		"Deployed":                    "deploying",
		"Drained":                     "draining",
		"Downloaded":                  "downloading",
		"etcd":                        "provisioning",
		"Inactive":                    "deactivating",
		"Initialized":                 "initializing",
		"Installed":                   "installing",
		"NodesCreated":                "provisioning",
		"Pending":                     "pending",
		"PodScheduled":                "scheduling",
		"Provisioned":                 "provisioning",
		"Refreshed":                   "refreshed",
		"Registered":                  "registering",
		"Removed":                     "removing",
		"Saved":                       "saving",
		"Updated":                     "updating",
		"Updating":                    "updating",
		"Upgraded":                    "upgrading",
		"Waiting":                     "waiting",
		"InitialRolesPopulated":       "activating",
		"ScalingActive":               "pending",
		"AbleToScale":                 "pending",
		"RunCompleted":                "running",
		"Processed":                   "processed",
	}

	// True == error
	// False ==
	// Unknown ==
	ErrorTrue = map[string]bool{
		"OutOfDisk":           true,
		"MemoryPressure":      true,
		"DiskPressure":        true,
		"NetworkUnavailable":  true,
		"KernelHasNoDeadlock": true,
		"Unschedulable":       true,
		"ReplicaFailure":      true,
	}

	// True ==
	// False == error
	// Unknown ==
	ErrorFalse = map[string]bool{
		"Failed":      true,
		"Progressing": true,
	}

	// True ==
	// False == transitioning
	// Unknown == error
	TransitioningFalse = map[string]string{
		"Completed":   "activating",
		"Ready":       "unavailable",
		"Available":   "updating",
		"Progressing": "inactive",
	}

	Summarizers []Summarizer
)

type Summarizer func(obj data.Object, conditions []Condition, summary Summary) Summary

func init() {
	Summarizers = []Summarizer{
		checkErrors,
		checkTransitioning,
		checkActive,
		checkPhase,
		checkInitializing,
		checkRemoving,
		checkLoadBalancer,
	}
}

func checkErrors(_ data.Object, conditions []Condition, summary Summary) Summary {
	for _, c := range conditions {
		if (ErrorFalse[c.Type()] && c.Status() == "False") || c.Reason() == "Error" {
			summary.Error = true
			summary.Message = append(summary.Message, c.Message())
			break
		}
	}

	if summary.Error {
		return summary
	}

	for _, c := range conditions {
		if ErrorTrue[c.Type()] && c.Status() == "True" {
			summary.Error = true
			summary.Message = append(summary.Message, c.Message())
		}
	}
	return summary
}

func checkTransitioning(_ data.Object, conditions []Condition, summary Summary) Summary {
	for _, c := range conditions {
		newState, ok := TransitioningUnknown[c.Type()]
		if !ok {
			continue
		}

		if c.Status() == "False" {
			summary.Error = true
			summary.State = newState
			summary.Message = append(summary.Message, c.Message())
		} else if c.Status() == "Unknown" && summary.State == "" {
			summary.Transitioning = true
			summary.State = newState
			summary.Message = append(summary.Message, c.Message())
		}
	}

	for _, c := range conditions {
		if summary.State != "" {
			break
		}
		newState, ok := TransitioningFalse[c.Type()]
		if !ok {
			continue
		}
		if c.Status() == "False" {
			summary.Transitioning = true
			summary.State = newState
			summary.Message = append(summary.Message, c.Message())
		} else if c.Status() == "Unknown" {
			summary.Error = true
			summary.State = newState
			summary.Message = append(summary.Message, c.Message())
		}
	}

	return summary
}

func checkActive(obj data.Object, _ []Condition, summary Summary) Summary {
	if summary.State != "" {
		return summary
	}

	switch obj.String("spec", "active") {
	case "true":
		summary.State = "active"
	case "false":
		summary.State = "inactive"
	}

	return summary
}

func checkPhase(obj data.Object, _ []Condition, summary Summary) Summary {
	phase := obj.String("status", "phase")
	if phase == "Succeeded" {
		summary.State = "succeeded"
		summary.Transitioning = false
	} else if phase != "" && summary.State == "" {
		summary.State = phase
	}
	return summary
}

func checkInitializing(obj data.Object, conditions []Condition, summary Summary) Summary {
	apiVersion := obj.String("apiVersion")
	_, hasConditions := obj.Map("status")["conditions"]
	if summary.State == "" && hasConditions && len(conditions) == 0 && strings.Contains(apiVersion, "cattle.io") {
		val := obj.String("metadata", "created")
		if i, err := convert.ToTimestamp(val); err == nil {
			if time.Unix(i/1000, 0).Add(5 * time.Second).After(time.Now()) {
				summary.State = "initializing"
				summary.Transitioning = true
			}
		}
	}
	return summary
}

func checkRemoving(obj data.Object, conditions []Condition, summary Summary) Summary {
	removed := obj.String("metadata", "removed")
	if removed == "" {
		return summary
	}

	summary.State = "removing"
	summary.Transitioning = true

	finalizers := obj.StringSlice("metadata", "finalizers")
	if len(finalizers) == 0 {
		finalizers = obj.StringSlice("spec", "finalizers")
	}

	for _, cond := range conditions {
		if cond.Type() == "Removed" && (cond.Status() == "Unknown" || cond.Status() == "False") && cond.Message() != "" {
			summary.Message = append(summary.Message, cond.Message())
		}
	}

	if len(finalizers) == 0 {
		return summary
	}

	_, f := kv.RSplit(finalizers[0], "controller.cattle.io/")
	if f == "foregroundDeletion" {
		f = "object cleanup"
	}

	summary.Message = append(summary.Message, "waiting on "+f)
	if i, err := convert.ToTimestamp(removed); err == nil {
		if time.Unix(i/1000, 0).Add(5 * time.Minute).Before(time.Now()) {
			summary.Error = true
		}
	}

	return summary
}

func checkLoadBalancer(obj data.Object, _ []Condition, summary Summary) Summary {
	if (summary.State == "active" || summary.State == "") &&
		obj.String("kind") == "Service" &&
		obj.String("spec", "serviceKind") == "LoadBalancer" {
		addresses := obj.Slice("status", "loadBalancer", "ingress")
		if len(addresses) == 0 {
			summary.State = "pending"
			summary.Transitioning = true
			summary.Message = append(summary.Message, "Load balancer is being provisioned")
		}
	}

	return summary
}

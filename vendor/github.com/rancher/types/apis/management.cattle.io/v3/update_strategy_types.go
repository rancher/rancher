package v3

import (
	appsv1 "k8s.io/api/apps/v1"
)

/* All fields in this file are copied over from apps/v1. Referencing k8s fields caused problems because of the "Type" field in
DeploymentStrategy and DaemonSetUpdateStrategy*/

// DeploymentStrategy describes how to replace existing pods with new ones.
type DeploymentStrategy struct {
	// Type of deployment. Can be "Recreate" or "RollingUpdate". Default is RollingUpdate.
	// +optional
	Strategy appsv1.DeploymentStrategyType `json:"strategy,omitempty" protobuf:"bytes,1,opt,name=type,casttype=DeploymentStrategyType"`

	// Rolling update config params. Present only if DeploymentStrategyType =
	// RollingUpdate.
	//---
	// TODO: Update this to follow our convention for oneOf, whatever we decide it
	// to be.
	// +optional
	RollingUpdate *appsv1.RollingUpdateDeployment `json:"rollingUpdate,omitempty" protobuf:"bytes,2,opt,name=rollingUpdate"`
}

// DaemonSetUpdateStrategy is a struct used to control the update strategy for a DaemonSet.
type DaemonSetUpdateStrategy struct {
	// Type of daemon set update. Can be "RollingUpdate" or "OnDelete". Default is RollingUpdate.
	// +optional
	Strategy appsv1.DaemonSetUpdateStrategyType `json:"strategy,omitempty" protobuf:"bytes,1,opt,name=type"`

	// Rolling update config params. Present only if type = "RollingUpdate".
	//---
	// TODO: Update this to follow our convention for oneOf, whatever we decide it
	// to be. Same as Deployment `strategy.rollingUpdate`.
	// See https://github.com/kubernetes/kubernetes/issues/35345
	// +optional
	RollingUpdate *appsv1.RollingUpdateDaemonSet `json:"rollingUpdate,omitempty" protobuf:"bytes,2,opt,name=rollingUpdate"`
}

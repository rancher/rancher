package pspdelete

import (
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v1beta12 "github.com/rancher/rancher/pkg/generated/norman/policy/v1beta1"
)

const (
	globalUnrestrictedAnnotation = "psp.rke2.io/global-unrestricted"
	globalRestrictedAnnotation   = "psp.rke2.io/global-restricted"
)

type handler struct {
	clusterName         string
	clusterCache        provisioningcontrollers.ClusterCache
	podSecurityPolicies v1beta12.PodSecurityPolicyInterface
}

func has(data map[string]string, key string) bool {
	_, ok := data[key]
	return ok
}

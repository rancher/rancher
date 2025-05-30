package psautils

import (
	"github.com/rancher/rancher/pkg/apis/management.cattle.io"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	rbacmgmt "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"
)

const (
	UpdatepsaVerb = "updatepsa"
)

// IsPSAAllowed get a list of roletemplates to verify if updatepsa verb is enabled on their rules
func IsPSAAllowed(rules []v1.PolicyRule) bool {
	auth := authorizer.AttributesRecord{
		Verb:            UpdatepsaVerb,
		APIGroup:        management.GroupName,
		Resource:        v3.ProjectResourceName,
		ResourceRequest: true,
	}
	if rbacmgmt.RulesAllow(auth, rules...) {
		return true
	}
	return false
}

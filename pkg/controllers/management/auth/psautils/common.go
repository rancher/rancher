package auth

import (
	"github.com/rancher/rancher/pkg/apis/management.cattle.io"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	rbacmgmt "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"
)

const (
	UpdatepsaVerb = "updatepsa"
)

// IsPSAAllowed get a list of roletemplates to verify if updatepsa verb is enabled on their rules
func IsPSAAllowed(roleTemplates []*v3.RoleTemplate) bool {
	isUpdatepsaAllowed := false
	auth := authorizer.AttributesRecord{
		Verb:            UpdatepsaVerb,
		APIGroup:        management.GroupName,
		Resource:        v3.ProjectResourceName,
		ResourceRequest: true,
	}
	for _, rt := range roleTemplates {
		if rbacmgmt.RulesAllow(auth, rt.Rules...) {
			isUpdatepsaAllowed = true
		}
	}
	return isUpdatepsaAllowed
}

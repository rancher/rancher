package rbac

import (
	"regexp"

	"k8s.io/apimachinery/pkg/runtime"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var crbNamePattern = regexp.MustCompile("clusterrolebinding-.+")
var namespaceRoleRefPattern = regexp.MustCompile(".+-namespaces-.+")
var promotedRoleRefPattern = regexp.MustCompile(".+-promoted")

const createNSRoleRef = "create-ns"

func newLegacyCRBCleaner(m *manager) *crbCleaner {
	return &crbCleaner{
		m: m,
	}
}

type crbCleaner struct {
	m *manager
}

func (p *crbCleaner) sync(key string, obj *rbacv1.ClusterRoleBinding) (runtime.Object, error) {
	if key == "" || obj == nil {
		return nil, nil
	}

	eligible, err := p.eligibleForDeletion(obj)
	if err != nil {
		return nil, err
	}

	if eligible {
		if err := p.m.workload.RBAC.ClusterRoleBindings("").Delete(obj.Name, &metav1.DeleteOptions{}); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		}
	}

	return obj, nil
}

func (p *crbCleaner) eligibleForDeletion(crb *rbacv1.ClusterRoleBinding) (bool, error) {
	if !crbNamePattern.MatchString(crb.Name) {
		return false, nil
	}

	if noOwners, err := p.m.noRemainingOwnerLabels(crb); !noOwners || err != nil {
		return false, err
	}

	roleRefName := crb.RoleRef.Name

	if roleRefName == createNSRoleRef {
		return true, nil
	}

	if namespaceRoleRefPattern.MatchString(roleRefName) {
		return true, nil
	}

	if promotedRoleRefPattern.MatchString(roleRefName) {
		return true, nil
	}

	return false, nil
}

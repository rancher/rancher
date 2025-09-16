package rbac

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	crbNamePattern          = regexp.MustCompile("clusterrolebinding-.+")
	namespaceRoleRefPattern = regexp.MustCompile(".+-namespaces-.+")
	promotedRoleRefPattern  = regexp.MustCompile(".+-promoted")
)

const (
	createNSRoleRef                     = "create-ns"
	crbAdminGlobalRoleMissingAnnotation = "authz.cluster.cattle.io/admin-globalrole-missing"
)

func newLegacyCRBCleaner(m *manager) *crbCleaner {
	return &crbCleaner{
		clusterName:            m.clusterName,
		noRemainingOwnerLabels: m.noRemainingOwnerLabels,
		crbs:                   m.workload.RBACw.ClusterRoleBinding(),
		grCache:                m.workload.Management.Wrangler.Mgmt.GlobalRole().Cache(),
		grbCache:               m.workload.Management.Wrangler.Mgmt.GlobalRoleBinding().Cache(),
	}
}

type crbCleaner struct {
	clusterName            string
	noRemainingOwnerLabels func(*rbacv1.ClusterRoleBinding) (bool, error)
	crbs                   wrbacv1.ClusterRoleBindingClient
	grCache                mgmtv3.GlobalRoleCache
	grbCache               mgmtv3.GlobalRoleBindingCache
}

func (c *crbCleaner) sync(key string, crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
	if key == "" || crb == nil {
		return nil, nil
	}

	eligible, err := c.eligibleForDeletion(crb)
	if err != nil {
		return nil, err
	}

	if eligible {
		if err := c.crbs.Delete(crb.Name, &metav1.DeleteOptions{}); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		}
		return crb, nil
	}

	// We may have a global admin ClusterRoleBindings that was left behind.
	if strings.HasPrefix(crb.Name, rbac.GlobalAdminCRBPrefix) &&
		crb.RoleRef.Name == rbac.ClusterAdminRoleName &&
		crb.Annotations[rbac.CrbAdminGlobalRoleCheckedAnnotation] != "true" {

		crb = crb.DeepCopy()
		if crb.Annotations == nil {
			crb.Annotations = map[string]string{}
		}

		if len(crb.Subjects) == 1 { // Rancher global admin CRBs have only one subject.
			ok, err := c.hasGlobalAdminRole(crb.Subjects[0])
			if err != nil {
				return nil, fmt.Errorf("checking if ClusterRoleBinding %s has global admin role: %w", crb.Name, err)
			}
			if !ok {
				logrus.Warnf("crbCleaner: %s: marking ClusterRoleBinding %s as orphaned", c.clusterName, key)
				crb.Annotations[crbAdminGlobalRoleMissingAnnotation] = "true"
			}
		}

		crb.Annotations[rbac.CrbAdminGlobalRoleCheckedAnnotation] = "true"
		if _, err := c.crbs.Update(crb); err != nil {
			return nil, fmt.Errorf("updating ClusterRoleBinding %s: %w", crb.Name, err)
		}
	}

	return crb, nil
}

// hasGlobalAdminRole checks if there is at least one GlobalRoleBinding
// that grants global admin role to the given subject.
func (c *crbCleaner) hasGlobalAdminRole(subj rbacv1.Subject) (bool, error) {
	if subj.Kind != rbacv1.UserKind && subj.Kind != rbacv1.GroupKind {
		return false, nil
	}

	grs, err := c.grCache.List(labels.Everything())
	if err != nil {
		return false, fmt.Errorf("listing global roles: %w", err)
	}

	var adminRoles []string
	for _, gr := range grs {
		if rbac.IsAdminGlobalRole(gr) {
			adminRoles = append(adminRoles, gr.Name)
		}
	}

	grbs, err := c.grbCache.List(labels.Everything())
	if err != nil {
		return false, fmt.Errorf("listing global role bindings: %w", err)
	}

	for _, grb := range grbs {
		if !slices.Contains(adminRoles, grb.GlobalRoleName) {
			continue
		}

		switch subj.Kind {
		case rbacv1.UserKind:
			if subj.Name == grb.UserName {
				return true, nil
			}
		case rbacv1.GroupKind:
			if subj.Name == grb.GroupPrincipalName {
				return true, nil
			}
		default:
			continue
		}
	}

	return false, nil
}

func (c *crbCleaner) eligibleForDeletion(crb *rbacv1.ClusterRoleBinding) (bool, error) {
	if !crbNamePattern.MatchString(crb.Name) {
		return false, nil
	}

	if noOwners, err := c.noRemainingOwnerLabels(crb); !noOwners || err != nil {
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

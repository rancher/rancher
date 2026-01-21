package roletemplates

import (
	"fmt"

	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/impersonation"
	"github.com/rancher/rancher/pkg/types/config"
	rbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	// Statuses
	reconcileClusterRoleBindings     = "ReconcileClusterRoleBindings"
	deleteClusterRoleBindings        = "DeleteClusterRoleBindings"
	ensureServiceAccountImpersonator = "EnsureServiceAccountImpersonator"
	deleteServiceAccountImpersonator = "DeleteServiceAccountImpersonator"
	// Reasons
	clusterRoleBindingExists          = "ClusterRoleBindingExists"
	clusterRoleBindingsDeleted        = "ClusterRoleBindingsDeleted"
	serviceAccountImpersonatorExists  = "ServiceAccountImpersonatorExists"
	failureToDeleteServiceAccount     = "FailureToDeleteServiceAccount"
	failureToBuildClusterRoleBinding  = "FailureToBuildClusterRoleBinding"
	failureToListClusterRoleBindings  = "FailureToListClusterRoleBindings"
	failureToDeleteClusterRoleBinding = "FailureToDeleteClusterRoleBinding"
	failureToCreateClusterRoleBinding = "FailureToCreateClusterRoleBinding"
	failureToGetRoleTemplate          = "FailureToGetRoleTemplate"
)

type impersonationHandler struct {
	clusterName  string
	impersonator config.Impersonator
	crtbCache    mgmtv3.ClusterRoleTemplateBindingCache
	prtbCache    mgmtv3.ProjectRoleTemplateBindingCache
	crClient     rbacv1.ClusterRoleController
}

// ensureServiceAccountImpersonator ensures a Service Account Impersonator exists for a given user. If not it creates one.
func (ih *impersonationHandler) ensureServiceAccountImpersonator(username string) error {
	logrus.Debugf("ensuring service account impersonator for %s", username)
	err := ih.impersonator.SetUpImpersonation(&user.DefaultInfo{UID: username})
	if apierrors.IsNotFound(err) {
		logrus.Warnf("could not find user %s, will not create impersonation account on cluster", username)
		return nil
	}
	return err
}

// deleteServiceAccountImpersonator checks if there are any CRTBs or PRTBs for this user. If there are none, remove their Service Account Impersonator.
// Currently uses custom indexers to get CRTBs and PRTBs. Once Rancher's minimum support is >1.31,
// the indexers can be replaced by crd selectable fields https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#crd-selectable-fields
func (ih *impersonationHandler) deleteServiceAccountImpersonator(username string) error {
	indexKey := name.SafeConcatName(ih.clusterName, username)
	crtbs, err := ih.crtbCache.GetByIndex(crtbByUsernameIndex, indexKey)
	if err != nil {
		return err
	}
	prtbs, err := ih.prtbCache.GetByIndex(prtbByUsernameIndex, indexKey)
	if err != nil {
		return err
	}
	if len(crtbs)+len(prtbs) > 0 {
		return nil
	}
	roleName := impersonation.ImpersonationPrefix + username
	logrus.Debugf("deleting service account impersonator for %s", username)
	err = ih.crClient.Delete(roleName, &metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// isRoleTemplateExternal returns the value of RoleTemplate.External
func isRoleTemplateExternal(rtName string, rtClient mgmtv3.RoleTemplateController) (bool, error) {
	rt, err := rtClient.Get(rtName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if rt == nil {
		return false, fmt.Errorf("roletemplate %s is nil", rtName)
	}
	return rt.External, nil
}

package roletemplates

import (
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/impersonation"
	"github.com/rancher/rancher/pkg/types/config"
	rbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	aggregatorSuffix = "aggregator"
)

// createOrUpdateResource creates or updates the given resource
//   - getResource is a func that returns a single object and an error
//   - obj is the resource to create or update
//   - client is the Wrangler client to use to get/create/update resource
//   - areResourcesTheSame is a func that compares two resources and returns (true, nil) if they are equal, and (false, T) when not the same where T is an updated resource
func createOrUpdateResource[T generic.RuntimeMetaObject, TList runtime.Object](obj T, client generic.NonNamespacedClientInterface[T, TList], areResourcesTheSame func(T, T) (bool, T)) error {
	// attempt to get the resource
	resource, err := client.Get(obj.GetName(), v1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil
		}
		// resource doesn't exist, create it
		_, err = client.Create(obj)
		return err
	}

	// check that the existing resource is the same as the one we want
	if same, updatedResource := areResourcesTheSame(resource, obj); !same {
		// if it has changed, update it to the correct version
		_, err := client.Update(updatedResource)
		if err != nil {
			return err
		}
	}
	return nil
}

type impersonationHandler struct {
	userContext *config.UserContext
	crtbClient  mgmtv3.ClusterRoleTemplateBindingController
	prtbClient  mgmtv3.ProjectRoleTemplateBindingController
	crClient    rbacv1.ClusterRoleController
}

// ensureServiceAccountImpersonator ensures a Service Account Impersonator exists for a given user. If not it creates one.
func (ih *impersonationHandler) ensureServiceAccountImpersonator(username string) error {
	logrus.Debugf("ensuring service account impersonator for %s", username)
	i, err := impersonation.New(&user.DefaultInfo{UID: username}, ih.userContext)
	if apierrors.IsNotFound(err) {
		logrus.Warnf("could not find user %s, will not create impersonation account on cluster", username)
		return nil
	}
	if err != nil {
		return err
	}
	_, err = i.SetUpImpersonation()
	return err
}

// deleteServiceAccountImpersonator checks if there are any CRBTs or PRTBs for this user. If there are none, remove their Service Account Impersonator.
func (ih *impersonationHandler) deleteServiceAccountImpersonator(username string) error {
	lo := v1.ListOptions{FieldSelector: "userName=" + username}
	crtbs, err := ih.crtbClient.List(ih.userContext.ClusterName, lo)
	if err != nil {
		return err
	}
	prtbs, err := ih.prtbClient.List(ih.userContext.ClusterName, lo)
	if err != nil {
		return err
	}
	if len(crtbs.Items)+len(prtbs.Items) > 0 {
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

// clusterRoleNameFor returns safe version of a string to be used for a clusterRoleName
func clusterRoleNameFor(s string) string {
	return name.SafeConcatName(s)
}

// promotedClusterRoleNameFor appends the promoted suffix to a string safely (ie <= 63 characters)
func promotedClusterRoleNameFor(s string) string {
	return name.SafeConcatName(s + promotedSuffix)
}

// addAggregatorSuffix appends the aggregation suffix to a string safely (ie <= 63 characters)
func aggregatedClusterRoleNameFor(s string) string {
	return name.SafeConcatName(s + aggregatorSuffix)
}

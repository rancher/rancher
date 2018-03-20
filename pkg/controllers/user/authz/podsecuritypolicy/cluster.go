package podsecuritypolicy

import (
	"fmt"

	v12 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

type clusterManager struct {
	// remove unneeded members for clarity

	templateLister            v3.PodSecurityPolicyTemplateLister
	policyLister              v1beta1.PodSecurityPolicyLister
	policies                  v1beta1.PodSecurityPolicyInterface
	serviceAccountLister      v12.ServiceAccountLister
	serviceAccountsController v12.ServiceAccountController
}

// RegisterCluster updates the pod security policy if the pod security policy template default for this cluster has been
// updated, then resyncs all service accounts in this namespace.
func RegisterCluster(context *config.UserContext) {
	logrus.Infof("registering podsecuritypolicy cluster handler for cluster %v", context.ClusterName)

	m := &clusterManager{
		policies: context.Extensions.PodSecurityPolicies(""),

		templateLister:            context.Management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
		policyLister:              context.Extensions.PodSecurityPolicies("").Controller().Lister(),
		serviceAccountLister:      context.Core.ServiceAccounts("").Controller().Lister(),
		serviceAccountsController: context.Core.ServiceAccounts("").Controller(),
	}

	context.Management.Management.Clusters("").AddHandler("ClusterSyncHandler", m.sync)
}

// BUG: this handler will get events for ALL clusters, not just the current user cluster. You need to do a check on
// obj.Name == context.ClusterName (context being config.UserContext from register function
func (m *clusterManager) sync(key string, obj *v3.Cluster) error {
	if obj == nil {
		// Nothing to do
		return nil
	}

	id := obj.Spec.DefaultPodSecurityPolicyTemplateName

	if id == "" {
		logrus.Debugf("No pod security policy template found for cluster %v", obj.Name)
		return nil
	}

	policies, err := m.policyLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("error getting policy list: %v", err)
	}

	// I dont see a need for this logic. policies shoudl be updated via the template handler, not this one
	for _, policy := range policies {
		if policy.Annotations[podSecurityTemplateParentAnnotation] == id {
			err := updatePolicyIfOutdated(m.templateLister, m.policies, m.policyLister, id, policy.Name)
			if err != nil {
				return err
			}
		}
	}

	// will get expensive to do this every time this cluster handler runs. We need to move to a spec & status model
	// for this field so that we can tell when its changed
	return resyncServiceAccounts(m.serviceAccountLister, m.serviceAccountsController, "")
}

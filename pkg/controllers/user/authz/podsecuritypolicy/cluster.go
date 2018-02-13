package podsecuritypolicy

import (
	"github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

type clusterManager struct {
	clusterLister  v3.ClusterLister
	clusters       v3.ClusterInterface
	templateLister v3.PodSecurityPolicyTemplateLister
	policyLister   v1beta1.PodSecurityPolicyLister
	policies       v1beta1.PodSecurityPolicyInterface
}

func RegisterCluster(context *config.UserContext) {
	m := &clusterManager{
		clusters: context.Management.Management.Clusters(""),
		policies: context.Extensions.PodSecurityPolicies(""),

		clusterLister:  context.Management.Management.Clusters("").Controller().Lister(),
		templateLister: context.Management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
		policyLister:   context.Extensions.PodSecurityPolicies("").Controller().Lister(),
	}

	context.Management.Management.Clusters("").AddHandler("ClusterSyncHandler", m.sync)
}

func (m *clusterManager) sync(key string, obj *v3.Cluster) error {
	// check if default template still matches children projects
	id := obj.Spec.DefaultPodSecurityPolicyTemplateName

	if id == "" {
		logrus.Debugf("No PSPTs found for cluster %v", obj.Name)
		return nil
	}

	// if not then update
	template, err := m.templateLister.Get("", id)
	if err != nil {
		return err
	}

	policy, err := m.policyLister.Get("", id)
	if err != nil {
		return err
	}

	if policy.Annotations[podSecurityVersionAnnotation] != template.ResourceVersion {
		_, err = FromTemplate(m.policies, m.policyLister, policy.Name, template)
		if err != nil {
			return err
		}
	}

	return nil
}

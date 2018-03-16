package podsecuritypolicy

import (
	"fmt"

	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	v1beta13 "k8s.io/api/extensions/v1beta1"
)

// RegisterProject updates the pod security policy for this project if it has been changed.  Also resync service
// accounts so they pick up the change.  If no policy exists then exits without doing anything.
func RegisterProject(context *config.UserContext) {
	logrus.Infof("registering podsecuritypolicy project handler for cluster %v", context.ClusterName)

	m := &projectManager{
		policyLister: context.Extensions.PodSecurityPolicies("").Controller().Lister(),
		policies:     context.Extensions.PodSecurityPolicies(""),
		templateLister: context.Management.Management.PodSecurityPolicyTemplates("").Controller().
			Lister(),
		projectLister: context.Management.Management.Projects("").Controller().Lister(),
		psptpbLister: context.Management.Management.PodSecurityPolicyTemplateProjectBindings("").
			Controller().Lister(),
		psptpb:               context.Management.Management.PodSecurityPolicyTemplateProjectBindings(""),
		clusterLister:        context.Management.Management.Clusters("").Controller().Lister(),
		serviceAccountLister: context.Core.ServiceAccounts("").Controller().Lister(),
		serviceAccounts:      context.Core.ServiceAccounts("").Controller(),
	}

	context.Management.Management.PodSecurityPolicyTemplateProjectBindings("").
		AddHandler("PodSecurityPolicyTemplateProjectBindingsSyncHandler", m.sync)
}

type projectManager struct {
	clusterLister        v3.ClusterLister
	projectLister        v3.ProjectLister
	policyLister         v1beta1.PodSecurityPolicyLister
	policies             v1beta1.PodSecurityPolicyInterface
	psptpb               v3.PodSecurityPolicyTemplateProjectBindingInterface
	psptpbLister         v3.PodSecurityPolicyTemplateProjectBindingLister
	templateLister       v3.PodSecurityPolicyTemplateLister
	serviceAccountLister v1.ServiceAccountLister
	serviceAccounts      v1.ServiceAccountController
}

func (m *projectManager) sync(key string, obj *v3.PodSecurityPolicyTemplateProjectBinding) error {
	if obj == nil {
		return nil
	}

	template, err := m.templateLister.Get("", obj.PodSecurityPolicyTemplateID)
	if err != nil {
		return fmt.Errorf("error getting pod security policy template: %v", err)
	}

	var policy *v1beta13.PodSecurityPolicy

	if !doesPolicyExist(m.policyLister, keyToPolicyName(key)) {
		policy, err = fromTemplate(m.policies, m.policyLister, key, template)
		if err != nil {
			return err
		}
	} else {
		policy, err = m.policyLister.Get("", keyToPolicyName(key))
		if err != nil {
			return fmt.Errorf("error getting pod security policy: %v", err)
		}
	}

	if template.ResourceVersion != policy.Annotations[podSecurityVersionAnnotation] {
		_, err := fromTemplate(m.policies, m.policyLister, key, template)
		if err != nil {
			return err
		}
	}

	return resyncServiceAccounts(m.serviceAccountLister, m.serviceAccounts, "")
}

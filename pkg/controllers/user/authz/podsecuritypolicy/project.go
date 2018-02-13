package podsecuritypolicy

import (
	"fmt"
	"strings"

	"github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	v1beta13 "k8s.io/api/extensions/v1beta1"
)

func RegisterProject(context *config.UserContext) {
	m := &projectManager{
		policyLister:   context.Extensions.PodSecurityPolicies("").Controller().Lister(),
		policies:       context.Extensions.PodSecurityPolicies(""),
		templateLister: context.Management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
		projectLister:  context.Management.Management.Projects("").Controller().Lister(),
		clusterLister:  context.Management.Management.Clusters("").Controller().Lister(),
	}

	context.Management.Management.Projects("").AddHandler("ProjectSyncHandler", m.sync)
}

type projectManager struct {
	clusterLister  v3.ClusterLister
	projectLister  v3.ProjectLister
	policyLister   v1beta1.PodSecurityPolicyLister
	policies       v1beta1.PodSecurityPolicyInterface
	templateLister v3.PodSecurityPolicyTemplateLister
}

func (m *projectManager) sync(key string, obj *v3.Project) error {
	split := strings.Split(key, "/")

	if len(split) != 2 {
		return fmt.Errorf("could not parse project id annotation: %v", key)
	}

	clusterName, projectID := split[0], split[1]

	podSecurityPolicyTemplateID, err := GetPodSecurityPolicyTemplateID(m.projectLister, m.clusterLister, projectID,
		clusterName)
	if err != nil {
		return err
	}

	if podSecurityPolicyTemplateID == "" {
		logrus.Debugf("no pod security policy template is assigned to %v", key)
		return nil
	}

	template, err := m.templateLister.Get("", podSecurityPolicyTemplateID)
	if err != nil {
		return fmt.Errorf("error getting pod security policy template: %v", err)
	}

	var policy *v1beta13.PodSecurityPolicy

	if !DoesPolicyExist(m.policyLister, KeyToPolicyName(key)) {
		policy, err = FromTemplate(m.policies, m.policyLister, key, template)

		return err
	}

	policy, err = m.policyLister.Get("", KeyToPolicyName(key))
	if err != nil {
		return fmt.Errorf("error getting pod security policy: %v", err)
	}

	if template.ResourceVersion != policy.Annotations[podSecurityVersionAnnotation] {
		_, err := FromTemplate(m.policies, m.policyLister, key, template)

		if err != nil {
			return err
		}
	}

	return nil
}

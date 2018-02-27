package podsecuritypolicy

import (
	"fmt"
	"strings"

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
		projectLister:        context.Management.Management.Projects("").Controller().Lister(),
		clusterLister:        context.Management.Management.Clusters("").Controller().Lister(),
		serviceAccountLister: context.Core.ServiceAccounts("").Controller().Lister(),
		serviceAccounts:      context.Core.ServiceAccounts("").Controller(),
	}

	context.Management.Management.Projects("").AddHandler("ProjectSyncHandler", m.sync)
}

type projectManager struct {
	clusterLister        v3.ClusterLister
	projectLister        v3.ProjectLister
	policyLister         v1beta1.PodSecurityPolicyLister
	policies             v1beta1.PodSecurityPolicyInterface
	templateLister       v3.PodSecurityPolicyTemplateLister
	serviceAccountLister v1.ServiceAccountLister
	serviceAccounts      v1.ServiceAccountController
}

func (m *projectManager) sync(key string, obj *v3.Project) error {
	if obj == nil {
		return nil
	}

	split := strings.Split(key, "/")

	if len(split) != 2 {
		return fmt.Errorf("could not parse project id annotation: %v", key)
	}

	clusterName, projectID := split[0], split[1]

	podSecurityPolicyTemplateID, err := getPodSecurityPolicyTemplateID(m.projectLister, m.clusterLister, projectID,
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

	if !doesPolicyExist(m.policyLister, keyToPolicyName(key)) {
		policy, err = fromTemplate(m.policies, m.policyLister, key, template)

		if err != nil {
			return err
		}
	}

	policy, err = m.policyLister.Get("", keyToPolicyName(key))
	if err != nil {
		return fmt.Errorf("error getting pod security policy: %v", err)
	}

	if template.ResourceVersion != policy.Annotations[podSecurityVersionAnnotation] {
		_, err := fromTemplate(m.policies, m.policyLister, key, template)

		if err != nil {
			return err
		}
	}

	return resyncServiceAccounts(m.serviceAccountLister, m.serviceAccounts, obj.Namespace)
}

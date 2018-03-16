package podsecuritypolicy

import (
	"fmt"

	"strings"

	v1beta12 "github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// RegisterTemplate propagates updates to pod security policy templates to their associated pod security policies.
// Ignores pod security policy templates not assigned to a cluster or project.
func RegisterTemplate(context *config.UserContext) {
	logrus.Infof("registering podsecuritypolicy template handler for cluster %v", context.ClusterName)

	m := &templateManager{
		policies:     context.Extensions.PodSecurityPolicies(""),
		policyLister: context.Extensions.PodSecurityPolicies("").Controller().Lister(),
	}

	context.Management.Management.PodSecurityPolicyTemplates("").AddHandler(
		"PodSecurityPolicyTemplateSyncHandler", m.sync)
}

type templateManager struct {
	policies     v1beta12.PodSecurityPolicyInterface
	policyLister v1beta12.PodSecurityPolicyLister
}

func (m *templateManager) sync(key string, obj *v3.PodSecurityPolicyTemplate) error {
	policies, err := m.policyLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("error getting policies: %v", err)
	}

	var childPolicies []*v1beta1.PodSecurityPolicy

	for _, candidate := range policies {
		if candidate.Annotations[podSecurityTemplateParentAnnotation] == obj.Name {
			childPolicies = append(childPolicies, candidate)
		}
	}

	if len(policies) == 0 {
		// this pspt is not used so return immediately
		return nil
	}

	for _, policy := range childPolicies {
		if policy.Annotations[podSecurityVersionAnnotation] != obj.ResourceVersion {
			_, err := fromTemplateExplicitName(m.policies, m.policyLister, policy.Name, obj, policy.Name)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
func fromTemplate(policies v1beta12.PodSecurityPolicyInterface, policyLister v1beta12.PodSecurityPolicyLister,
	key string, originalTemplate *v3.PodSecurityPolicyTemplate) (*v1beta1.PodSecurityPolicy, error) {
	return fromTemplateExplicitName(policies, policyLister, keyToPolicyName(key), originalTemplate, key)
}

func fromTemplateExplicitName(policies v1beta12.PodSecurityPolicyInterface,
	policyLister v1beta12.PodSecurityPolicyLister, key string,
	originalTemplate *v3.PodSecurityPolicyTemplate, originalKey string) (*v1beta1.PodSecurityPolicy, error) {
	template := originalTemplate.DeepCopy()

	objectMeta := v1.ObjectMeta{}
	objectMeta.Name = key
	objectMeta.Annotations = make(map[string]string)
	objectMeta.Annotations[podSecurityTemplateParentAnnotation] = template.Name
	objectMeta.Annotations[podSecurityVersionAnnotation] = template.ResourceVersion
	objectMeta.Annotations[podSecurityPolicyTemplateKey] = originalKey

	psp := &v1beta1.PodSecurityPolicy{
		TypeMeta: v1.TypeMeta{
			Kind:       podSecurityPolicy,
			APIVersion: apiVersion,
		},
		ObjectMeta: objectMeta,
		Spec:       template.Spec,
	}

	var policy *v1beta1.PodSecurityPolicy
	var err error

	if !doesPolicyExist(policyLister, key) {
		policy, err = policies.Create(psp)
	} else {
		policy, err = policies.Update(psp)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating pod security policy: %v", err)
	}

	logrus.Debugf("created/updated a pod security policy with name %v", objectMeta.Name)

	return policy, nil
}

func doesPolicyExist(policyLister v1beta12.PodSecurityPolicyLister, name string) bool {
	_, err := policyLister.Get("", name)

	return !errors.IsNotFound(err)
}

func getPodSecurityPolicyTemplateID(psptcbLister v3.PodSecurityPolicyTemplateProjectBindingLister, clusterLister v3.ClusterLister, projectID string,
	clusterName string) (string, error) {
	psptpbs, err := psptcbLister.List("", labels.Everything())
	if err != nil {
		return "", fmt.Errorf("error getting projects: %v", err)
	}

	var psptpb *v3.PodSecurityPolicyTemplateProjectBinding

	for _, candidate := range psptpbs {
		candidateProjectID := strings.Split(candidate.ProjectID, ":")[1]
		if candidateProjectID == projectID {
			psptpb = candidate
			break
		}
	}

	var podSecurityPolicyTemplateID string
	if psptpb != nil {
		podSecurityPolicyTemplateID = psptpb.PodSecurityPolicyTemplateID
	}

	if podSecurityPolicyTemplateID == "" {
		// check cluster
		cluster, err := clusterLister.Get("", clusterName)
		if err != nil {
			return "", fmt.Errorf("error getting clusters: %v", err)
		}

		podSecurityPolicyTemplateID = cluster.Spec.DefaultPodSecurityPolicyTemplateName

		if podSecurityPolicyTemplateID == "" {
			logrus.Debugf("No pod security policy templates found for project %v and cluster %v", projectID,
				clusterName)
			return "", nil
		}
	}

	return podSecurityPolicyTemplateID, nil
}

func keyToPolicyName(key string) string {
	return fmt.Sprintf("%v-psp", strings.Replace(key, "/", "-", -1))
}

func updatePolicyIfOutdated(templateLister v3.PodSecurityPolicyTemplateLister,
	policies v1beta12.PodSecurityPolicyInterface, policyLister v1beta12.PodSecurityPolicyLister, templateID string, policyID string) error {
	template, err := templateLister.Get("", templateID)
	if err != nil {
		return err
	}

	policy, err := policyLister.Get("", policyID)
	if err != nil {
		return err
	}

	if policy.Annotations[podSecurityVersionAnnotation] != template.ResourceVersion {
		_, err = fromTemplate(policies, policyLister, policy.Name, template)
		if err != nil {
			return err
		}
	}

	return nil
}

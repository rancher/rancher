package podsecuritypolicy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	v12 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/policy/v1beta1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v1beta13 "k8s.io/api/policy/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type clusterManager struct {
	clusterName               string
	templateLister            v3.PodSecurityPolicyTemplateLister
	policyLister              v1beta1.PodSecurityPolicyLister
	policies                  v1beta1.PodSecurityPolicyInterface
	serviceAccountLister      v12.ServiceAccountLister
	serviceAccountsController v12.ServiceAccountController
	clusterRoleLister         v1.ClusterRoleLister
	clusterRoles              v1.ClusterRoleInterface
	clusterLister             v3.ClusterLister
	clusters                  v3.ClusterInterface
}

// RegisterCluster updates the pod security policy if the pod security policy template default for this cluster has been
// updated, then resyncs all service accounts in this namespace.
func RegisterCluster(ctx context.Context, context *config.UserContext) {
	logrus.Infof("registering podsecuritypolicy cluster handler for cluster %v", context.ClusterName)

	m := &clusterManager{
		clusterName:               context.ClusterName,
		policies:                  context.Policy.PodSecurityPolicies(""),
		clusters:                  context.Management.Management.Clusters(""),
		clusterLister:             context.Management.Management.Clusters("").Controller().Lister(),
		templateLister:            context.Management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
		policyLister:              context.Policy.PodSecurityPolicies("").Controller().Lister(),
		clusterRoleLister:         context.RBAC.ClusterRoles("").Controller().Lister(),
		clusterRoles:              context.RBAC.ClusterRoles(""),
		serviceAccountLister:      context.Core.ServiceAccounts("").Controller().Lister(),
		serviceAccountsController: context.Core.ServiceAccounts("").Controller(),
	}

	context.Management.Management.Clusters("").AddHandler(ctx, "ClusterSyncHandler", m.sync)
}

func (m *clusterManager) sync(key string, obj *v3.Cluster) (runtime.Object, error) {
	if obj == nil ||
		m.clusterName != obj.Name ||
		obj.Spec.DefaultPodSecurityPolicyTemplateName == obj.Status.AppliedPodSecurityPolicyTemplateName {
		// Nothing to do
		return nil, nil
	}

	err := CheckClusterVersion(m.clusterName, m.clusterLister)
	if err != nil {
		if errors.Is(err, ErrClusterVersionIncompatible) {
			if obj.Status.AppliedPodSecurityPolicyTemplateName != "" {
				obj = obj.DeepCopy()
				obj.Status.AppliedPodSecurityPolicyTemplateName = ""
				obj, err = m.clusters.Update(obj)
				if err != nil {
					return nil, fmt.Errorf("error updating cluster for dropping the applied pspt: %v", err)
				}
			}
			return obj, nil
		}
		return obj, fmt.Errorf("error checking cluster version for Cluster controller: %w", err)
	}

	if obj.Spec.DefaultPodSecurityPolicyTemplateName != "" {
		podSecurityPolicyName := fmt.Sprintf("%v-psp", obj.Spec.DefaultPodSecurityPolicyTemplateName)

		_, err := m.policyLister.Get("", podSecurityPolicyName)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				template, err := m.templateLister.Get("", obj.Spec.DefaultPodSecurityPolicyTemplateName)
				if err != nil {
					return nil, fmt.Errorf("error getting pspt: %v", err)
				}

				objectMeta := metav1.ObjectMeta{}
				objectMeta.Name = podSecurityPolicyName
				objectMeta.Annotations = make(map[string]string)
				objectMeta.Annotations[podSecurityPolicyTemplateParentAnnotation] = template.Name
				objectMeta.Annotations[podSecurityPolicyTemplateVersionAnnotation] = template.ResourceVersion

				// Setting annotations that doesn't contains podSecurityPolicyTemplateFilterAnnotation
				for k, v := range template.Annotations {
					if !strings.Contains(k, podSecurityPolicyTemplateFilterAnnotation) {
						objectMeta.Annotations[k] = v
					}
				}

				psp := &v1beta13.PodSecurityPolicy{
					TypeMeta: metav1.TypeMeta{
						Kind:       podSecurityPolicy,
						APIVersion: apiVersion,
					},
					ObjectMeta: objectMeta,
					Spec:       template.Spec,
				}

				_, err = m.policies.Create(psp)
				if err != nil {
					return nil, fmt.Errorf("error creating psp: %v", err)
				}
			} else {
				return nil, fmt.Errorf("error getting policy: %v", err)
			}
		}

		clusterRoleName := fmt.Sprintf("%v-clusterrole", obj.Spec.DefaultPodSecurityPolicyTemplateName)
		_, err = m.clusterRoleLister.Get("", clusterRoleName)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				newRole := &rbac.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
						Name:        clusterRoleName,
					},
					TypeMeta: metav1.TypeMeta{
						Kind: "ClusterRole",
					},
					Rules: []rbac.PolicyRule{
						{
							APIGroups:     []string{"extensions"},
							Resources:     []string{"podsecuritypolicies"},
							Verbs:         []string{"use"},
							ResourceNames: []string{podSecurityPolicyName},
						},
					},
				}
				newRole.Annotations[podSecurityPolicyTemplateParentAnnotation] = obj.Spec.DefaultPodSecurityPolicyTemplateName

				_, err := m.clusterRoles.Create(newRole)
				if err != nil {
					return nil, fmt.Errorf("error creating cluster role: %v", err)
				}
			} else {
				return nil, fmt.Errorf("error getting cluster role: %v", err)
			}
		}

		obj = obj.DeepCopy()
		obj.Status.AppliedPodSecurityPolicyTemplateName = obj.Spec.DefaultPodSecurityPolicyTemplateName
		_, err = m.clusters.Update(obj)
		if err != nil {
			return nil, fmt.Errorf("error updating cluster with the applied pspt: %v", err)
		}

		return nil, resyncServiceAccounts(m.serviceAccountLister, m.serviceAccountsController, "")
	}

	return nil, nil
}

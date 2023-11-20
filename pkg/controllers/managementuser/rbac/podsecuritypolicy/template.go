package podsecuritypolicy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1beta12 "github.com/rancher/rancher/pkg/generated/norman/policy/v1beta1"
	v12 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const policyByPSPTParentAnnotationIndex = "podsecuritypolicy.rbac.user.cattle.io/parent-annotation"
const clusterRoleByPSPTNameIndex = "podsecuritypolicy.rbac.user.cattle.io/pspt-name"

// RegisterTemplate propagates updates to pod security policy templates to their associated pod security policies.
// Ignores pod security policy templates not assigned to a cluster or project.
func RegisterTemplate(ctx context.Context, context *config.UserContext) {
	logrus.Infof("registering podsecuritypolicy template handler for cluster %v", context.ClusterName)

	policyInformer := context.Policy.PodSecurityPolicies("").Controller().Informer()
	policyIndexers := map[string]cache.IndexFunc{
		policyByPSPTParentAnnotationIndex: policyByPSPTParentAnnotation,
	}
	policyInformer.AddIndexers(policyIndexers)

	clusterRoleInformer := context.RBAC.ClusterRoles("").Controller().Informer()
	clusterRoleIndexer := map[string]cache.IndexFunc{
		clusterRoleByPSPTNameIndex: clusterRoleByPSPTName,
	}
	clusterRoleInformer.AddIndexers(clusterRoleIndexer)

	handler := &psptHandler{
		policies:                   context.Policy.PodSecurityPolicies(""),
		podSecurityPolicyTemplates: context.Management.Management.PodSecurityPolicyTemplates(""),
		clusterLister:              context.Management.Management.Clusters("").Controller().Lister(),
		clusterRoles:               context.RBAC.ClusterRoles(""),
		clusterName:                context.ClusterName,

		policyIndexer:      policyInformer.GetIndexer(),
		clusterRoleIndexer: clusterRoleInformer.GetIndexer(),
	}

	context.Management.Management.PodSecurityPolicyTemplates("").AddHandler(ctx, "pspt-sync", handler.sync)
}

type psptHandler struct {
	policies                   v1beta12.PodSecurityPolicyInterface
	podSecurityPolicyTemplates v3.PodSecurityPolicyTemplateInterface
	clusterRoles               v12.ClusterRoleInterface

	policyIndexer      cache.Indexer
	clusterRoleIndexer cache.Indexer
	clusterLister      v3.ClusterLister
	clusterName        string
}

func (p *psptHandler) sync(key string, obj *v3.PodSecurityPolicyTemplate) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}

	err := CheckClusterVersion(p.clusterName, p.clusterLister)
	if err != nil {
		if errors.Is(err, ErrClusterVersionIncompatible) {
			return obj, nil
		}
		return obj, fmt.Errorf("error checking cluster version for PodSecurityPolicyTemplate controller: %w", err)
	}

	if _, ok := obj.Annotations[podSecurityPolicyTemplateUpgrade]; !ok {
		return p.cleanPSPT(obj)
	}

	if obj.DeletionTimestamp != nil {
		return p.remove(obj)
	}

	policies, err := p.policyIndexer.ByIndex(policyByPSPTParentAnnotationIndex, obj.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting policies: %v", err)
	}

	for _, rawPolicy := range policies {
		policy := rawPolicy.(*policyv1beta1.PodSecurityPolicy)

		if policy.Annotations[podSecurityPolicyTemplateVersionAnnotation] != obj.ResourceVersion {
			newPolicy := policy.DeepCopy()
			newPolicy.Spec = obj.Spec
			newPolicy.Annotations[podSecurityPolicyTemplateVersionAnnotation] = obj.ResourceVersion

			// Setting annotations that doesn't contains podSecurityPolicyTemplateFilterAnnotation
			for k, v := range obj.Annotations {
				if !strings.Contains(k, podSecurityPolicyTemplateFilterAnnotation) {
					newPolicy.Annotations[k] = v
				}
			}

			_, err = p.policies.Update(newPolicy)
			if err != nil {
				return nil, fmt.Errorf("error updating psp: %v", err)
			}
		}
	}

	return obj, nil
}

func (p *psptHandler) remove(obj *v3.PodSecurityPolicyTemplate) (runtime.Object, error) {
	policies, err := p.policyIndexer.ByIndex(policyByPSPTParentAnnotationIndex, obj.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting policies: %v", err)
	}

	for _, rawPolicy := range policies {
		policy := rawPolicy.(*policyv1beta1.PodSecurityPolicy)
		err = p.policies.Delete(policy.Name, &v1.DeleteOptions{})
		if err != nil {
			return nil, fmt.Errorf("error deleting policy: %v", err)
		}
	}

	clusterRoles, err := p.clusterRoleIndexer.ByIndex(clusterRoleByPSPTNameIndex, obj.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster roles: %v", err)
	}

	for _, rawClusterRole := range clusterRoles {
		clusterRole := rawClusterRole.(*rbac.ClusterRole)
		err = p.clusterRoles.DeleteNamespaced(clusterRole.Namespace, clusterRole.Name, &v1.DeleteOptions{})
		if err != nil {
			return nil, fmt.Errorf("error deleting cluster role: %v", err)
		}
	}

	return obj, nil
}

func (p *psptHandler) cleanPSPT(obj *v3.PodSecurityPolicyTemplate) (runtime.Object, error) {
	var newFinalizers []string
	for _, finalizer := range obj.Finalizers {
		if strings.HasPrefix(finalizer, "clusterscoped.controller.cattle.io/cluster-pspt-sync_") {
			continue
		}
		newFinalizers = append(newFinalizers, finalizer)
	}

	newAnnos := make(map[string]string)
	for k, v := range obj.Annotations {
		if strings.HasPrefix(k, "lifecycle.cattle.io/create.cluster-pspt-sync_") {
			continue
		}
		newAnnos[k] = v
	}

	obj.Finalizers = newFinalizers
	obj.Annotations = newAnnos
	obj.Annotations[podSecurityPolicyTemplateUpgrade] = "true"
	return p.podSecurityPolicyTemplates.Update(obj)
}

func policyByPSPTParentAnnotation(obj interface{}) ([]string, error) {
	policy, ok := obj.(*policyv1beta1.PodSecurityPolicy)
	if !ok || policy.Annotations[podSecurityPolicyTemplateParentAnnotation] == "" {
		return []string{}, nil
	}

	return []string{policy.Annotations[podSecurityPolicyTemplateParentAnnotation]}, nil
}

func clusterRoleByPSPTName(obj interface{}) ([]string, error) {
	clusterRole, ok := obj.(*rbac.ClusterRole)
	if !ok || clusterRole.Annotations[podSecurityPolicyTemplateParentAnnotation] == "" {
		return []string{}, nil
	}

	return []string{clusterRole.Annotations[podSecurityPolicyTemplateParentAnnotation]}, nil
}

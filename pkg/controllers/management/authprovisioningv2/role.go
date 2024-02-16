package authprovisioningv2

import (
	"fmt"
	"sort"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	apiextcontrollers "github.com/rancher/wrangler/v2/pkg/generated/controllers/apiextensions.k8s.io/v1"
	"github.com/rancher/wrangler/v2/pkg/generic"
	"github.com/rancher/wrangler/v2/pkg/name"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	clusterIndexed        = "clusterIndexed"
	clusterIndexedLabel   = "auth.cattle.io/cluster-indexed"
	clusterNameLabel      = "cluster.cattle.io/name"
	clusterNamespaceLabel = "cluster.cattle.io/namespace"
	reenqueueTime         = time.Second * 5
)

func (h *handler) initializeCRDs(crdClient apiextcontrollers.CustomResourceDefinitionClient) error {
	crds, err := crdClient.List(metav1.ListOptions{
		LabelSelector: clusterIndexedLabel + "=true",
	})
	if err != nil {
		return err
	}

	for _, crd := range crds.Items {
		match := crdToResourceMatch(&crd)
		if match == nil {
			continue
		}
		h.modifyResources(*match, true)
	}

	return nil
}

func crdToResourceMatch(crd *apiextv1.CustomResourceDefinition) *resourceMatch {
	if crd.Status.AcceptedNames.Kind == "" || len(crd.Spec.Versions) == 0 {
		return nil
	}

	gvk := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: crd.Spec.Versions[0].Name,
		Kind:    crd.Status.AcceptedNames.Kind,
	}

	return &resourceMatch{
		GVK:      gvk,
		Resource: crd.Status.AcceptedNames.Plural,
	}
}

func (h *handler) gvkMatcher(gvk schema.GroupVersionKind) bool {
	h.resourcesLock.RLock()
	defer h.resourcesLock.RUnlock()
	_, ok := h.resources[gvk]
	return ok
}

func (h *handler) modifyResources(resource resourceMatch, addResource bool) {
	h.resourcesLock.Lock()
	defer h.resourcesLock.Unlock()
	_, resourceExists := h.resources[resource.GVK]
	if addResource && !resourceExists {
		h.resources[resource.GVK] = resource
	} else if !addResource && resourceExists {
		delete(h.resources, resource.GVK)
	} else {
		return
	}

	resources := make([]resourceMatch, 0, len(h.resources))
	for _, v := range h.resources {
		resources = append(resources, v)
	}
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].GVK.String() < resources[j].GVK.String()
	})
	h.resourcesList = resources
}

func (h *handler) OnCRD(key string, crd *apiextv1.CustomResourceDefinition) (*apiextv1.CustomResourceDefinition, error) {
	if crd == nil || crd.Labels[clusterIndexedLabel] != "true" {
		return crd, nil
	}

	if resourceMatch := crdToResourceMatch(crd); resourceMatch != nil {
		h.modifyResources(*resourceMatch, crd.DeletionTimestamp.IsZero())
	}

	return crd, nil
}

func (h *handler) OnClusterObjectChanged(obj runtime.Object) (runtime.Object, error) {
	clusterNames, err := getObjectClusterNames(obj)
	if err != nil {
		return nil, err
	}
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	for _, clusterName := range clusterNames {
		h.roleTemplateController.Enqueue(fmt.Sprintf("cluster/%s/%s", objMeta.GetNamespace(), clusterName))
	}
	return obj, nil
}

func (h *handler) OnChange(key string, rt *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	if rt != nil {
		if rt.DeletionTimestamp != nil {
			return rt, nil
		}
		return rt, h.objects(rt, true, nil)
	}

	if strings.HasPrefix(key, "cluster/") {
		parts := strings.Split(key, "/")
		if len(parts) != 3 {
			return rt, nil
		}

		cluster, err := h.clusters.Get(parts[1], parts[2])
		if apierror.IsNotFound(err) {
			// ignore not found
			return rt, nil
		} else if err != nil {
			return rt, err
		}

		rts, err := h.roleTemplatesCache.List(labels.Everything())
		if err != nil {
			return rt, err
		}
		for _, rt := range rts {
			if err := h.objects(rt, false, cluster); err != nil {
				return nil, err
			}
		}
	}

	return rt, nil
}

func (h *handler) hasAnnotationsForDeletingCluster(annotations map[string]string, isClusterRole bool) (bool, error) {
	clusterName, cOK := annotations[clusterNameLabel]
	clusterNamespace, cnOK := annotations[clusterNamespaceLabel]
	if !cOK || (!cnOK && !isClusterRole) {
		// if this role doesn't have a clustername/namespace label it isn't one of the protected roles, so move on
		return false, nil
	}

	mgmtCluster, err := h.mgmtClusters.Get(clusterName)
	if err != nil {
		if !apierror.IsNotFound(err) {
			return false, err
		}
		// if management cluster was not found, ensure it's nil
		mgmtCluster = nil
	}

	cluster, err := h.clusters.Get(clusterNamespace, clusterName)
	if err != nil {
		if !apierror.IsNotFound(err) {
			return false, err
		}
		// if cluster was not found, ensure cluster is nil
		cluster = nil
	}

	if (cluster != nil && cluster.DeletionTimestamp != nil) || (mgmtCluster != nil && mgmtCluster.DeletionTimestamp != nil) {
		// if the cluster is being deleted
		return true, nil
	}

	return false, nil
}

func (h *handler) OnRemoveRole(key string, role *rbacv1.Role) (*rbacv1.Role, error) {
	shouldEnqueue, err := h.hasAnnotationsForDeletingCluster(role.Annotations, false)
	if err != nil {
		return role, err
	}
	if !shouldEnqueue {
		return role, nil
	}
	// Enqueue to ensure perms remain until full delete. Err skips don't re-enqueue, so we do that manually
	h.roleController.EnqueueAfter(role.Namespace, role.Name, reenqueueTime)
	return role, generic.ErrSkip
}

func (h *handler) OnRemoveRoleBinding(key string, roleBinding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
	shouldEnqueue, err := h.hasAnnotationsForDeletingCluster(roleBinding.Annotations, false)
	if err != nil {
		return roleBinding, err
	}
	if !shouldEnqueue {
		return roleBinding, nil
	}
	// Enqueue to ensure perms remain until full delete. Err skips don't re-enqueue, so we do that manually
	h.roleBindingController.EnqueueAfter(roleBinding.Namespace, roleBinding.Name, reenqueueTime)
	return roleBinding, generic.ErrSkip
}

func (h *handler) OnRemoveClusterRole(key string, role *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
	shouldEnqueue, err := h.hasAnnotationsForDeletingCluster(role.Annotations, true)
	if err != nil {
		return role, err
	}
	if !shouldEnqueue {
		return role, nil
	}
	// Enqueue to ensure perms remain until full delete. Err skips don't re-enqueue, so we do that manually
	h.clusterRoleController.EnqueueAfter(role.Name, reenqueueTime)
	return role, generic.ErrSkip
}

func (h *handler) OnRemoveClusterRoleBinding(key string, roleBinding *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
	shouldEnqueue, err := h.hasAnnotationsForDeletingCluster(roleBinding.Annotations, true)
	if err != nil {
		return roleBinding, err
	}
	if !shouldEnqueue {
		return roleBinding, nil
	}
	// Enqueue to ensure perms remain until full delete. Err skips don't re-enqueue, so we do that manually
	h.clusterRoleBindingController.EnqueueAfter(roleBinding.Name, reenqueueTime)
	return roleBinding, generic.ErrSkip
}

func (h *handler) objects(rt *v3.RoleTemplate, enqueue bool, cluster *v1.Cluster) error {
	var (
		matchResults []match
	)

	if rt.Context != "cluster" {
		return nil
	}

	rules, err := rbac.RulesFromTemplate(h.clusterRoleCache, h.roleTemplatesCache, rt)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if len(rule.NonResourceURLs) > 0 || len(rule.ResourceNames) > 0 {
			continue
		}
		matches, err := h.getMatchingClusterIndexedTypes(rule)
		if err != nil {
			return err
		}
		for _, matched := range matches {
			matchResults = append(matchResults, match{
				Rule: rbacv1.PolicyRule{
					Verbs:     rule.Verbs,
					APIGroups: []string{matched.GVK.Group},
					Resources: []string{matched.Resource},
				},
				Match: matched,
			})
		}
	}

	if len(matchResults) == 0 {
		return nil
	}

	if enqueue {
		crtbs, err := h.clusterRoleTemplateBindings.GetByIndex(crbtByRoleTemplateName, rt.Name)
		if err != nil {
			return err
		}
		for _, crtb := range crtbs {
			h.clusterRoleTemplateBindingController.Enqueue(crtb.Namespace, crtb.Name)
		}
	}

	var clusters []*v1.Cluster
	if cluster == nil {
		var err error
		clusters, err = h.clusters.List("", labels.Everything())
		if err != nil {
			return err
		}
	} else {
		clusters = []*v1.Cluster{cluster}
	}

	for _, cluster := range clusters {
		err := h.createRoleForCluster(rt, matchResults, cluster)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *handler) getResourceNames(resourceMatch resourceMatch, cluster *v1.Cluster) ([]string, error) {
	objs, err := h.dynamic.GetByIndex(resourceMatch.GVK, clusterIndexed, fmt.Sprintf("%s/%s", cluster.Namespace, cluster.Name))
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(objs))
	for _, obj := range objs {
		objMeta, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}
		result = append(result, objMeta.GetName())
	}
	return result, nil
}

func roleTemplateRoleName(rtName, clusterName string) string {
	return name.SafeConcatName("crt", clusterName, rtName)
}

func (h *handler) createRoleForCluster(rt *v3.RoleTemplate, matches []match, cluster *v1.Cluster) error {
	h.roleLocker.Lock(cluster.Namespace + "/" + cluster.Name)
	defer h.roleLocker.Unlock(cluster.Namespace + "/" + cluster.Name)

	role := rbacv1.Role{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:        roleTemplateRoleName(rt.Name, cluster.Name),
			Namespace:   cluster.Namespace,
			Annotations: map[string]string{clusterNameLabel: cluster.GetName(), clusterNamespaceLabel: cluster.GetNamespace()},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: cluster.APIVersion,
					Kind:       cluster.Kind,
					Name:       cluster.Name,
					UID:        cluster.UID,
				},
			},
		},
	}

	for _, match := range matches {
		names, err := h.getResourceNames(match.Match, cluster)
		if err != nil {
			return err
		}
		if len(names) == 0 {
			continue
		}
		role.Rules = append(role.Rules, rbacv1.PolicyRule{
			Verbs:         match.Rule.Verbs,
			APIGroups:     match.Rule.APIGroups,
			Resources:     match.Rule.Resources,
			ResourceNames: names,
		})
	}

	return h.apply.
		WithListerNamespace(role.Namespace).
		WithSetID("auth-prov-v2-roletemplate" + "-" + cluster.Name).
		WithOwner(rt).
		ApplyObjects(&role)
}

type match struct {
	Rule  rbacv1.PolicyRule
	Match resourceMatch
}

type resourceMatch struct {
	GVK      schema.GroupVersionKind
	Resource string
}

func (r *resourceMatch) Matches(rule rbacv1.PolicyRule) bool {
	return r.matchesGroup(rule) && r.matchesResource(rule)
}

func (r *resourceMatch) matchesGroup(rule rbacv1.PolicyRule) bool {
	for _, group := range rule.APIGroups {
		if group == "*" || group == r.GVK.Group {
			return true
		}
	}
	return false
}

func (r *resourceMatch) matchesResource(rule rbacv1.PolicyRule) bool {
	for _, resource := range rule.Resources {
		if resource == "*" || resource == r.Resource {
			return true
		}
	}
	return false
}

func (h *handler) candidateTypes() []resourceMatch {
	h.resourcesLock.RLock()
	defer h.resourcesLock.RUnlock()
	return h.resourcesList
}

func (h *handler) getMatchingClusterIndexedTypes(rule rbacv1.PolicyRule) (result []resourceMatch, _ error) {
	for _, candidate := range h.candidateTypes() {
		if candidate.Matches(rule) {
			result = append(result, candidate)
		}
	}
	return
}

package rbac

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/slice"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v32 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/wrangler/pkg/name"
	k8srbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	wranglerName "github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	NamespaceID                         = "namespaceId"
	ProjectID                           = "projectId"
	ClusterID                           = "clusterId"
	GlobalAdmin                         = "admin"
	GlobalAdminCRBPrefix                = "globaladmin-"
	GlobalRestrictedAdmin               = "restricted-admin"
	ClusterCRDsClusterRole              = "cluster-crd-clusterRole"
	RestrictedAdminClusterRoleBinding   = "restricted-admin-rb-cluster"
	ProjectCRDsClusterRole              = "project-crd-clusterRole"
	RestrictedAdminProjectRoleBinding   = "restricted-admin-rb-project"
	RestrictedAdminCRForClusters        = "restricted-admin-cr-clusters"
	RestrictedAdminCRBForClusters       = "restricted-admin-crb-clusters"
	CrtbOwnerLabel                      = "authz.cluster.cattle.io/crtb-owner"
	PrtbOwnerLabel                      = "authz.cluster.cattle.io/prtb-owner"
	AggregationLabel                    = "management.cattle.io/aggregates"
	ClusterRoleOwnerLabel               = "authz.cluster.cattle.io/clusterrole-owner"
	aggregatorSuffix                    = "aggregator"
	promotedSuffix                      = "promoted"
	namespaceSuffix                     = "namespaces"
	clusterManagementPlaneSuffix        = "cluster-mgmt"
	projectManagementPlaneSuffix        = "project-mgmt"
	ClusterAdminRoleName                = "cluster-admin"
	CrbGlobalRoleAnnotation             = "authz.cluster.cattle.io/globalrole"
	CrbGlobalRoleBindingAnnotation      = "authz.cluster.cattle.io/globalrolebinding"
	CrbAdminGlobalRoleCheckedAnnotation = "authz.cluster.cattle.io/admin-globalrole-checked"
	AggregationFeatureLabel             = "management.cattle.io/roletemplate-aggregation"
)

// BuildSubjectFromRTB This function will generate
// PRTB and CRTB to the subject with user, group
// or service account
func BuildSubjectFromRTB(object metav1.Object) (rbacv1.Subject, error) {
	var userName, groupPrincipalName, groupName, name, kind, sa, namespace string
	switch rtb := object.(type) {
	case *v3.ProjectRoleTemplateBinding:
		userName = rtb.UserName
		groupPrincipalName = rtb.GroupPrincipalName
		groupName = rtb.GroupName
		sa = rtb.ServiceAccount
	case *v3.ClusterRoleTemplateBinding:
		userName = rtb.UserName
		groupPrincipalName = rtb.GroupPrincipalName
		groupName = rtb.GroupName
	default:
		objectName := ""
		if object != nil {
			objectName = object.GetName()
		}
		return rbacv1.Subject{}, fmt.Errorf("unrecognized roleTemplateBinding type: %v", objectName)
	}

	if userName != "" {
		name = userName
		kind = "User"
	}

	if groupPrincipalName != "" {
		if name != "" {
			return rbacv1.Subject{}, fmt.Errorf("roletemplatebinding has more than one subject fields set: %v", object.GetName())
		}
		name = groupPrincipalName
		kind = "Group"
	}

	if groupName != "" {
		if name != "" {
			return rbacv1.Subject{}, fmt.Errorf("roletemplatebinding has more than one subject fields set: %v", object.GetName())
		}
		name = groupName
		kind = "Group"
	}

	if sa != "" {
		parts := strings.SplitN(sa, ":", 2)
		if len(parts) < 2 {
			return rbacv1.Subject{}, fmt.Errorf("service account %s of projectroletemplatebinding is invalid: %v", sa, object.GetName())
		}
		namespace = parts[0]
		name = parts[1]
		kind = "ServiceAccount"
	}

	if name == "" {
		return rbacv1.Subject{}, fmt.Errorf("roletemplatebinding doesn't have any subject fields set: %v", object.GetName())
	}

	// apiGroup default for both User and Group
	apiGroup := rbacv1.GroupName

	if kind == "ServiceAccount" {
		// ServiceAccount default is empty string
		apiGroup = ""
	}
	return rbacv1.Subject{
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
		APIGroup:  apiGroup,
	}, nil
}

func GrbCRBName(grb *v3.GlobalRoleBinding) string {
	return GlobalAdminCRBPrefix + GetGRBTargetKey(grb)
}

// GetGRBSubject creates and returns a subject that is
// determined by inspecting the the GRB's target fields
func GetGRBSubject(grb *v3.GlobalRoleBinding) rbacv1.Subject {
	kind := "User"
	name := grb.UserName
	if name == "" && grb.GroupPrincipalName != "" {
		kind = "Group"
		name = grb.GroupPrincipalName
	}

	return rbacv1.Subject{
		Kind:     kind,
		Name:     name,
		APIGroup: rbacv1.GroupName,
	}
}

// getGRBTargetKey returns a key that uniquely identifies the given GRB's target.
// If a user is being targeted, then the user's name is returned.
// Otherwise, the group principal name is converted to a valid user string and
// is returned.
func GetGRBTargetKey(grb *v3.GlobalRoleBinding) string {
	name := grb.UserName

	if name == "" {
		hasher := sha256.New()
		hasher.Write([]byte(grb.GroupPrincipalName))
		sha := base32.StdEncoding.WithPadding(-1).EncodeToString(hasher.Sum(nil))[:10]
		name = "u-" + strings.ToLower(sha)
	}
	return name
}

// Returns object with available information to check against users permissions, used in combination with CanDo
func ObjFromContext(apiContext *types.APIContext, resource *types.RawResource) map[string]interface{} {
	var obj map[string]interface{}
	if resource != nil && resource.Values["id"] != nil {
		obj = resource.Values
	}
	if obj == nil {
		obj = map[string]interface{}{
			"id": apiContext.ID,
		}
		// collection endpoint without id needs to know which cluster-namespace for rbac check
		if apiContext.Query.Get(ClusterID) != "" {
			obj[NamespaceID] = apiContext.Query.Get(ClusterID)
		}
		if apiContext.Query.Get(ProjectID) != "" {
			_, obj[NamespaceID] = ref.Parse(apiContext.Query.Get(ProjectID))
		}
	}
	return obj
}

func TypeFromContext(apiContext *types.APIContext, resource *types.RawResource) string {
	if resource == nil {
		return apiContext.Type
	}
	return resource.Type
}

func GetRTBLabel(objMeta metav1.ObjectMeta) string {
	return wranglerName.SafeConcatName(objMeta.Namespace + "_" + objMeta.Name)
}

// NameForRoleBinding returns a deterministic name for a RoleBinding with the provided namespace, roleName, and subject
func NameForRoleBinding(namespace string, role rbacv1.RoleRef, subject rbacv1.Subject) string {
	var name strings.Builder
	name.WriteString("rb-")
	name.WriteString(getBindingHash(namespace, role, subject))
	nm := name.String()
	logrus.Debugf("RoleBinding with namespace=%s role.kind=%s role.name=%s subject.kind=%s subject.name=%s has name: %s", namespace, role.Kind, role.Name, subject.Kind, subject.Name, nm)
	return nm
}

// NameForClusterRoleBinding returns a deterministic name for a ClusterRoleBinding with the provided roleName and subject
func NameForClusterRoleBinding(role rbacv1.RoleRef, subject rbacv1.Subject) string {
	var name strings.Builder
	name.WriteString("crb-")
	name.WriteString(getBindingHash("", role, subject))
	nm := name.String()
	logrus.Debugf("ClusterRoleBinding with role.kind=%s role.name=%s subject.kind=%s subject.name=%s has name: %s", role.Kind, role.Name, subject.Kind, subject.Name, nm)
	return nm
}

// getBindingHash returns a hash created from the passed in arguments
// uses base32 encoding for hash, since all characters in encoding scheme are valid in k8s resource names
// probability of collision is: 1/32^10 == 1/(2^5)^10 == 1/2^50 (sufficiently low)
func getBindingHash(namespace string, role rbacv1.RoleRef, subject rbacv1.Subject) string {
	var input strings.Builder
	input.WriteString(namespace)
	input.WriteString(role.Kind)
	input.WriteString(role.Name)
	input.WriteString(subject.Kind)
	input.WriteString(subject.Name)

	hasher := sha256.New()
	hasher.Write([]byte(input.String()))
	digest := base32.StdEncoding.WithPadding(-1).EncodeToString(hasher.Sum(nil))
	return strings.ToLower(digest[:10])
}

// RulesFromTemplate gets all rules from the template and all referenced templates
func RulesFromTemplate(clusterRoles k8srbacv1.ClusterRoleCache, roleTemplates v32.RoleTemplateCache, rt *v3.RoleTemplate) ([]rbacv1.PolicyRule, error) {
	var rules []rbacv1.PolicyRule
	var err error
	templatesSeen := make(map[string]bool)

	// Kickoff gathering rules
	rules, err = gatherRules(clusterRoles, roleTemplates, rt, rules, templatesSeen)
	if err != nil {
		return rules, err
	}
	return rules, nil
}

// gatherRules appends the rules from current template and does a recursive call to get all inherited roles referenced
func gatherRules(clusterRoles k8srbacv1.ClusterRoleCache, roleTemplates v32.RoleTemplateCache, rt *v3.RoleTemplate, rules []rbacv1.PolicyRule, seen map[string]bool) ([]rbacv1.PolicyRule, error) {
	seen[rt.Name] = true

	if rt.External {
		if rt.ExternalRules != nil {
			rules = append(rules, rt.ExternalRules...)
		} else if rt.Context == "cluster" {
			cr, err := clusterRoles.Get(rt.Name)
			if err != nil {
				return nil, err
			}
			rules = append(rules, cr.Rules...)
		}
	}

	rules = append(rules, rt.Rules...)

	for _, r := range rt.RoleTemplateNames {
		// If we have already seen the roleTemplate, skip it
		if seen[r] {
			continue
		}
		next, err := roleTemplates.Get(r)
		if err != nil {
			return nil, err
		}
		rules, err = gatherRules(clusterRoles, roleTemplates, next, rules, seen)
		if err != nil {
			return nil, err
		}
	}
	return rules, nil
}

func ProvisioningClusterAdminName(cluster *provv1.Cluster) string {
	return wranglerName.SafeConcatName("crt", cluster.Name, "cluster-owner")
}

func RuleGivesResourceAccess(rule rbacv1.PolicyRule, resourceName string) bool {
	if !isRuleInTargetAPIGroup(rule) {
		// if we don't list the target api group, don't bother looking for the resources
		return false
	}
	for _, resource := range rule.Resources {
		if resource == resourceName || resource == "*" {
			return true
		}
	}
	return false
}

func isRuleInTargetAPIGroup(rule rbacv1.PolicyRule) bool {
	for _, group := range rule.APIGroups {
		if group == mgmt.GroupName || group == "*" {
			return true
		}
	}
	return false
}

// IsAdminGlobalRole returns true is a GlobalRole has admin permissions.
// A global role is considered to have admin permissions if it is the built-in admin role
// or it gives full access to all resources and non-resource URLs, as in:
// apiVersion: management.cattle.io/v3
// displayName: custom-admin
// kind: GlobalRole
// metadata:
//
//	name: custom-admin
//
// rules:
// - apiGroups:
//   - '*'
//     resources:
//   - '*'
//     verbs:
//   - '*'
//
// - nonResourceURLs:
//   - '*'
//     verbs:
//   - '*'
func IsAdminGlobalRole(gr *v3.GlobalRole) bool {
	if gr == nil {
		return false
	}

	// Global role is the built-in admin role.
	if gr.Builtin && gr.Name == GlobalAdmin {
		return true
	}

	var hasResourceRule, hasNonResourceRule bool
	for _, rule := range gr.Rules {
		if slice.ContainsString(rule.Resources, "*") && slice.ContainsString(rule.APIGroups, "*") && slice.ContainsString(rule.Verbs, "*") {
			hasResourceRule = true
			continue
		}
		if slice.ContainsString(rule.NonResourceURLs, "*") && slice.ContainsString(rule.Verbs, "*") {
			hasNonResourceRule = true
			continue
		}
	}

	// Global role gives full access to all resources and non-resource URLs.
	return hasResourceRule && hasNonResourceRule
}

// CreateOrUpdateResource creates or updates the given non-namespaced resource
//   - obj is the resource to create or update.
//   - client is the Wrangler client to use to get/create/update resource.
//   - areResourcesTheSame is a func that compares two resources and returns (true, nil) if they are equal, and (false, T) when not the same.
//     T is an updated version of the resource.
func CreateOrUpdateResource[T generic.RuntimeMetaObject, TList runtime.Object](obj T, client generic.NonNamespacedClientInterface[T, TList], areResourcesTheSame func(T, T) (bool, T)) error {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	// attempt to get the resource
	resource, err := client.Get(obj.GetName(), metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get %s %s: %w", kind, obj.GetName(), err)
		}
		logrus.Infof("%s %s is being created", kind, obj.GetName())
		// resource doesn't exist, create it
		_, err = client.Create(obj)
		if err != nil {
			return fmt.Errorf("failed to create %s %s: %w", kind, obj.GetName(), err)
		}
		return nil
	}

	// check that the existing resource is the same as the one we want
	if same, updatedResource := areResourcesTheSame(resource, obj); !same {
		logrus.Infof("%s %s needs to be updated", kind, obj.GetName())
		// if it has changed, update it to the correct version
		_, err := client.Update(updatedResource)
		if err != nil {
			return fmt.Errorf("failed to update %s %s: %w", kind, obj.GetName(), err)
		}
	}
	return nil
}

// CreateOrUpdateNamespacedResource creates or updates the given namespaced resource.
//   - obj is the resource to create or update.
//   - client is the Wrangler client to use to get/create/update resource.
//   - areResourcesTheSame is a func that compares two resources and returns (true, nil) if they are equal, and (false, T) when not the same.
//     T is an updated version of the resource.
func CreateOrUpdateNamespacedResource[T generic.RuntimeMetaObject, TList runtime.Object](obj T, client generic.ClientInterface[T, TList], areResourcesTheSame func(T, T) (bool, T)) error {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	resource, err := client.Get(obj.GetNamespace(), obj.GetName(), metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get %s %s in namespace %s: %w", kind, obj.GetName(), obj.GetNamespace(), err)
		}
		logrus.Infof("%s %s is being created in namespace %s", kind, obj.GetName(), obj.GetNamespace())
		// resource doesn't exist, create it
		_, err = client.Create(obj)
		if err != nil {
			return fmt.Errorf("failed to create %s %s in namespace %s: %w", kind, obj.GetName(), obj.GetNamespace(), err)
		}
		return nil
	}

	// check that the existing resource is the same as the one we want
	if same, updatedResource := areResourcesTheSame(resource, obj); !same {
		logrus.Infof("%s %s in namespace %s needs to be updated", kind, obj.GetName(), obj.GetNamespace())
		// if it has changed, update it to the correct version
		_, err := client.Update(updatedResource)
		if err != nil {
			return fmt.Errorf("failed to update %s %s in namespace %s: %w", kind, obj.GetName(), obj.GetNamespace(), err)
		}
	}
	return nil
}

// AreClusterRolesSame returns true if the current ClusterRole has the same fields present in the desired ClusterRole.
// If not, it also updates the current ClusterRole fields to match the desired ClusterRole.
// The fields it checks are:
//
//   - Rules or AggregationRule
//   - Cluster role owner annotation
//   - Aggregation label
func AreClusterRolesSame(currentCR, wantedCR *rbacv1.ClusterRole) (bool, *rbacv1.ClusterRole) {
	same := true

	if wantedCR.AggregationRule == nil {
		if currentCR.AggregationRule != nil {
			same = false
			currentCR.AggregationRule = nil
		}
		if !equality.Semantic.DeepEqual(currentCR.Rules, wantedCR.Rules) {
			same = false
			currentCR.Rules = wantedCR.Rules
		}
	} else {
		if !equality.Semantic.DeepEqual(currentCR.AggregationRule, wantedCR.AggregationRule) {
			same = false
			currentCR.AggregationRule = wantedCR.AggregationRule
		}
		if len(currentCR.Rules) > 0 {
			same = false
			currentCR.Rules = nil
		}
	}
	if got, want := currentCR.Labels[ClusterRoleOwnerLabel], wantedCR.Labels[ClusterRoleOwnerLabel]; got != want {
		same = false
		metav1.SetMetaDataLabel(&currentCR.ObjectMeta, ClusterRoleOwnerLabel, want)
	}
	if got, want := currentCR.Labels[AggregationLabel], wantedCR.Labels[AggregationLabel]; got != want {
		same = false
		metav1.SetMetaDataLabel(&currentCR.ObjectMeta, AggregationLabel, want)
	}
	return same, currentCR
}

// DeleteResource deletes a non namespaced resource
func DeleteResource[T generic.RuntimeMetaObject, TList runtime.Object](name string, client generic.NonNamespacedClientInterface[T, TList]) error {
	logrus.Infof("Deleting %T %s", *new(T), name)
	err := client.Delete(name, &metav1.DeleteOptions{})
	// If the resource is already gone, don't treat it as an error
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to delete %T %s: %w", *new(T), name, err)
	}
	return nil
}

// DeleteNamespacedResource deletes a namespaced resource
func DeleteNamespacedResource[T generic.RuntimeMetaObject, TList runtime.Object](namespace, name string, client generic.ClientInterface[T, TList]) error {
	logrus.Infof("Deleting %T %s in namespace %s", *new(T), name, namespace)
	err := client.Delete(namespace, name, &metav1.DeleteOptions{})
	// If the resource is already gone, don't treat it as an error
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to delete %T %s in namespace %s: %w", *new(T), name, namespace, err)
	}
	return nil
}

// BuildClusterRole creates a cluster role with an aggregation label
//   - name: name of the cluster role
//   - ownerName: name of the creator of this cluster role
//   - rules: list of policy rules for the cluster role
func BuildClusterRole(name, ownerName string, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				AggregationLabel:        name,
				ClusterRoleOwnerLabel:   ownerName,
				AggregationFeatureLabel: "true",
			},
		},
		Rules: rules,
	}
}

// BuildAggregatingClusterRole returns a ClusterRole with AggregationRules
//   - rt the role template to base it off of. Most importantly, it adds an aggregation label for each role template in RoleTemplateNames
//   - nameTransformer a function that takes a string and returns one that represents the cluster role name. It also applies to all inherited role templates.
func BuildAggregatingClusterRole(rt *v3.RoleTemplate, nameTransformer func(string) string) *rbacv1.ClusterRole {
	crName := nameTransformer(rt.Name)
	ownerName := rt.Name

	// aggregate our own cluster role
	roleTemplateLabels := []metav1.LabelSelector{{MatchLabels: map[string]string{AggregationLabel: crName}}}

	// aggregate every inherited role template
	for _, roleTemplateName := range rt.RoleTemplateNames {
		labelSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{AggregationLabel: AggregatedClusterRoleNameFor(nameTransformer(roleTemplateName))},
		}
		roleTemplateLabels = append(roleTemplateLabels, labelSelector)
	}

	aggregatingCRName := AggregatedClusterRoleNameFor(crName)
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: aggregatingCRName,
			Labels: map[string]string{
				// Label so other cluster roles can aggregate this one
				AggregationLabel: aggregatingCRName,
				// Label to identify who owns this cluster role
				ClusterRoleOwnerLabel:   ownerName,
				AggregationFeatureLabel: "true",
			},
		},
		AggregationRule: &rbacv1.AggregationRule{
			ClusterRoleSelectors: roleTemplateLabels,
		},
	}
}

// BuildAggregatingRoleBindingFromRTB returns a RoleBinding for a RTB. It is bound to the Aggregating ClusterRole.
func BuildAggregatingRoleBindingFromRTB(rtb metav1.Object, roleRefName string) (*rbacv1.RoleBinding, error) {
	rb, err := BuildRoleBindingFromRTB(rtb, AggregatedClusterRoleNameFor(roleRefName))
	if rb != nil && rb.Labels != nil {
		rb.Labels[AggregationFeatureLabel] = "true"
	}
	return rb, err
}

// BuildRoleBindingFromRTB returns a RoleBinding for a RTB. It is bound to the ClusterRole specified by roleRefName.
func BuildRoleBindingFromRTB(rtb metav1.Object, roleRefName string) (*rbacv1.RoleBinding, error) {
	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     roleRefName,
	}

	subject, err := BuildSubjectFromRTB(rtb)
	if err != nil {
		return nil, err
	}

	var ownerLabel string
	switch rtb.(type) {
	case *v3.ProjectRoleTemplateBinding:
		ownerLabel = GetPRTBOwnerLabel(rtb.GetName())
	case *v3.ClusterRoleTemplateBinding:
		ownerLabel = GetCRTBOwnerLabel(rtb.GetName())
	default:
		return nil, fmt.Errorf("unrecognized roleTemplateBinding type: %T", rtb)
	}

	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NameForRoleBinding(rtb.GetNamespace(), roleRef, subject),
			Namespace: rtb.GetNamespace(),
			Labels:    map[string]string{ownerLabel: "true"},
		},
		RoleRef:  roleRef,
		Subjects: []rbacv1.Subject{subject},
	}, nil
}

// BuildAggregatingClusterRoleBindingFromRTB returns the ClusterRoleBinding needed for a RTB. It is bound to the Aggregating ClusterRole.
func BuildAggregatingClusterRoleBindingFromRTB(rtb metav1.Object, roleRefName string) (*rbacv1.ClusterRoleBinding, error) {
	crb, err := BuildClusterRoleBindingFromRTB(rtb, AggregatedClusterRoleNameFor(roleRefName))
	if crb != nil && crb.Labels != nil {
		crb.Labels[AggregationFeatureLabel] = "true"
	}
	return crb, err
}

// BuildClusterRoleBindingFromRTB returns the ClusterRoleBinding needed for a RTB. It is bound to the ClusterRole specified by roleRefName.
func BuildClusterRoleBindingFromRTB(rtb metav1.Object, roleRefName string) (*rbacv1.ClusterRoleBinding, error) {
	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     roleRefName,
	}

	subject, err := BuildSubjectFromRTB(rtb)
	if err != nil {
		return nil, err
	}

	var ownerLabel string
	switch rtb.(type) {
	case *v3.ProjectRoleTemplateBinding:
		ownerLabel = GetPRTBOwnerLabel(rtb.GetName())
	case *v3.ClusterRoleTemplateBinding:
		ownerLabel = GetCRTBOwnerLabel(rtb.GetName())
	default:
		return nil, fmt.Errorf("unrecognized roleTemplateBinding type: %T", rtb)
	}

	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   NameForClusterRoleBinding(roleRef, subject),
			Labels: map[string]string{ownerLabel: "true"},
		},
		RoleRef:  roleRef,
		Subjects: []rbacv1.Subject{subject},
	}, nil
}

// IsClusterRoleBindingContentSame compares the Subjects and RoleRef fields of two Cluster Role Bindings.
func IsClusterRoleBindingContentSame(crb1, crb2 *rbacv1.ClusterRoleBinding) bool {
	return equality.Semantic.DeepEqual(crb1.Subjects, crb2.Subjects) &&
		equality.Semantic.DeepEqual(crb1.RoleRef, crb2.RoleRef)
}

// IsRoleBindingContentSame compares the Subjects and RoleRef fields of two Role Bindings.
func IsRoleBindingContentSame(rb1, rb2 *rbacv1.RoleBinding) bool {
	return equality.Semantic.DeepEqual(rb1.Subjects, rb2.Subjects) &&
		equality.Semantic.DeepEqual(rb1.RoleRef, rb2.RoleRef)
}

// ClusterRoleNameFor returns safe version of a string to be used for a clusterRoleName
func ClusterRoleNameFor(s string) string {
	return name.SafeConcatName(s)
}

// PromotedClusterRoleNameFor appends the promoted suffix to a string safely (ie <= 63 characters)
func PromotedClusterRoleNameFor(s string) string {
	return name.SafeConcatName(s, promotedSuffix)
}

// NamespaceClusterRoleNameFor appends the namespace suffix to a string safely (ie <= 63 characters)
func NamespaceClusterRoleNameFor(s string) string {
	return name.SafeConcatName(s, namespaceSuffix)
}

// AggregatedClusterRoleNameFor appends the aggregation suffix to a string safely (ie <= 63 characters)
func AggregatedClusterRoleNameFor(s string) string {
	return name.SafeConcatName(s, aggregatorSuffix)
}

// ClusterManagementPlaneClusterRoleNameFor appends the cluster management plane suffix to a string safely (ie <= 63 characters)
func ClusterManagementPlaneClusterRoleNameFor(s string) string {
	return name.SafeConcatName(s, clusterManagementPlaneSuffix)
}

// ProjectManagementPlaneClusterRoleNameFor appends the project management plane suffix to a string safely (ie <= 63 characters)
func ProjectManagementPlaneClusterRoleNameFor(s string) string {
	return name.SafeConcatName(s, projectManagementPlaneSuffix)
}

// GetAuthV2OwnerLabel creates the owner label for the RoleTemplateBinding in the style used in pkg/controllers/management/authprovisioningv2.
// Either:
//
//	authz.cluster.cattle.io/crtb-owner: <crtb.Name>
//	authz.cluster.cattle.io/prtb-owner: <prtb.Name>
func GetAuthV2OwnerLabel(rtb metav1.Object) string {
	switch obj := rtb.(type) {
	case *v3.ProjectRoleTemplateBinding:
		return PrtbOwnerLabel + "=" + obj.Name
	case *v3.ClusterRoleTemplateBinding:
		return CrtbOwnerLabel + "=" + obj.Name
	}
	return ""
}

// GetPRTBOwnerLabel gets the owner label for a PRTB.
// The label is always authz.cluster.cattle.io/prtb-owner-<prtb.name>: "true"
// The reason it isn't a key value pair is because we have multiple of these labels on a single RoleBinding/ClusterRoleBinding, so we need unique labels.
func GetPRTBOwnerLabel(s string) string {
	return name.SafeConcatName(PrtbOwnerLabel, s)
}

// GetCRTBOwnerLabel gets the owner label for a CRTB.
// The label is always authz.cluster.cattle.io/crtb-owner-<crtb.name>: "true"
// The reason it isn't a key value pair is because we have multiple of these labels on a single RoleBinding/ClusterRoleBinding, so we need unique labels.
func GetCRTBOwnerLabel(s string) string {
	return name.SafeConcatName(CrtbOwnerLabel, s)
}

// GetClusterRoleOwnerLabel gets the owner label for a ClusterRole.
func GetClusterRoleOwnerLabel(s string) string {
	return ClusterRoleOwnerLabel + "=" + s
}

// GetClusterAndProjectNameFromPRTB gets the cluster and project belonging to a ProjectRoleTemplateBinding.
// The ProjectName field is of the form <cluster-name>:<project-name>
func GetClusterAndProjectNameFromPRTB(prtb *v3.ProjectRoleTemplateBinding) (string, string) {
	cluster, project, _ := strings.Cut(prtb.ProjectName, ":")
	return cluster, project
}

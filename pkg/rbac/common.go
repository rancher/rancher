package rbac

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	NamespaceID                       = "namespaceId"
	ProjectID                         = "projectId"
	ClusterID                         = "clusterId"
	GlobalAdmin                       = "admin"
	GlobalRestrictedAdmin             = "restricted-admin"
	ClusterCRDsClusterRole            = "cluster-crd-clusterRole"
	RestrictedAdminClusterRoleBinding = "restricted-admin-rb-cluster"
	ProjectCRDsClusterRole            = "project-crd-clusterRole"
	RestrictedAdminProjectRoleBinding = "restricted-admin-rb-project"
	RestrictedAdminCRForClusters      = "restricted-admin-cr-clusters"
	RestrictedAdminCRBForClusters     = "restricted-admin-crb-clusters"
	CrtbOwnerLabel                    = "authz.cluster.cattle.io/crtb-owner"
	PrtbOwnerLabel                    = "authz.cluster.cattle.io/prtb-owner"
	aggregationLabel                  = "management.cattle.io/aggregates"
	clusterRoleOwnerAnnotation        = "authz.cluster.cattle.io/clusterrole-owner"
	aggregatorSuffix                  = "aggregator"
	promotedSuffix                    = "promoted"
	clusterManagementPlaneSuffix      = "cluster-mgmt"
	projectManagementPlaneSuffix      = "project-mgmt"
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
		return rbacv1.Subject{}, errors.Errorf("unrecognized roleTemplateBinding type: %v", objectName)
	}

	if userName != "" {
		name = userName
		kind = "User"
	}

	if groupPrincipalName != "" {
		if name != "" {
			return rbacv1.Subject{}, errors.Errorf("roletemplatebinding has more than one subject fields set: %v", object.GetName())
		}
		name = groupPrincipalName
		kind = "Group"
	}

	if groupName != "" {
		if name != "" {
			return rbacv1.Subject{}, errors.Errorf("roletemplatebinding has more than one subject fields set: %v", object.GetName())
		}
		name = groupName
		kind = "Group"
	}

	if sa != "" {
		parts := strings.SplitN(sa, ":", 2)
		if len(parts) < 2 {
			return rbacv1.Subject{}, errors.Errorf("service account %s of projectroletemplatebinding is invalid: %v", sa, object.GetName())
		}
		namespace = parts[0]
		name = parts[1]
		kind = "ServiceAccount"
	}

	if name == "" {
		return rbacv1.Subject{}, errors.Errorf("roletemplatebinding doesn't have any subject fields set: %v", object.GetName())
	}

	// apiGroup default for both User and Group
	apiGroup := "rbac.authorization.k8s.io"

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
	return "globaladmin-" + GetGRBTargetKey(grb)
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

// CreateOrUpdateResource creates or updates the given resource
//   - getResource is a func that returns a single object and an error
//   - obj is the resource to create or update
//   - client is the Wrangler client to use to get/create/update resource
//   - areResourcesTheSame is a func that compares two resources and returns (true, nil) if they are equal, and (false, T) when not the same where T is an updated resource
func CreateOrUpdateResource[T generic.RuntimeMetaObject, TList runtime.Object](obj T, client generic.NonNamespacedClientInterface[T, TList], areResourcesTheSame func(T, T) (bool, T)) error {
	// attempt to get the resource
	resource, err := client.Get(obj.GetName(), metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil
		}
		// resource doesn't exist, create it
		_, err = client.Create(obj)
		return err
	}

	// check that the existing resource is the same as the one we want
	if same, updatedResource := areResourcesTheSame(resource, obj); !same {
		// if it has changed, update it to the correct version
		_, err := client.Update(updatedResource)
		if err != nil {
			return err
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
		if !reflect.DeepEqual(currentCR.Rules, wantedCR.Rules) {
			same = false
			currentCR.Rules = wantedCR.Rules
		}
	} else {
		if !reflect.DeepEqual(currentCR.AggregationRule, wantedCR.AggregationRule) {
			same = false
			currentCR.AggregationRule = wantedCR.AggregationRule
		}
		if len(currentCR.Rules) > 0 {
			same = false
			currentCR.Rules = nil
		}
	}
	if got, want := currentCR.Annotations[clusterRoleOwnerAnnotation], wantedCR.Annotations[clusterRoleOwnerAnnotation]; got != want {
		same = false
		metav1.SetMetaDataAnnotation(&currentCR.ObjectMeta, clusterRoleOwnerAnnotation, want)
	}
	if got, want := currentCR.Labels[aggregationLabel], wantedCR.Labels[aggregationLabel]; got != want {
		same = false
		metav1.SetMetaDataLabel(&currentCR.ObjectMeta, aggregationLabel, want)
	}
	return same, currentCR
}

// DeleteResource deletes a non namespaced resource
func DeleteResource[T generic.RuntimeMetaObject, TList runtime.Object](name string, client generic.NonNamespacedClientInterface[T, TList]) error {
	err := client.Delete(name, &metav1.DeleteOptions{})
	// If the resource is already gone, don't treat it as an error
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// BuildClusterRole creates a cluster role with an aggregation label
//   - name: name of the cluster role
//   - ownerName: name of the creator of this cluster role
//   - rules: list of policy rules for the cluster role
func BuildClusterRole(name, ownerName string, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      map[string]string{aggregationLabel: name},
			Annotations: map[string]string{clusterRoleOwnerAnnotation: ownerName},
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
	roleTemplateLabels := []metav1.LabelSelector{{MatchLabels: map[string]string{aggregationLabel: crName}}}

	// aggregate every inherited role template
	for _, roleTemplateName := range rt.RoleTemplateNames {
		labelSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{aggregationLabel: nameTransformer(roleTemplateName)},
		}
		roleTemplateLabels = append(roleTemplateLabels, labelSelector)
	}

	aggregatingCRName := AggregatedClusterRoleNameFor(crName)
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: aggregatingCRName,
			// Label so other cluster roles can aggregate this one
			Labels: map[string]string{
				aggregationLabel: aggregatingCRName,
			},
			// Annotation to identify which role template owns the cluster role
			Annotations: map[string]string{clusterRoleOwnerAnnotation: ownerName},
		},
		AggregationRule: &rbacv1.AggregationRule{
			ClusterRoleSelectors: roleTemplateLabels,
		},
	}
}

// BuildClusterRoleBindingFromRTB returns the ClusterRoleBinding needed for a RTB. It is bound to the ClusterRole specified by roleRefName.
func BuildClusterRoleBindingFromRTB(rtb metav1.Object, roleRefName string) (*rbacv1.ClusterRoleBinding, error) {
	roleRef := rbacv1.RoleRef{
		Kind: "ClusterRole",
		Name: AggregatedClusterRoleNameFor(roleRefName),
	}

	subject, err := BuildSubjectFromRTB(rtb)
	if err != nil {
		return nil, err
	}

	var ownerLabel string
	switch rtb.(type) {
	case *v3.ProjectRoleTemplateBinding:
		ownerLabel = PrtbOwnerLabel
	case *v3.ClusterRoleTemplateBinding:
		ownerLabel = CrtbOwnerLabel
	default:
		return nil, fmt.Errorf("unrecognized roleTemplateBinding type: %T", rtb)
	}

	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "crb-",
			Labels:       map[string]string{ownerLabel: rtb.GetName()},
		},
		RoleRef:  roleRef,
		Subjects: []rbacv1.Subject{subject},
	}, nil
}

// AreClusterRoleBindingsSame compares the Subjects and RoleRef fields of two Cluster Role Bindings.
func AreClusterRoleBindingContentsSame(crb1, crb2 *rbacv1.ClusterRoleBinding) bool {
	return reflect.DeepEqual(crb1.Subjects, crb2.Subjects) &&
		reflect.DeepEqual(crb1.RoleRef, crb2.RoleRef)
}

// ClusterRoleNameFor returns safe version of a string to be used for a clusterRoleName
func ClusterRoleNameFor(s string) string {
	return name.SafeConcatName(s)
}

// PromotedClusterRoleNameFor appends the promoted suffix to a string safely (ie <= 63 characters)
func PromotedClusterRoleNameFor(s string) string {
	return name.SafeConcatName(s, promotedSuffix)
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

func GetPRTBOwnerLabel(s string) string {
	return PrtbOwnerLabel + "=" + s
}

func GetCRTBOwnerLabel(s string) string {
	return CrtbOwnerLabel + "=" + s
}

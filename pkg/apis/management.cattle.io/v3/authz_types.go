package v3

import (
	"strings"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	NamespaceBackedResource                   condition.Cond = "BackingNamespaceCreated"
	CreatorMadeOwner                          condition.Cond = "CreatorMadeOwner"
	DefaultNetworkPolicyCreated               condition.Cond = "DefaultNetworkPolicyCreated"
	ProjectConditionDefaultNamespacesAssigned condition.Cond = "DefaultNamespacesAssigned"
	ProjectConditionInitialRolesPopulated     condition.Cond = "InitialRolesPopulated"
	ProjectConditionSystemNamespacesAssigned  condition.Cond = "SystemNamespacesAssigned"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Project is a group of namespaces.
// Projects are used to create a multi-tenant environment within a Kubernetes cluster by managing namespace operations,
// such as role assignments or quotas, as a group.
type Project struct {
	types.Namespaced `json:",inline"`
	metav1.TypeMeta  `json:",inline"`

	// Standard object metadata; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the desired configuration for the project.
	// +optional
	Spec ProjectSpec `json:"spec,omitempty"`

	// Status is the most recently observed status of the project.
	// +optional
	Status ProjectStatus `json:"status,omitempty"`
}

func (p *Project) ObjClusterName() string {
	return p.Spec.ObjClusterName()
}

// ProjectStatus represents the most recently observed status of the project.
type ProjectStatus struct {
	// Conditions are a set of indicators about aspects of the project.
	// +optional
	Conditions []ProjectCondition `json:"conditions,omitempty"`
}

// ProjectCondition is the status of an aspect of the project.
type ProjectCondition struct {
	// Type of project condition.
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// Status of the condition, one of True, False, Unknown.
	// +kubebuilder:validation:Required
	Status v1.ConditionStatus `json:"status"`

	// The last time this condition was updated.
	// +optional
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`

	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`

	// The reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Human-readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty"`
}

// ProjectSpec is a description of the project.
type ProjectSpec struct {

	// DisplayName is the human-readable name for the project.
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName" norman:"required"`

	// Description is a human-readable description of the project.
	// +optional
	Description string `json:"description,omitempty"`

	// ClusterName is the name of the cluster the project belongs to. Immutable.
	// +kubebuilder:validation:Required
	ClusterName string `json:"clusterName" norman:"required,type=reference[cluster]"`

	// ResourceQuota is a specification for the total amount of quota for standard resources that will be shared by all namespaces in the project.
	// Must provide NamespaceDefaultResourceQuota if ResourceQuota is specified.
	// See https://kubernetes.io/docs/concepts/policy/resource-quotas/ for more details.
	// +optional
	ResourceQuota *ProjectResourceQuota `json:"resourceQuota,omitempty"`

	// NamespaceDefaultResourceQuota is a specification of the default ResourceQuota that a namespace will receive if none is provided.
	// Must provide ResourceQuota if NamespaceDefaultResourceQuota is specified.
	// See https://kubernetes.io/docs/concepts/policy/resource-quotas/ for more details.
	// +optional
	NamespaceDefaultResourceQuota *NamespaceResourceQuota `json:"namespaceDefaultResourceQuota,omitempty"`

	// ContainerDefaultResourceLimit is a specification for the default LimitRange for the namespace.
	// See https://kubernetes.io/docs/concepts/policy/limit-range/ for more details.
	// +optional
	ContainerDefaultResourceLimit *ContainerResourceLimit `json:"containerDefaultResourceLimit,omitempty"`
}

func (p *ProjectSpec) ObjClusterName() string {
	return p.ClusterName
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.summary"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalRole defines rules that can be applied to the local cluster and or every downstream cluster.
type GlobalRole struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object metadata; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// DisplayName is the human-readable name displayed in the UI for this resource.
	// +optional
	DisplayName string `json:"displayName,omitempty" norman:"required"`

	// Description holds text that describes the resource.
	// +optional
	Description string `json:"description,omitempty"`

	// Rules holds a list of PolicyRules that are applied to the local cluster only.
	// +optional
	Rules []rbacv1.PolicyRule `json:"rules,omitempty"`

	// NewUserDefault specifies that all new users created should be bound to this GlobalRole if true.
	// +optional
	NewUserDefault bool `json:"newUserDefault,omitempty" norman:"required"`

	// Builtin specifies that this GlobalRole was created by Rancher if true. Immutable.
	// +optional
	Builtin bool `json:"builtin,omitempty" norman:"nocreate,noupdate"`

	// InheritedClusterRoles are the names of RoleTemplates whose permissions are granted by this GlobalRole in every
	// cluster besides the local cluster. To grant permissions in the local cluster, use the Rules field.
	// +optional
	InheritedClusterRoles []string `json:"inheritedClusterRoles,omitempty"`

	// NamespacedRules are the rules that are active in each namespace of this GlobalRole.
	// These are applied to the local cluster only.
	// * has no special meaning in the keys - these keys are read as raw strings
	// and must exactly match with one existing namespace.
	// +optional
	NamespacedRules map[string][]rbacv1.PolicyRule `json:"namespacedRules,omitempty"`

	// InheritedFleetWorkspacePermissions are the permissions granted by this GlobalRole in every fleet workspace besides
	// the local one.
	// +optional
	InheritedFleetWorkspacePermissions *FleetWorkspacePermission `json:"inheritedFleetWorkspacePermissions,omitempty"`

	// Status is the most recently observed status of the GlobalRole.
	// +optional
	Status GlobalRoleStatus `json:"status,omitempty"`
}

// FleetWorkspacePermission defines permissions that will apply to all fleet workspaces except local.
type FleetWorkspacePermission struct {
	// ResourceRules rules granted in all backing namespaces for all fleet workspaces besides the local one.
	ResourceRules []rbacv1.PolicyRule `json:"resourceRules,omitempty" yaml:"resourceRules,omitempty"`
	// WorkspaceVerbs verbs used to grant permissions to the cluster-wide fleetworkspace resources. ResourceNames for
	// this rule will contain all fleet workspace names except local.
	WorkspaceVerbs []string `json:"workspaceVerbs,omitempty" yaml:"workspaceVerbs,omitempty"`
}

// GlobalRoleStatus represents the most recently observed status of the GlobalRole.
type GlobalRoleStatus struct {
	// ObservedGeneration is the most recent generation (metadata.generation in GlobalRole CR)
	// observed by the controller. Populated by the system.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastUpdate is a k8s timestamp of the last time the status was updated.
	// +optional
	LastUpdate string `json:"lastUpdateTime,omitempty"`

	// Summary is a string. One of "Complete", "InProgress" or "Error".
	// +optional
	Summary string `json:"summary,omitempty"`

	// Conditions is a slice of Condition, indicating the status of specific backing RBAC objects.
	// There is one condition per ClusterRole and Role managed by the GlobalRole.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalRoleBinding binds a given subject user or group to a GlobalRole.
type GlobalRoleBinding struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object metadata; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// UserName is the name of the user subject to be bound. Immutable.
	// +optional
	UserName string `json:"userName,omitempty" norman:"noupdate,type=reference[user]"`

	// GroupPrincipalName is the name of the group principal subject to be bound. Immutable.
	// +optional
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`

	// GlobalRoleName is the name of the Global Role that the subject will be bound to. Immutable.
	// +kubebuilder:validation:Required
	GlobalRoleName string `json:"globalRoleName" norman:"required,noupdate,type=reference[globalRole]"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RoleTemplate holds configuration for a template that is used to create kubernetes Roles and ClusterRoles
// (in the rbac.authorization.k8s.io group) for a cluster or project.
type RoleTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// DisplayName is the human-readable name displayed in the UI for this resource.
	DisplayName string `json:"displayName,omitempty" norman:"required"`

	// Description holds text that describes the resource.
	// +optional
	Description string `json:"description"`

	// Rules hold all the PolicyRules for this RoleTemplate.
	// +optional
	Rules []rbacv1.PolicyRule `json:"rules,omitempty"`

	// Builtin if true specifies that this RoleTemplate was created by Rancher and is immutable.
	// Default to false.
	// +optional
	Builtin bool `json:"builtin" norman:"nocreate,noupdate"`

	// External if true specifies that rules for this RoleTemplate should be gathered from a ClusterRole with the matching name.
	// If set to true the Rules on the template will not be evaluated.
	// External's value is only evaluated if the RoleTemplate's context is set to "cluster"
	// Default to false.
	// +optional
	External bool `json:"external"`

	// ExternalRules hold the external PolicyRules that will be used for authorization.
	// This field is required when External=true and no underlying ClusterRole exists in the local cluster.
	// This field is just used when the feature flag 'external-rules' is on.
	// +optional
	ExternalRules []rbacv1.PolicyRule `json:"externalRules,omitempty"`

	// Hidden if true informs the Rancher UI not to display this RoleTemplate.
	// Default to false.
	// +optional
	Hidden bool `json:"hidden"`

	// Locked if true, new bindings will not be able to use this RoleTemplate.
	// Default to false.
	// +optional
	Locked bool `json:"locked,omitempty" norman:"type=boolean"`

	// ClusterCreatorDefault if true, a binding with this RoleTemplate will be created for a users when they create a new cluster.
	// ClusterCreatorDefault is only evaluated if the context of the RoleTemplate is set to cluster.
	// Default to false.
	// +optional
	ClusterCreatorDefault bool `json:"clusterCreatorDefault,omitempty" norman:"required"`

	// ProjectCreatorDefault if true, a binding with this RoleTemplate will be created for a user when they create a new project.
	// ProjectCreatorDefault is only evaluated if the context of the RoleTemplate is set to project.
	// Default to false.
	// +optional
	ProjectCreatorDefault bool `json:"projectCreatorDefault,omitempty" norman:"required"`

	// Context describes if the roleTemplate applies to clusters or projects.
	// Valid values are "project", "cluster" or "".
	// +kubebuilder:validation:Enum={"project","cluster",""}
	Context string `json:"context,omitempty" norman:"type=string,options=project|cluster"`

	// RoleTemplateNames list of RoleTemplate names that this RoleTemplate will inherit.
	// This RoleTemplate will grant all rules defined in an inherited RoleTemplate.
	// Inherited RoleTemplates must already exist.
	// +optional
	RoleTemplateNames []string `json:"roleTemplateNames,omitempty" norman:"type=array[reference[roleTemplate]]"`

	// Administrative if false, and context is set to cluster this RoleTemplate will not grant access to "CatalogTemplates" and "CatalogTemplateVersions" for any project in the cluster.
	// Default is false.
	// +optional
	Administrative bool `json:"administrative,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectRoleTemplateBinding is the object representing membership of a subject in a project with permissions
// specified by a given role template.
type ProjectRoleTemplateBinding struct {
	types.Namespaced  `json:",inline"`
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// UserName is the name of the user subject added to the project. Immutable.
	// +optional
	UserName string `json:"userName,omitempty" norman:"noupdate,type=reference[user]"`

	// UserPrincipalName is the name of the user principal subject added to the project. Immutable.
	// +optional
	UserPrincipalName string `json:"userPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`

	// GroupName is the name of the group subject added to the project. Immutable.
	// +optional
	GroupName string `json:"groupName,omitempty" norman:"noupdate,type=reference[group]"`

	// GroupPrincipalName is the name of the group principal subject added to the project. Immutable.
	// +optional
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`

	// ProjectName is the name of the project to which a subject is added. Immutable.
	// +kubebuilder:validation:Required
	ProjectName string `json:"projectName" norman:"required,noupdate,type=reference[project]"`

	// RoleTemplateName is the name of the role template that defines permissions to perform actions on resources in the project. Immutable.
	// +kubebuilder:validation:Required
	RoleTemplateName string `json:"roleTemplateName" norman:"required,noupdate,type=reference[roleTemplate]"`

	// ServiceAccount is the name of the service account bound as a subject. Immutable.
	// Deprecated.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty" norman:"nocreate,noupdate"`
}

func (p *ProjectRoleTemplateBinding) ObjClusterName() string {
	if parts := strings.SplitN(p.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterRoleTemplateBinding is the object representing membership of a subject in a cluster with permissions
// specified by a given role template.
type ClusterRoleTemplateBinding struct {
	types.Namespaced `json:",inline"`
	metav1.TypeMeta  `json:",inline"`

	// Standard object metadata; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// UserName is the name of the user subject added to the cluster. Immutable.
	// +optional
	UserName string `json:"userName,omitempty" norman:"noupdate,type=reference[user]"`

	// UserPrincipalName is the name of the user principal subject added to the cluster. Immutable.
	// +optional
	UserPrincipalName string `json:"userPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`

	// GroupName is the name of the group subject added to the cluster. Immutable.
	// +optional
	GroupName string `json:"groupName,omitempty" norman:"noupdate,type=reference[group]"`

	// GroupPrincipalName is the name of the group principal subject added to the cluster. Immutable.
	// +optional
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`

	// ClusterName is the metadata.name of the cluster to which a subject is added.
	// Must match the namespace. Immutable.
	// +kubebuilder:validation:Required
	ClusterName string `json:"clusterName" norman:"required,noupdate,type=reference[cluster]"`

	// RoleTemplateName is the name of the role template that defines permissions to perform actions on resources in the cluster. Immutable.
	// +kubebuilder:validation:Required
	RoleTemplateName string `json:"roleTemplateName" norman:"required,noupdate,type=reference[roleTemplate]"`
}

func (c *ClusterRoleTemplateBinding) ObjClusterName() string {
	return c.ClusterName
}

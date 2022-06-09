package v3

import (
	"strings"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	NamespaceBackedResource                   condition.Cond = "BackingNamespaceCreated"
	CreatorMadeOwner                          condition.Cond = "CreatorMadeOwner"
	DefaultNetworkPolicyCreated               condition.Cond = "DefaultNetworkPolicyCreated"
	ProjectConditionDefaultNamespacesAssigned condition.Cond = "DefaultNamespacesAssigned"
	ProjectConditionInitialRolesPopulated     condition.Cond = "InitialRolesPopulated"
	ProjectConditionMonitoringEnabled         condition.Cond = "MonitoringEnabled"
	ProjectConditionMetricExpressionDeployed  condition.Cond = "MetricExpressionDeployed"
	ProjectConditionSystemNamespacesAssigned  condition.Cond = "SystemNamespacesAssigned"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Project struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec,omitempty"`
	Status ProjectStatus `json:"status"`
}

func (p *Project) ObjClusterName() string {
	return p.Spec.ObjClusterName()
}

type ProjectStatus struct {
	Conditions                    []ProjectCondition `json:"conditions"`
	PodSecurityPolicyTemplateName string             `json:"podSecurityPolicyTemplateId"`
	MonitoringStatus              *MonitoringStatus  `json:"monitoringStatus,omitempty" norman:"nocreate,noupdate"`
}

type ProjectCondition struct {
	// Type of project condition.
	Type string `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}

type ProjectSpec struct {
	DisplayName                   string                  `json:"displayName,omitempty" norman:"required"`
	Description                   string                  `json:"description"`
	ClusterName                   string                  `json:"clusterName,omitempty" norman:"required,type=reference[cluster]"`
	ResourceQuota                 *ProjectResourceQuota   `json:"resourceQuota,omitempty"`
	NamespaceDefaultResourceQuota *NamespaceResourceQuota `json:"namespaceDefaultResourceQuota,omitempty"`
	ContainerDefaultResourceLimit *ContainerResourceLimit `json:"containerDefaultResourceLimit,omitempty"`
	EnableProjectMonitoring       bool                    `json:"enableProjectMonitoring" norman:"default=false"`
}

func (p *ProjectSpec) ObjClusterName() string {
	return p.ClusterName
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GlobalRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName    string              `json:"displayName,omitempty" norman:"required"`
	Description    string              `json:"description"`
	Rules          []rbacv1.PolicyRule `json:"rules,omitempty"`
	NewUserDefault bool                `json:"newUserDefault,omitempty" norman:"required"`
	Builtin        bool                `json:"builtin" norman:"nocreate,noupdate"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GlobalRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	UserName           string `json:"userName,omitempty" norman:"noupdate,type=reference[user]"`
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`
	GlobalRoleName     string `json:"globalRoleName,omitempty" norman:"required,noupdate,type=reference[globalRole]"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RoleTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName           string              `json:"displayName,omitempty" norman:"required"`
	Description           string              `json:"description"`
	Rules                 []rbacv1.PolicyRule `json:"rules,omitempty"`
	Builtin               bool                `json:"builtin" norman:"nocreate,noupdate"`
	External              bool                `json:"external"`
	Hidden                bool                `json:"hidden"`
	Locked                bool                `json:"locked,omitempty" norman:"type=boolean"`
	ClusterCreatorDefault bool                `json:"clusterCreatorDefault,omitempty" norman:"required"`
	ProjectCreatorDefault bool                `json:"projectCreatorDefault,omitempty" norman:"required"`
	Context               string              `json:"context" norman:"type=string,options=project|cluster"`
	RoleTemplateNames     []string            `json:"roleTemplateNames,omitempty" norman:"type=array[reference[roleTemplate]]"`
	Administrative        bool                `json:"administrative,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PodSecurityPolicyTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Description string                         `json:"description"`
	Spec        policyv1.PodSecurityPolicySpec `json:"spec,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PodSecurityPolicyTemplateProjectBinding struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	PodSecurityPolicyTemplateName string `json:"podSecurityPolicyTemplateId" norman:"required,type=reference[podSecurityPolicyTemplate]"`
	TargetProjectName             string `json:"targetProjectId" norman:"required,type=reference[project]"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ProjectRoleTemplateBinding struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	UserName           string `json:"userName,omitempty" norman:"noupdate,type=reference[user]"`
	UserPrincipalName  string `json:"userPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`
	GroupName          string `json:"groupName,omitempty" norman:"noupdate,type=reference[group]"`
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`
	ProjectName        string `json:"projectName,omitempty" norman:"required,noupdate,type=reference[project]"`
	RoleTemplateName   string `json:"roleTemplateName,omitempty" norman:"required,type=reference[roleTemplate]"`
	ServiceAccount     string `json:"serviceAccount,omitempty" norman:"nocreate,noupdate"`
}

func (p *ProjectRoleTemplateBinding) ObjClusterName() string {
	if parts := strings.SplitN(p.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterRoleTemplateBinding struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	UserName           string `json:"userName,omitempty" norman:"noupdate,type=reference[user]"`
	UserPrincipalName  string `json:"userPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`
	GroupName          string `json:"groupName,omitempty" norman:"noupdate,type=reference[group]"`
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`
	ClusterName        string `json:"clusterName,omitempty" norman:"required,noupdate,type=reference[cluster]"`
	RoleTemplateName   string `json:"roleTemplateName,omitempty" norman:"required,type=reference[roleTemplate]"`
}

func (c *ClusterRoleTemplateBinding) ObjClusterName() string {
	return c.ClusterName
}

type SetPodSecurityPolicyTemplateInput struct {
	PodSecurityPolicyTemplateName string `json:"podSecurityPolicyTemplateId" norman:"required,type=reference[podSecurityPolicyTemplate]"`
}

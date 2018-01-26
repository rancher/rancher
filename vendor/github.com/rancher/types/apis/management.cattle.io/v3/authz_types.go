package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	"k8s.io/api/core/v1"
	extv1 "k8s.io/api/extensions/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	NamespaceBackedResource condition.Cond = "BackingNamespaceCreated"
	CreatorMadeOwner        condition.Cond = "CreatorMadeOwner"
)

type Project struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec,omitempty"`
	Status ProjectStatus `json:"status"`
}

type ProjectStatus struct {
	Conditions []ProjectCondition `json:"conditions"`
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
	DisplayName                   string `json:"displayName,omitempty" norman:"required"`
	Description                   string `json:"description"`
	ClusterName                   string `json:"clusterName,omitempty" norman:"required,type=reference[cluster]"`
	PodSecurityPolicyTemplateName string `json:"podSecurityPolicyTemplateName,omitempty" norman:"type=reference[podSecurityPolicyTemplate]"`
}

type GlobalRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName string              `json:"displayName,omitempty" norman:"required"`
	Description string              `json:"description"`
	Rules       []rbacv1.PolicyRule `json:"rules,omitempty"`
	Builtin     bool                `json:"builtin" norman:"nocreate,noupdate"`
}

type GlobalRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	UserName       string `json:"userName,omitempty" norman:"required,type=reference[user]"`
	GlobalRoleName string `json:"globalRoleName,omitempty" norman:"required,type=reference[globalRole]"`
}

type RoleTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName       string              `json:"displayName,omitempty" norman:"required"`
	Description       string              `json:"description"`
	Rules             []rbacv1.PolicyRule `json:"rules,omitempty"`
	Builtin           bool                `json:"builtin" norman:"nocreate,noupdate"`
	External          bool                `json:"external"`
	Hidden            bool                `json:"hidden"`
	Context           string              `json:"context" norman:"type=string,options=project|cluster"`
	RoleTemplateNames []string            `json:"roleTemplateNames,omitempty" norman:"type=array[reference[roleTemplate]]"`
}

type PodSecurityPolicyTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Description string                      `json:"description"`
	Spec        extv1.PodSecurityPolicySpec `json:"spec,omitempty"`
}

type ProjectRoleTemplateBinding struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	UserName         string `json:"userName,omitempty" norman:"required,type=reference[user]"`
	ProjectName      string `json:"projectName,omitempty" norman:"required,type=reference[project]"`
	RoleTemplateName string `json:"roleTemplateName,omitempty" norman:"required,type=reference[roleTemplate]"`
}

type ClusterRoleTemplateBinding struct {
	types.Namespaced
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	UserName         string `json:"userName,omitempty" norman:"required,type=reference[user]"`
	ClusterName      string `json:"clusterName,omitempty" norman:"required,type=reference[cluster]"`
	RoleTemplateName string `json:"roleTemplateName,omitempty" norman:"required,type=reference[roleTemplate]"`
}

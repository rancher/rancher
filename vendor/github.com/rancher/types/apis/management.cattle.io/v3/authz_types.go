package v3

import (
	extv1 "k8s.io/api/extensions/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProjectSpec `json:"spec,omitempty"`
}

type ProjectSpec struct {
	DisplayName                   string `json:"displayName,omitempty" norman:"required"`
	ClusterName                   string `json:"clusterName,omitempty" norman:"required,type=reference[cluster]"`
	PodSecurityPolicyTemplateName string `json:"podSecurityPolicyTemplateName,omitempty" norman:"type=reference[podSecurityPolicyTemplate]"`
}

type GlobalRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName string              `json:"displayName,omitempty" norman:"required"`
	Rules       []rbacv1.PolicyRule `json:"rules,omitempty"`
	Builtin     bool                `json:"builtin"`
}

type GlobalRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Subject        rbacv1.Subject `json:"subject,omitempty"`
	GlobalRoleName string         `json:"globalRoleName,omitempty" norman:"type=reference[globalRole]"`
}

type RoleTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName       string              `json:"displayName,omitempty" norman:"required"`
	Rules             []rbacv1.PolicyRule `json:"rules,omitempty"`
	Builtin           bool                `json:"builtin"`
	External          bool                `json:"external"`
	Hidden            bool                `json:"hidden"`
	RoleTemplateNames []string            `json:"roleTemplateNames,omitempty" norman:"type=array[reference[roleTemplate]]"`
}

type PodSecurityPolicyTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec extv1.PodSecurityPolicySpec `json:"spec,omitempty"`
}

type ProjectRoleTemplateBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Subject rbacv1.Subject `json:"subject,omitempty"`

	ProjectName      string `json:"projectName,omitempty" norman:"type=reference[project]"`
	RoleTemplateName string `json:"roleTemplateName,omitempty" norman:"type=reference[roleTemplate]"`
}

type ClusterRoleTemplateBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Subject rbacv1.Subject `json:"subject,omitempty"`

	ClusterName      string `json:"clusterName,omitempty" norman:"type=reference[cluster]"`
	RoleTemplateName string `json:"roleTemplateName,omitempty" norman:"type=reference[roleTemplate]"`
}

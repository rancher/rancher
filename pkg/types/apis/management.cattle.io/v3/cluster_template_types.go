package v3

import (
	"github.com/rancher/norman/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterTemplate struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterTemplateSpec `json:"spec"`
}

type ClusterTemplateSpec struct {
	DisplayName         string `json:"displayName" norman:"required"`
	Description         string `json:"description"`
	DefaultRevisionName string `json:"defaultRevisionName,omitempty" norman:"type=reference[clusterTemplateRevision]"`

	Members []Member `json:"members,omitempty"`
}

type ClusterTemplateRevision struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the the cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Spec ClusterTemplateRevisionSpec `json:"spec"`
}

type ClusterTemplateRevisionSpec struct {
	DisplayName         string `json:"displayName" norman:"required"`
	Enabled             *bool  `json:"enabled,omitempty" norman:"default=true"`
	ClusterTemplateName string `json:"clusterTemplateName,omitempty" norman:"type=reference[clusterTemplate],required,noupdate"`

	Questions     []Question       `json:"questions,omitempty"`
	ClusterConfig *ClusterSpecBase `json:"clusterConfig" norman:"required"`
}

type ClusterTemplateQuestionsOutput struct {
	Questions []Question `json:"questions,omitempty"`
}

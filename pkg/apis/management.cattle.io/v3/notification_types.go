package v3

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RancherUserNotification struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ComponentName string `json:"componentName"`
	Message       string `json:"message"`
}

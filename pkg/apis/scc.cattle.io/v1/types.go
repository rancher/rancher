package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
)

// RegistrationMode enforces the valid registration modes
// +kubebuilder:validation:Enum=online;offline
type RegistrationMode string

func (rm *RegistrationMode) Valid() bool {
	return *rm == Online || *rm == Offline
}

const (
	Online  RegistrationMode = "online"
	Offline RegistrationMode = "offline"
)

const (
	ResourceConditionDone        condition.Cond = "Done"
	ResourceConditionFailure     condition.Cond = "Failure"
	ResourceConditionProgressing condition.Cond = "Progressing"
	ResourceConditionReady       condition.Cond = "Ready"

	RegistrationConditionOfflineRequestReady     condition.Cond = "OfflineRequestReady"
	RegistrationConditionOfflineCertificateReady condition.Cond = "OfflineCertificateReady"
	ActivationConditionOfflineDone               condition.Cond = "OfflineActivationDone"

	RegistrationConditionAnnounced   condition.Cond = "RegistrationAnnounced"
	RegistrationConditionSccUrlReady condition.Cond = "RegistrationSccUrlReady"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Registration Active",type=boolean,JSONPath=`.status.activationStatus.activated`
// +kubebuilder:printcolumn:name="Last Sync",type="date",JSONPath=".status.activationStatus.lastValidatedTS"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Registration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistrationSpec   `json:"spec,omitempty"`
	Status RegistrationStatus `json:"status,omitempty"`
}

type RegistrationRequest struct {
	RegistrationCodeSecretRef *corev1.SecretReference `json:"registrationCodeSecretRef,omitempty"`
	// +optional
	SeverUrl string `json:"severUrl,omitempty"`
	// +optional
	ServerCertificateSecretRef *corev1.SecretReference `json:"serverCertficateSecretRef,omitempty"`
}

// RegistrationSpec is a description of a registration config
type RegistrationSpec struct {
	// +default:value="online"
	Mode RegistrationMode `json:"mode"`
	// +optional
	RegistrationRequest                     *RegistrationRequest    `json:"registrationRequest,omitempty"`
	OfflineRegistrationCertificateSecretRef *corev1.SecretReference `json:"offlineRegistrationCertificateSecretRef,omitempty"`
	CheckNow                                bool                    `json:"checkNow,omitempty"`
}

func (rs *RegistrationSpec) WithoutCheckNow() *RegistrationSpec {
	return &RegistrationSpec{
		Mode:                                    rs.Mode,
		RegistrationRequest:                     rs.RegistrationRequest,
		OfflineRegistrationCertificateSecretRef: rs.OfflineRegistrationCertificateSecretRef,
	}
}

type RegistrationStatus struct {
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// +optional
	RegistrationProcessedTS *metav1.Time `json:"registrationProcessedTS,omitempty"`
	// +optional
	SCCSystemId int `json:"sccSystemId,omitempty"`

	// +optional
	ActivationStatus SystemActivationState `json:"activationStatus,omitempty"`
	// +optional
	SystemCredentialsSecretRef *corev1.SecretReference `json:"systemCredentialsSecretRef,omitempty"`
	// +optional
	OfflineRegistrationRequest *corev1.SecretReference `json:"offlineRegistrationRequest,omitempty"`
}

type SystemActivationState struct {
	// +default:value=false
	Activated bool `json:"activated"`
	// +optional
	LastValidatedTS *metav1.Time `json:"lastValidatedTS"`
	// +optional
	ValidUntilTS *metav1.Time `json:"validUntilTS"`
	Certificate  string       `json:"certificate,omitempty"`
	SccUrl       string       `json:"sccUrl,omitempty"`
}

func (r *Registration) HasCondition(matchCond condition.Cond) bool {
	conditions := r.Status.Conditions
	for _, cond := range conditions {
		if cond.Type == string(matchCond) {
			return true
		}
	}

	return false
}

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
	return *rm == RegistrationModeOnline || *rm == RegistrationModeOffline
}

const (
	RegistrationModeOnline  RegistrationMode = "online"
	RegistrationModeOffline RegistrationMode = "offline"
)

// resource conditions ordered by: general-use, offline specific, general registration
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
	RegistrationConditionActivated   condition.Cond = "RegistrationActivated"
	RegistrationConditionKeepalive   condition.Cond = "RegistrationKeepalive"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.mode`
// +kubebuilder:printcolumn:name="Registration Active",type=boolean,JSONPath=`.status.activationStatus.activated`
// +kubebuilder:printcolumn:name="System ID",type=integer,JSONPath=`.status.sccSystemId`
// +kubebuilder:printcolumn:name="Last Sync",type="date",JSONPath=".status.activationStatus.lastValidatedTS"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Registration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistrationSpec   `json:"spec,omitempty"`
	Status RegistrationStatus `json:"status,omitempty"`
}

// RegistrationSpec is a description of a registration config
type RegistrationSpec struct {
	// +default:value="online"
	Mode RegistrationMode `json:"mode"`
	// +optional
	RegistrationRequest                     *RegistrationRequest    `json:"registrationRequest,omitempty"`
	OfflineRegistrationCertificateSecretRef *corev1.SecretReference `json:"offlineRegistrationCertificateSecretRef,omitempty"`
	SyncNow                                 *bool                   `json:"syncNow,omitempty"`
}

func (rs *RegistrationSpec) WithoutSyncNow() *RegistrationSpec {
	return &RegistrationSpec{
		Mode:                                    rs.Mode,
		RegistrationRequest:                     rs.RegistrationRequest,
		OfflineRegistrationCertificateSecretRef: rs.OfflineRegistrationCertificateSecretRef,
	}
}

type RegistrationRequest struct {
	RegistrationCodeSecretRef *corev1.SecretReference `json:"registrationCodeSecretRef,omitempty"`
	// +optional
	RegistrationAPIUrl *string `json:"registrationAPIUrl,omitempty"`
	// +optional
	RegistrationAPICertificateSecretRef *corev1.SecretReference `json:"registrationAPICertificateSecretRef,omitempty"`
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
	SCCSystemId *int `json:"sccSystemId,omitempty"`
	// +optional
	RegisteredProduct *string `json:"registeredProduct"`
	// +optional
	RegistrationExpiresAt *metav1.Time `json:"registrationExpiresAt,omitempty"`

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
	SystemUrl *string `json:"systemUrl,omitempty"`
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

func (r *Registration) ToOwnerRef() *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion: r.TypeMeta.APIVersion,
		Kind:       r.TypeMeta.Kind,
		UID:        r.GetUID(),
		Name:       r.GetName(),
	}
}

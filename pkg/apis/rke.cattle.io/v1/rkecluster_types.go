package v1

import (
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

type RKEClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint *capi.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`
}

type RKEClusterStatus struct {
	// Conditions is a representation of the current state of the RKE cluster.
	// +optional
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`

	// Ready denotes that the RKE cluster infrastructure is fully provisioned.
	// NOTE:
	// This field is part of the Cluster API contract, and it is used to
	// orchestrate provisioning.
	// The value of this field is never updated after provisioning is completed.
	// Please use conditions to check the operational state of the cluster.
	// Deprecated: use Initialization.Provisioned instead.
	// +deprecated
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Initialization provides observations of the RKECluster initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.
	// +optional
	Initialization RKEClusterInitializationStatus `json:"initialization,omitempty,omitzero"`
}

// RKEClusterInitializationStatus provides observations of the RKECluster initialization process.
// +kubebuilder:validation:MinProperties=1
type RKEClusterInitializationStatus struct {
	// Provisioned is true when the infrastructure provider reports that the Cluster's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Cluster provisioning.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=rkeclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels={"cluster.x-k8s.io/v1beta1=v1","cluster.x-k8s.io/v1beta2=v1","auth.cattle.io/cluster-indexed=true"}

// RKECluster represents the InfraCluster resource required by Cluster API
// to provide the necessary infrastructure prerequisites for running machines.
// It is referenced by the core Cluster API resource, Cluster.
// More info: https://cluster-api.sigs.k8s.io/developer/providers/contracts/infra-cluster
type RKECluster struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the RKECluster.
	Spec RKEClusterSpec `json:"spec"`

	// Status is the observed state of the RKECluster.
	// +optional
	Status RKEClusterStatus `json:"status,omitempty"`
}

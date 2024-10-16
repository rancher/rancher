// +kubebuilder:skip
package v1

// This file should contain type definitions for the ext.cattle.io/v1 API group.
// Below is an example of what it could look like. This comment can be removed once
// types have been added to this package.
//
//	import (
// 		metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	)
//
// 	// +genclient
// 	// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// 	type TestType struct {
// 		metav1.TypeMeta   `json:",inline"`
// 		metav1.ObjectMeta `json:"metadata,omitempty"`
//
// 		Spec TestTypeSpec `json:"ok"`
// 	}
//
// 	type TestTypeSpec struct {
// 		Enabled bool `json:"enabled"`
// 	}

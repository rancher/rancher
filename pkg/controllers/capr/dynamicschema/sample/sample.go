// +groupName=rke-machine.cattle.io
package sample

import (
	_ "embed"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/yaml"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed rke-machine.cattle.io_samples.yaml
var sampleBytes []byte

// ---
// This sample struct is used for generating a CRD for extracting
// some of the openapi field definitions for static types, so that they can be
// injected into dynamically generated CRDs. It wouldn't be possible to access their
// comments for descriptions at run-time by importing their schemas using wrangler.
//
// This sample CRD is only read as an embedded file, it is never installed
// in any cluster.
//
// This sample CRD is used to inject static fields in both the dynamically generated
// InfrastructureMachine and InfrastructureMachineTemplate CRDs, for each infrastructure
// provider.
type Sample struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec SampleSpec `json:"spec"`

	// Observed status of the Machine.
	// ---
	// The status field is only needed for the InfrastructureMachine CRD.
	Status rkev1.RKEMachineStatus `json:"status"`
}

type SampleSpec struct {
	// Generic machine configuration.
	Common rkev1.RKECommonNodeConfig `json:"common"`
}

func GetSampleProps() (*apiextv1.JSONSchemaProps, error) {
	var sampleCRD apiextv1.CustomResourceDefinition

	err := yaml.Unmarshal(sampleBytes, &sampleCRD)
	if err != nil {
		return nil, err
	}

	return sampleCRD.Spec.Versions[0].Schema.OpenAPIV3Schema, nil
}

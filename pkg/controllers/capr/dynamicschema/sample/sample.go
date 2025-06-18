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

type Sample struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec SampleSpec `json:"spec"`

	// Observed status of the Machine.
	Status rkev1.RKEMachineStatus `json:"status"`
}

type SampleSpec struct {
	// Machine configuration not specific to the infrastructure provider.
	Common rkev1.RKECommonNodeConfig `json:"common"`

	Foo string `json:"common"`
}

func GetSampleProps() (*apiextv1.JSONSchemaProps, error) {
	var sampleCRD apiextv1.CustomResourceDefinition

	err := yaml.Unmarshal(sampleBytes, &sampleCRD)
	if err != nil {
		return nil, err
	}

	return sampleCRD.Spec.Versions[0].Schema.OpenAPIV3Schema, nil
}

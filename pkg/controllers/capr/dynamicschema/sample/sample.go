// +groupName=rke-machine.cattle.io
package sample

import (
	"embed"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/yaml"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed rke-machine.cattle.io_samples.yaml
var crdFS embed.FS

type Sample struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// Spec for this sample
	Spec SampleSpec `json:"spec"`

	// Status for this sample
	Status rkev1.RKEMachineStatus `json:"status"`
}

type SampleSpec struct {
	// Common RKE node config
	Common rkev1.RKECommonNodeConfig `json:"common"`
}

func GetSampleProps() (*apiextv1.JSONSchemaProps, error) {
	sampleCRDFile, err := crdFS.ReadFile("rke-machine.cattle.io_samples.yaml")
	if err != nil {
		return nil, err
	}

	var sampleCRD apiextv1.CustomResourceDefinition
	err = yaml.Unmarshal(sampleCRDFile, &sampleCRD)
	if err != nil {
		return nil, err
	}

	return sampleCRD.Spec.Versions[0].Schema.OpenAPIV3Schema, nil
}

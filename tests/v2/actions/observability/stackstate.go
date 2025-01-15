package observability

import (
	"context"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/defaults"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Public Constants
	StackstateName         = "stackstate"
	ObservabilitySteveType = "configurations.observability.rancher.io"
	CrdGroup               = "observability.rancher.io"
	ApiExtenisonsCRD       = "apiextensions.k8s.io.customresourcedefinition"

	// Private Constants
	localURL      = "local://"
	inactiveState = "inactive"
	activeState   = "active"
)

// NewStackstateCRDConfiguration is a constructor that takes in the configuration and creates an unstructured type to install the CRD
func NewStackstateCRDConfiguration(namespace, name string, stackstateCRDConfig *StackStateConfig) *unstructured.Unstructured {

	crdConfig := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"url":          stackstateCRDConfig.Url,
				"serviceToken": stackstateCRDConfig.ServiceToken,
			},
		},
	}
	return crdConfig
}

// WhitelistStackstateDomains is a helper that utilizes the rancher client and add the stackstate domains to whitelist them.
// This is a temporary solution from upstream and will need to be removed once this has been fixed.
func WhitelistStackstateDomains(client *rancher.Client, whitelistDomains []string) error {

	nodedriver := &management.NodeDriver{
		Name:             StackstateName,
		Active:           false,
		WhitelistDomains: whitelistDomains,
		URL:              localURL,
		State:            inactiveState,
	}

	stackstateNodeDriver, err := client.Management.NodeDriver.Create(nodedriver)
	if err != nil {
		return err
	}

	err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.TwoMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		resp, err := client.Management.NodeDriver.ByID(stackstateNodeDriver.ID)
		if err != nil {
			return false, err
		}

		if resp.State == inactiveState {
			return true, nil
		}
		return false, nil
	})

	return err
}

// InstallStackstateCRD is a helper that utilizes the rancher client and installs the stackstate crds.
func InstallStackstateCRD(client *rancher.Client) error {
	stackstateCRDConfig := apiextv1.CustomResourceDefinition{
		TypeMeta:   metav1.TypeMeta{Kind: "CustomResourceDefinition", APIVersion: "apiextensions.k8s.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: ObservabilitySteveType},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: CrdGroup,
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				0: {Name: "v1beta1",
					Served:  true,
					Storage: true,
					Schema: &apiextv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextv1.JSONSchemaProps{
								"spec": {
									Type: "object",
									Properties: map[string]apiextv1.JSONSchemaProps{
										"url": {Type: "string"},
										"serviceToken": {
											Type: "string",
										},
										"apiToken": {
											Type: "string",
										},
									},
								},
							},
						},
					},
				},
			},
			Names: apiextv1.CustomResourceDefinitionNames{
				Plural:   "configurations",
				Singular: "configuration",
				Kind:     "Configuration",
				ListKind: "ConfigurationList",
			},
			Scope: "Namespaced",
		},
	}

	crd, err := client.Steve.SteveType(ApiExtenisonsCRD).Create(stackstateCRDConfig)
	if err != nil {
		return err
	}

	client.Session.RegisterCleanupFunc(func() error {
		err := client.Steve.SteveType(ApiExtenisonsCRD).Delete(crd)
		if err != nil {
			return err
		}

		err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.TenSecondTimeout, true, func(ctx context.Context) (done bool, err error) {
			_, err = client.Steve.SteveType(ApiExtenisonsCRD).ByID(crd.ID)
			if err != nil {
				return false, nil
			}
			return done, nil
		})

		return err
	})

	err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.TwoMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		resp, err := client.Steve.SteveType(ApiExtenisonsCRD).ByID(ObservabilitySteveType)
		if err != nil {
			return false, err
		}

		if resp.ObjectMeta.State.Name == activeState {
			return true, nil
		}
		return false, nil
	})

	return err

}

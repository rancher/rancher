package dashboard

import (
	"context"

	fleetv1alpha1api "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	uiv1 "github.com/rancher/rancher/pkg/apis/ui.cattle.io/v1"
	"github.com/rancher/rancher/pkg/crds"
	"github.com/rancher/rancher/pkg/crds/provisioningv2"
	"github.com/rancher/rancher/pkg/features"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/wrangler/v2/pkg/apply"
	"github.com/rancher/wrangler/v2/pkg/crd"
	"github.com/rancher/wrangler/v2/pkg/generated/controllers/apiextensions.k8s.io"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

var (
	bootstrapFleet = map[string]interface{}{
		"bundles.fleet.cattle.io":       fleetv1alpha1api.Bundle{},
		"clusters.fleet.cattle.io":      fleetv1alpha1api.Cluster{},
		"clustergroups.fleet.cattle.io": fleetv1alpha1api.ClusterGroup{},
	}
)

func FeatureCRD() crd.CRD {
	return newCRD(&v3.Feature{}, func(c crd.CRD) crd.CRD {
		c.NonNamespace = true
		return c.
			WithColumn("Custom Value", ".spec.value").
			WithColumn("Default", ".status.default").
			WithColumn("Description", ".status.description")
	})
}

func List(cfg *rest.Config) (_ []crd.CRD, err error) {
	result := []crd.CRD{
		newCRD(&uiv1.NavLink{}, func(c crd.CRD) crd.CRD {
			c.Status = false
			c.NonNamespace = true
			c.GVK.Kind = "NavLink"
			c.GVK.Group = "ui.cattle.io"
			c.GVK.Version = "v1"
			return c
		}),
		newCRD(&v3.PodSecurityAdmissionConfigurationTemplate{}, func(c crd.CRD) crd.CRD {
			c.NonNamespace = true
			c.GVK.Kind = "PodSecurityAdmissionConfigurationTemplate"
			c.GVK.Version = "v3"
			return c
		}),
		newCRD(&v3.Cluster{}, func(c crd.CRD) crd.CRD {
			c.Status = false
			c.NonNamespace = true
			c.GVK.Kind = "Cluster"
			c.GVK.Group = "management.cattle.io"
			c.GVK.Version = "v3"
			c.SchemaObject = nil
			return c
		}),
		newCRD(&v3.APIService{}, func(c crd.CRD) crd.CRD {
			c.NonNamespace = true
			c.Status = true
			return c
		}),
		newCRD(&v3.ClusterRegistrationToken{}, func(c crd.CRD) crd.CRD {
			return c
		}),
		newCRD(&v3.Setting{}, func(c crd.CRD) crd.CRD {
			c.NonNamespace = true
			return c.
				WithColumn("Value", ".value")
		}),
		newCRD(&v3.Preference{}, func(c crd.CRD) crd.CRD {
			return c.
				WithColumn("Value", ".value")
		}),
		FeatureCRD(),
		newCRD(&catalogv1.ClusterRepo{}, func(c crd.CRD) crd.CRD {
			c.NonNamespace = true
			return c.
				WithStatus().
				WithCategories("catalog").
				WithColumn("URL", ".spec.url")
		}),
		newCRD(&catalogv1.Operation{}, func(c crd.CRD) crd.CRD {
			return c.
				WithStatus().
				WithCategories("catalog").
				WithColumn("Target Namespace", ".status.podNamespace").
				WithColumn("Command", ".status.command")
		}),
		newCRD(&catalogv1.App{}, func(c crd.CRD) crd.CRD {
			return c.
				WithStatus().
				WithCategories("catalog").
				WithColumn("Chart", ".spec.chart.metadata.name").
				WithColumn("Version", ".spec.chart.metadata.version").
				WithColumn("Release Name", ".spec.name").
				WithColumn("Release Version", ".spec.version").
				WithColumn("Status", ".spec.info.status")
		}),
	}

	if features.Fleet.Enabled() {
		result = append(result, crd.CRD{
			SchemaObject: v3.FleetWorkspace{},
			NonNamespace: true,
		})
		result, err = fleetBootstrap(result, cfg)
		if err != nil {
			return nil, err
		}
		if features.ProvisioningV2.Enabled() {
			result = append(result, crd.CRD{
				SchemaObject: v3.ManagedChart{},
			}.WithStatus())
		}
	}

	if features.ProvisioningV2.Enabled() {
		result = append(result, provisioningv2.List()...)
	}
	for i := len(result) - 1; i >= 0; i-- {
		if crds.MigratedResources[result[i].Name()] {
			// remove the migrated resource from the result slice so we do not install a dynamic definition
			result = append(result[:i], result[i+1:]...)
		}
	}
	return result, nil
}

func fleetBootstrap(crds []crd.CRD, cfg *rest.Config) ([]crd.CRD, error) {
	if !features.Fleet.Enabled() {
		return crds, nil
	}

	f, err := apiextensions.NewFactoryFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	for name, schemaObj := range bootstrapFleet {
		_, err = f.Apiextensions().V1().CustomResourceDefinition().Get(name, metav1.GetOptions{})
		if err == nil {
			continue
		} else if !apierror.IsNotFound(err) {
			return nil, err
		}

		crds = append(crds, crd.CRD{
			SchemaObject: schemaObj,
			Status:       true,
			// Ensure labels/annotations are set so that helm will manage this
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "Helm",
			},
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      fleetconst.CRDChartName,
				"meta.helm.sh/release-namespace": fleetconst.ReleaseNamespace,
			},
		})
	}

	return crds, nil
}

func Webhooks() []runtime.Object {
	if features.ProvisioningV2.Enabled() {
		return provisioningv2.Webhooks()
	}
	return nil
}

func CreateFeatureCRD(ctx context.Context, cfg *rest.Config) error {
	factory, err := crd.NewFactoryFromClient(cfg)
	if err != nil {
		return err
	}

	return factory.BatchCreateCRDs(ctx, FeatureCRD()).BatchWait()
}

func Create(ctx context.Context, cfg *rest.Config) error {
	factory, err := crd.NewFactoryFromClient(cfg)
	if err != nil {
		return err
	}

	apply, err := apply.NewForConfig(cfg)
	if err != nil {
		return err
	}
	apply = apply.
		WithSetID("crd-webhooks").
		WithDynamicLookup().
		WithNoDelete()
	if err := apply.ApplyObjects(Webhooks()...); err != nil {
		return err
	}

	crds, err := List(cfg)
	if err != nil {
		return err
	}

	return factory.BatchCreateCRDs(ctx, crds...).BatchWait()
}

func newCRD(obj interface{}, customize func(crd.CRD) crd.CRD) crd.CRD {
	crd := crd.CRD{
		SchemaObject: obj,
	}
	if customize != nil {
		crd = customize(crd)
	}
	return crd
}

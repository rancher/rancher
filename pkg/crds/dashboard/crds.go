package dashboard

import (
	"context"

	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/pkg/crd"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

func List() []crd.CRD {
	return []crd.CRD{
		newCRD(&v3.Cluster{}, func(c crd.CRD) crd.CRD {
			c.Status = false
			c.NonNamespace = true
			c.GVK.Kind = "Cluster"
			c.GVK.Group = "management.cattle.io"
			c.GVK.Version = "v3"
			c.SchemaObject = nil
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
		newCRD(&v3.Feature{}, func(c crd.CRD) crd.CRD {
			c.NonNamespace = true
			return c.
				WithColumn("Custom Value", ".spec.value").
				WithColumn("Default", ".status.default").
				WithColumn("Description", ".status.description")
		}),
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
		newCRD(&catalogv1.Release{}, func(c crd.CRD) crd.CRD {
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
}

func Objects() (result []runtime.Object, err error) {
	for _, crdDef := range List() {
		crd, err := crdDef.ToCustomResourceDefinition()
		if err != nil {
			return nil, err
		}
		result = append(result, &crd)
	}
	return
}

func Create(ctx context.Context, cfg *rest.Config) error {
	factory, err := crd.NewFactoryFromClient(cfg)
	if err != nil {
		return err
	}

	return factory.BatchCreateCRDs(ctx, List()...).BatchWait()
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

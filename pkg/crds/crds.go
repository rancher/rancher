package crds

import (
	"context"

	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/wrangler/pkg/crd"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

func List() []crd.CRD {
	return []crd.CRD{
		newCRD(&catalogv1.Repo{}, func(c crd.CRD) crd.CRD {
			return c.
				WithCategories("catalog").
				WithColumn("URL", ".spec.url")
		}),
		newCRD(&catalogv1.ClusterRepo{}, func(c crd.CRD) crd.CRD {
			return c.
				WithCategories("catalog").
				WithColumn("URL", ".spec.url")
		}),
		newCRD(&catalogv1.Operation{}, func(c crd.CRD) crd.CRD {
			return c.
				WithCategories("catalog").
				WithColumn("Target Namespace", ".status.podNamespace").
				WithColumn("Command", ".status.command")
		}),
		newCRD(&catalogv1.Release{}, func(c crd.CRD) crd.CRD {
			return c.
				WithCategories("catalog").
				WithColumn("Chart", ".spec.chart.metadata.name").
				WithColumn("Version", ".spec.chart.metadata.version").
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
		Status:       true,
		SchemaObject: obj,
	}
	if customize != nil {
		crd = customize(crd)
	}
	return crd
}

package dashboard

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/wrangler/pkg/crd"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

func List() []crd.CRD {
	return []crd.CRD{
		// Add CRDs here in the same style as pkg/crds/dashboard
		newCRD(v3.FleetWorkspace{}, func(c crd.CRD) crd.CRD {
			c.NonNamespace = true
			return c
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

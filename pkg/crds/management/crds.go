package dashboard

import (
	"context"

	"github.com/rancher/wrangler/pkg/crd"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

func List() []crd.CRD {
	return []crd.CRD{
		// Add CRDs hear in the same style as pkg/crds/dashboard
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

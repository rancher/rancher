package ingress

import (
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/config"
	"github.com/rancher/workload-controller/controller/ingress"
	"github.com/satori/uuid"
)

type Store struct {
	types.Store
	proxyStore *proxy.Store
	controller *ingress.Controller
}

func NewStore(workload *config.WorkloadContext) *Store {
	return &Store{
		Store: proxy.NewProxyStore(workload.UnversionedClient,
			[]string{"apis"},
			"extensions",
			"v1beta1",
			"Ingress",
			"ingresses"),
		proxyStore: proxy.NewRawProxyStore(workload.UnversionedClient,
			[]string{"apis"},
			"extensions",
			"v1beta1",
			"Ingress",
			"ingresses"),
		controller: ingress.NewIngressWorkloadController(workload),
	}
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	data["uuid"] = uuid.NewV4().String()
	_, err := s.controller.Reconcile(data, true)
	if err != nil {
		return nil, err
	}
	return s.Store.Create(apiContext, schema, data)
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	existing, err := s.Store.ByID(apiContext, schema, id)
	if err != nil {
		return existing, err
	}

	for k, v := range data {
		existing[k] = v
	}

	_, err = s.controller.Reconcile(existing, true)
	if err != nil {
		return nil, err
	}

	return s.Store.Update(apiContext, schema, data, id)
}

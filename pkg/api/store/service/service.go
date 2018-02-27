package service

import (
	"strconv"

	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	v3 "github.com/rancher/types/client/project/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func New(store types.Store) types.Store {
	return &transform.Store{
		Store: &Store{
			store,
		},
		Transformer: func(apiContext *types.APIContext, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			ownerReferences, ok := values.GetSlice(data, "ownerReferences")
			if !ok {
				return data, nil
			}

			for _, ownerReference := range ownerReferences {
				controller, _ := ownerReference["controller"].(bool)
				if controller {
					return nil, nil
				}
			}
			return data, nil
		},
	}
}

type Store struct {
	types.Store
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	formatData(data)
	data, err := p.Store.Create(apiContext, schema, data)
	return data, err
}

func formatData(data map[string]interface{}) {
	var ports []interface{}
	port := int64(42)
	servicePort := v3.ServicePort{
		Port:       &port,
		TargetPort: intstr.Parse(strconv.FormatInt(42, 10)),
		Protocol:   "TCP",
		Name:       "default",
	}
	m, err := convert.EncodeToMap(servicePort)
	if err != nil {
		logrus.Warnf("Failed to transform service port to map: %v", err)
		return
	}
	ports = append(ports, m)
	data["ports"] = ports
}

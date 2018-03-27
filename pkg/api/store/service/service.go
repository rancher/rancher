package service

import (
	"strconv"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v3 "github.com/rancher/types/client/project/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func New(store types.Store) types.Store {
	return &Store{
		store,
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
	servicePort := v3.ServicePort{
		Port:       42,
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

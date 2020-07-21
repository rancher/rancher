package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
)

const (
	extIPField = "externalIpAddress"
)

type NodeAddressMapper struct {
}

func (n NodeAddressMapper) FromInternal(data map[string]interface{}) {
	addresses, _ := values.GetSlice(data, "addresses")
	for _, address := range addresses {
		t := address["type"]
		a := address["address"]
		if t == "InternalIP" {
			data["ipAddress"] = a
		} else if t == "ExternalIP" {
			data[extIPField] = a
		} else if t == "Hostname" {
			data["hostname"] = a
		}
	}
}

func (n NodeAddressMapper) ToInternal(data map[string]interface{}) error {
	return nil
}

func (n NodeAddressMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}

type NodeAddressAnnotationMapper struct {
}

func (n NodeAddressAnnotationMapper) FromInternal(data map[string]interface{}) {
	externalIP, ok := values.GetValue(data, "status", "nodeAnnotations", "rke.cattle.io/external-ip")
	if ok {
		data[extIPField] = externalIP
	}
}

func (n NodeAddressAnnotationMapper) ToInternal(data map[string]interface{}) error {
	return nil
}

func (n NodeAddressAnnotationMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}

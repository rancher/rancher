package mapper

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/mapper"
	"github.com/sirupsen/logrus"
)

type ContainerPorts struct {
}

func (n ContainerPorts) FromInternal(data map[string]interface{}) {
	field := mapper.AnnotationField{
		Field: "ports",
		List:  true,
	}
	field.FromInternal(data)

	containers := convert.ToInterfaceSlice(data["containers"])
	ports := convert.ToInterfaceSlice(data["ports"])

	for i := 0; i < len(ports) && i < len(containers); i++ {
		container := convert.ToMapInterface(containers[i])
		if container != nil {
			portsSlice := convert.ToInterfaceSlice(ports[i])
			var containerPorts []interface{}
			for _, port := range portsSlice {
				asMap, err := convert.EncodeToMap(port)
				if err != nil {
					logrus.Warnf("Failed to convert container port to map %v", err)
					continue
				}
				asMap["type"] = "/v3/project/schemas/containerPort"
				containerPorts = append(containerPorts, asMap)
			}

			container["ports"] = containerPorts
		}
	}
}

func (n ContainerPorts) ToInternal(data map[string]interface{}) error {
	field := mapper.AnnotationField{
		Field: "ports",
		List:  true,
	}

	var ports []interface{}
	path := []string{"containers", "{ARRAY}", "ports"}
	convert.Transform(data, path, func(obj interface{}) interface{} {
		if l, ok := obj.([]interface{}); ok {
			for _, p := range l {
				mapped, err := convert.EncodeToMap(p)
				if err != nil {
					logrus.Warnf("Failed to encode port: %v", err)
					return obj
				}
				if strings.EqualFold(convert.ToString(mapped["kind"]), "HostPort") {
					if _, ok := mapped["sourcePort"]; ok {
						mapped["hostPort"] = mapped["sourcePort"]
					}
				}
			}
			ports = append(ports, l)
		}
		return obj
	})

	if len(ports) != 0 {
		data["ports"] = ports
		return field.ToInternal(data)
	}

	return nil
}

func (n ContainerPorts) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}

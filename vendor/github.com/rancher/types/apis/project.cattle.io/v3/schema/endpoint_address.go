package schema

import (
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/mapper"
	"k8s.io/api/core/v1"
)

type EndpointAddressMapper struct {
}

func (e EndpointAddressMapper) FromInternal(data map[string]interface{}) {
	if data == nil {
		return
	}

	var subsets []v1.EndpointSubset
	if err := convert.ToObj(data["subsets"], &subsets); err != nil {
		log.Errorf("Failed to convert subset: %v", err)
		return
	}

	var noPortsIPs []string
	var noPortsUnavailIPs []string
	var podIDs []string
	var result []interface{}
	for _, subset := range subsets {
		var ips []string
		var unAvailIPs []string
		for _, ip := range subset.Addresses {
			if ip.IP != "" {
				ips = append(ips, ip.IP)
			}
			if ip.Hostname != "" {
				ips = append(ips, ip.Hostname)
			}
			if ip.TargetRef != nil && ip.TargetRef.Kind == "Pod" {
				podIDs = append(podIDs, fmt.Sprintf("%s:%s", ip.TargetRef.Namespace,
					ip.TargetRef.Name))
			}
		}

		for _, ip := range subset.NotReadyAddresses {
			if ip.IP != "" {
				unAvailIPs = append(ips, ip.IP)
			}
			if ip.Hostname != "" {
				unAvailIPs = append(ips, ip.Hostname)
			}
		}

		if len(subset.Ports) == 0 {
			noPortsIPs = append(noPortsIPs, ips...)
			noPortsUnavailIPs = append(noPortsIPs, unAvailIPs...)
		} else {
			for _, port := range subset.Ports {
				if len(ips) > 0 {
					result = append(result, map[string]interface{}{
						"addresses":         ips,
						"notReadyAddresses": unAvailIPs,
						"port":              port.Port,
						"protocol":          port.Protocol,
					})
				}
			}
		}
	}

	if len(noPortsIPs) > 0 {
		result = append(result, map[string]interface{}{
			"addresses":         noPortsIPs,
			"notReadyAddresses": noPortsUnavailIPs,
		})
	}

	if len(result) > 0 {
		data["targets"] = result
	}
	if len(podIDs) > 0 {
		data["podIds"] = podIDs
	}
}

func (e EndpointAddressMapper) ToInternal(data map[string]interface{}) {
	if data == nil {
		return
	}

	var addresses []Target
	var subsets []v1.EndpointSubset
	if err := convert.ToObj(data["targets"], &addresses); err != nil {
		log.Errorf("Failed to convert addresses: %v", err)
		return
	}

	for _, address := range addresses {
		subset := v1.EndpointSubset{}
		for _, ip := range address.Addresses {
			subset.Addresses = append(subset.Addresses, v1.EndpointAddress{
				IP: ip,
			})
		}
		if address.Port != nil {
			subset.Ports = append(subset.Ports, v1.EndpointPort{
				Port:     *address.Port,
				Protocol: v1.Protocol(address.Protocol),
			})
		}
		subsets = append(subsets, subset)
	}

	if len(subsets) > 0 {
		data["subsets"] = subsets
	}
}

func (e EndpointAddressMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	mapper.ValidateField("subsets", schema)
	delete(schema.ResourceFields, "subsets")
	return nil
}

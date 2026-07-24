package mapper

import (
	"net/netip"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

const (
	extIPField   = "externalIpAddress"
	ipv4Field    = "ipv4Address"
	ipv6Field    = "ipv6Address"
	extIPv4Field = "externalIpv4Address"
	extIPv6Field = "externalIpv6Address"
)

type NodeAddressMapper struct {
}

func (n NodeAddressMapper) FromInternal(data map[string]interface{}) {
	addresses, _ := values.GetSlice(data, "addresses")
	for _, address := range addresses {
		t := convert.ToString(address["type"])
		a := convert.ToString(address["address"])
		if a == "" {
			continue
		}

		switch t {
		case "InternalIP":
			data["ipAddress"] = a
			setFamilyAddress(data, a, ipv4Field, ipv6Field)
		case "ExternalIP":
			data[extIPField] = a
			setFamilyAddress(data, a, extIPv4Field, extIPv6Field)
		case "Hostname":
			data["hostname"] = a
		}
	}
}

// setFamilyAddress records the first IPv4 and IPv6 address seen for the given fields.
// Existing ipAddress/externalIpAddress remain last-wins for backward compatibility.
func setFamilyAddress(data map[string]interface{}, address, v4Field, v6Field string) {
	addr, err := netip.ParseAddr(address)
	if err != nil {
		return
	}
	if addr.Is4() || addr.Is4In6() {
		if _, exists := data[v4Field]; !exists {
			data[v4Field] = addr.Unmap().String()
		}
		return
	}
	if addr.Is6() {
		if _, exists := data[v6Field]; !exists {
			data[v6Field] = addr.String()
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

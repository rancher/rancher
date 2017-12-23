package schema

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

type ServiceKindMapper struct {
}

func (s ServiceKindMapper) FromInternal(data map[string]interface{}) {
	if data == nil {
		return
	}

	targetWorkloadIds := data["targetWorkloadIds"]
	targetServiceIds := data["targetServiceIds"]
	clusterIP := data["clusterIp"]
	hostname := data["hostname"]

	if !convert.IsEmpty(targetWorkloadIds) || !convert.IsEmpty(targetServiceIds) {
		data["serviceKind"] = "Alias"
	} else if !convert.IsEmpty(hostname) {
		data["serviceKind"] = "CName"
	} else if clusterIP == "None" {
		data["serviceKind"] = "ARecord"
	}
}

func (s ServiceKindMapper) ToInternal(data map[string]interface{}) {
	if data == nil {
		return
	}

	str := convert.ToString(data["serviceKind"])
	switch str {
	case "Alias":
		fallthrough
	case "ARecord":
		fallthrough
	case "CName":
		data["serviceKind"] = "ClusterIP"
		data["clusterIp"] = "None"
	}

	if !convert.IsEmpty(data["hostname"]) {
		data["kind"] = "ExternalName"
		data["clusterIp"] = "None"
	}
}

func (s ServiceKindMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}

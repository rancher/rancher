package mapper

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

type OSInfo struct {
}

func (o OSInfo) FromInternal(data map[string]interface{}) {
	if data == nil {
		return
	}
	cpuInfo := map[string]interface{}{
		"count": values.GetValueN(data, "capacity", "cpu"),
	}

	kib := strings.TrimSuffix(convert.ToString(values.GetValueN(data, "capacity", "memory")), "Ki")
	memoryInfo := map[string]interface{}{}

	kibNum, err := convert.ToNumber(kib)
	if err == nil {
		memoryInfo["memTotalKiB"] = kibNum
	}

	osInfo := map[string]interface{}{
		"dockerVersion":   strings.TrimPrefix(convert.ToString(values.GetValueN(data, "nodeInfo", "containerRuntimeVersion")), "docker://"),
		"kernelVersion":   values.GetValueN(data, "nodeInfo", "kernelVersion"),
		"operatingSystem": values.GetValueN(data, "nodeInfo", "osImage"),
	}

	data["info"] = map[string]interface{}{
		"cpu":    cpuInfo,
		"memory": memoryInfo,
		"os":     osInfo,
		"kubernetes": map[string]interface{}{
			"kubeletVersion":   values.GetValueN(data, "nodeInfo", "kubeletVersion"),
			"kubeProxyVersion": values.GetValueN(data, "nodeInfo", "kubeletVersion"),
		},
	}
}

func (o OSInfo) ToInternal(data map[string]interface{}) error {
	return nil
}

func (o OSInfo) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}

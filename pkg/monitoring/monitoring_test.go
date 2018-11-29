package monitoring

import (
	"reflect"
	"testing"
)

func TestOverwriteAppAnswers(t *testing.T) {
	rawAnswers := make(map[string]string, 8)
	overwriteAnswers := map[string]string{
		"_key-Data_Retention":              "prometheus.retention",
		"_key-Node_Exporter_Host_Port":     "exporter-node.ports.metrics.port",
		"_key-Grafana_Storage_Enabled":     "grafana.persistence.enabled",
		"_tpl-Node_Selector":               "nodeSelector#(prometheus,grafana,exporter-kube-state)",
		"_tpl-Storage_Class":               "persistence#(prometheus,grafana)",
		"prometheus.retention":             "360h",
		"exporter-node.ports.metrics.port": "9100",
		"grafana.persistence.enabled":      "false",
		"nodeSelector.region":              "region-a",
		"nodeSelector.zone":                "zone-b",
		"persistence.enabled":              "true",
		"persistence.storageClass":         "default",
		"persistence.accessMode":           "ReadWriteOnce",
		"persistence.size":                 "50Gi",
	}

	rawAnswers = OverwriteAppAnswers(rawAnswers, overwriteAnswers)
	if !reflect.DeepEqual(rawAnswers, map[string]string{
		"prometheus.retention":                    "360h",
		"exporter-node.ports.metrics.port":        "9100",
		"grafana.persistence.enabled":             "false",
		"prometheus.nodeSelector.region":          "region-a",
		"prometheus.nodeSelector.zone":            "zone-b",
		"grafana.nodeSelector.region":             "region-a",
		"grafana.nodeSelector.zone":               "zone-b",
		"exporter-kube-state.nodeSelector.region": "region-a",
		"exporter-kube-state.nodeSelector.zone":   "zone-b",
		"prometheus.persistence.enabled":          "true",
		"prometheus.persistence.storageClass":     "default",
		"prometheus.persistence.accessMode":       "ReadWriteOnce",
		"prometheus.persistence.size":             "50Gi",
		"grafana.persistence.storageClass":        "default",
		"grafana.persistence.accessMode":          "ReadWriteOnce",
		"grafana.persistence.size":                "50Gi",
	}) {
		t.Error("failed")
		return
	}

	t.Log("success")
}

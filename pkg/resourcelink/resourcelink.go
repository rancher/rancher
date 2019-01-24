package resourcelink

var ExportResourcePrefixMappings = map[string]string{
	"pods":                   "api/v1",
	"configmaps":             "api/v1",
	"services":               "api/v1",
	"replicationcontrollers": "api/v1",
	"deployments":            "apis/extensions/v1beta1",
	"daemonsets":             "apis/extensions/v1beta1",
	"replicasets":            "apis/extensions/v1beta1",
	"statefulsets":           "apis/apps/v1beta1",
	"jobs":                   "apis/batch/v1",
	"cronjobs":               "apis/batch/v1beta1",
}

package resourcelink

var ExportResourcePrefixMappings = map[string]string{
	"pods":                   "api/v1",
	"configmaps":             "api/v1",
	"services":               "api/v1",
	"replicationcontrollers": "api/v1",
	"deployments":            "apis/apps/v1",
	"daemonsets":             "apis/apps/v1",
	"replicasets":            "apis/apps/v1",
	"statefulsets":           "apis/apps/v1",
	"jobs":                   "apis/batch/v1",
	"cronjobs":               "apis/batch/v1beta1",
}

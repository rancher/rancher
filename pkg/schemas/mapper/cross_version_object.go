package mapper

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/types/values"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

var (
	kindMap = map[string]string{
		"deployment":            "Deployment",
		"replicationcontroller": "ReplicationController",
		"statefulset":           "StatefulSet",
		"daemonset":             "DaemonSet",
		"job":                   "Job",
		"cronjob":               "CronJob",
		"replicaset":            "ReplicaSet",
	}
	groupVersionMap = map[string]string{
		"deployment":            "apps/v1beta2",
		"replicationcontroller": "core/v1",
		"statefulset":           "apps/v1beta2",
		"daemonset":             "apps/v1beta2",
		"job":                   "batch/v1",
		"cronjob":               "batch/v1beta1",
		"replicaset":            "apps/v1beta2",
	}
)

type CrossVersionObjectToWorkload struct {
	Field string
}

func (c CrossVersionObjectToWorkload) ToInternal(data map[string]interface{}) error {
	obj, ok := values.GetValue(data, strings.Split(c.Field, "/")...)
	if !ok {
		return nil
	}
	workloadID := convert.ToString(obj)
	parts := strings.SplitN(workloadID, ":", 3)
	newObj := map[string]interface{}{
		"kind":       getKind(parts[0]),
		"apiVersion": groupVersionMap[parts[0]],
		"name":       parts[2],
	}
	values.PutValue(data, newObj, strings.Split(c.Field, "/")...)
	return nil
}

func (c CrossVersionObjectToWorkload) FromInternal(data map[string]interface{}) {
	obj, ok := values.GetValue(data, strings.Split(c.Field, "/")...)
	if !ok {
		return
	}
	cvo := convert.ToMapInterface(obj)
	ns := convert.ToString(data["namespaceId"])
	values.PutValue(data,
		fmt.Sprintf("%s:%s:%s",
			strings.ToLower(convert.ToString(cvo["kind"])),
			ns,
			convert.ToString(cvo["name"])),
		strings.Split(c.Field, "/")...,
	)
}

func (c CrossVersionObjectToWorkload) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}

func getKind(i string) string {
	return kindMap[i]
}

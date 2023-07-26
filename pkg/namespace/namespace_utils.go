package namespace

import (
	"encoding/json"
	"time"

	v1 "k8s.io/api/core/v1"
)

const (
	statusAnn                   = "cattle.io/status"
	System                      = "cattle-system"
	GlobalNamespace             = "cattle-global-data"
	NodeTemplateGlobalNamespace = "cattle-global-nt"
	ProvisioningCAPINamespace   = "cattle-provisioning-capi-system"
)

func SetNamespaceCondition(namespace *v1.Namespace, d time.Duration, conditionType string, conditionStatus bool, message string) error {
	annotations := namespace.ObjectMeta.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}
	ann := annotations[statusAnn]
	conditionStatusStr := "False"
	if conditionStatus {
		conditionStatusStr = "True"
	}
	bAnn, err := GenerateConditionAnnotation(ann, d, conditionType, conditionStatusStr, message)
	if err != nil {
		return err
	}
	annotations[statusAnn] = bAnn

	namespace.ObjectMeta.Annotations = annotations

	return nil
}

func GenerateConditionAnnotation(ann string, d time.Duration, conditionType string, conditionStatus string, message string) (string, error) {
	status := &status{}
	if ann != "" {
		err := json.Unmarshal([]byte(ann), status)
		if err != nil {
			return "", err
		}
	}
	if status.Conditions == nil {
		status.Conditions = []condition{}
	}

	var idx int
	found := false
	for i, c := range status.Conditions {
		if c.Type == conditionType {
			idx = i
			found = true
			break
		}
	}

	conditionMessage := ""
	if conditionStatus != "True" {
		conditionMessage = message
	}

	cond := condition{
		Type:           conditionType,
		Status:         conditionStatus,
		Message:        conditionMessage,
		LastUpdateTime: time.Now().Add(d).Format(time.RFC3339),
	}

	if found {
		status.Conditions[idx] = cond
	} else {
		status.Conditions = append(status.Conditions, cond)
	}

	bAnn, err := json.Marshal(status)
	if err != nil {
		return "", err
	}
	return string(bAnn), nil
}

func IsNamespaceConditionSet(namespace *v1.Namespace, conditionType string, conditionStatus bool) (bool, error) {
	if namespace.ObjectMeta.Annotations == nil {
		return false, nil
	}
	ann := namespace.ObjectMeta.Annotations[statusAnn]
	if ann == "" {
		return false, nil
	}
	status := &status{}
	err := json.Unmarshal([]byte(ann), status)
	if err != nil {
		return false, err
	}
	conditionStatusStr := "False"
	if conditionStatus {
		conditionStatusStr = "True"
	}
	for _, c := range status.Conditions {
		if c.Type == conditionType && c.Status == conditionStatusStr {
			return true, nil
		}
	}
	return false, nil
}

type status struct {
	Conditions []condition
}

type condition struct {
	Type           string
	Status         string
	Message        string
	LastUpdateTime string
}

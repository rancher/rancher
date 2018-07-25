package namespace

import (
	"encoding/json"
	"time"

	"k8s.io/api/core/v1"
)

const (
	statusAnn = "cattle.io/status"
)

func SetNamespaceCondition(namespace *v1.Namespace, d time.Duration, conditionType string, conditionStatus bool, message string) error {
	if namespace.ObjectMeta.Annotations == nil {
		namespace.ObjectMeta.Annotations = map[string]string{}
	}

	ann := namespace.ObjectMeta.Annotations[statusAnn]
	status := &status{}
	if ann != "" {
		err := json.Unmarshal([]byte(ann), status)
		if err != nil {
			return err
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

	conditionStatusStr := "False"
	conditionMessage := ""
	if conditionStatus {
		conditionStatusStr = "True"
	} else {
		conditionMessage = message
	}

	cond := condition{
		Type:           conditionType,
		Status:         conditionStatusStr,
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
		return err
	}
	namespace.ObjectMeta.Annotations[statusAnn] = string(bAnn)

	return nil
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

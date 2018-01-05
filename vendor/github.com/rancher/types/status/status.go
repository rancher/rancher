package status

import (
	"strings"

	"time"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

type conditionMapping struct {
	Name        string
	State       string
	Transition  bool
	Error       bool
	FalseIsGood bool
}

type condition struct {
	Type    string
	Status  string
	Message string
}

var conditionMappings = []conditionMapping{
	{Name: "Initialized", Transition: true, State: "initializing"},
	{Name: "Available", Transition: true, State: "activating"},
	{Name: "Progressing", Transition: true, State: "updating"},
	{Name: "Provisioned", Transition: true, State: "provisioning"},
	{Name: "Saved", Transition: true, State: "saving"},
	{Name: "AgentInstalled", Transition: true, State: "installing"},
	{Name: "Updating", Transition: true, FalseIsGood: true, State: "updating"},
	{Name: "ConfigOK", Transition: true, State: "configuring"},
	{Name: "PodScheduled", Transition: true, State: "scheduling"},
	{Name: "Completed", State: "completed"},
	{Name: "Failed", Error: true, State: "error"},
	{Name: "OutOfDisk", Error: true, FalseIsGood: true},
	{Name: "MemoryPressure", Error: true, FalseIsGood: true},
	{Name: "DiskPressure", Error: true, FalseIsGood: true},
	{Name: "NetworkUnavailable", Error: true, FalseIsGood: true},
	{Name: "KernelHasNoDeadlock", Error: true, FalseIsGood: true},
	{Name: "Unschedulable", Error: true, FalseIsGood: true},
	{Name: "ReplicaFailure", Error: true, FalseIsGood: true},
	{Name: "Ready", Transition: false, State: "activating"},
	{Name: "BackingNamespaceCreated", Transition: true, State: "activating"},
	{Name: "CreatorMadeOwner", Transition: true, State: "activating"},
}

func Set(data map[string]interface{}) {
	if data == nil {
		return
	}

	val, ok := values.GetValue(data, "metadata", "removed")
	if ok && val != "" && val != nil {
		data["state"] = "removing"
		data["transitioning"] = "yes"

		finalizers, ok := values.GetStringSlice(data, "metadata", "finalizers")
		if ok && len(finalizers) > 0 {
			data["transitioningMessage"] = "Waiting on " + finalizers[0]
			if i, err := convert.ToTimestamp(val); err == nil {
				if time.Unix(i/1000, 0).Add(5 * time.Minute).Before(time.Now()) {
					data["transitioning"] = "error"
				}
			}
		}

		return
	}

	val, ok = values.GetValue(data, "status", "conditions")
	var conditions []condition
	if err := convert.ToObj(val, &conditions); err != nil {
		// ignore error
		return
	}

	conditionMap := map[string]condition{}
	for _, c := range conditions {
		conditionMap[c.Type] = condition{
			Status:  c.Status,
			Message: c.Message,
		}
	}

	state := ""
	error := false
	transitioning := false
	message := ""

	for _, conditionMapping := range conditionMappings {
		good := true
		condition, ok := conditionMap[conditionMapping.Name]
		if !ok {
			continue
		}

		if conditionMapping.FalseIsGood && condition.Status == "True" {
			good = false
		} else if !conditionMapping.FalseIsGood && condition.Status == "False" {
			good = false
		} else if conditionMapping.Transition && !conditionMapping.FalseIsGood && condition.Status == "Unknown" {
			good = false
		}

		if !good && conditionMapping.Transition {
			transitioning = true
			if len(message) > 0 {
				message = strings.Join([]string{message, condition.Message}, ",")
			} else {
				message = condition.Message
			}
		}

		if !good && state == "" && conditionMapping.State != "" {
			state = conditionMapping.State
		}

		if !good && conditionMapping.Error {
			error = true
			if len(message) > 0 {
				message = strings.Join([]string{message, condition.Message}, ",")
			} else {
				message = condition.Message
			}
		}
	}

	if state == "" {
		if val, ok := values.GetValue(data, "metadata", "created"); ok {
			if i, err := convert.ToTimestamp(val); err == nil {
				if time.Unix(i/1000, 0).Add(5 * time.Second).Before(time.Now()) {
					if state == "" {
						val, _ := values.GetValueN(data, "status", "phase").(string)
						if val != "" {
							state = val
						}
					}

					if state == "" {
						val, ok := values.GetValue(data, "spec", "active")
						if ok {
							if convert.ToBool(val) {
								state = "active"
							} else {
								state = "inactive"
							}
						}
					}

					if state == "" {
						state = "active"
					}
				}
			}
		}

		if state == "" {
			state = "initializing"
			transitioning = true
		}
	}

	data["state"] = strings.ToLower(state)
	if error {
		data["transitioning"] = "error"
	} else if transitioning {
		data["transitioning"] = "yes"
	} else {
		data["transitioning"] = "no"
	}

	data["transitioningMessage"] = message
}

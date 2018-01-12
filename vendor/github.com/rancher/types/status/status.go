package status

import (
	"strings"

	"time"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

type condition struct {
	Type    string
	Status  string
	Message string
}

// True ==
// False == error
// Unknown == transitioning
var transitioningMap = map[string]string{
	"AgentInstalled":           "installing",
	"Available":                "activating",
	"BackingNamespaceCreated":  "configuring",
	"ConfigOK":                 "configuring",
	"CreatorMadeOwner":         "configuring",
	"DefaultNamespaceAssigned": "configuring",
	"DefaultProjectCreated":    "configuring",
	"Initialized":              "initializing",
	"MachinesCreated":          "provisioning",
	"PodScheduled":             "scheduling",
	"Progressing":              "updating",
	"Provisioned":              "provisioning",
	"Removed":                  "removing",
	"Saved":                    "saving",
	"Updated":                  "updating",
	"Updating":                 "updating",
}

// True == error
// False ==
// Unknown ==
var reverseErrorMap = map[string]bool{
	"OutOfDisk":           true,
	"MemoryPressure":      true,
	"DiskPressure":        true,
	"NetworkUnavailable":  true,
	"KernelHasNoDeadlock": true,
	"Unschedulable":       true,
	"ReplicaFailure":      true,
}

// True ==
// False == error
// Unknown ==
var errorMapping = map[string]bool{
	"Failed": true,
}

// True ==
// False == transitioning
// Unknown ==
var doneMap = map[string]string{
	"Completed": "activating",
	"Ready":     "unavailable",
}

func concat(str, next string) string {
	if str == "" {
		return next
	}
	if next == "" {
		return str
	}
	return str + ", " + next
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

	state := ""
	error := false
	transitioning := false
	message := ""

	for _, c := range conditions {
		if errorMapping[c.Type] && c.Status == "False" {
			error = true
			message = c.Message
			break
		}
	}

	if !error {
		for _, c := range conditions {
			if reverseErrorMap[c.Type] && c.Status == "True" {
				error = true
				message = concat(message, c.Message)
			}
		}
	}

	for _, c := range conditions {
		newState, ok := transitioningMap[c.Type]
		if !ok {
			continue
		}

		if c.Status == "False" {
			error = true
			state = newState
			message = concat(message, c.Message)
		} else if c.Status == "Unknown" && state == "" {
			transitioning = true
			state = newState
			message = concat(message, c.Message)
		}
	}

	for _, c := range conditions {
		if state != "" {
			break
		}
		newState, ok := doneMap[c.Type]
		if !ok {
			continue
		}
		if c.Status == "False" {
			transitioning = true
			state = newState
			message = concat(message, c.Message)
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
		val, ok := values.GetValueN(data, "status", "phase").(string)
		if val != "" && ok {
			state = val
		}
	}

	if state == "" && len(conditions) == 0 {
		if val, ok := values.GetValue(data, "metadata", "created"); ok {
			if i, err := convert.ToTimestamp(val); err == nil {
				if time.Unix(i/1000, 0).Add(5 * time.Second).After(time.Now()) {
					state = "initializing"
					transitioning = true
				}
			}
		}
	}

	if state == "" {
		state = "active"
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

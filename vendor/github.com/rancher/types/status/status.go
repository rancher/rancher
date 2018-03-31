package status

import (
	"strings"

	"time"

	"encoding/json"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/sirupsen/logrus"
)

type status struct {
	Conditions []condition `json:"conditions"`
}

type condition struct {
	Type    string
	Status  string
	Message string
}

// True ==
// False == error
// Unknown == transitioning
var transitioningMap = map[string]string{
	"Active":                      "activating",
	"AddonDeploy":                 "provisioning",
	"AgentDeployed":               "provisioning",
	"BackingNamespaceCreated":     "configuring",
	"CertsGenerated":              "provisioning",
	"ConfigOK":                    "configuring",
	"Created":                     "creating",
	"CreatorMadeOwner":            "configuring",
	"DefaultNamespaceAssigned":    "configuring",
	"DefaultNetworkPolicyCreated": "configuring",
	"DefaultProjectCreated":       "configuring",
	"DockerProvisioned":           "provisioning",
	"Downloaded":                  "downloading",
	"etcd":                        "provisioning",
	"Inactive":                    "deactivating",
	"Initialized":                 "initializing",
	"Installed":                   "installing",
	"NodesCreated":                "provisioning",
	"Pending":                     "pending",
	"PodScheduled":                "scheduling",
	"Provisioned":                 "provisioning",
	"Registered":                  "registering",
	"Removed":                     "removing",
	"Saved":                       "saving",
	"Updated":                     "updating",
	"Updating":                    "updating",
	"Waiting":                     "waiting",
	"InitialRolesPopulated":       "activating",
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
	"Failed":      true,
	"Progressing": true,
}

// True ==
// False == transitioning
// Unknown == error
var doneMap = map[string]string{
	"Completed": "activating",
	"Ready":     "unavailable",
	"Available": "updating",
}

// True == transitioning
// False ==
// Unknown ==
var progressMap = map[string]string{}

func concat(str, next string) string {
	if str == "" {
		return next
	}
	if next == "" {
		return str
	}
	return str + "; " + next
}

func Set(data map[string]interface{}) {
	if data == nil {
		return
	}

	val, conditionsOk := values.GetValue(data, "status", "conditions")
	var conditions []condition
	convert.ToObj(val, &conditions)

	statusAnn, annOK := values.GetValue(data, "metadata", "annotations", "cattle.io/status")
	if annOK {
		status := &status{}
		s, ok := statusAnn.(string)
		if ok {
			err := json.Unmarshal([]byte(s), status)
			if err != nil {
				logrus.Warnf("Unable to unmarshal cattle status %v. Error: %v", s, err)
			}
		}
		if len(status.Conditions) > 0 {
			conditions = append(conditions, status.Conditions...)
		}
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
		} else if c.Status == "Unknown" {
			error = true
			state = newState
			message = concat(message, c.Message)
		}
	}

	for _, c := range conditions {
		if state != "" {
			break
		}
		newState, ok := progressMap[c.Type]
		if !ok {
			continue
		}
		if c.Status == "True" {
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

	apiVersion, _ := values.GetValueN(data, "apiVersion").(string)
	if state == "" && conditionsOk && len(conditions) == 0 && strings.Contains(apiVersion, "cattle.io") {
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

	if error {
		data["transitioning"] = "error"
	} else if transitioning {
		data["transitioning"] = "yes"
	} else {
		data["transitioning"] = "no"
	}

	data["state"] = strings.ToLower(state)
	data["transitioningMessage"] = message

	val, ok := values.GetValue(data, "metadata", "removed")
	if ok && val != "" && val != nil {
		data["state"] = "removing"
		data["transitioning"] = "yes"

		finalizers, ok := values.GetStringSlice(data, "metadata", "finalizers")
		if !ok {
			finalizers, ok = values.GetStringSlice(data, "spec", "finalizers")
		}

		msg := message
		for _, cond := range conditions {
			if cond.Type == "Removed" && (cond.Status == "Unknown" || cond.Status == "False") && cond.Message != "" {
				msg = cond.Message
			}
		}

		if ok && len(finalizers) > 0 {
			parts := strings.Split(finalizers[0], "controller.cattle.io/")
			f := parts[len(parts)-1]

			if len(msg) > 0 {
				msg = msg + "; waiting on " + f
			} else {
				msg = "waiting on " + f
			}
			data["transitioningMessage"] = msg
			if i, err := convert.ToTimestamp(val); err == nil {
				if time.Unix(i/1000, 0).Add(5 * time.Minute).Before(time.Now()) {
					data["transitioning"] = "error"
				}
			}
		}

		return
	}
}

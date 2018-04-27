package mapper

import (
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

type ContainerStatus struct {
}

type containerState struct {
	state        string
	message      string
	exitCode     interface{}
	restartCount int64
}

func (n ContainerStatus) FromInternal(data map[string]interface{}) {
	containerStates := map[string]containerState{}
	containerStatus := convert.ToMapSlice(values.GetValueN(data, "status", "containerStatuses"))
	for _, status := range containerStatus {
		name := convert.ToString(status["name"])
		restartCount, _ := convert.ToNumber(status["restartCount"])
		s := containerState{
			state:        "pending",
			restartCount: restartCount,
		}
		for k, v := range convert.ToMapInterface(status["state"]) {
			m := convert.ToMapInterface(v)
			switch k {
			case "terminated":
				s.state = "terminated"
				s.message = fmt.Sprintf("%s: %s", m["reason"], m["message"])
				s.exitCode = m["exitCode"]
			case "running":
				s.state = "running"
			case "waiting":
				s.state = "waiting"
				s.message = fmt.Sprintf("%s: %s", m["reason"], m["message"])
			}
		}

		containerStates[name] = s
	}

	containers := convert.ToMapSlice(values.GetValueN(data, "containers"))
	for _, container := range containers {
		if container == nil {
			continue
		}

		name := convert.ToString(container["name"])
		state, ok := containerStates[name]
		if ok {
			container["state"] = state.state
			container["transitioningMessage"] = state.message
			container["restartCount"] = state.restartCount
			container["exitCode"] = state.exitCode
		} else {
			container["state"] = "unknown"
		}
	}
}

func (n ContainerStatus) ToInternal(data map[string]interface{}) {
}

func (n ContainerStatus) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}

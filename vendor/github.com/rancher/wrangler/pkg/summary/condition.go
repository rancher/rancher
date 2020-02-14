package summary

import (
	"encoding/json"

	"github.com/rancher/wrangler/pkg/data"
)

func getRawConditions(obj data.Object) []data.Object {
	statusAnn := obj.String("metadata", "annotations", "cattle.io/status")
	if statusAnn != "" {
		status := data.Object{}
		if err := json.Unmarshal([]byte(statusAnn), &status); err == nil {
			return append(obj.Slice("status", "conditions"), status.Slice("conditions")...)
		}
	}
	return obj.Slice("status", "conditions")
}

func getConditions(obj data.Object) (result []Condition) {
	for _, condition := range getRawConditions(obj) {
		result = append(result, Condition{d: condition})
	}
	return
}

type Condition struct {
	d data.Object
}

func (c Condition) Type() string {
	return c.d.String("type")
}

func (c Condition) Status() string {
	return c.d.String("status")
}

func (c Condition) Reason() string {
	return c.d.String("reason")
}

func (c Condition) Message() string {
	return c.d.String("message")
}

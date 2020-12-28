package hpa

import (
	"time"

	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
)

const (
	condition = "ScalingActive"
	status    = "False"
	reason    = "FailedGetResourceMetric"
)

func NewIgnoreTransitioningErrorStore(store types.Store, d time.Duration, expectedState string) types.Store {
	return &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			transitioning, _ := values.GetValue(data, "transitioning")
			if transitioning != "error" {
				return data, nil
			}
			conditions, ok := values.GetSlice(data, "conditions")
			if !ok {
				return data, nil
			}
			var found bool
			for _, c := range conditions {
				t, _ := values.GetValue(c, "type")
				s, _ := values.GetValue(c, "status")
				r, _ := values.GetValue(c, "reason")
				if t == condition && s == status && r == reason {
					found = true
					break
				}
			}
			created, ok := values.GetValue(data, "created")
			if !ok {
				return data, nil
			}
			t, err := time.Parse(time.RFC3339, created.(string))
			if err != nil {
				return data, nil
			}
			if time.Now().Sub(t).Nanoseconds()-d.Nanoseconds() > 0 {
				return data, nil
			}
			if found {
				values.PutValue(data, "yes", "transitioning")
				values.PutValue(data, "", "transitioningMessage")
				if expectedState != "" {
					values.PutValue(data, expectedState, "state")
				}
			}
			return data, nil
		},
	}
}

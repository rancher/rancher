package project

import "testing"

func TestValidateLimitRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		data        map[string]any
		errExpected bool
	}{
		{
			name: "missing limit produces no error",
		},
		{
			name: "valid case",
			data: map[string]any{
				"containerDefaultResourceLimit": map[string]any{
					"requestsCpu":    "1m",
					"limitsCpu":      "20m",
					"requestsMemory": "1Mi",
					"limitsMemory":   "20Mi",
				},
			},
		},
		{
			name: "cpu and memory requests equal to limits",
			data: map[string]any{
				"containerDefaultResourceLimit": map[string]any{
					"requestsCpu":    "20m",
					"limitsCpu":      "20m",
					"requestsMemory": "30Mi",
					"limitsMemory":   "30Mi",
				},
			},
		},
		{
			name: "cpu request over limit",
			data: map[string]any{
				"containerDefaultResourceLimit": map[string]any{
					"requestsCpu":    "30m",
					"limitsCpu":      "20m",
					"requestsMemory": "1Mi",
					"limitsMemory":   "20Mi",
				},
			},
			errExpected: true,
		},
		{
			name: "cpu and memory requests over limits",
			data: map[string]any{
				"containerDefaultResourceLimit": map[string]any{
					"requestsCpu":    "30m",
					"limitsCpu":      "20m",
					"requestsMemory": "30Mi",
					"limitsMemory":   "20Mi",
				},
			},
			errExpected: true,
		},
		{
			name: "negative values cause an error",
			data: map[string]any{
				"containerDefaultResourceLimit": map[string]any{
					"requestsCpu":    "-1m",
					"limitsCpu":      "20m",
					"requestsMemory": "1Mi",
					"limitsMemory":   "20Mi",
				},
			},
			errExpected: true,
		},
	}

	var store projectStore
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := store.validateContainerDefaultResourceLimit(test.data)
			if test.errExpected && err == nil {
				t.Fatalf("expected an error, but did not get it")
			}
			if !test.errExpected && err != nil {
				t.Fatalf("got an unexpected error")
			}
		})
	}
}

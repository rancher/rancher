package status

import (
	"testing"

	"github.com/rancher/wrangler/v3/pkg/summary"
)

func TestCheckMachineSet(t *testing.T) {
	tests := []struct {
		name              string
		obj               map[string]interface{}
		expectedState     string
		expectedTransit   bool
		expectedError     bool
		expectedMessage   string
		stateAlreadySet   bool
		stateAlreadyValue string
	}{
		{
			name: "Non-MachineSet object - should be skipped",
			obj: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
				"status": map[string]interface{}{
					"replicas":          int64(3),
					"availableReplicas": int64(3),
				},
			},
			expectedState:   "",
			expectedTransit: false,
			expectedError:   false,
		},
		{
			name: "MachineSet scaled down to 0",
			obj: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta1",
				"kind":       "MachineSet",
				"spec": map[string]interface{}{
					"replicas": int64(0),
				},
				"status": map[string]interface{}{
					"replicas":          int64(0),
					"availableReplicas": int64(0),
				},
			},
			expectedState:   "Scaled down",
			expectedTransit: false,
			expectedError:   false,
		},
		{
			name: "MachineSet scaling up from 0",
			obj: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta1",
				"kind":       "MachineSet",
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
				"status": map[string]interface{}{
					"replicas":          int64(0),
					"availableReplicas": int64(0),
				},
			},
			expectedState:   "Scaling up",
			expectedTransit: true,
			expectedError:   false,
			expectedMessage: "0 of 3 replicas",
		},
		{
			name: "MachineSet scaling up (partial)",
			obj: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta1",
				"kind":       "MachineSet",
				"spec": map[string]interface{}{
					"replicas": int64(5),
				},
				"status": map[string]interface{}{
					"replicas":          int64(3),
					"availableReplicas": int64(3),
				},
			},
			expectedState:   "Scaling up",
			expectedTransit: true,
			expectedError:   false,
			expectedMessage: "3 of 5 replicas",
		},
		{
			name: "MachineSet scaling down",
			obj: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta1",
				"kind":       "MachineSet",
				"spec": map[string]interface{}{
					"replicas": int64(1),
				},
				"status": map[string]interface{}{
					"replicas":          int64(3),
					"availableReplicas": int64(3),
				},
			},
			expectedState:   "Scaling down",
			expectedTransit: true,
			expectedError:   false,
			expectedMessage: "3 of 1 replicas",
		},
		{
			name: "MachineSet updating - replicas match but not all available",
			obj: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta1",
				"kind":       "MachineSet",
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
				"status": map[string]interface{}{
					"replicas":          int64(3),
					"availableReplicas": int64(1),
				},
			},
			expectedState:   "Updating",
			expectedTransit: true,
			expectedError:   false,
		},
		{
			name: "MachineSet active - all replicas ready",
			obj: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta1",
				"kind":       "MachineSet",
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
				"status": map[string]interface{}{
					"replicas":          int64(3),
					"availableReplicas": int64(3),
				},
			},
			expectedState:   "active",
			expectedTransit: false,
			expectedError:   false,
		},
		{
			name: "MachineSet with state already set - should not override",
			obj: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta1",
				"kind":       "MachineSet",
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
				"status": map[string]interface{}{
					"replicas":          int64(3),
					"availableReplicas": int64(3),
				},
			},
			stateAlreadySet:   true,
			stateAlreadyValue: "custom-state",
			expectedState:     "custom-state",
			expectedTransit:   false,
			expectedError:     false,
		},
		{
			name: "MachineSet without spec.replicas - defaults to 1",
			obj: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta1",
				"kind":       "MachineSet",
				"spec":       map[string]interface{}{},
				"status": map[string]interface{}{
					"replicas":          int64(1),
					"availableReplicas": int64(1),
				},
			},
			expectedState:   "active",
			expectedTransit: false,
			expectedError:   false,
		},
		{
			name: "MachineSet without spec.replicas scaling up to default 1",
			obj: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta1",
				"kind":       "MachineSet",
				"spec":       map[string]interface{}{},
				"status": map[string]interface{}{
					"replicas":          int64(0),
					"availableReplicas": int64(0),
				},
			},
			expectedState:   "Scaling up",
			expectedTransit: true,
			expectedError:   false,
			expectedMessage: "0 of 1 replicas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := summary.Summary{}
			if tt.stateAlreadySet {
				s.State = tt.stateAlreadyValue
			}

			result := checkMachineSet(tt.obj, nil, s)

			if result.State != tt.expectedState {
				t.Errorf("Expected state %q, got %q", tt.expectedState, result.State)
			}
			if result.Transitioning != tt.expectedTransit {
				t.Errorf("Expected transitioning %v, got %v", tt.expectedTransit, result.Transitioning)
			}
			if result.Error != tt.expectedError {
				t.Errorf("Expected error %v, got %v", tt.expectedError, result.Error)
			}
			if tt.expectedMessage != "" {
				found := false
				for _, msg := range result.Message {
					if msg == tt.expectedMessage {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected message %q in %v", tt.expectedMessage, result.Message)
				}
			}
		})
	}
}

func TestScalingMessage(t *testing.T) {
	tests := []struct {
		current  int64
		desired  int64
		expected string
	}{
		{0, 3, "0 of 3 replicas"},
		{1, 5, "1 of 5 replicas"},
		{10, 5, "10 of 5 replicas"},
	}

	for _, tt := range tests {
		result := scalingMessage(tt.current, tt.desired)
		if result != tt.expected {
			t.Errorf("scalingMessage(%d, %d) = %q, expected %q", tt.current, tt.desired, result, tt.expected)
		}
	}
}

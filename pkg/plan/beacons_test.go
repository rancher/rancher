package plan

import (
	"testing"

	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
)

func TestAuthorizedForBeacon(t *testing.T) {

}

func TestIsOwningBeaconHolder(t *testing.T) {

}

func TestIsDelegateBeaconHolder(t *testing.T) {

}

func TestPushDelegate(t *testing.T) {
	tests := []struct {
		name     string
		beacon   *planv1alpha1.Beacon
		delegate string
		expected *planv1alpha1.Beacon
	}{
		{
			name:   "nil beacon",
			beacon: nil,
		},
		{
			name:   "nil labels",
			beacon: nil,
		},
		{
			name: "simple delegate",
			delegate: "test-1",
			beacon: &planv1alpha1.Beacon{
				Status: planv1alpha1.BeaconStatus{
					Owner: "test-0",
				},
			},
			expected: &planv1alpha1.Beacon{
				Status: planv1alpha1.BeaconStatus{
					Owner: "test-0",
					Delegates: []string{
						"test-1",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

		})
	}
}

func TestPopDelegate(t *testing.T) {

}

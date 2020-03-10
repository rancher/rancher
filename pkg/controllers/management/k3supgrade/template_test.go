package k3supgrade

import "testing"

func Test_parseVersion(t *testing.T) {

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			"base",
			"v1.17.3+k3s1",
			"v1.17.3-k3s1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseVersion(tt.version); got != tt.want {
				t.Errorf("parseVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

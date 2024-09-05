package managesystemagent

import "testing"

func Test_CurrentVersionResolvesGH5551(t *testing.T) {
	type test struct {
		name           string
		currentVersion string
		expectedResult bool
	}

	tests := []test{
		{
			name:           "1.30 version resolved 5551",
			currentVersion: "v1.30.4+rke2r1",
			expectedResult: true,
		},
		{
			name:           "1.30 version has not resolved 5551",
			currentVersion: "v1.30.1+rke2r1",
			expectedResult: false,
		},
		{
			name:           "1.29 version resolved 5551",
			currentVersion: "v1.29.8+rke2r1",
			expectedResult: true,
		},
		{
			name:           "1.29 version has not resolved 5551",
			currentVersion: "v1.29.7+rke2r1",
			expectedResult: false,
		},
		{
			name:           "1.28 version resolved 5551",
			currentVersion: "v1.28.13+rke2r1",
			expectedResult: true,
		},
		{
			name:           "1.28 version has not resolved 5551",
			currentVersion: "v1.28.12+rke2r1",
			expectedResult: false,
		},
		{
			name:           "1.27 version resolved 5551",
			currentVersion: "v1.27.16+rke2r1",
			expectedResult: true,
		},
		{
			name:           "1.27 version has not resolved 5551",
			currentVersion: "v1.27.15+rke2r1",
			expectedResult: false,
		},
		{
			name:           "version greater than 1.30 has resolved 5551",
			currentVersion: "v1.61.1+rke2r1",
			expectedResult: true,
		},
		{
			name:           "version less than 1.27 has not resolved 5551",
			currentVersion: "v1.26.1+rke2r1",
			expectedResult: false,
		},
		{
			name:           "RC version has not resolved 5551",
			currentVersion: "v1.30.1-rc1+rke2r1",
			expectedResult: false,
		},
		{
			name:           "RC version has resolved 5551",
			currentVersion: "v1.30.5-rc1+rke2r1",
			expectedResult: true,
		},
	}

	t.Parallel()
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			shouldCreatePlan := currentVersionResolvesGH5551(tc.currentVersion)
			if shouldCreatePlan != tc.expectedResult {
				t.Logf("expected %t when providing rke2 version %s but got %t", tc.expectedResult, tc.currentVersion, shouldCreatePlan)
				t.Fail()
			}
		})
	}
}

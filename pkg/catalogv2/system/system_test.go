package system

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

func TestGetIntervalOrDefault(t *testing.T) {
	t.Parallel()
	asserts := assert.New(t)
	type testCase struct {
		name     string
		input    string
		expected time.Duration
	}

	testCases := []testCase{
		{
			name:     "Should return the default value of 21600 if input string is empty",
			input:    "",
			expected: 21600 * time.Second,
		},
		{
			name:     "Should return the default value of 21600 if input string is invalid",
			input:    "foo",
			expected: 21600 * time.Second,
		},
		{
			name:     "Should return the time.Duration that corresponds to the given input",
			input:    "60",
			expected: 60 * time.Second,
		},
	}

	for _, test := range testCases {
		actual := getIntervalOrDefault(test.input)
		asserts.Equal(test.expected, actual, test.name)
	}
}

func TestIsInstalled(t *testing.T) {
	t.Parallel()

	standardValues := map[string]any{
		"name": "Pablo",
	}
	newValues := map[string]any{
		"name": "Winston",
	}

	tests := []struct {
		name          string
		latestVersion string
		minVersion    string
		desiredValues map[string]any

		expectedInstalled bool
		expectedVersion   string
		expectedValues    map[string]any
		expectedErr       bool
	}{
		{
			name:          "latest and min are the same as current",
			latestVersion: "1.0.0",
			minVersion:    "1.0.0",
			desiredValues: standardValues,

			expectedInstalled: true,
			expectedVersion:   "",
			expectedValues:    nil,
			expectedErr:       false,
		},
		{
			name:          "latest and min are the same as current but values changed",
			latestVersion: "1.0.0",
			minVersion:    "1.0.0",
			desiredValues: newValues,

			expectedInstalled: false,
			expectedVersion:   "1.0.0",
			expectedValues:    newValues,
			expectedErr:       false,
		},
		{
			name:          "new available latest is ignored",
			latestVersion: "1.1.0",
			minVersion:    "1.0.0",
			desiredValues: standardValues,

			expectedInstalled: true,
			expectedVersion:   "",
			expectedValues:    nil,
			expectedErr:       false,
		},
		{
			name:          "new latest is ignored but new values are honored",
			latestVersion: "1.1.0",
			minVersion:    "1.0.0",
			desiredValues: newValues,

			expectedInstalled: false,
			expectedVersion:   "1.0.0",
			expectedValues:    newValues,
			expectedErr:       false,
		},
		{
			name:          "higher min and even higher latest",
			latestVersion: "1.2.0",
			minVersion:    "1.1.0",
			desiredValues: standardValues,

			expectedInstalled: false,
			expectedVersion:   "1.2.0",
			expectedValues:    standardValues,
			expectedErr:       false,
		},
		{
			name:          "higher min and even higher latest with values changed",
			latestVersion: "1.2.0",
			minVersion:    "1.1.0",
			desiredValues: newValues,

			expectedInstalled: false,
			expectedVersion:   "1.2.0",
			expectedValues:    newValues,
			expectedErr:       false,
		},
		{
			name:          "both min and latest are higher",
			latestVersion: "1.2.0",
			minVersion:    "1.2.0",
			desiredValues: standardValues,

			expectedInstalled: false,
			expectedVersion:   "1.2.0",
			expectedValues:    standardValues,
			expectedErr:       false,
		},
		{
			name:          "both min and latest are higher but values changed",
			latestVersion: "1.2.0",
			minVersion:    "1.2.0",
			desiredValues: newValues,

			expectedInstalled: false,
			expectedVersion:   "1.2.0",
			expectedValues:    newValues,
			expectedErr:       false,
		},
		{
			name:          "latest is higher but min is lower than current",
			latestVersion: "1.1.0",
			minVersion:    "0.9.0",
			desiredValues: standardValues,

			expectedInstalled: true,
			expectedVersion:   "",
			expectedValues:    nil,
			expectedErr:       false,
		},
		{
			name:          "latest is higher but min is lower than current but values changed",
			latestVersion: "1.1.0",
			minVersion:    "0.9.0",
			desiredValues: newValues,

			expectedInstalled: false,
			expectedVersion:   "1.0.0",
			expectedValues:    newValues,
			expectedErr:       false,
		},
		{
			name:          "min version is higher but latest is lower than current",
			latestVersion: "0.9.0",
			minVersion:    "2.0.0",
			desiredValues: standardValues,

			expectedInstalled: false,
			expectedVersion:   "",
			expectedValues:    nil,
			expectedErr:       true,
		},
		{
			name:          "invalid latest version",
			latestVersion: "pug",
			minVersion:    "0.9.0",
			desiredValues: standardValues,

			expectedInstalled: false,
			expectedVersion:   "",
			expectedValues:    nil,
			expectedErr:       true,
		},
		{
			name:          "invalid min version",
			latestVersion: "1.3.0",
			minVersion:    "pug",
			desiredValues: standardValues,

			expectedInstalled: false,
			expectedVersion:   "",
			expectedValues:    nil,
			expectedErr:       true,
		},
	}

	releases := []*release.Release{
		{
			Name: "rancher-webhook",
			Info: &release.Info{Status: release.StatusDeployed},
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: "1.0.0",
				},
			},
			Config: standardValues,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			installed, version, values, err := isInstalled(releases, test.latestVersion, test.minVersion, test.desiredValues)
			assert.Equal(t, test.expectedInstalled, installed)
			assert.Equal(t, test.expectedVersion, version)
			assert.Equal(t, test.expectedValues, values)
			assert.Equal(t, test.expectedErr, err != nil)
		})
	}
}

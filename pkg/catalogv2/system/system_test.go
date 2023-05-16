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
		{
			name:          "min and latest are both unset",
			latestVersion: "",
			minVersion:    "",
			desiredValues: standardValues,

			expectedInstalled: false,
			expectedVersion:   "",
			expectedValues:    nil,
			expectedErr:       true,
		},
		{
			name:          "values are merged",
			latestVersion: "1.3.0",
			minVersion:    "1.2.0",
			desiredValues: map[string]any{
				"foo": "bar",
			},

			expectedInstalled: false,
			expectedVersion:   "1.3.0",
			expectedValues: map[string]any{
				"name": "Pablo",
				"foo":  "bar",
			},
			expectedErr: false,
		},
		{
			name:          "new values are empty yet old ones remain in merge",
			latestVersion: "1.3.0",
			minVersion:    "1.2.0",
			desiredValues: map[string]any{},

			expectedInstalled: false,
			expectedVersion:   "1.3.0",
			expectedValues: map[string]any{
				"name": "Pablo",
			},
			expectedErr: false,
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
			installed, version, values, err := desiredVersionAndValues(releases, test.minVersion, test.latestVersion, false, test.desiredValues)
			assert.Equal(t, test.expectedInstalled, installed)
			assert.Equal(t, test.expectedVersion, version)
			assert.Equal(t, test.expectedValues, values)
			assert.Equal(t, test.expectedErr, err != nil)
		})
	}
}

func TestIsInstalledExactVersion(t *testing.T) {
	t.Parallel()

	standardValues := map[string]any{
		"name": "Pablo",
	}
	tests := []struct {
		name           string
		desiredVersion string
		desiredValues  map[string]any
		isExact        bool

		expectedInstalled bool
		expectedVersion   string
		expectedValues    map[string]any
		expectedErr       bool
	}{
		{
			name:           "exact is higher than current",
			desiredVersion: "2.0.0",
			desiredValues:  standardValues,
			isExact:        true,

			expectedInstalled: false,
			expectedVersion:   "2.0.0",
			expectedValues:    standardValues,
			expectedErr:       false,
		},
		{
			name:           "exact is lower than current with no downgrade",
			desiredVersion: "0.9.0",
			desiredValues:  standardValues,

			expectedInstalled: true,
			expectedVersion:   "",
			expectedValues:    nil,
			expectedErr:       false,
		},
		{
			name:           "exact is lower than current with downgrade",
			desiredVersion: "0.9.0",
			desiredValues:  standardValues,
			isExact:        true,

			expectedInstalled: false,
			expectedVersion:   "0.9.0",
			expectedValues:    standardValues,
			expectedErr:       false,
		},
		{
			name:           "exact matches current",
			desiredVersion: "1.0.0",
			desiredValues:  nil,
			isExact:        true,

			expectedInstalled: true,
			expectedVersion:   "",
			expectedValues:    nil,
			expectedErr:       false,
		},
		{
			name:           "exact matches current but values changed and got merged",
			desiredVersion: "1.0.0",
			desiredValues: map[string]any{
				"foo": "bar",
			},
			isExact: true,

			expectedInstalled: false,
			expectedVersion:   "1.0.0",
			expectedValues: map[string]any{
				"name": "Pablo",
				"foo":  "bar",
			},
			expectedErr: false,
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
			// Note that the minVersion argument must be an empty string.
			installed, version, values, err := desiredVersionAndValues(releases, "", test.desiredVersion, test.isExact, test.desiredValues)
			assert.Equal(t, test.expectedInstalled, installed)
			assert.Equal(t, test.expectedVersion, version)
			assert.Equal(t, test.expectedValues, values)
			assert.Equal(t, test.expectedErr, err != nil)
		})
	}
}

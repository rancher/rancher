package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	types2 "github.com/rancher/rancher/pkg/api/steve/catalog/types"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/system/mocks"
	"github.com/stretchr/testify/mock"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	"io"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestInstall(t *testing.T) {
	t.Parallel()
	asserts := assert.New(t)
	type testInput struct {
		namespace            string
		name                 string
		minVersion           string
		exactVersion         string
		values               map[string]interface{}
		forceAdopt           bool
		installImageOverride string
	}
	type testMocks struct {
		indexOutput               *repo.IndexFile
		indexError                error
		isInstalledReleasesOutput []*release.Release
		isInstalledReleasesError  error
		hasStatusOutput           []*release.Release
		hasStatusError            error
		upgradeOutput             *catalog.Operation
		upgradeError              error
		podGetOutput              *v1.Pod
		podGetError               error
	}
	type testCase struct {
		name     string
		input    testInput
		mocks    testMocks
		expected error
		skip     bool
	}
	mockIndex := &repo.IndexFile{
		Entries: map[string]repo.ChartVersions{
			"test-chart": []*repo.ChartVersion{{Metadata: &chart.Metadata{Version: "102.0.0+up1.0.0"}}},
		},
	}

	testCases := []testCase{
		{
			name:     "Should return error if not able to get index file from rancher-charts",
			mocks:    testMocks{indexError: errors.New("error")},
			expected: errors.New("error"),
		},
		{
			name: "Should return error if the exact version of the chart does not exists",
			input: testInput{
				name:         "test-chart",
				exactVersion: "100.1.0+up0.6.1",
			},
			mocks: testMocks{
				indexOutput: mockIndex,
			},
			expected: fmt.Errorf("no chart version found for test-chart-100.1.0+up0.6.1"),
		},
		{
			name: "Should do nothing if the chart is already installed",
			input: testInput{
				namespace: "test",
				name:      "test-chart",
			},
			mocks: testMocks{
				indexOutput: mockIndex,
				isInstalledReleasesOutput: []*release.Release{{
					Info:  &release.Info{Status: release.StatusDeployed},
					Chart: &chart.Chart{Metadata: &chart.Metadata{Version: "102.0.0+up1.0.0"}},
				}},
			},
			expected: nil,
		},
		{
			name: "Should return error if not able list helm releases",
			input: testInput{
				name: "test-chart",
			},
			mocks: testMocks{
				indexOutput:              mockIndex,
				isInstalledReleasesError: errors.New("error"),
			},
			expected: errors.New("error"),
		},
		{
			name: "Should return error if the available chart version is lower than the min version",
			input: testInput{
				namespace:  "test",
				name:       "test-chart",
				minVersion: "102.0.0+up1.0.0",
			},
			mocks: testMocks{
				indexOutput: &repo.IndexFile{
					Entries: map[string]repo.ChartVersions{
						"test-chart": []*repo.ChartVersion{{Metadata: &chart.Metadata{Version: "101.0.0+up1.0.0"}}},
					},
				},
				isInstalledReleasesOutput: []*release.Release{{
					Info:  &release.Info{Status: release.StatusDeployed},
					Chart: &chart.Chart{Metadata: &chart.Metadata{Version: "100.0.0+up1.0.0"}},
				}},
			},
			expected: repo.ErrNoChartName,
		},
		{
			name: "Should do nothing if chart is pending upgrade, pending install or pending rollback",
			input: testInput{
				name: "test-chart",
			},
			mocks: testMocks{
				indexOutput: mockIndex,
				hasStatusOutput: []*release.Release{{
					Chart: &chart.Chart{Metadata: &chart.Metadata{Version: "102.0.0+up1.0.0"}},
				}},
			},
			expected: nil,
		},
		{
			name: "Should return an error if it fails to create an upgrade operation",
			input: testInput{
				name: "test-chart",
			},
			mocks: testMocks{
				indexOutput:  mockIndex,
				upgradeError: errors.New("error"),
			},
			expected: errors.New("error"),
		},
		{
			name: "Should return nil after the operation pod finishes successfully",
			input: testInput{
				name: "test-chart",
			},
			mocks: testMocks{
				indexOutput: mockIndex,
				upgradeOutput: &catalog.Operation{
					Status: catalog.OperationStatus{
						PodName:      "install-operation",
						PodNamespace: "cattle-system",
					},
				},
				podGetOutput: &v1.Pod{Status: v1.PodStatus{ContainerStatuses: []v1.ContainerStatus{
					{
						Name:  "helm",
						State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{ExitCode: 0}},
					},
				}}},
			},
			expected: nil,
		},
		{
			name: "Should return nil after the operation pod finishes successfully with exact version install",
			input: testInput{
				name:         "test-chart",
				exactVersion: "102.0.0+up1.0.0",
			},
			mocks: testMocks{
				indexOutput: &repo.IndexFile{
					Entries: map[string]repo.ChartVersions{
						"test-chart": []*repo.ChartVersion{
							{Metadata: &chart.Metadata{Version: "102.0.0+up1.0.0"}},
							{Metadata: &chart.Metadata{Version: "102.1.0+up1.0.0"}},
						},
					},
				},
				upgradeOutput: &catalog.Operation{
					Status: catalog.OperationStatus{
						PodName:      "install-operation",
						PodNamespace: "cattle-system",
					},
				},
				podGetOutput: &v1.Pod{Status: v1.PodStatus{ContainerStatuses: []v1.ContainerStatus{
					{
						Name:  "helm",
						State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{ExitCode: 0}},
					},
				}}},
			},
			expected: nil,
		},
	}

	for _, test := range testCases {
		if test.skip {
			continue
		}
		var versionMatcher interface{}
		if test.input.exactVersion != "" {
			versionMatcher = mock.MatchedBy(func(r io.Reader) bool {
				upgradeArgs := &types2.ChartUpgradeAction{}
				_ = json.NewDecoder(r).Decode(upgradeArgs)
				return upgradeArgs.Charts[0].Version == test.input.exactVersion
			})
		} else {
			versionMatcher = mock.Anything
		}

		contentMock, opsMock, podsMock, settingsMock, helmMock, clusterRepoMock :=
			&mocks.ContentClient{}, &mocks.OperationClient{}, &mocks.PodClient{}, &mocks.SettingController{}, &mocks.HelmClient{}, &mocks.ClusterRepoController{}

		contentMock.On("Index", "", "rancher-charts", true).Return(test.mocks.indexOutput, test.mocks.indexError)
		helmMock.On("ListReleases", test.input.namespace, test.input.name, action.ListDeployed).Return(test.mocks.isInstalledReleasesOutput, test.mocks.isInstalledReleasesError)
		helmMock.On("ListReleases", test.input.namespace, test.input.name, action.ListPendingInstall|action.ListPendingUpgrade|action.ListPendingRollback).Return(test.mocks.hasStatusOutput, test.mocks.hasStatusError)
		opsMock.On("Upgrade", context.TODO(), installUser, "", "rancher-charts", versionMatcher, test.input.installImageOverride).Return(test.mocks.upgradeOutput, test.mocks.upgradeError)
		if test.mocks.podGetOutput != nil || test.mocks.podGetError != nil {
			podsMock.On("Get", test.mocks.upgradeOutput.Status.PodNamespace, test.mocks.upgradeOutput.Status.PodName, metav1.GetOptions{}).Return(test.mocks.podGetOutput, test.mocks.podGetError)
		}
		manager, _ := NewManager(context.TODO(), contentMock, opsMock, podsMock, settingsMock, clusterRepoMock, helmMock)
		err := manager.install(test.input.namespace, test.input.name, test.input.minVersion, test.input.exactVersion, test.input.values, test.input.forceAdopt, test.input.installImageOverride)
		asserts.Equal(test.expected, err, test.name)
	}
}

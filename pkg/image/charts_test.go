package image

import (
	"testing"

	assertlib "github.com/stretchr/testify/assert"
)

func TestPickImagesFromValuesMap(t *testing.T) {
	testCases := []struct {
		description         string
		values              map[interface{}]interface{}
		chartNameAndVersion string
		osType              OSType
		tagToIgnore         string
		expectedImagesSet   map[string]map[string]struct{}
	}{
		{
			description: "Want linux images",
			values: map[interface{}]interface{}{
				"repository": "test-repository",
				"tag":        "1.2.3",
				"os":         "Linux",
			},
			chartNameAndVersion: "chart:0.1.2",
			osType:              Linux,
			tagToIgnore:         "",
			expectedImagesSet: map[string]map[string]struct{}{
				"test-repository:1.2.3": {
					"chart:0.1.2": struct{}{},
				},
			},
		},
		{
			description: "Want Windows images",
			values: map[interface{}]interface{}{
				"repository": "test-repository",
				"tag":        "1.2.3",
				"os":         "windows,linux",
			},
			chartNameAndVersion: "chart:0.1.2",
			osType:              Windows,
			tagToIgnore:         "",
			expectedImagesSet: map[string]map[string]struct{}{
				"test-repository:1.2.3": {
					"chart:0.1.2": struct{}{},
				},
			},
		},
		{
			description: "No images of the given OS (want Windows, but images are Linux)",
			values: map[interface{}]interface{}{
				"repository": "test-repository",
				"tag":        "1.2.3",
				"os":         "linux",
			},
			chartNameAndVersion: "chart:0.1.2",
			osType:              Windows,
			tagToIgnore:         "",
			expectedImagesSet:   map[string]map[string]struct{}{},
		},
		{
			description: "No OS provided, default to Linux",
			values: map[interface{}]interface{}{
				"repository": "test-repository",
				"tag":        "1.2.3",
			},
			chartNameAndVersion: "chart:0.1.2",
			osType:              Linux,
			tagToIgnore:         "",
			expectedImagesSet: map[string]map[string]struct{}{
				"test-repository:1.2.3": {
					"chart:0.1.2": struct{}{},
				},
			},
		},
		{
			description: "Unsupported OS provided",
			values: map[interface{}]interface{}{
				"repository": "test-repository",
				"tag":        "1.2.3",
				"os":         "unsupported-os",
			},
			chartNameAndVersion: "chart:0.1.2",
			osType:              Linux,
			tagToIgnore:         "",
			expectedImagesSet:   map[string]map[string]struct{}{},
		},
		{
			description:         "Missing required information in values file",
			values:              map[interface{}]interface{}{},
			chartNameAndVersion: "chart:0.1.2",
			osType:              Linux,
			tagToIgnore:         "",
			expectedImagesSet:   map[string]map[string]struct{}{},
		},
		{
			description: "Ignore an non-matching tag",
			values: map[interface{}]interface{}{
				"repository": "test-repository",
				"tag":        "1.2.3",
				"os":         "Linux",
			},
			chartNameAndVersion: "chart:0.1.2",
			osType:              Linux,
			tagToIgnore:         "latest",
			expectedImagesSet: map[string]map[string]struct{}{
				"test-repository:1.2.3": {
					"chart:0.1.2": struct{}{},
				},
			},
		},
		{
			description: "Ignore a matching tag",
			values: map[interface{}]interface{}{
				"repository": "test-repository",
				"tag":        "1.2.3",
				"os":         "Linux",
			},
			chartNameAndVersion: "chart:0.1.2",
			osType:              Linux,
			tagToIgnore:         "1.2.3",
			expectedImagesSet:   map[string]map[string]struct{}{},
		},
	}
	assert := assertlib.New(t)
	for _, tc := range testCases {
		actualImagesSet := make(map[string]map[string]struct{})
		err := pickImagesFromValuesMap(actualImagesSet, tc.values, tc.chartNameAndVersion, tc.osType, tc.tagToIgnore)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		assert.Equalf(tc.expectedImagesSet, actualImagesSet, "testcase: %s", tc.description)
	}
}

func TestMinMaxToConstraintStr(t *testing.T) {
	testCases := []struct {
		min      string
		max      string
		expected string
	}{
		{"2.5.8", "2.6", "2.5.8 - 2.6"},
		{"2.5.8", "", ">= 2.5.8"},
		{"", "2.6", "<= 2.6"},
		{"", "", ""},
	}
	assert := assertlib.New(t)
	for _, tc := range testCases {
		actual := minMaxToConstraintStr(tc.min, tc.max)
		assert.Equal(tc.expected, actual)
	}
}

func TestCompareRancherVersionToConstraint(t *testing.T) {
	testCases := []struct {
		rancherVersion string
		constraintStr  string
		expected       bool
		isErr          bool
	}{
		// Assert true
		{"2.6", "<= 2.6", true, false},
		{"2.5.8", ">= 2.5.7", true, false},
		{"2.5.7", "2.5.7 - 2.5.7", true, false},
		{"2.5.7", "2.5.7-rc1 - 2.5.7", true, false},
		{"2.5.7-rc1", "2.5.6 - 2.5.8-rc1", true, false},
		{"2.5.7+up1", ">= 2.5.7-rc1", true, false},
		{"2.5.7", ">= 2.5.7-rc1", true, false},
		{"2.5.7-rc1", ">= 2.5.7-patch1", true, false},
		{"2.5.7-patch1", ">= 2.5.7-beta1", true, false},
		{"2.5.7-beta1", ">= 2.5.7-alpha1", true, false},
		{"2.5.7-alpha1", ">= 2.5.7-0", true, false},
		{"2.6.4-rc1", "2.6.3 - 2.6.5", true, false},
		// Assert false
		{"2.5.7", "2.5.8-rc1 - 2.5.8-rc2", false, false},
		// Assert error
		{"", "", false, true},
		{"2.5.8", "", false, true},
		// Assert Rancher version 2.6.99 is changed to 2.6.98 to handle edge case when compared against 2.6.99-0
		{"2.6.99", "2.5.99 - 2.6.99-0", true, false},
	}
	assert := assertlib.New(t)
	for _, tc := range testCases {
		actual, err := compareRancherVersionToConstraint(tc.rancherVersion, tc.constraintStr)
		if err != nil {
			if tc.isErr {
				assert.Error(err)
			} else {
				t.Errorf("unexpected error: %s", err)
			}
		}
		assert.Equalf(tc.expected, actual, "testcase: %v", tc)
	}
}

package apiservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getVersionName(t *testing.T) {
	type testcase struct {
		version           string
		expectVersionName string
	}
	testcases := []testcase{
		testcase{version: "v1", expectVersionName: "v1."},
		testcase{version: "autoscaling/v2beta2", expectVersionName: "v2beta2.autoscaling"},
	}
	for _, c := range testcases {
		assert.Equalf(t, c.expectVersionName, getAPIVersionName(c.version), "Failed to parse version name from %s", c.version)
	}
}

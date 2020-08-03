package service

import (
	"fmt"
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/rancher/pkg/kontainer-engine/types"
	"github.com/stretchr/testify/assert"
	"gopkg.in/check.v1"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) {
	check.TestingT(t)
}

type StubTestSuite struct {
}

type testCase struct {
	input       string
	expectedVal string
	expectedErr error
}

var _ = check.Suite(&StubTestSuite{})

func (s *StubTestSuite) SetUpSuite(c *check.C) {
}

func newTestCase(input string, expectedVal string, isErr bool) testCase {
	var expectedErr error

	if isErr {
		expectedErr = fmt.Errorf("failed to parse port from address [%s]", input)
	}
	return testCase{
		input:       input,
		expectedVal: expectedVal,
		expectedErr: expectedErr,
	}
}

func (s *StubTestSuite) TestFlatten(c *check.C) {
	config := v32.MapStringInterface{
		"projectId":  "test",
		"zone":       "test",
		"diskSizeGb": 50,
		"labels": map[string]string{
			"foo": "bar",
		},
		"enableAlphaFeature": true,
		"masterVersion":      "1.7.1",
		"nodeVersion":        "1.7.1",
		"nodeCount":          3,
	}
	opts, err := toMap(config, "json")
	if err != nil {
		c.Fatal(err)
	}
	driverOptions := types.DriverOptions{
		BoolOptions:        make(map[string]bool),
		StringOptions:      make(map[string]string),
		IntOptions:         make(map[string]int64),
		StringSliceOptions: make(map[string]*types.StringSlice),
	}
	flatten(opts, &driverOptions)
	fmt.Println(driverOptions)
	boolResult := map[string]bool{
		"enableAlphaFeature": true,
	}
	stringResult := map[string]string{
		"projectId":     "test",
		"zone":          "test",
		"masterVersion": "1.7.1",
		"nodeVersion":   "1.7.1",
	}
	intResult := map[string]int64{
		"diskSizeGb": 50,
		"nodeCount":  3,
	}
	stringSliceResult := map[string]types.StringSlice{
		"labels": {
			Value: []string{"foo=bar"},
		},
	}
	c.Assert(driverOptions.BoolOptions, check.DeepEquals, boolResult)
	c.Assert(driverOptions.IntOptions, check.DeepEquals, intResult)
	c.Assert(driverOptions.StringOptions, check.DeepEquals, stringResult)
	c.Assert(driverOptions.StringSliceOptions["labels"].Value, check.DeepEquals, stringSliceResult["labels"].Value)
}

func TestPortOnly(t *testing.T) {
	assert := assert.New(t)

	testCases := []testCase{
		// strings should be of the form "string:port"
		newTestCase("asdf", "", true),
		newTestCase("a:asdf", "", true),
		newTestCase("3000", "", true),
		newTestCase("300:asdf", "", true),
		newTestCase("300!asdf", "", true),
		newTestCase("a:as:300", "", true),
		newTestCase(":300:", "", true),
		newTestCase(":::", "", true),
		newTestCase("asdf.asdf:99999999", "", true),
		newTestCase("asdf.asdf:-99999999", "", true),
		newTestCase("300.com:3000", "3000", false),
		newTestCase("a:200", "200", false),
		newTestCase("a.com:3000", "3000", false),
	}

	for _, test := range testCases {
		port, err := portOnly(test.input)

		assert.Equal(test.expectedVal, port)
		if test.expectedErr != nil {
			assert.Contains(err.Error(), test.expectedErr.Error())
		} else {
			assert.Nil(err)
		}
	}
}

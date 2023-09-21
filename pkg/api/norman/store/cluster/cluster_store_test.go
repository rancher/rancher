package cluster

import (
	"testing"

	mVersion "github.com/mcuadros/go-version"
)

func TestVersionCompare(t *testing.T) {
	type testCase struct {
		input  string
		expect bool
	}

	testCases := []testCase{
		{
			input:  "v1.21.14",
			expect: false,
		},
		{
			input:  "v1.23.0",
			expect: false,
		},
		{
			input:  "v1.23.16-rancher2-3",
			expect: false,
		},
		{
			input:  "v1.24.0",
			expect: true,
		},
		{
			input:  "v1.24.0-rancher1-1",
			expect: true,
		},
		{
			input:  "v1.24.1",
			expect: false,
		},
		{
			input:  "v1.25.0",
			expect: true,
		},
		{
			input:  "v1.26.8",
			expect: true,
		},
	}

	for _, tc := range testCases {
		if tc.expect != mVersion.Compare(tc.input, "v1.24.0-rancher1-1", ">=") {
			t.Fail()
			t.Logf("failed: %s", tc.input)
		}
	}
}

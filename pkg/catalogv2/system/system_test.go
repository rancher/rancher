package system

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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

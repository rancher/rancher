package nodedriver

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestParseKeyValueString(t *testing.T) {
	_, hook := test.NewNullLogger()
	logrus.SetLevel(logrus.DebugLevel)
	logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))
	logrus.AddHook(hook)

	testCases := []struct {
		name               string
		input              string
		expectedResult     map[string]string
		expectedLog        string
		expectedLogEntries int
		expectedLogLevel   logrus.Level
	}{
		{
			name:  "valid key-value pair",
			input: "userdata:userdata,cloudConfig:cloud-config",
			expectedResult: map[string]string{
				"userdata":    "userdata",
				"cloudConfig": "cloud-config",
			},
		},
		{
			name:  "key-value pairs with extra space",
			input: "userdata: userdata, cloudConfig:cloud-config",
			expectedResult: map[string]string{
				"userdata":    "userdata",
				"cloudConfig": "cloud-config",
			},
		},
		{
			name:               "empty pair",
			input:              "",
			expectedResult:     map[string]string{},
			expectedLog:        "Empty input string",
			expectedLogEntries: 1,
			expectedLogLevel:   logrus.DebugLevel,
		},
		{
			name:               "empty key",
			input:              ":cloud-config",
			expectedResult:     map[string]string{},
			expectedLog:        "failed to parse pair: \":cloud-config\" (expected key:value)",
			expectedLogEntries: 1,
			expectedLogLevel:   logrus.ErrorLevel,
		},
		{
			name:               "invalid pair",
			input:              "userdata:cloudConfig:cloud-config",
			expectedResult:     map[string]string{},
			expectedLog:        "failed to parse pair: \"userdata:cloudConfig:cloud-config\" (expected key:value)",
			expectedLogEntries: 1,
			expectedLogLevel:   logrus.ErrorLevel,
		},
	}

	for _, tc := range testCases {
		hook.Reset()

		annotations := ParseKeyValueString(tc.input)
		assert.Equal(t, tc.expectedResult, annotations, tc.name)

		if tc.expectedLog != "" {
			found := false
			for _, entry := range hook.AllEntries() {
				if entry.Level == tc.expectedLogLevel && strings.Contains(entry.Message, tc.expectedLog) {
					found = true
					break
				}
			}
			assert.Equal(t, true, found, "expected log '%s' of level %s not found", tc.expectedLog, tc.expectedLogLevel)
		} else {
			assert.Equal(t, 0, len(hook.AllEntries()), "expected no log entries")
		}
	}
}

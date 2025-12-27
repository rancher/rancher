package nodedriver

import (
	"strconv"
	"strings"
	"testing"

	"github.com/rancher/machine/libmachine/mcnflag"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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

func TestFlagToField_Focused(t *testing.T) {
	tests := []struct {
		name     string
		flag     mcnflag.Flag
		expected v32.Field
		wantErr  bool
	}{
		{
			name: "StringFlag with default",
			flag: &mcnflag.StringFlag{
				Name:  "driver-test-flag",
				Usage: "A test string flag",
				Value: "default-string",
			},
			expected: v32.Field{
				Create:      true,
				Update:      true,
				Type:        "string",
				Description: "A test string flag",
				Default:     v32.Values{StringValue: "default-string"},
			},
			wantErr: false,
		},
		{
			name: "StringFlag no default",
			flag: &mcnflag.StringFlag{
				Name:  "driver-test-flag-no-default",
				Usage: "A test string flag with no default",
				Value: "",
			},
			expected: v32.Field{
				Create:      true,
				Update:      true,
				Type:        "string",
				Description: "A test string flag with no default",
				Default:     v32.Values{StringValue: ""},
			},
			wantErr: false,
		},
		{
			name: "IntFlag with default",
			flag: &mcnflag.IntFlag{
				Name:  "driver-test-int",
				Usage: "A test int flag",
				Value: 7,
			},
			expected: v32.Field{
				Create:      true,
				Update:      true,
				Type:        "string",
				Description: "A test int flag",
				Default:     v32.Values{StringValue: strconv.Itoa(7)},
			},
			wantErr: false,
		},
		{
			name: "IntFlag no default",
			flag: &mcnflag.IntFlag{
				Name:  "driver-test-int-no-default",
				Usage: "A test int flag with no default",
				Value: 0,
			},
			expected: v32.Field{
				Create:      true,
				Update:      true,
				Type:        "string",
				Description: "A test int flag with no default",
				Default:     v32.Values{StringValue: "0"},
			},
			wantErr: false,
		},
		{
			name: "BoolFlag default true",
			flag: &mcnflag.BoolFlag{
				Name:  "driver-test-bool",
				Usage: "A test bool flag",
				Value: true,
			},
			expected: v32.Field{
				Create:      true,
				Update:      true,
				Type:        "boolean",
				Description: "A test bool flag",
				Default:     v32.Values{BoolValue: true},
			},
			wantErr: false,
		},
		{
			name: "BoolFlag default false",
			flag: &mcnflag.BoolFlag{
				Name:  "driver-test-bool-false",
				Usage: "A test bool flag default false",
				Value: false,
			},
			expected: v32.Field{
				Create:      true,
				Update:      true,
				Type:        "boolean",
				Description: "A test bool flag default false",
				Default:     v32.Values{BoolValue: false},
			},
			wantErr: false,
		},
		{
			name: "StringSliceFlag",
			flag: &mcnflag.StringSliceFlag{
				Name:  "driver-test-stringslice",
				Usage: "A test string slice flag",
				Value: []string{"one", "two"},
			},
			expected: v32.Field{
				Create:      true,
				Update:      true,
				Type:        "array[string]",
				Description: "A test string slice flag",
				Nullable:    true,
				Default:     v32.Values{StringSliceValue: []string{"one", "two"}},
			},
			wantErr: false,
		},
		{
			name: "BoolPointerFlag nil",
			flag: &BoolPointerFlag{
				Name:  "driver-test-boolpointer-nil",
				Usage: "A test bool pointer flag nil",
				Value: nil,
			},
			expected: v32.Field{
				Create:      true,
				Update:      true,
				Type:        "boolean",
				Description: "A test bool pointer flag nil",
				Default:     v32.Values{BoolValue: false},
			},
			wantErr: false,
		},
		{
			name: "BoolPointerFlag true",
			flag: &BoolPointerFlag{
				Name:  "driver-test-boolpointer-true",
				Usage: "A test bool pointer flag true",
				Value: func() *bool { b := true; return &b }(),
			},
			expected: v32.Field{
				Create:      true,
				Update:      true,
				Type:        "boolean",
				Description: "A test bool pointer flag true",
				Default:     v32.Values{BoolValue: true},
			},
			wantErr: false,
		},
		{
			name: "UnsupportedFlag",
			flag: &mockFlag{
				name: "unsupported-flag",
			},
			expected: v32.Field{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, field, err := FlagToField(tt.flag)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, field)
			assert.NotEmpty(t, name)
			assert.NotContains(t, name, "-")
		})
	}
}

// mockFlag implements mcnflag.Flag but is not one of the handled concrete types
type mockFlag struct {
	name string
}

func (m *mockFlag) String() string       { return m.name }
func (m *mockFlag) Default() interface{} { return nil }

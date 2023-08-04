package settings

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	v1 "k8s.io/api/core/v1"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRelease(t *testing.T) {
	inputs := map[string]bool{
		"dev":         false,
		"master-head": false,
		"master":      false,
		"v2.5.2":      true,
		"v2":          true,
		"v2.0":        true,
		"v2.x":        true,
		"v2.5-head":   false,
		"2.5":         false,
		"2.5-head":    false,
	}
	a := assert.New(t)
	for key, value := range inputs {
		if err := ServerVersion.Set(key); err != nil {
			t.Errorf("Encountered error while setting temp version: %v\n", err)
		}
		result := IsRelease()
		a.Equal(value, result, fmt.Sprintf("Expected value [%t] for key [%s]. Got value [%t]", value, key, result))
	}
}

// TestSystemDefaultRegistryDefault tests that the default registry is either
// the value set by the environment variable CATTLE_BASE_REGISTRY or the build
// time value set through InjectDefaults.
func TestSystemDefaultRegistryDefault(t *testing.T) {
	expect := os.Getenv("CATTLE_BASE_REGISTRY")
	if InjectDefaults != "" {
		defaults := map[string]string{}
		if err := json.Unmarshal([]byte(InjectDefaults), &defaults); err != nil {
			t.Errorf("Unable to parse InjectDefaults: %v", err)
		}

		if value, ok := defaults["system-default-registry"]; ok {
			expect = value
		}
	}

	got := SystemDefaultRegistry.Get()
	if got != expect {
		t.Errorf("The System Default Registry of %q is not the expected value %q", got, expect)
	}

}

// TestSystemFeatureChartRefreshSecondsDefault tests that the default refresh time is either
// the default value of 21600 seconds or the build time value set through InjectDefaults.
func TestSystemFeatureChartRefreshSecondsDefault(t *testing.T) {
	expect := "21600"
	if InjectDefaults != "" {
		defaults := map[string]string{}
		if err := json.Unmarshal([]byte(InjectDefaults), &defaults); err != nil {
			t.Fatalf("Unable to parse InjectDefaults: %v", err)
		}

		if value, ok := defaults["system-feature-chart-refresh-seconds"]; ok {
			expect = value
		}
	}

	got := SystemFeatureChartRefreshSeconds.Get()
	if got != expect {
		t.Errorf("The System Feature Chart Refresh Seconds of %q is not the expected value %q", got, expect)
	}

}

func TestGetMachineProvisionImagePullPolicy(t *testing.T) {
	defalutLogger := logrus.StandardLogger().Out
	logrus.SetOutput(io.Discard)
	defer func() {
		logrus.SetOutput(defalutLogger)
	}()

	testCases := []struct {
		settingValue string
		want         v1.PullPolicy
	}{
		{
			settingValue: "Always",
			want:         v1.PullAlways,
		},
		{
			settingValue: "Never",
			want:         v1.PullNever,
		},
		{
			settingValue: "IfNotPresent",
			want:         v1.PullIfNotPresent,
		},
		{
			settingValue: "wrongValue",
			want:         v1.PullAlways,
		},
	}

	for _, v := range testCases {
		t.Run(v.settingValue, func(t *testing.T) {
			if err := MachineProvisionImagePullPolicy.Set(v.settingValue); err != nil {
				t.Errorf("Failed to test GetMachineProvisionImagePullPolicy(), unable to set the value: %v", err)
			}
			assert.Equalf(t, v.want, GetMachineProvisionImagePullPolicy(), fmt.Sprintf("test %s failed with value %s", v.settingValue, v.want))
		})
	}
}

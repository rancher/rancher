package settings

import (
	"encoding/json"
	"fmt"
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
// "" (an empty string) or the build time value to ensure backward compatibility.
// Note, if CATTLE_BASE_REGISTRY is set this test in the testing environment
// this test will fail.
func TestSystemDefaultRegistryDefault(t *testing.T) {
	var expect string
	if InjectDefaults != "" {
		defaults := map[string]string{}
		if err := json.Unmarshal([]byte(InjectDefaults), &defaults); err != nil {
			t.Errorf("Unable to parse InjectDefaults: %v", err)
		}

		if value, ok := settings["system-default-registry"]; ok {
			expect = value.Get()
		}
	}

	got := SystemDefaultRegistry.Get()
	if got != expect {
		t.Errorf("The System Default Registry of %q is not the expected value %q", got, expect)
	}

}

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManagerRemovesRelease(t *testing.T) {
	t.Parallel()
	webhook := desiredKey{
		namespace:            "cattle-system",
		name:                 "rancher-webhook",
		minVersion:           "1.1.1",
		exactVersion:         "1.2.0",
		installImageOverride: "some-image",
	}
	charts := map[desiredKey]map[string]any{
		webhook: {"foo": "bar"},
	}

	manager := Manager{desiredCharts: charts}
	manager.Remove("hello", "world")
	assert.Equal(t, charts, manager.desiredCharts)

	manager.Remove("cattle-system", "rancher-webhook")
	assert.Equal(t, map[desiredKey]map[string]any{}, manager.desiredCharts)

	// Assert that the lookup of key to delete only needs namespace and name.
	webhook = desiredKey{
		namespace: "cattle-system",
		name:      "rancher-webhook",
	}
	charts[webhook] = map[string]any{}
	assert.Equal(t, charts, manager.desiredCharts)
	manager.Remove("cattle-system", "rancher-webhook")
	assert.Equal(t, map[desiredKey]map[string]any{}, manager.desiredCharts)
}

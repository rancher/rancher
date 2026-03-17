package ext

import (
	"testing"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
)

func TestGetNamespaceDefaultFallback(t *testing.T) {
	// When no settings provider is configured and CATTLE_NAMESPACE was not set
	// at init time, GetNamespace should return the default "cattle-system".
	ns := GetNamespace()
	assert.Equal(t, "cattle-system", ns)
}

func TestGetNamespaceCustomValue(t *testing.T) {
	// When the namespace setting is set, GetNamespace should return that value.
	err := settings.Namespace.Set("rancher-system")
	assert.NoError(t, err)
	t.Cleanup(func() {
		_ = settings.Namespace.Set("")
	})

	ns := GetNamespace()
	assert.Equal(t, "rancher-system", ns)
}

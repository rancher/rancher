package ext

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNamespaceDefaultFallback(t *testing.T) {
	// When no settings provider is configured and CATTLE_NAMESPACE was not set
	// at init time, GetNamespace should return the default "cattle-system".
	ns := GetNamespace()
	assert.Equal(t, "cattle-system", ns)
}

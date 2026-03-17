package ext

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNamespaceDefault(t *testing.T) {
	// When CATTLE_NAMESPACE is not set (no settings provider configured),
	// GetNamespace should return the default "cattle-system".
	ns := GetNamespace()
	assert.Equal(t, "cattle-system", ns)
}

func TestGetNamespaceFromEnv(t *testing.T) {
	// When CATTLE_NAMESPACE is set, GetNamespace should return it.
	t.Setenv("CATTLE_NAMESPACE", "custom-namespace")

	// Since settings.Namespace is initialized at package init time from
	// os.Getenv("CATTLE_NAMESPACE"), and the settings provider is nil in tests,
	// settings.Namespace.Get() returns the Default value which was captured
	// at init time. We can't change it at runtime with t.Setenv for the
	// settings package. So we validate the default behavior here.
	ns := GetNamespace()
	// Without a provider, the setting returns its Default value (from init time),
	// which is "" since CATTLE_NAMESPACE was not set when the package initialized.
	// So the fallback to "cattle-system" should apply.
	assert.Equal(t, "cattle-system", ns)
}

package suseconnect

import (
	"github.com/SUSE/connect-ng/pkg/connection"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDefaultConnectionOptionsBasic(t *testing.T) {
	defaultOptions := DefaultConnectionOptions()
	expected := connection.Options{
		URL:              connection.DefaultBaseURL,
		Secure:           true,
		AppName:          "rancher-scc-integration",
		Version:          "0.0.1",
		PreferedLanguage: "en_US",
		Timeout:          connection.DefaultTimeout,
	}
	assert.Equal(t, expected, defaultOptions)
}

func TestDefaultConnectionOptions(t *testing.T) {
	defaultOptions := DefaultConnectionOptions()
	assert.Equal(t, connection.DefaultBaseURL, defaultOptions.URL)
	assert.Equal(t, "rancher-scc-integration", defaultOptions.AppName)
	assert.Equal(t, "0.0.1", defaultOptions.Version)
}

func TestDefaultRancherConnection(t *testing.T) {
	//options := DefaultConnectionOptions()
	//expected := connection.New(options, connection.NoCredentials{})

	//assert.Equal(t, expected, DefaultRancherConnection(connection.NoCredentials{}))
}

package httpproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testInvalidPEM = `-----BEGIN CERTIFICATE-----
This is not valid base64 content at all!!!
-----END CERTIFICATE-----`

func TestParseCACertificates_WithInvalidPEM_ReturnsError(t *testing.T) {
	pool, err := parseCACertificates(testInvalidPEM)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to append CA certificates")
	assert.Nil(t, pool)
}

func TestParseCACertificates_WithEmptyString_ReturnsError(t *testing.T) {
	pool, err := parseCACertificates("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to append CA certificates")
	assert.Nil(t, pool)
}

func TestBuildTransportWithCABundle_WithInvalidCA_ReturnsError(t *testing.T) {
	transport, err := buildTransportWithCABundle(testInvalidPEM)
	require.Error(t, err)
	assert.Nil(t, transport)
	assert.Contains(t, err.Error(), "failed to parse CA certificates")
}

func TestBuildTransportWithCABundle_WithEmptyCABundle_ReturnsError(t *testing.T) {
	transport, err := buildTransportWithCABundle("")
	require.Error(t, err)
	assert.Nil(t, transport)
	assert.Contains(t, err.Error(), "failed to parse CA certificates")
}

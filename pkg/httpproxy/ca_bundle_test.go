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
	assert.Contains(t, err.Error(), "failed to parse CA bundle PEM data")
	assert.Nil(t, pool)
}

func TestParseCACertificates_WithEmptyString_ReturnsError(t *testing.T) {
	pool, err := parseCACertificates("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CA bundle must contain at least one CERTIFICATE PEM block")
	assert.Nil(t, pool)
}

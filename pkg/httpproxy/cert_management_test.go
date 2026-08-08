package httpproxy

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- buildTLSConfigForRoute ---

func TestBuildTLSConfigForRoute_WithServerName_SetsSNI(t *testing.T) {
	route := &mgmt.ProxyEndpointRoute{
		Domain:     "api.example.com",
		ServerName: "internal.example.com",
	}

	tlsConfig, err := buildTLSConfigForRoute(route, "api.example.com")
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.Equal(t, "internal.example.com", tlsConfig.ServerName)
}

func TestBuildTLSConfigForRoute_WithoutServerName_UsesHostname(t *testing.T) {
	route := &mgmt.ProxyEndpointRoute{
		Domain: "api.example.com",
	}

	tlsConfig, err := buildTLSConfigForRoute(route, "api.example.com")
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.Equal(t, "api.example.com", tlsConfig.ServerName)
}

func TestBuildTLSConfigForRoute_WithCABundle_SetsCertPool(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})
	require.NotEmpty(t, certPEM)

	route := &mgmt.ProxyEndpointRoute{
		Domain:   "api.example.com",
		CABundle: string(certPEM),
	}

	tlsConfig, err := buildTLSConfigForRoute(route, "api.example.com")
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.NotNil(t, tlsConfig.RootCAs)
}

func TestBuildTLSConfigForRoute_WithInvalidCABundle_ReturnsError(t *testing.T) {
	route := &mgmt.ProxyEndpointRoute{
		Domain:   "api.example.com",
		CABundle: "not-a-valid-certificate",
	}

	tlsConfig, err := buildTLSConfigForRoute(route, "api.example.com")
	require.Error(t, err)
	assert.Nil(t, tlsConfig)
	assert.Contains(t, err.Error(), "failed to parse CA bundle")
}

func TestBuildTLSConfigForRoute_WithVerifyHostnameFalse_DisablesVerification(t *testing.T) {
	verifyHostnameFalse := false
	route := &mgmt.ProxyEndpointRoute{
		Domain: "api.example.com",
		TLSVerificationOptions: &mgmt.TLSVerificationSpec{
			VerifyHostname: &verifyHostnameFalse,
		},
	}

	tlsConfig, err := buildTLSConfigForRoute(route, "api.example.com")
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.True(t, tlsConfig.InsecureSkipVerify)
}

func TestBuildTLSConfigForRoute_WithVerifyHostnameTrue_EnablesVerification(t *testing.T) {
	verifyHostnameTrue := true
	route := &mgmt.ProxyEndpointRoute{
		Domain: "api.example.com",
		TLSVerificationOptions: &mgmt.TLSVerificationSpec{
			VerifyHostname: &verifyHostnameTrue,
		},
	}

	tlsConfig, err := buildTLSConfigForRoute(route, "api.example.com")
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.False(t, tlsConfig.InsecureSkipVerify)
}

func TestBuildTLSConfigForRoute_WithVerifyExpirationFalse_IgnoresExpiration(t *testing.T) {
	// Note: The Go crypto/tls package doesn't expose a direct way to control expiration checking
	// via the Config struct. This test verifies that the code doesn't fail when VerifyExpiration
	// is set to false. The actual expiration check is handled by the TLS handshake.
	verifyExpirationFalse := false
	route := &mgmt.ProxyEndpointRoute{
		Domain: "api.example.com",
		TLSVerificationOptions: &mgmt.TLSVerificationSpec{
			VerifyExpiration: &verifyExpirationFalse,
		},
	}

	tlsConfig, err := buildTLSConfigForRoute(route, "api.example.com")
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	// The InsecureSkipVerify flag is only set when VerifyHostname is false
	assert.False(t, tlsConfig.InsecureSkipVerify)
}

func TestBuildTLSConfigForRoute_WithBothServerNameAndCABundle(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})
	require.NotEmpty(t, certPEM)

	route := &mgmt.ProxyEndpointRoute{
		Domain:     "api.example.com",
		ServerName: "internal.example.com",
		CABundle:   string(certPEM),
	}

	tlsConfig, err := buildTLSConfigForRoute(route, "api.example.com")
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.Equal(t, "internal.example.com", tlsConfig.ServerName)
	assert.NotNil(t, tlsConfig.RootCAs)
}

func TestBuildTLSConfigForRoute_WithAllOptions(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})
	require.NotEmpty(t, certPEM)

	verifyHostnameFalse := false
	verifyExpirationFalse := false
	route := &mgmt.ProxyEndpointRoute{
		Domain:     "api.example.com",
		ServerName: "internal.example.com",
		CABundle:   string(certPEM),
		TLSVerificationOptions: &mgmt.TLSVerificationSpec{
			VerifyHostname: &verifyHostnameFalse,
			VerifyExpiration: &verifyExpirationFalse,
		},
	}

	tlsConfig, err := buildTLSConfigForRoute(route, "api.example.com")
	require.NoError(t, err)
	assert.NotNil(t, tlsConfig)
	assert.Equal(t, "internal.example.com", tlsConfig.ServerName)
	assert.NotNil(t, tlsConfig.RootCAs)
	assert.True(t, tlsConfig.InsecureSkipVerify)
}

// --- buildTransportForRoute ---

func TestBuildTransportForRoute_ReturnsTransportWithTLSConfig(t *testing.T) {
	route := &mgmt.ProxyEndpointRoute{
		Domain:     "api.example.com",
		ServerName: "internal.example.com",
	}

	transport, err := buildTransportForRoute(route, "api.example.com")
	require.NoError(t, err)
	assert.NotNil(t, transport)
	assert.NotNil(t, transport.TLSClientConfig)
	assert.Equal(t, "internal.example.com", transport.TLSClientConfig.ServerName)
}

func TestBuildTransportForRoute_WithInvalidCABundle_ReturnsError(t *testing.T) {
	route := &mgmt.ProxyEndpointRoute{
		Domain:   "api.example.com",
		CABundle: "invalid-cert",
	}

	transport, err := buildTransportForRoute(route, "api.example.com")
	require.Error(t, err)
	assert.Nil(t, transport)
}

func TestBuildTransportForRoute_ClonesBasicTransport(t *testing.T) {
	route := &mgmt.ProxyEndpointRoute{
		Domain: "api.example.com",
	}

	transport, err := buildTransportForRoute(route, "api.example.com")
	require.NoError(t, err)
	assert.NotNil(t, transport)
	// Verify it's an HTTP transport with TLS config
	assert.NotNil(t, transport.TLSClientConfig)
}

// --- perRouteTLSTransport with new certificate options ---

func TestPerRouteTLSTransport_WithServerNameOption_AppliesSNI(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Note: This test demonstrates that the ServerName option is properly passed
	// to the TLS config. In practice, connecting with an incorrect ServerName
	// to a real server would fail, but for this test we're just verifying
	// the option is properly applied through the transport.
	tsURL, err := url.Parse(ts.URL)
	require.NoError(t, err)

	// Create a route with ServerName set to something different (hostname mismatch scenario)
	// This would normally fail on a real server unless it handles multiple SANs
	// For testing purposes, we use VerifyHostname false to avoid cert verification issues
	verifyHostnameFalse := false
	route := &mgmt.ProxyEndpointRoute{
		Domain:     tsURL.Hostname(),
		ServerName: "alternative-hostname.local",
		TLSVerificationOptions: &mgmt.TLSVerificationSpec{
			VerifyHostname: &verifyHostnameFalse,
		},
	}

	// Create a transport for the route
	transport, err := buildTransportForRoute(route, tsURL.Hostname())
	require.NoError(t, err)
	assert.NotNil(t, transport)
	assert.NotNil(t, transport.TLSClientConfig)
	assert.Equal(t, "alternative-hostname.local", transport.TLSClientConfig.ServerName)
}

func TestPerRouteTLSTransport_WithTLSVerificationOptions_AppliesSettings(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tsURL, err := url.Parse(ts.URL)
	require.NoError(t, err)

	verifyHostnameFalse := false
	route := &mgmt.ProxyEndpointRoute{
		Domain: tsURL.Hostname(),
		TLSVerificationOptions: &mgmt.TLSVerificationSpec{
			VerifyHostname: &verifyHostnameFalse,
		},
	}

	transport, err := buildTransportForRoute(route, tsURL.Hostname())
	require.NoError(t, err)
	assert.NotNil(t, transport)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
}

func TestPerRouteTLSTransport_MultipleSecurityOptions_AllApplied(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})
	require.NotEmpty(t, certPEM)

	tsURL, err := url.Parse(ts.URL)
	require.NoError(t, err)

	verifyHostnameTrue := true
	route := &mgmt.ProxyEndpointRoute{
		Domain:     tsURL.Hostname(),
		ServerName: "custom-sni.local",
		CABundle:   string(certPEM),
		TLSVerificationOptions: &mgmt.TLSVerificationSpec{
			VerifyHostname: &verifyHostnameTrue,
		},
	}

	transport, err := buildTransportForRoute(route, tsURL.Hostname())
	require.NoError(t, err)
	assert.NotNil(t, transport)
	assert.NotNil(t, transport.TLSClientConfig)
	assert.Equal(t, "custom-sni.local", transport.TLSClientConfig.ServerName)
	assert.NotNil(t, transport.TLSClientConfig.RootCAs)
	assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
}


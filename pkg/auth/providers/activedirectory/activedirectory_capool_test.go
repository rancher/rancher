package activedirectory

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestCACert returns a self-signed CA certificate in PEM.
func newTestCACert(t *testing.T, commonName string) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	certificate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	require.NoError(t, err)

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// newTestADProvider returns a provider with an Active Directory AuthConfig
// for each of the named certificates.
func newTestADProvider(t *testing.T, certs map[string]string) *adProvider {
	t.Helper()

	var configs []map[string]any
	for name, cert := range certs {
		configs = append(configs, map[string]any{
			"metadata":    map[string]any{"name": name},
			"type":        client.ActiveDirectoryConfigType,
			"certificate": cert,
		})
	}

	return &adProvider{authConfigs: authconfigtest.NewClient(t, configs...)}
}

// wantCAPool returns the system pool with cert added, built independently of
// newCAPool so that a broken newCAPool can't also break the expected pool.
func wantCAPool(t *testing.T, cert string) *x509.CertPool {
	t.Helper()

	pool, err := x509.SystemCertPool()
	require.NoError(t, err)
	require.True(t, pool.AppendCertsFromPEM([]byte(cert)), "the test certificate can't be parsed")

	return pool
}

// Each AuthConfig's CA pool must hold its own certificate, also when
// AuthConfigs with different certificates are used concurrently. Run with
// -race to catch unsynchronized shared state.
func TestGetActiveDirectoryConfigCAPoolPerConfig(t *testing.T) {
	certs := map[string]string{
		"ad-corp": newTestCACert(t, "corp-ca"),
		"ad-it":   newTestCACert(t, "it-ca"),
	}
	wantPools := map[string]*x509.CertPool{}
	for name, cert := range certs {
		wantPools[name] = wantCAPool(t, cert)
	}
	require.False(t, wantPools["ad-corp"].Equal(wantPools["ad-it"]), "the configs' pools must differ for the test to mean anything")
	p := newTestADProvider(t, certs)

	var wg sync.WaitGroup
	for range 10 {
		for name := range certs {
			wg.Go(func() {
				_, pool, err := p.getActiveDirectoryConfig(name)
				if !assert.NoError(t, err) {
					return
				}
				assert.True(t, pool.Equal(wantPools[name]), "%s got the CA pool of another AuthConfig", name)
			})
		}
	}
	wg.Wait()
}

// A CA pool must not be shared between calls, so that one request can't see
// another's pool. Unlike TestGetActiveDirectoryConfigCAPoolPerConfig this
// doesn't depend on timing or on -race.
func TestGetActiveDirectoryConfigCAPoolNotShared(t *testing.T) {
	certs := map[string]string{
		"ad-corp": newTestCACert(t, "corp-ca"),
		"ad-it":   newTestCACert(t, "it-ca"),
	}
	p := newTestADProvider(t, certs)

	_, first, err := p.getActiveDirectoryConfig("ad-corp")
	require.NoError(t, err)
	require.True(t, first.Equal(wantCAPool(t, certs["ad-corp"])))

	// Changing the pool returned to one call must not change the pool
	// returned to the next.
	first.AppendCertsFromPEM([]byte(newTestCACert(t, "other-ca")))

	_, second, err := p.getActiveDirectoryConfig("ad-corp")
	require.NoError(t, err)
	assert.True(t, second.Equal(wantCAPool(t, certs["ad-corp"])), "the CA pool is shared between calls")
}

package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

var (
	insecureHTTPTransport http.RoundTripper = func() *http.Transport {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		return tr
	}()
)

const (
	kubernetesSSLCertsDir = "/etc/kubernetes/ssl/certs"
	caFilename            = "serverca"
	caFileLocation        = kubernetesSSLCertsDir + "/" + caFilename
	dockerCertsDir        = "/etc/docker/certs.d"
	dockerCertFilename    = "ca.crt"
)

// preStart is meant as a replacement for entrypoint logic.
// These actions were previously executed as part of the agent image's entrypoint script, before actually executing the agent binary
// The logic has been migrated from that shell script to native Go
func preStart(ctx context.Context) error {
	cattleServer := os.Getenv("CATTLE_SERVER")
	if err := pingCattleServer(ctx, cattleServer); err != nil {
		return fmt.Errorf("error pinging %s: %w", cattleServer, err)
	}

	if err := printResolvedCattleServerHostname(ctx, cattleServer, net.DefaultResolver); err != nil {
		return err
	}

	if cattleCAChecksum := os.Getenv("CATTLE_CA_CHECKSUM"); cattleCAChecksum != "" {
		// eases testing
		certsDestination := certsDirs{
			kubernetesSSLCertsDir: kubernetesSSLCertsDir,
			dockerCertsDir:        dockerCertsDir,
		}
		if err := populateRancherCACerts(ctx, cattleServer, cattleCAChecksum, certsDestination); err != nil {
			return err
		}
	}
	return nil
}

// pingCattleServer checks Rancher server's connectivity
func pingCattleServer(ctx context.Context, cattleServerEnv string) (pingError error) {
	pingURL := cattleServerEnv + "/ping"
	req, err := http.NewRequestWithContext(ctx, "GET", pingURL, nil)
	if err != nil {
		return err
	}

	defer func() {
		if pingError != nil {
			logrus.Errorf("%s is not accessible: %v", pingURL, pingError)
		} else {
			logrus.Infof("%s is accessible", pingURL)
		}
	}()

	httpClient := http.Client{
		Timeout:   5 * time.Minute,
		Transport: insecureHTTPTransport,
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	if _, err := io.Copy(io.Discard, res.Body); err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("received %q status response", res.Status)
	}

	return nil
}

type netResolver interface {
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
}

// printResolvedCattleServerHostname prints the IP(s) to which the provided CATTLE_SERVER hostname resolves
// If an IP was already used, then it is a no-op
func printResolvedCattleServerHostname(ctx context.Context, cattleServerEnv string, resolver netResolver) error {
	parsed, err := url.Parse(cattleServerEnv)
	if err != nil {
		return err
	}
	cattleServerHostname := parsed.Hostname()

	resolved, err := resolver.LookupHost(ctx, cattleServerHostname)
	if err != nil {
		return err
	} else if len(resolved) == 1 && resolved[0] == cattleServerHostname {
		// It's an IP address, nothing to resolve
		return nil
	}

	logrus.Infof("%s resolves to %s", cattleServerHostname, strings.Join(resolved, ", "))
	return nil
}

// populateRancherCACerts will retrieve root CA certificates from Rancher, verify them against the provided checksum, then finally write it to the filesystem.
func populateRancherCACerts(ctx context.Context, cattleServerEnv string, cattleCAChecksumEnv string, certs certsDirs) error {
	certsURL := cattleServerEnv + "/v3/settings/cacerts"
	rootCA, err := retrieveRancherCACerts(ctx, certsURL)
	if err != nil {
		return err
	} else if len(rootCA) == 0 {
		return fmt.Errorf("The environment variable CATTLE_CA_CHECKSUM is set but there is no CA certificate configured at %s", certsURL)
	}

	// Verify integrity
	sha256Hash := sha256.Sum256(rootCA)
	if calculated := hex.EncodeToString(sha256Hash[:]); calculated != cattleCAChecksumEnv {
		return fmt.Errorf("Configured cacerts checksum (%s) does not match given --ca-checksum (%s)\nPlease check if the correct certificate is configured at %s", calculated, cattleCAChecksumEnv, certsURL)
	}

	// Verify certificates validity
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(rootCA); !ok {
		return fmt.Errorf("Value from %s does not look like an x509 certificate\nRetrieved cacerts:\n%s", certsURL, rootCA)
	}
	logrus.Infof("Value from %s is an x509 certificate", certsURL)

	return certs.writeRootCACerts(rootCA, cattleServerEnv)
}

// retrieveRancherCACerts retrieves the "cacerts" setting from Rancher via the provided URL, decodes the response and returns the armored certificate payload.
// It's up to the caller to verify the integrity and contents of this data.
func retrieveRancherCACerts(ctx context.Context, certsSettingsURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", certsSettingsURL, nil)
	if err != nil {
		return nil, err
	}

	httpClient := http.Client{
		Timeout:   5 * time.Minute,
		Transport: insecureHTTPTransport,
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get CA certificates: %s", res.Status)
	}

	// Decode JSON response
	var cacertsSetting *v3.Setting
	if err := json.NewDecoder(res.Body).Decode(&cacertsSetting); err != nil {
		return nil, fmt.Errorf("failed to parse CA certificates setting: %w", err)
	}
	// Ensure it ends with newline (expected by the checksum)
	value := strings.TrimSuffix(cacertsSetting.Value, "\n") + "\n"
	return []byte(value), nil
}

type certsDirs struct {
	kubernetesSSLCertsDir string
	dockerCertsDir        string
}

// writeRootCACerts writes the root CA certificates retrieved from Rancher to 2 different places:
// - /etc/kubernetes/ssl/certs/serverca
// - /etc/docker/certs.d/$CATTLE_SERVER_HOSTNAME_WITH_PORT/ca.crt
func (w certsDirs) writeRootCACerts(rootCA []byte, cattleServerEnv string) error {
	if err := os.MkdirAll(w.kubernetesSSLCertsDir, 0755); err != nil {
		return err
	}
	if err := os.Chmod(w.kubernetesSSLCertsDir, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(w.kubernetesSSLCertsDir, caFilename), rootCA, 0600); err != nil {
		return err
	}

	parsed, err := url.Parse(cattleServerEnv)
	if err != nil {
		return err
	}
	cattleServerHostnameWithPort := parsed.Host

	dest := filepath.Join(w.dockerCertsDir, cattleServerHostnameWithPort, dockerCertFilename)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	return os.WriteFile(dest, rootCA, 0600)
}

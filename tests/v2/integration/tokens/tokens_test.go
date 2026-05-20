package integration

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	rancherClient "github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/rest"
)

type TokensTestSuite struct {
	suite.Suite
	client  *rancherClient.Client
	session *session.Session
}

func (s *TokensTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancherClient.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *TokensTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *TokensTestSuite) httpClient() *http.Client {
	httpClient, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	s.Require().NoError(err)
	return httpClient
}

// insecureHTTPClient returns an unauthenticated HTTP client that skips TLS
// verification, for use with public (unauthenticated) endpoints.
func (s *TokensTestSuite) insecureHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// TestCurrentToken verifies that listing tokens returns exactly one token
// marked as current, and that its userId matches the admin user.
func (s *TokensTestSuite) TestCurrentToken() {
	tokens, err := s.client.Management.Token.ListAll(nil)
	s.Require().NoError(err)

	// Find the current user's ID from the admin token itself.
	adminToken := s.client.WranglerContext.RESTConfig.BearerToken
	var adminUserID string
	for _, t := range tokens.Data {
		if t.Token == adminToken || t.Current {
			adminUserID = t.UserID
			break
		}
	}

	currentCount := 0
	for _, t := range tokens.Data {
		if t.Current {
			s.Equal(adminUserID, t.UserID)
			currentCount++
		}
	}
	s.Equal(1, currentCount)
}

// TestWebsocket verifies that requests with websocket-like upgrade headers are
// rejected with 403 Forbidden.
func (s *TokensTestSuite) TestWebsocket() {
	host := s.client.WranglerContext.RESTConfig.Host
	httpClient := s.httpClient()

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://%s/v3/clusters", host), nil)
	s.Require().NoError(err)
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Origin", "badStuff")
	req.Header.Set("User-Agent", "Mozilla")

	resp, err := httpClient.Do(req)
	s.Require().NoError(err)
	io.ReadAll(resp.Body) //nolint:errcheck
	resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode)
}

// TestAPITokenTTL verifies that a token created with ttl=0 is capped to the
// max TTL configured in the auth-token-max-ttl-minutes setting.
func (s *TokensTestSuite) TestAPITokenTTL() {
	subSession := s.session.NewSession()
	s.T().Cleanup(subSession.Cleanup)
	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	maxTTLSetting, err := client.Management.Setting.ByID("auth-token-max-ttl-minutes")
	s.Require().NoError(err)
	maxTTLMins, err := strconv.ParseInt(maxTTLSetting.Value, 10, 64)
	s.Require().NoError(err)

	created, err := client.Management.Token.Create(&management.Token{
		TTLMillis: 0,
	})
	s.Require().NoError(err)

	// TTLMillis is in milliseconds; convert to minutes.
	tokenTTLMins := created.TTLMillis / 60000
	s.Equal(maxTTLMins, tokenTTLMins)
}

// TestKubeconfigTokenTTL verifies that kubeconfig tokens respect the
// kubeconfig-default-token-ttl-minutes setting and expire correctly, for both
// the /v3-public and /v1-public login endpoints.
func (s *TokensTestSuite) TestKubeconfigTokenTTL() {
	client := s.client
	host := client.RancherConfig.Host
	adminPassword := client.RancherConfig.AdminPassword
	httpClient := s.insecureHTTPClient()

	// Delete any existing kubeconfig token for this admin user.
	adminTokens, err := client.Management.Token.ListAll(nil)
	s.Require().NoError(err)
	for i := range adminTokens.Data {
		t := &adminTokens.Data[i]
		if t.Current {
			kubeconfigTokenName := "kubeconfig-" + t.UserID
			existing, err := client.Management.Token.ByID(kubeconfigTokenName)
			if err == nil && existing != nil {
				_ = client.Management.Token.Delete(existing)
			}
			break
		}
	}

	// Save original setting values so they can be restored at the end.
	origGenerateSetting, err := client.Management.Setting.ByID("kubeconfig-generate-token")
	s.Require().NoError(err)
	origTTLSetting, err := client.Management.Setting.ByID("kubeconfig-default-token-ttl-minutes")
	s.Require().NoError(err)

	s.T().Cleanup(func() {
		_, _ = client.Management.Setting.Update(origGenerateSetting, &management.Setting{Value: origGenerateSetting.Value})
		_, _ = client.Management.Setting.Update(origTTLSetting, &management.Setting{Value: origTTLSetting.Value})
	})

	// Disable kubeconfig token generation and set a very short TTL (0.01 min ≈ 600ms).
	_, err = client.Management.Setting.Update(origGenerateSetting, &management.Setting{Value: "false"})
	s.Require().NoError(err)
	_, err = client.Management.Setting.Update(origTTLSetting, &management.Setting{Value: "0.01"})
	s.Require().NoError(err)

	const kubeconfigTTLSecs = 0.01 * 60 // 0.01 minutes in seconds

	// --- /v3-public endpoint ---
	token1 := s.loginV3(httpClient, host, adminPassword)
	s.NotEmpty(token1["token"])
	s.NotEmpty(token1["expiresAt"])
	s.NotEmpty(token1["id"])
	s.True(fmt.Sprintf("%v", token1["token"]) != "" &&
		len(fmt.Sprintf("%v", token1["token"])) > len(fmt.Sprintf("%v", token1["id"])))
	s.Equal("token", token1["type"])
	s.Equal("token", token1["baseType"])

	// Wait for the token to expire.
	time.Sleep(time.Duration(kubeconfigTTLSecs * float64(time.Second)))

	// A new login should generate a different token.
	token2 := s.loginV3(httpClient, host, adminPassword)
	s.NotEmpty(token2["token"])
	s.NotEmpty(token2["expiresAt"])
	s.NotEqual(token1["token"], token2["token"])

	// Wait for token2 to expire before testing the v1 endpoint.
	time.Sleep(time.Duration(kubeconfigTTLSecs * float64(time.Second)))

	// --- /v1-public endpoint ---
	token3 := s.loginV1(httpClient, host, adminPassword)
	s.NotEmpty(token3["token"])
	s.NotEmpty(token3["expiresAt"])

	// Wait for the token to expire.
	time.Sleep(time.Duration(kubeconfigTTLSecs * float64(time.Second)))

	token4 := s.loginV1(httpClient, host, adminPassword)
	s.NotEmpty(token4["token"])
	s.NotEmpty(token4["expiresAt"])
}

// loginV3 calls the /v3-public login endpoint with responseType=kubeconfig.
func (s *TokensTestSuite) loginV3(httpClient *http.Client, host, password string) map[string]any {
	body, err := json.Marshal(map[string]any{
		"username":     "admin",
		"password":     password,
		"responseType": "kubeconfig",
	})
	s.Require().NoError(err)

	resp, err := httpClient.Post(
		fmt.Sprintf("https://%s/v3-public/localProviders/local?action=login", host),
		"application/json",
		bytes.NewReader(body),
	)
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d: %s", resp.StatusCode, string(respBody))

	var result map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &result))
	return result
}

// loginV1 calls the /v1-public/login endpoint with responseType=kubeconfig.
func (s *TokensTestSuite) loginV1(httpClient *http.Client, host, password string) map[string]any {
	body, err := json.Marshal(map[string]any{
		"type":         "localProvider",
		"username":     "admin",
		"password":     password,
		"responseType": "kubeconfig",
	})
	s.Require().NoError(err)

	resp, err := httpClient.Post(
		fmt.Sprintf("https://%s/v1-public/login", host),
		"application/json",
		bytes.NewReader(body),
	)
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d: %s", resp.StatusCode, string(respBody))

	var result map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &result))
	return result
}

func TestTokens(t *testing.T) {
	suite.Run(t, new(TokensTestSuite))
}

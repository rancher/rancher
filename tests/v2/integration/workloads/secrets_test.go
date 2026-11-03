package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/rancher/rancher/tests/v2/integration/actions/namespaces"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type SecretsTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *SecretsTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *SecretsTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *SecretsTestSuite) httpClient() *http.Client {
	httpClient, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	s.Require().NoError(err)
	return httpClient
}

func (s *SecretsTestSuite) resourceURL(project *management.Project, resourceType string) string {
	return fmt.Sprintf("https://%s/v3/project/%s/%s",
		s.client.WranglerContext.RESTConfig.Host, project.ID, resourceType)
}

func (s *SecretsTestSuite) post(httpClient *http.Client, url string, body map[string]any) map[string]any {
	b, err := json.Marshal(body)
	s.Require().NoError(err)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(b))
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

func (s *SecretsTestSuite) put(httpClient *http.Client, baseURL, id string, body map[string]any) map[string]any {
	b, err := json.Marshal(body)
	s.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s", baseURL, id), bytes.NewReader(b))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
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

func (s *SecretsTestSuite) get(httpClient *http.Client, baseURL, id string) map[string]any {
	resp, err := httpClient.Get(fmt.Sprintf("%s/%s", baseURL, id))
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var result map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &result))
	return result
}

func (s *SecretsTestSuite) del(httpClient *http.Client, baseURL, id string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%s", baseURL, id), nil)
	s.Require().NoError(err)
	resp, err := httpClient.Do(req)
	s.Require().NoError(err)
	io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d deleting resource", resp.StatusCode)
}

func (s *SecretsTestSuite) assertInList(httpClient *http.Client, listURL, id string) {
	resp, err := httpClient.Get(listURL)
	s.Require().NoError(err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var list struct {
		Data []map[string]any `json:"data"`
	}
	s.Require().NoError(json.Unmarshal(body, &list))

	for _, item := range list.Data {
		if item["id"] == id {
			return
		}
	}
	s.Failf("resource not found in list", "id %s not found in %s", id, listURL)
}

func (s *SecretsTestSuite) setupProject() (*rancher.Client, *management.Project) {
	subSession := s.session.NewSession()
	s.T().Cleanup(subSession.Cleanup)

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	project, err := client.Management.Project.Create(&management.Project{
		ClusterID: "local",
		Name:      namegen.AppendRandomString("project-"),
	})
	s.Require().NoError(err)

	return client, project
}

func (s *SecretsTestSuite) setupProjectNS() (*rancher.Client, *management.Project, string) {
	subSession := s.session.NewSession()
	s.T().Cleanup(subSession.Cleanup)

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	project, err := client.Management.Project.Create(&management.Project{
		ClusterID: "local",
		Name:      namegen.AppendRandomString("project-"),
	})
	s.Require().NoError(err)

	ns, err := namespaces.CreateNamespace(client, namegen.AppendRandomString("ns-"), "", map[string]string{}, map[string]string{}, project)
	s.Require().NoError(err)

	return client, project, ns.Name
}

// TestSecrets asserts that a project-scoped Opaque secret can be created,
// updated, listed, and deleted via the Norman project API.
func (s *SecretsTestSuite) TestSecrets() {
	_, project := s.setupProject()
	httpClient := s.httpClient()
	url := s.resourceURL(project, "secrets")

	name := namegen.AppendRandomString("secret-")
	created := s.post(httpClient, url, map[string]any{
		"name":       name,
		"stringData": map[string]any{"foo": "bar"},
	})

	s.Equal("secret", created["type"])
	s.Equal("Opaque", created["kind"])
	s.Equal(name, created["name"])
	data := created["data"].(map[string]any)
	s.Equal("YmFy", data["foo"])

	id := created["id"].(string)

	// Update: add a new key to data.
	s.put(httpClient, url, id, map[string]any{
		"data": map[string]any{"foo": "YmFy", "baz": "YmFy"},
	})

	// Reload via GET.
	reloaded := s.get(httpClient, url, id)
	s.Equal("secret", reloaded["baseType"])
	s.Equal("secret", reloaded["type"])
	s.Equal("Opaque", reloaded["kind"])
	s.Equal(name, reloaded["name"])
	reloadedData := reloaded["data"].(map[string]any)
	s.Equal("YmFy", reloadedData["foo"])
	s.Equal("YmFy", reloadedData["baz"])
	s.Nil(reloaded["namespaceId"])
	s.NotContains(reloadedData, "namespace")
	s.Equal(project.ID, reloaded["projectId"])

	s.assertInList(httpClient, url, id)
	s.del(httpClient, url, id)
}

// TestCertificates asserts that a project-scoped certificate can be created,
// listed, and fetched by ID via the Norman project API.
func (s *SecretsTestSuite) TestCertificates() {
	_, project := s.setupProject()
	httpClient := s.httpClient()
	url := s.resourceURL(project, "certificates")

	name := namegen.AppendRandomString("cert-")
	created := s.post(httpClient, url, map[string]any{
		"name":  name,
		"key":   keyPEM,
		"certs": certPEM,
	})

	s.Equal("secret", created["baseType"])
	s.Equal("2026-06-28T01:13:32Z", created["expiresAt"])
	s.Equal("certificate", created["type"])
	s.Equal(name, created["name"])
	s.Equal(certPEM, created["certs"])
	s.Nil(created["namespaceId"])
	s.NotContains(created, "namespace")

	id := created["id"].(string)

	s.assertInList(httpClient, url, id)

	byID := s.get(httpClient, url, id)
	s.NotNil(byID)

	s.del(httpClient, url, id)
}

// TestDockerCredential asserts that a project-scoped docker credential can be
// created, updated, listed, and fetched by ID via the Norman project API.
func (s *SecretsTestSuite) TestDockerCredential() {
	_, project := s.setupProject()
	httpClient := s.httpClient()
	url := s.resourceURL(project, "dockerCredentials")

	name := namegen.AppendRandomString("dockercred-")
	created := s.post(httpClient, url, map[string]any{
		"name": name,
		"registries": map[string]any{
			"index.docker.io": map[string]any{
				"username": "foo",
				"password": "bar",
			},
		},
	})

	s.Equal("secret", created["baseType"])
	s.Equal("dockerCredential", created["type"])
	s.Equal(name, created["name"])
	regs := created["registries"].(map[string]any)
	dockerIO := regs["index.docker.io"].(map[string]any)
	s.Equal("foo", dockerIO["username"])
	s.Contains(dockerIO, "password")
	s.Contains(dockerIO, "auth")
	s.Nil(created["namespaceId"])
	s.NotContains(created, "namespace")
	s.Equal(project.ID, created["projectId"])

	id := created["id"].(string)

	// Update: add a second registry.
	s.put(httpClient, url, id, map[string]any{
		"registries": map[string]any{
			"index.docker.io": map[string]any{
				"username": "foo",
				"password": "bar",
			},
			"two": map[string]any{
				"username": "blah",
			},
		},
	})

	// Reload via GET.
	reloaded := s.get(httpClient, url, id)
	s.Equal("secret", reloaded["baseType"])
	s.Equal("dockerCredential", reloaded["type"])
	s.Equal(name, reloaded["name"])
	reloadedRegs := reloaded["registries"].(map[string]any)
	reloadedDockerIO := reloadedRegs["index.docker.io"].(map[string]any)
	s.Equal("foo", reloadedDockerIO["username"])
	reloadedTwo := reloadedRegs["two"].(map[string]any)
	s.Equal("blah", reloadedTwo["username"])
	s.NotContains(reloadedDockerIO, "password")
	s.Nil(reloaded["namespaceId"])
	s.NotContains(reloaded, "namespace")
	s.Equal(project.ID, reloaded["projectId"])

	s.assertInList(httpClient, url, id)

	byID := s.get(httpClient, url, id)
	s.NotNil(byID)

	s.del(httpClient, url, id)
}

// TestBasicAuth asserts that a project-scoped basic-auth secret can be
// created, updated, listed, and fetched by ID via the Norman project API.
func (s *SecretsTestSuite) TestBasicAuth() {
	_, project := s.setupProject()
	httpClient := s.httpClient()
	url := s.resourceURL(project, "basicAuths")

	name := namegen.AppendRandomString("basicauth-")
	created := s.post(httpClient, url, map[string]any{
		"name":     name,
		"username": "foo",
		"password": "bar",
	})

	s.Equal("secret", created["baseType"])
	s.Equal("basicAuth", created["type"])
	s.Equal(name, created["name"])
	s.Equal("foo", created["username"])
	s.Contains(created, "password")
	s.Nil(created["namespaceId"])
	s.NotContains(created, "namespace")
	s.Equal(project.ID, created["projectId"])

	id := created["id"].(string)

	// Update username.
	s.put(httpClient, url, id, map[string]any{
		"username": "foo2",
	})

	// Reload via GET.
	reloaded := s.get(httpClient, url, id)
	s.Equal("secret", reloaded["baseType"])
	s.Equal("basicAuth", reloaded["type"])
	s.Equal(name, reloaded["name"])
	s.Equal("foo2", reloaded["username"])
	s.NotContains(reloaded, "password")
	s.Nil(reloaded["namespaceId"])
	s.NotContains(reloaded, "namespace")
	s.Equal(project.ID, reloaded["projectId"])

	s.assertInList(httpClient, url, id)

	byID := s.get(httpClient, url, id)
	s.NotNil(byID)

	s.del(httpClient, url, id)
}

// TestSSHAuth asserts that a project-scoped SSH auth secret can be created,
// updated, listed, and fetched by ID via the Norman project API.
func (s *SecretsTestSuite) TestSSHAuth() {
	_, project := s.setupProject()
	httpClient := s.httpClient()
	url := s.resourceURL(project, "sshAuths")

	name := namegen.AppendRandomString("sshauth-")
	created := s.post(httpClient, url, map[string]any{
		"name":       name,
		"privateKey": "foo",
	})

	s.Equal("secret", created["baseType"])
	s.Equal("sshAuth", created["type"])
	s.Equal(name, created["name"])
	s.Contains(created, "privateKey")
	s.Nil(created["namespaceId"])
	s.NotContains(created, "namespace")
	s.Equal(project.ID, created["projectId"])

	id := created["id"].(string)

	// Update the private key.
	s.put(httpClient, url, id, map[string]any{
		"privateKey": "foo2",
	})

	// Reload via GET.
	reloaded := s.get(httpClient, url, id)
	s.Equal("secret", reloaded["baseType"])
	s.Equal("sshAuth", reloaded["type"])
	s.Equal(name, reloaded["name"])
	s.NotContains(reloaded, "privateKey")
	s.Nil(reloaded["namespaceId"])
	s.NotContains(reloaded, "namespace")
	s.Equal(project.ID, reloaded["projectId"])

	s.assertInList(httpClient, url, id)

	byID := s.get(httpClient, url, id)
	s.NotNil(byID)

	s.del(httpClient, url, id)
}

// TestSecretCreationKubectl asserts that a TLS secret created directly via the
// Kubernetes API is accessible as a namespacedCertificate through the Rancher
// project API, with an RSA algorithm and valid certificate metadata.
func (s *SecretsTestSuite) TestSecretCreationKubectl() {
	_, project, nsName := s.setupProjectNS()
	httpClient := s.httpClient()
	url := s.resourceURL(project, "namespacedCertificates")

	secretName := namegen.AppendRandomString("tlssecret-")
	_, err := s.client.WranglerContext.Core.Secret().Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: nsName,
		},
		StringData: map[string]string{
			"tls.key": keyPEM,
			"tls.crt": certPEM,
		},
		Type: corev1.SecretTypeTLS,
	})
	s.Require().NoError(err)

	certID := fmt.Sprintf("%s:%s", nsName, secretName)
	cert := s.get(httpClient, url, certID)

	algorithm, _ := cert["algorithm"].(string)
	s.Contains(algorithm, "RSA")
	s.NotNil(cert["expiresAt"])
	s.NotNil(cert["issuedAt"])
}

// TestMalformedSecretParse asserts that a TLS secret with a malformed
// certificate created directly via the Kubernetes API can still be retrieved
// as a namespacedCertificate through the Rancher project API.
func (s *SecretsTestSuite) TestMalformedSecretParse() {
	_, project, nsName := s.setupProjectNS()
	httpClient := s.httpClient()
	url := s.resourceURL(project, "namespacedCertificates")

	secretName := namegen.AppendRandomString("malformedcert-")
	_, err := s.client.WranglerContext.Core.Secret().Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: nsName,
		},
		StringData: map[string]string{
			"tls.key": keyPEM,
			"tls.crt": malformedCertPEM,
		},
		Type: corev1.SecretTypeTLS,
	})
	s.Require().NoError(err)

	certID := fmt.Sprintf("%s:%s", nsName, secretName)
	cert := s.get(httpClient, url, certID)
	s.NotEmpty(cert)
}

func TestSecrets(t *testing.T) {
	suite.Run(t, new(SecretsTestSuite))
}

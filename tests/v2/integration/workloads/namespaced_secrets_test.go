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
	"k8s.io/client-go/rest"
)

// certPEM and keyPEM are a self-signed certificate/key pair used for
// namespacedCertificate tests.
const certPEM = `-----BEGIN CERTIFICATE-----
MIIDEDCCAfgCCQC+HwE8rpMN7jANBgkqhkiG9w0BAQUFADBKMQswCQYDVQQGEwJV
UzEQMA4GA1UECBMHQXJpem9uYTEVMBMGA1UEChMMUmFuY2hlciBMYWJzMRIwEAYD
VQQDEwlsb2NhbGhvc3QwHhcNMTYwNjMwMDExMzMyWhcNMjYwNjI4MDExMzMyWjBK
MQswCQYDVQQGEwJVUzEQMA4GA1UECBMHQXJpem9uYTEVMBMGA1UEChMMUmFuY2hl
ciBMYWJzMRIwEAYDVQQDEwlsb2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQC1PR0EiJjM0wbFQmU/yKSb7AuQdzhdW02ya+RQe+31/B+sOTMr
z9b473KCKf8LiFKFOIQUhR5fPvwyrrIWKCEV9pCp/wM474fX32j0zYaH6ezZjL0r
L6hTeGFScGse3dk7ej2+6nNWexpujos0djFi9Gu11iVHIJyT2Sx66kPPPZVRkJO9
5Pfetm5SLIQtJHUwy5iWv5Br+AbdXlUAjTYUqS4mhKIIbblAPbOKrYRxGXX/6oDV
J5OGLle8Uvlb8poxqmy67FPyMObNHhjggKwboXhmNuuT2OGf/VeZANMYubs4JP2V
ZLs3U/1tFMAOaQM+PbT9JuwMSmGYFX0Qiuh/AgMBAAEwDQYJKoZIhvcNAQEFBQAD
ggEBACpkRCQpCn/zmTOwboBckkOFeqMVo9cvSu0Sez6EPED4WUv/6q5tlJeHekQm
6YVcsXeOMkpfZ7qtGmBDwR+ly7D43dCiPKplm0uApO1CkogG5ePv0agvKHEybd36
xu9pt0fnxDdrP2NrP6trHq1D+CzPZooLRfmYqbt1xmIb00GpnyiJIUNuMu7GUM3q
NxWGK3eq+1cyt6xr8nLOC5zaGeSyZikw4+9vqLudNSyYdnw9mdHtrYT0GlcEP1Vc
NK+yrhDCvEWH6+4+pp8Ve2P2Le5tvbA1m24AxyuC9wHS5bUmiNHweLXNpxLFTjK8
BBUi6y1Vm9jrDi/LiiHcN4sJEoU=
-----END CERTIFICATE-----`

const keyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAtT0dBIiYzNMGxUJlP8ikm+wLkHc4XVtNsmvkUHvt9fwfrDkz
K8/W+O9ygin/C4hShTiEFIUeXz78Mq6yFighFfaQqf8DOO+H199o9M2Gh+ns2Yy9
Ky+oU3hhUnBrHt3ZO3o9vupzVnsabo6LNHYxYvRrtdYlRyCck9kseupDzz2VUZCT
veT33rZuUiyELSR1MMuYlr+Qa/gG3V5VAI02FKkuJoSiCG25QD2ziq2EcRl1/+qA
1SeThi5XvFL5W/KaMapsuuxT8jDmzR4Y4ICsG6F4Zjbrk9jhn/1XmQDTGLm7OCT9
lWS7N1P9bRTADmkDPj20/SbsDEphmBV9EIrofwIDAQABAoIBAGehHxN1i3EqhKeL
9FrJPh4NlPswwCDZUQ7hFDZU9lZ9qBqQxkqZ18CVIXN90eBlPVIBY7xb9Wbem9Pb
AecbYPeu+T7KmqwWgiUUEG5RikfyoMQv7gZghK3dmkBKGWYX0dtpZR7h7bsYPp/S
j5QatNhxC5l4be5CnmUHe6B4jPdUt8kRfTj0ukYGm/h3cOm/tEQeRYIIN/N6JN2Z
JWYzsyqGmlOTp7suczkRIUS0AjiljT1186bQSou62iMtMqEgArusFFb9m/dXCCYo
t/Q1SR4lRodDfzcF/CRbdR/ZC8gZlyCdbI4WHOw9IwwHnmrllx4MXFP/p6p+gEtl
cKMzHXECgYEA27KnkDnz338qKC2cCGkMf3ARfTX6gSlqmvgM9zOa8FLWp6GR6Rvo
NgVLUi63bQqv9D5qYSsweAp1QTvIxJffWMJDTWtxowOXVW5P8WJ8jp/pAXoWGRbd
pnavy6Ih0XT57huwT7fGGIikXYfw/kB85PPJL3FsT/b6G4ay2+Z7OGkCgYEA0y+d
bxUewYZkpNy7+kIh0x4vrJvNqSL9ZwiP2R159zu7zDwDph/fkhXej0FEtbXybt+O
4s9M3l4nNsY6AS9sIPCB5SxWguhx0z76U5cz1qFFZwIHtL8r1jHrl5iwkVyOAtVV
0BokmJG4Pn07yZo/iCmSTEfwcePvCMvOsPtcvKcCgYEAu5+SbKChfhBaz19MLv6P
ttHdjcIogl/9dAU9BWxj+LO2MAjS1HKJ2ICi97d/3LbQ19TqArvgs9OymZhV+Fb/
Xgzhb1+/94icmFASI8KJP0CfvCwobRrTBlO8BDsdiITO4SNyalI28kLXpCzxiiFG
yDzOZx8FcjEpHZLmctgeCWkCgYAO0rDCM0FNZBl8WOH41tt47g16mBT/Yi1XJgqy
upbs+4xa8XtwFZyjrFVKyNIBzxuNHLPyx4olsYYfGhrIKoP0a+0yIMKRva7/nNQF
Of+xePBeIo5X6XMyPZ7DrTv3d/+fw0maqbsX2mKMQE4KAIGlFQXnxMTjuZP1khiX
44zG0QKBgGwQ8T4DGZK5ukLQmhLi9npCaAW99s/uuKArMzAG9xd/I8YntM/kVY0V
VUi3lKqwXhtReYdrqVTPdjnyGIYIGGNRD7EKqQe15IRfbpy536DSN+LvL65Fdyis
iNITDKNP1H3hedFNFfbTGpueYdRX6QaptK4+NB4+dOm7hn8iqq7U
-----END RSA PRIVATE KEY-----`

const updatedCertPEM = `-----BEGIN CERTIFICATE-----
MIIDEDCCAfgCCQC+HwE8rpMN7jANBgkqhkiG9w0BAQUFADBKMQswCQYDVQQGEwJV
UzEQMA4GA1UECBMHQXJpem9uYTEVMBMGA1UEChMMUmFuY2hlciBMYWJzMRIwEAYD
VQQDEwlsb2NhbGhvc3QwHhcNMTYwNjMwMDExMzMyWhcNMjYwNjI4MDExMzMyWjBK
MQswCQYDVQQGEwJVUzEQMA4GA1UECBMHQXJpem9uYTEVMBMGA1UEChMMUmFuY2hl
ciBMYWJzMRIwEAYDVQQDEwlsb2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQC1PR0EiJjM0wbFQmU/yKSb7AuQdzhdW02ya+RQe+31/B+sOTMr
z9b473KCKf8LiFKFOIQUhR5fPvwyrrIWKCEV9pCp/wM474fX32j0zYaH6ezZjL0r
L6hTeGFScGse3dk7ej2+6nNWexpujos0djFi9Gu11iVHIJyT2Sx66kPPPZVRkJO9
5Pfetm5SLIQtJHUwy5iWv5Br+AbdXlUAjTYUqS4mhKIIbblAPbOKrYRxGXX/6oDV
J5OGLle8Uvlb8poxqmy67FPyMObNHhjggKwboXhmNuuT2OGf/VeZANMYubs4JP2V
ZLs3U/1tFMAOaQM+PbT9JuwMSmGYFX0Qiuh/AgMBAAEwDQYJKoZIhvcNAQEFBQAD
ggEBACpkRCQpCn/zmTOwboBckkOFeqMVo9cvSu0Sez6EPED4WUv/6q5tlJeHekQm
6YVcsXeOMkpfZ7qtGmBDwR+ly7D43dCiPKplm0uApO1CkogG5ePv0agvKHEybd36
xu9pt0fnxDdrP2NrP6trHq1D+CzPZooLRfmYqbt1xmIb00GpnyiJIUNuMu7GUM3q
NxWGK3eq+1cyt6xr8nLOC5zaGeSyZikw4+9vqLudNSyYdnw9mdHtrYT0GlcEP1Vc
NK+yrhDCvEWH6+4+pp8Ve2P2Le5tvbA1m24AxyuC9wHS5bUmiNHweLXNpxLFTjK8
BBUi6y1Vm9jrDi/LiiHcN4sJEoP=
-----END CERTIFICATE-----`

type NamespacedSecretsTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *NamespacedSecretsTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *NamespacedSecretsTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *NamespacedSecretsTestSuite) httpClient() *http.Client {
	httpClient, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	s.Require().NoError(err)
	return httpClient
}

func (s *NamespacedSecretsTestSuite) resourceURL(project *management.Project, resourceType string) string {
	return fmt.Sprintf("https://%s/v3/project/%s/%s",
		s.client.WranglerContext.RESTConfig.Host, project.ID, resourceType)
}

// post sends a POST to the given URL and returns the parsed response body.
func (s *NamespacedSecretsTestSuite) post(httpClient *http.Client, url string, body map[string]any) map[string]any {
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

// put sends a PUT to {baseURL}/{id} and returns the parsed response body.
func (s *NamespacedSecretsTestSuite) put(httpClient *http.Client, baseURL, id string, body map[string]any) map[string]any {
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

// get sends a GET to {baseURL}/{id} and returns the parsed response body.
func (s *NamespacedSecretsTestSuite) get(httpClient *http.Client, baseURL, id string) map[string]any {
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

// del sends a DELETE to {baseURL}/{id}.
func (s *NamespacedSecretsTestSuite) del(httpClient *http.Client, baseURL, id string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%s", baseURL, id), nil)
	s.Require().NoError(err)
	resp, err := httpClient.Do(req)
	s.Require().NoError(err)
	io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d deleting resource", resp.StatusCode)
}

// assertInList asserts that a resource with the given ID appears in the
// collection returned by listing the given URL.
func (s *NamespacedSecretsTestSuite) assertInList(httpClient *http.Client, listURL, id string) {
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

// setupProjectNS is a helper that creates a project and namespace for a sub-session.
func (s *NamespacedSecretsTestSuite) setupProjectNS() (*rancher.Client, *management.Project, string) {
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

// TestNamespacedSecrets asserts that an Opaque namespaced secret can be created,
// updated, listed, and deleted via the Norman project API.
func (s *NamespacedSecretsTestSuite) TestNamespacedSecrets() {
	_, project, nsName := s.setupProjectNS()
	httpClient := s.httpClient()
	url := s.resourceURL(project, "namespacedSecrets")

	name := namegen.AppendRandomString("secret-")
	created := s.post(httpClient, url, map[string]any{
		"name":        name,
		"namespaceId": nsName,
		"stringData":  map[string]any{"foo": "bar"},
	})

	s.Equal("namespacedSecret", created["baseType"])
	s.Equal("namespacedSecret", created["type"])
	s.Equal("Opaque", created["kind"])
	s.Equal(name, created["name"])
	data := created["data"].(map[string]any)
	s.Equal("YmFy", data["foo"])

	id := created["id"].(string)

	// Update: add a new key to data.
	updated := s.put(httpClient, url, id, map[string]any{
		"data": map[string]any{"foo": "YmFy", "baz": "YmFy"},
	})
	s.NotNil(updated)

	// Reload via GET.
	reloaded := s.get(httpClient, url, id)
	s.Equal("namespacedSecret", reloaded["baseType"])
	s.Equal("namespacedSecret", reloaded["type"])
	s.Equal("Opaque", reloaded["kind"])
	s.Equal(name, reloaded["name"])
	reloadedData := reloaded["data"].(map[string]any)
	s.Equal("YmFy", reloadedData["foo"])
	s.Equal("YmFy", reloadedData["baz"])
	s.Equal(nsName, reloaded["namespaceId"])
	s.NotContains(reloadedData, "namespace")
	s.Equal(project.ID, reloaded["projectId"])

	s.assertInList(httpClient, url, id)
	s.del(httpClient, url, id)
}

// TestNamespacedCertificates asserts that a namespaced TLS certificate secret
// can be created, updated, listed, and fetched by ID via the Norman project API.
func (s *NamespacedSecretsTestSuite) TestNamespacedCertificates() {
	_, project, nsName := s.setupProjectNS()
	httpClient := s.httpClient()
	url := s.resourceURL(project, "namespacedCertificates")

	name := namegen.AppendRandomString("cert-")
	created := s.post(httpClient, url, map[string]any{
		"name":        name,
		"namespaceId": nsName,
		"certs":       certPEM,
		"key":         keyPEM,
	})

	s.Equal("namespacedSecret", created["baseType"])
	s.Equal("namespacedCertificate", created["type"])
	s.Equal(name, created["name"])
	s.Equal(certPEM, created["certs"])
	s.Equal(nsName, created["namespaceId"])
	s.Equal(project.ID, created["projectId"])
	s.NotContains(created, "namespace")

	id := created["id"].(string)

	// Update the certificate.
	updated := s.put(httpClient, url, id, map[string]any{
		"certs": updatedCertPEM,
	})
	s.Equal(nsName, updated["namespaceId"])
	s.Equal(project.ID, updated["projectId"])

	// Reload via GET.
	reloaded := s.get(httpClient, url, id)
	s.Equal("namespacedSecret", reloaded["baseType"])
	s.Equal("namespacedCertificate", reloaded["type"])
	s.Equal(name, reloaded["name"])
	s.Equal(updatedCertPEM, reloaded["certs"])
	s.Equal(nsName, reloaded["namespaceId"])
	s.Equal(project.ID, reloaded["projectId"])

	s.assertInList(httpClient, url, id)

	// Get by ID explicitly.
	byID := s.get(httpClient, url, id)
	s.NotNil(byID)

	s.del(httpClient, url, id)
}

// TestNamespacedDockerCredential asserts that a namespaced docker credential
// can be created, updated, listed, and fetched by ID via the Norman project API.
func (s *NamespacedSecretsTestSuite) TestNamespacedDockerCredential() {
	_, project, nsName := s.setupProjectNS()
	httpClient := s.httpClient()
	url := s.resourceURL(project, "namespacedDockerCredentials")

	name := namegen.AppendRandomString("dockercred-")
	created := s.post(httpClient, url, map[string]any{
		"name":        name,
		"namespaceId": nsName,
		"registries": map[string]any{
			"index.docker.io": map[string]any{
				"username": "foo",
				"password": "bar",
			},
		},
	})

	s.Equal("namespacedSecret", created["baseType"])
	s.Equal("namespacedDockerCredential", created["type"])
	s.Equal(name, created["name"])
	regs := created["registries"].(map[string]any)
	dockerIO := regs["index.docker.io"].(map[string]any)
	s.Equal("foo", dockerIO["username"])
	s.Contains(dockerIO, "password")
	s.Equal(nsName, created["namespaceId"])
	s.Equal(project.ID, created["projectId"])

	id := created["id"].(string)

	// Update: add a second registry.
	updated := s.put(httpClient, url, id, map[string]any{
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
	_ = updated
	reloaded := s.get(httpClient, url, id)
	s.Equal("namespacedSecret", reloaded["baseType"])
	s.Equal("namespacedDockerCredential", reloaded["type"])
	s.Equal(name, reloaded["name"])
	reloadedRegs := reloaded["registries"].(map[string]any)
	reloadedDockerIO := reloadedRegs["index.docker.io"].(map[string]any)
	s.Equal("foo", reloadedDockerIO["username"])
	reloadedTwo := reloadedRegs["two"].(map[string]any)
	s.Equal("blah", reloadedTwo["username"])
	// Password is write-only; should not be present after reload.
	s.NotContains(reloadedDockerIO, "password")
	s.Equal(nsName, reloaded["namespaceId"])
	s.NotContains(reloaded, "namespace")
	s.Equal(project.ID, reloaded["projectId"])

	s.assertInList(httpClient, url, id)

	byID := s.get(httpClient, url, id)
	s.NotNil(byID)

	s.del(httpClient, url, id)
}

// TestNamespacedBasicAuth asserts that a namespaced basic-auth secret can be
// created, updated, listed, and fetched by ID via the Norman project API.
func (s *NamespacedSecretsTestSuite) TestNamespacedBasicAuth() {
	_, project, nsName := s.setupProjectNS()
	httpClient := s.httpClient()
	url := s.resourceURL(project, "namespacedBasicAuths")

	name := namegen.AppendRandomString("basicauth-")
	created := s.post(httpClient, url, map[string]any{
		"name":        name,
		"namespaceId": nsName,
		"username":    "foo",
		"password":    "bar",
	})

	s.Equal("namespacedSecret", created["baseType"])
	s.Equal("namespacedBasicAuth", created["type"])
	s.Equal(name, created["name"])
	s.Equal("foo", created["username"])
	s.Contains(created, "password")
	s.Equal(nsName, created["namespaceId"])
	s.NotContains(created, "namespace")
	s.Equal(project.ID, created["projectId"])

	id := created["id"].(string)

	// Update username.
	s.put(httpClient, url, id, map[string]any{
		"username": "foo2",
	})

	// Reload via GET.
	reloaded := s.get(httpClient, url, id)
	s.Equal("namespacedSecret", reloaded["baseType"])
	s.Equal("namespacedBasicAuth", reloaded["type"])
	s.Equal(name, reloaded["name"])
	s.Equal("foo2", reloaded["username"])
	// Password is write-only; should not be present after reload.
	s.NotContains(reloaded, "password")
	s.Equal(nsName, reloaded["namespaceId"])
	s.NotContains(reloaded, "namespace")
	s.Equal(project.ID, reloaded["projectId"])

	s.assertInList(httpClient, url, id)

	byID := s.get(httpClient, url, id)
	s.NotNil(byID)

	s.del(httpClient, url, id)
}

// TestNamespacedSSHAuth asserts that a namespaced SSH auth secret can be
// created, updated, listed, and fetched by ID via the Norman project API.
func (s *NamespacedSecretsTestSuite) TestNamespacedSSHAuth() {
	_, project, nsName := s.setupProjectNS()
	httpClient := s.httpClient()
	url := s.resourceURL(project, "namespacedSshAuths")

	name := namegen.AppendRandomString("sshauth-")
	created := s.post(httpClient, url, map[string]any{
		"name":        name,
		"namespaceId": nsName,
		"privateKey":  "foo",
	})

	s.Equal("namespacedSecret", created["baseType"])
	s.Equal("namespacedSshAuth", created["type"])
	s.Equal(name, created["name"])
	s.Contains(created, "privateKey")
	s.Equal(nsName, created["namespaceId"])
	s.NotContains(created, "namespace")
	s.Equal(project.ID, created["projectId"])

	id := created["id"].(string)

	// Update the private key.
	s.put(httpClient, url, id, map[string]any{
		"privateKey": "foo2",
	})

	// Reload via GET.
	reloaded := s.get(httpClient, url, id)
	s.Equal("namespacedSecret", reloaded["baseType"])
	s.Equal("namespacedSshAuth", reloaded["type"])
	s.Equal(name, reloaded["name"])
	// privateKey is write-only; should not be present after reload.
	s.NotContains(reloaded, "privateKey")
	s.Equal(nsName, reloaded["namespaceId"])
	s.NotContains(reloaded, "namespace")
	s.Equal(project.ID, reloaded["projectId"])

	s.assertInList(httpClient, url, id)

	byID := s.get(httpClient, url, id)
	s.NotNil(byID)

	s.del(httpClient, url, id)
}

func TestNamespacedSecrets(t *testing.T) {
	suite.Run(t, new(NamespacedSecretsTestSuite))
}

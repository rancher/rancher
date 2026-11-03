package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/rest"
)

type PVTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *PVTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *PVTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *PVTestSuite) httpClient() *http.Client {
	httpClient, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	s.Require().NoError(err)
	return httpClient
}

func (s *PVTestSuite) pvURL() string {
	return fmt.Sprintf("https://%s/v3/cluster/local/persistentVolumes",
		s.client.WranglerContext.RESTConfig.Host)
}

func (s *PVTestSuite) postPV(httpClient *http.Client, body map[string]any) map[string]any {
	b, err := json.Marshal(body)
	s.Require().NoError(err)
	resp, err := httpClient.Post(s.pvURL(), "application/json", bytes.NewReader(b))
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d creating PV: %s", resp.StatusCode, string(respBody))
	var result map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &result))
	return result
}

func (s *PVTestSuite) putPV(httpClient *http.Client, id string, body map[string]any) map[string]any {
	b, err := json.Marshal(body)
	s.Require().NoError(err)
	url := fmt.Sprintf("%s/%s", s.pvURL(), id)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(b))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d updating PV: %s", resp.StatusCode, string(respBody))
	var result map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &result))
	return result
}

// TestPersistentVolumeUpdate asserts that read-only fields within a
// persistentVolumeSource cannot be mutated after creation, and that the
// persistentVolumeSource type itself cannot be changed once set.
func (s *PVTestSuite) TestPersistentVolumeUpdate() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	httpClient := s.httpClient()

	name := namegen.AppendRandomString("pv-")
	pv := s.postPV(httpClient, map[string]any{
		"clusterId":   "local",
		"name":        name,
		"accessModes": []string{"ReadWriteOnce"},
		"capacity":    map[string]any{"storage": "10Gi"},
		"cinder": map[string]any{
			"readOnly": "false",
			"secretRef": map[string]any{
				"name":      "fss",
				"namespace": "fsf",
			},
			"volumeID": "fss",
			"fsType":   "fss",
		},
	})
	s.Require().NotNil(pv)

	id := pv["id"].(string)
	s.T().Cleanup(func() {
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%s", s.pvURL(), id), nil)
		s.Require().NoError(err)
		resp, err := httpClient.Do(req)
		if err == nil {
			io.ReadAll(resp.Body)
			resp.Body.Close()
		}
	})

	// Fields within the persistentVolumeSource should not be updated.
	updated := s.putPV(httpClient, id, map[string]any{
		"cinder": map[string]any{"readOnly": "true"},
	})
	cinder := updated["cinder"].(map[string]any)
	// readOnly must remain false — it is not updatable.
	s.False(cinder["readOnly"] == true || cinder["readOnly"] == "true",
		"cinder.readOnly should not have been updated to true")

	// The persistentVolumeSource type cannot be changed from cinder to azureFile.
	updated = s.putPV(httpClient, id, map[string]any{
		"azureFile": map[string]any{
			"readOnly":  "true",
			"shareName": "abc",
		},
		"cinder": map[string]any{},
	})
	_, hasAzureFile := updated["azureFile"]
	s.False(hasAzureFile, "azureFile should not be present after attempting to change PV source type")
}

func TestPV(t *testing.T) {
	suite.Run(t, new(PVTestSuite))
}

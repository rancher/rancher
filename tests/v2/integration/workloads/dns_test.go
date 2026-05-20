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

type DNSTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *DNSTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *DNSTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *DNSTestSuite) httpClient() *http.Client {
	httpClient, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	s.Require().NoError(err)
	return httpClient
}

func (s *DNSTestSuite) dnsRecordURL(project *management.Project) string {
	return fmt.Sprintf("https://%s/v3/project/%s/dnsRecords",
		s.client.WranglerContext.RESTConfig.Host, project.ID)
}

func (s *DNSTestSuite) schemaURL() string {
	return fmt.Sprintf("https://%s/v3/schemas/dnsRecord",
		s.client.WranglerContext.RESTConfig.Host)
}

// TestDNSFields verifies that the dnsRecord Norman schema exposes full CRUD
// access and that the expected resource fields are present with the correct
// create/update permissions.
func (s *DNSTestSuite) TestDNSFields() {
	httpClient := s.httpClient()

	resp, err := httpClient.Get(s.schemaURL())
	s.Require().NoError(err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var schema struct {
		CollectionMethods []string `json:"collectionMethods"`
		ResourceMethods   []string `json:"resourceMethods"`
		ResourceFields    map[string]struct {
			Create bool `json:"create"`
			Update bool `json:"update"`
		} `json:"resourceFields"`
	}
	s.Require().NoError(json.Unmarshal(body, &schema))

	// Verify CRUD access: collection supports GET+POST, resource supports GET+PUT+DELETE.
	s.Contains(schema.CollectionMethods, "GET")
	s.Contains(schema.CollectionMethods, "POST")
	s.Contains(schema.ResourceMethods, "GET")
	s.Contains(schema.ResourceMethods, "PUT")
	s.Contains(schema.ResourceMethods, "DELETE")

	// fieldAccess encodes the expected create/update permissions for each field.
	// 'c' = create, 'r' = read-only, 'u' = update, combinations are additive.
	type fieldSpec struct {
		create bool
		update bool
	}
	expected := map[string]fieldSpec{
		"allocateLoadBalancerNodePorts": {create: true, update: true},
		"clusterIp":                     {create: false, update: false},
		"clusterIPs":                    {create: true, update: true},
		"hostname":                      {create: true, update: true},
		"ipAddresses":                   {create: true, update: true},
		"ipFamilies":                    {create: true, update: true},
		"ipFamilyPolicy":                {create: true, update: true},
		"namespaceId":                   {create: true, update: false},
		"ports":                         {create: false, update: false},
		"projectId":                     {create: true, update: false},
		"publicEndpoints":               {create: false, update: false},
		"selector":                      {create: true, update: true},
		"targetDnsRecordIds":            {create: true, update: true},
		"targetWorkloadIds":             {create: true, update: true},
		"trafficDistribution":           {create: true, update: true},
		"workloadId":                    {create: false, update: false},
	}

	for fieldName, want := range expected {
		field, ok := schema.ResourceFields[fieldName]
		s.Truef(ok, "expected resourceField %q to be present in dnsRecord schema", fieldName)
		if !ok {
			continue
		}
		s.Equalf(want.create, field.Create, "field %q: unexpected create permission", fieldName)
		s.Equalf(want.update, field.Update, "field %q: unexpected update permission", fieldName)
	}
}

// TestDNSHostname asserts that a dnsRecord can be created with a hostname,
// retrieved by ID, updated, and listed via the Norman project API.
func (s *DNSTestSuite) TestDNSHostname() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	project, err := client.Management.Project.Create(&management.Project{
		ClusterID: "local",
		Name:      namegen.AppendRandomString("project-"),
	})
	s.Require().NoError(err)

	ns, err := namespaces.CreateNamespace(client, namegen.AppendRandomString("ns-"), "", map[string]string{}, map[string]string{}, project)
	s.Require().NoError(err)

	httpClient := s.httpClient()

	// Create dnsRecord with hostname.
	name := namegen.AppendRandomString("dns-")
	createBody := map[string]any{
		"name":        name,
		"hostname":    "target",
		"namespaceId": ns.Name,
	}
	createBytes, err := json.Marshal(createBody)
	s.Require().NoError(err)

	resp, err := httpClient.Post(s.dnsRecordURL(project), "application/json", bytes.NewReader(createBytes))
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d creating dnsRecord: %s", resp.StatusCode, string(respBody))

	var created map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &created))

	s.Equal("dnsRecord", created["baseType"])
	s.Equal("dnsRecord", created["type"])
	s.Equal(name, created["name"])
	s.Equal("target", created["hostname"])
	s.Nil(created["clusterIp"])
	s.Equal(ns.Name, created["namespaceId"])
	s.NotContains(created, "namespace")
	s.Equal(project.ID, created["projectId"])

	recordID := created["id"].(string)

	// Update the hostname.
	updateBody := map[string]any{"hostname": "target2"}
	updateBytes, err := json.Marshal(updateBody)
	s.Require().NoError(err)
	putURL := fmt.Sprintf("%s/%s", s.dnsRecordURL(project), recordID)
	req, err := http.NewRequest(http.MethodPut, putURL, bytes.NewReader(updateBytes))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = httpClient.Do(req)
	s.Require().NoError(err)
	respBody, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d updating dnsRecord: %s", resp.StatusCode, string(respBody))

	// Reload via GET by ID.
	resp, err = httpClient.Get(putURL)
	s.Require().NoError(err)
	respBody, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var updated map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &updated))

	s.Equal("dnsRecord", updated["baseType"])
	s.Equal("dnsRecord", updated["type"])
	s.Equal(name, updated["name"])
	s.Equal("target2", updated["hostname"])
	s.Nil(updated["clusterIp"])
	s.Equal(ns.Name, updated["namespaceId"])
	s.NotContains(updated, "namespace")
	s.Equal(project.ID, updated["projectId"])

	// Verify the record appears in the list.
	resp, err = httpClient.Get(s.dnsRecordURL(project))
	s.Require().NoError(err)
	listBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var list struct {
		Data []map[string]any `json:"data"`
	}
	s.Require().NoError(json.Unmarshal(listBody, &list))
	found := false
	for _, item := range list.Data {
		if item["id"] == recordID {
			found = true
			break
		}
	}
	s.True(found, "dnsRecord %s not found in list", recordID)

	// Get by ID explicitly.
	resp, err = httpClient.Get(putURL)
	s.Require().NoError(err)
	io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)

	// Delete.
	delReq, err := http.NewRequest(http.MethodDelete, putURL, nil)
	s.Require().NoError(err)
	resp, err = httpClient.Do(delReq)
	s.Require().NoError(err)
	io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d deleting dnsRecord", resp.StatusCode)
}

// TestDNSIPs asserts that a dnsRecord can be created with IP addresses, that
// IPs can be updated, and that creating a dnsRecord with a loopback IP in the
// default namespace is rejected with 422.
func (s *DNSTestSuite) TestDNSIPs() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	project, err := client.Management.Project.Create(&management.Project{
		ClusterID: "local",
		Name:      namegen.AppendRandomString("project-"),
	})
	s.Require().NoError(err)

	ns, err := namespaces.CreateNamespace(client, namegen.AppendRandomString("ns-"), "", map[string]string{}, map[string]string{}, project)
	s.Require().NoError(err)

	httpClient := s.httpClient()

	// Create dnsRecord with two IP addresses.
	name := namegen.AppendRandomString("dns-")
	createBody := map[string]any{
		"name":        name,
		"ipAddresses": []string{"1.1.1.1", "2.2.2.2"},
		"namespaceId": ns.Name,
	}
	createBytes, err := json.Marshal(createBody)
	s.Require().NoError(err)

	resp, err := httpClient.Post(s.dnsRecordURL(project), "application/json", bytes.NewReader(createBytes))
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d creating dnsRecord: %s", resp.StatusCode, string(respBody))

	var created map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &created))

	s.Equal("dnsRecord", created["baseType"])
	s.Equal("dnsRecord", created["type"])
	s.Equal(name, created["name"])
	s.NotContains(created, "hostname")
	ips := toStringSlice(created["ipAddresses"])
	s.Equal([]string{"1.1.1.1", "2.2.2.2"}, ips)
	s.Nil(created["clusterIp"])
	s.Equal(ns.Name, created["namespaceId"])
	s.NotContains(created, "namespace")
	s.Equal(project.ID, created["projectId"])

	recordID := created["id"].(string)
	putURL := fmt.Sprintf("%s/%s", s.dnsRecordURL(project), recordID)

	// Update IP addresses.
	updateBody := map[string]any{"ipAddresses": []string{"1.1.1.2", "2.2.2.1"}}
	updateBytes, err := json.Marshal(updateBody)
	s.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPut, putURL, bytes.NewReader(updateBytes))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err = httpClient.Do(req)
	s.Require().NoError(err)
	respBody, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d updating dnsRecord: %s", resp.StatusCode, string(respBody))

	// Reload via GET.
	resp, err = httpClient.Get(putURL)
	s.Require().NoError(err)
	respBody, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var reloaded map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &reloaded))

	s.Equal("dnsRecord", reloaded["baseType"])
	s.Equal("dnsRecord", reloaded["type"])
	s.Equal(name, reloaded["name"])
	s.NotContains(reloaded, "hostname")
	updatedIPs := toStringSlice(reloaded["ipAddresses"])
	s.Equal([]string{"1.1.1.2", "2.2.2.1"}, updatedIPs)
	s.Nil(reloaded["clusterIp"])
	s.Equal(ns.Name, reloaded["namespaceId"])
	s.NotContains(reloaded, "namespace")
	s.Equal(project.ID, reloaded["projectId"])

	// Creating a dnsRecord with a loopback IP in the default namespace should be rejected.
	loopbackBody := map[string]any{
		"name":        namegen.AppendRandomString("dns-"),
		"ipAddresses": []string{"127.0.0.2"},
		"namespaceId": "default",
	}
	loopbackBytes, err := json.Marshal(loopbackBody)
	s.Require().NoError(err)
	resp, err = httpClient.Post(s.dnsRecordURL(project), "application/json", bytes.NewReader(loopbackBytes))
	s.Require().NoError(err)
	io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Equal(http.StatusUnprocessableEntity, resp.StatusCode)

	// Verify the original record still appears in the list.
	resp, err = httpClient.Get(s.dnsRecordURL(project))
	s.Require().NoError(err)
	listBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var list struct {
		Data []map[string]any `json:"data"`
	}
	s.Require().NoError(json.Unmarshal(listBody, &list))
	found := false
	for _, item := range list.Data {
		if item["id"] == recordID {
			found = true
			break
		}
	}
	s.True(found, "dnsRecord %s not found in list", recordID)

	// Get by ID.
	resp, err = httpClient.Get(putURL)
	s.Require().NoError(err)
	io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)

	// Delete.
	delReq, err := http.NewRequest(http.MethodDelete, putURL, nil)
	s.Require().NoError(err)
	resp, err = httpClient.Do(delReq)
	s.Require().NoError(err)
	io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d deleting dnsRecord", resp.StatusCode)
}

// toStringSlice converts a []any from JSON unmarshalling to []string.
func toStringSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, len(raw))
	for i, item := range raw {
		out[i], _ = item.(string)
	}
	return out
}

func TestDNS(t *testing.T) {
	suite.Run(t, new(DNSTestSuite))
}

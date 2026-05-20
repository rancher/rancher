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

type IngressTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *IngressTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *IngressTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *IngressTestSuite) httpClient() *http.Client {
	httpClient, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	s.Require().NoError(err)
	return httpClient
}

func (s *IngressTestSuite) ingressURL(project *management.Project) string {
	return fmt.Sprintf("https://%s/v3/project/%s/ingresses",
		s.client.WranglerContext.RESTConfig.Host, project.ID)
}

func (s *IngressTestSuite) schemaURL(typeName string) string {
	return fmt.Sprintf("https://%s/v3/schemas/%s",
		s.client.WranglerContext.RESTConfig.Host, typeName)
}

// TestIngressFields verifies that the Norman schema for ingress, ingressBackend,
// ingressRule, and httpIngressPath exposes the expected fields with the correct
// create/update permissions.
func (s *IngressTestSuite) TestIngressFields() {
	httpClient := s.httpClient()

	type fieldSpec struct {
		create bool
		update bool
	}

	checkSchema := func(typeName string, expectedCRUD string, fields map[string]fieldSpec) {
		resp, err := httpClient.Get(s.schemaURL(typeName))
		s.Require().NoError(err)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		s.Require().NoError(err)
		s.Require().Equalf(http.StatusOK, resp.StatusCode, "schema %s not found", typeName)

		var schema struct {
			CollectionMethods []string `json:"collectionMethods"`
			ResourceMethods   []string `json:"resourceMethods"`
			ResourceFields    map[string]struct {
				Create bool `json:"create"`
				Update bool `json:"update"`
			} `json:"resourceFields"`
		}
		s.Require().NoError(json.Unmarshal(body, &schema))

		for _, ch := range expectedCRUD {
			switch ch {
			case 'c':
				s.Containsf(schema.CollectionMethods, "POST", "schema %s: expected POST in collectionMethods", typeName)
			case 'r':
				s.Truef(
					contains(schema.CollectionMethods, "GET") || contains(schema.ResourceMethods, "GET"),
					"schema %s: expected GET", typeName,
				)
			case 'u':
				s.Containsf(schema.ResourceMethods, "PUT", "schema %s: expected PUT in resourceMethods", typeName)
			case 'd':
				s.Containsf(schema.ResourceMethods, "DELETE", "schema %s: expected DELETE in resourceMethods", typeName)
			}
		}

		for fieldName, want := range fields {
			field, ok := schema.ResourceFields[fieldName]
			s.Truef(ok, "schema %s: expected field %q to be present", typeName, fieldName)
			if !ok {
				continue
			}
			s.Equalf(want.create, field.Create, "schema %s field %q: unexpected create permission", typeName, fieldName)
			s.Equalf(want.update, field.Update, "schema %s field %q: unexpected update permission", typeName, fieldName)
		}
	}

	cru := fieldSpec{create: true, update: true}
	cr := fieldSpec{create: true, update: false}
	r := fieldSpec{create: false, update: false}

	checkSchema("ingress", "crud", map[string]fieldSpec{
		"namespaceId":      cr,
		"projectId":        cr,
		"rules":            cru,
		"tls":              cru,
		"ingressClassName": cru,
		"backend":          cru,
		"defaultBackend":   cru,
		"publicEndpoints":  r,
		"status":           r,
	})

	checkSchema("ingressBackend", "", map[string]fieldSpec{
		"serviceId":   cru,
		"service":     cru,
		"targetPort":  cru,
		"resource":    cru,
		"workloadIds": cru,
	})

	checkSchema("ingressRule", "", map[string]fieldSpec{
		"host":  cru,
		"paths": cru,
	})

	checkSchema("httpIngressPath", "", map[string]fieldSpec{
		"resource":    cru,
		"pathType":    cru,
		"path":        cru,
		"serviceId":   cru,
		"service":     cru,
		"targetPort":  cru,
		"workloadIds": cru,
	})
}

// TestIngress asserts that an ingress can be created with a single rule via the
// Norman project API and that the rule's host, path, targetPort, workloadIds,
// and serviceId are stored correctly.
func (s *IngressTestSuite) TestIngress() {
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

	// Create a workload.
	workloadID := s.createWorkload(httpClient, project, ns.Name)

	// Create ingress with one rule referencing the workload.
	ingressName := namegen.AppendRandomString("ing-") + "." + namegen.AppendRandomString("suf-")
	body := map[string]any{
		"name":        ingressName,
		"namespaceId": ns.Name,
		"rules": []any{
			map[string]any{
				"host": "foo.com",
				"paths": []any{
					map[string]any{
						"path":        "/",
						"targetPort":  80,
						"workloadIds": []string{workloadID},
					},
				},
			},
		},
	}
	bodyBytes, err := json.Marshal(body)
	s.Require().NoError(err)

	resp, err := httpClient.Post(s.ingressURL(project), "application/json", bytes.NewReader(bodyBytes))
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d creating ingress: %s", resp.StatusCode, string(respBody))

	var ingress map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &ingress))

	rules := ingress["rules"].([]any)
	s.Require().Len(rules, 1)

	rule := rules[0].(map[string]any)
	s.Equal("foo.com", rule["host"])

	paths := rule["paths"].([]any)
	s.Require().NotEmpty(paths)
	path := paths[0].(map[string]any)

	s.Equal("/", path["path"])
	s.EqualValues(80, path["targetPort"])
	s.Equal([]string{workloadID}, toStringSlice(path["workloadIds"]))
	s.Nil(path["serviceId"])
}

// TestIngressRulesSameHostPortPath asserts that when two ingress rules share the
// same host and path, the Norman API merges them into a single rule whose path
// entry contains workload IDs from both original rules.
func (s *IngressTestSuite) TestIngressRulesSameHostPortPath() {
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

	workload1ID := s.createWorkload(httpClient, project, ns.Name)
	workload2ID := s.createWorkload(httpClient, project, ns.Name)

	ingressName := namegen.AppendRandomString("ing-")
	body := map[string]any{
		"name":        ingressName,
		"namespaceId": ns.Name,
		"rules": []any{
			map[string]any{
				"host": "foo.com",
				"paths": []any{
					map[string]any{
						"path":        "/",
						"targetPort":  80,
						"workloadIds": []string{workload1ID},
					},
				},
			},
			map[string]any{
				"host": "foo.com",
				"paths": []any{
					map[string]any{
						"path":        "/",
						"targetPort":  80,
						"workloadIds": []string{workload2ID},
					},
				},
			},
		},
	}
	bodyBytes, err := json.Marshal(body)
	s.Require().NoError(err)

	resp, err := httpClient.Post(s.ingressURL(project), "application/json", bytes.NewReader(bodyBytes))
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d creating ingress: %s", resp.StatusCode, string(respBody))

	var ingress map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &ingress))

	// The two rules with the same host+path should be merged into one.
	rules := ingress["rules"].([]any)
	s.Require().Len(rules, 1)

	rule := rules[0].(map[string]any)
	s.Equal("foo.com", rule["host"])

	paths := rule["paths"].([]any)
	s.Require().NotEmpty(paths)
	path := paths[0].(map[string]any)

	s.Equal("/", path["path"])
	s.EqualValues(80, path["targetPort"])

	workloadIDs := toStringSlice(path["workloadIds"])
	s.Len(workloadIDs, 2)
	s.ElementsMatch([]string{workload1ID, workload2ID}, workloadIDs)
	s.Nil(path["serviceId"])
}

// createWorkload is a helper that creates a minimal nginx workload via the Norman
// project API and returns its ID.
func (s *IngressTestSuite) createWorkload(httpClient *http.Client, project *management.Project, nsName string) string {
	workloadURL := fmt.Sprintf("https://%s/v3/project/%s/workloads",
		s.client.WranglerContext.RESTConfig.Host, project.ID)

	body := map[string]any{
		"name":        namegen.AppendRandomString("wl-"),
		"namespaceId": nsName,
		"scale":       1,
		"containers":  []any{map[string]any{"name": "one", "image": "nginx"}},
	}
	bodyBytes, err := json.Marshal(body)
	s.Require().NoError(err)

	resp, err := httpClient.Post(workloadURL, "application/json", bytes.NewReader(bodyBytes))
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d creating workload: %s", resp.StatusCode, string(respBody))

	var wl map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &wl))
	return wl["id"].(string)
}

// contains reports whether slice contains s.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func TestIngress(t *testing.T) {
	suite.Run(t, new(IngressTestSuite))
}

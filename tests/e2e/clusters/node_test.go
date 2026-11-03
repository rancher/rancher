package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/rest"
)

type NodeTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *NodeTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *NodeTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *NodeTestSuite) httpClient() *http.Client {
	httpClient, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	s.Require().NoError(err)
	return httpClient
}

func (s *NodeTestSuite) schemaURL(typeName string) string {
	return fmt.Sprintf("https://%s/v3/schemas/%s",
		s.client.WranglerContext.RESTConfig.Host, typeName)
}

// fetchSchema retrieves and unmarshals a Norman schema by type name.
func (s *NodeTestSuite) fetchSchema(typeName string) nodeSchema {
	resp, err := s.httpClient().Get(s.schemaURL(typeName))
	s.Require().NoError(err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Equalf(http.StatusOK, resp.StatusCode, "schema %q not found", typeName)

	var sc nodeSchema
	s.Require().NoError(json.Unmarshal(body, &sc))
	return sc
}

type nodeSchema struct {
	CollectionMethods []string `json:"collectionMethods"`
	ResourceMethods   []string `json:"resourceMethods"`
	ResourceFields    map[string]struct {
		Create bool `json:"create"`
		Update bool `json:"update"`
	} `json:"resourceFields"`
}

// TestNodeFields verifies that the Norman management schema for the node type
// exposes full CRUD access and that every explicitly named field has the
// expected create/update permissions. Fields whose names end with "Config"
// are expected to be create-only (cr), except customConfig which is (cru).
func (s *NodeTestSuite) TestNodeFields() {
	sc := s.fetchSchema("node")

	// Verify CRUD methods.
	s.Contains(sc.CollectionMethods, "GET")
	s.Contains(sc.CollectionMethods, "POST")
	s.Contains(sc.ResourceMethods, "GET")
	s.Contains(sc.ResourceMethods, "PUT")
	s.Contains(sc.ResourceMethods, "DELETE")

	type perm struct{ create, update bool }
	cr := perm{true, false}
	cru := perm{true, true}
	ru := perm{false, true}
	r := perm{false, false}

	explicit := map[string]perm{
		"allocatable":        r,
		"annotations":        cru,
		"appliedNodeVersion": r,
		"capacity":           r,
		"clusterId":          cr,
		"conditions":         r,
		"controlPlane":       cr,
		"declaredFeatures":   r,
		"dockerInfo":         r,
		"etcd":               cr,
		"externalIpAddress":  r,
		"features":           r,
		"hostname":           r,
		"imported":           cru,
		"info":               r,
		"ipAddress":          r,
		"labels":             cru,
		"limits":             r,
		"name":               cru,
		"namespaceId":        cr,
		"nodeName":           r,
		"nodeTaints":         r,
		"podCidr":            r,
		"podCidrs":           r,
		"providerId":         r,
		"publicEndpoints":    r,
		"requested":          r,
		"requestedHostname":  cr,
		"runtimeHandlers":    r,
		"scaledownTime":      cru,
		"taints":             ru,
		"unschedulable":      r,
		"volumesAttached":    r,
		"volumesInUse":       r,
		"worker":             cr,
	}

	// Check explicit fields.
	for fieldName, want := range explicit {
		field, ok := sc.ResourceFields[fieldName]
		s.Truef(ok, "expected field %q in node schema", fieldName)
		if !ok {
			continue
		}
		s.Equalf(want.create, field.Create, "field %q: unexpected create permission", fieldName)
		s.Equalf(want.update, field.Update, "field %q: unexpected update permission", fieldName)
	}

	// Fields ending in "Config" should be cr, except customConfig which is cru.
	for fieldName, field := range sc.ResourceFields {
		if !strings.HasSuffix(fieldName, "Config") {
			continue
		}
		if fieldName == "customConfig" {
			s.Truef(field.Create && field.Update,
				"field %q: expected cru (create=true, update=true)", fieldName)
		} else {
			s.Truef(field.Create && !field.Update,
				"field %q: expected cr (create=true, update=false)", fieldName)
		}
	}
}

// TestNodeDriverSchema asserts that the amazonec2config, digitaloceanconfig, and
// azureconfig schemas do not expose sensitive path fields that could allow
// local filesystem access.
func (s *NodeTestSuite) TestNodeDriverSchema() {
	drivers := []string{"amazonec2config", "digitaloceanconfig", "azureconfig"}
	badFields := []string{"sshKeypath", "sshKeyPath", "existingKeyPath"}

	for _, driver := range drivers {
		sc := s.fetchSchema(driver)
		for _, field := range badFields {
			_, present := sc.ResourceFields[field]
			s.Falsef(present, "driver %s should not expose field %q", driver, field)
		}
	}
}

// TestAmazonNodeDriverSchema asserts that the amazonec2config schema includes
// AWS-specific fields required for EBS volume encryption support.
func (s *NodeTestSuite) TestAmazonNodeDriverSchema() {
	sc := s.fetchSchema("amazonec2config")

	requiredFields := []string{"encryptEbsVolume"}
	for _, field := range requiredFields {
		_, present := sc.ResourceFields[field]
		s.Truef(present, "amazonec2config schema is missing required field %q", field)
	}
}

func TestNode(t *testing.T) {
	suite.Run(t, new(NodeTestSuite))
}

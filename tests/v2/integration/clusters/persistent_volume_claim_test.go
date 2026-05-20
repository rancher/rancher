package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/v2/integration/actions/namespaces"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

var storageClassGVR = schema.GroupVersionResource{
	Group:    "storage.k8s.io",
	Version:  "v1",
	Resource: "storageclasses",
}

type PVCTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *PVCTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *PVCTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *PVCTestSuite) httpClient() *http.Client {
	httpClient, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	s.Require().NoError(err)
	return httpClient
}

func (s *PVCTestSuite) storageClassURL() string {
	return fmt.Sprintf("https://%s/v3/cluster/local/storageClasses",
		s.client.WranglerContext.RESTConfig.Host)
}

func (s *PVCTestSuite) pvcURL(projectID string) string {
	return fmt.Sprintf("https://%s/v3/project/%s/persistentVolumeClaims",
		s.client.WranglerContext.RESTConfig.Host, projectID)
}

func (s *PVCTestSuite) post(httpClient *http.Client, url string, body map[string]any) (map[string]any, int) {
	b, err := json.Marshal(body)
	s.Require().NoError(err)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(b))
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	var result map[string]any
	_ = json.Unmarshal(respBody, &result)
	return result, resp.StatusCode
}

// createStorageClassDirect uses the k8s dynamic client to create a StorageClass
// directly, bypassing the Norman API's automatic default-filling of
// storageaccounttype/skuName parameters.
func (s *PVCTestSuite) createStorageClassDirect(client *rancher.Client, name, provisioner string, params map[string]any) {
	dynamicClient, err := client.GetDownStreamClusterClient("local")
	s.Require().NoError(err)

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "storage.k8s.io/v1",
			"kind":       "StorageClass",
			"metadata": map[string]any{
				"name": name,
			},
			"provisioner": provisioner,
			"parameters":  params,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = dynamicClient.Resource(storageClassGVR).Create(ctx, obj, metav1.CreateOptions{})
	s.Require().NoError(err)

	s.T().Cleanup(func() {
		_ = dynamicClient.Resource(storageClassGVR).Delete(
			context.Background(), name, metav1.DeleteOptions{})
	})
}

// createStorageClassNorman creates a StorageClass via the Norman cluster API.
func (s *PVCTestSuite) createStorageClassNorman(httpClient *http.Client, name, provisioner string, params map[string]any) string {
	body := map[string]any{
		"name":        name,
		"provisioner": provisioner,
		"parameters":  params,
	}
	result, status := s.post(httpClient, s.storageClassURL(), body)
	s.Require().Truef(status >= 200 && status < 300,
		"unexpected status %d creating StorageClass", status)
	return result["name"].(string)
}

// TestCannotCreateAzureNoAccountStorageType asserts that a PVC referencing a
// StorageClass with the azure-disk provisioner but no storageaccounttype or
// skuName parameter is rejected by the Norman API with a 422.
//
// The StorageClass is created via the k8s dynamic client to bypass Norman's
// automatic default-filling of those parameters.
func (s *PVCTestSuite) TestCannotCreateAzureNoAccountStorageType() {
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

	scName := namegen.AppendRandomString("sc-")
	// Create the StorageClass directly via k8s API to omit storageaccounttype/skuName.
	s.createStorageClassDirect(client, scName, "kubernetes.io/azure-disk", map[string]any{
		"kind": "shared",
	})

	httpClient := s.httpClient()
	result, status := s.post(httpClient, s.pvcURL(project.ID), map[string]any{
		"name":           namegen.AppendRandomString("pvc-"),
		"storageClassId": scName,
		"namespaceId":    ns.Name,
		"accessModes":    []string{"ReadWriteOnce"},
		"resources": map[string]any{
			"requests": map[string]any{
				"storage": "30Gi",
			},
		},
	})

	s.Equal(http.StatusUnprocessableEntity, status)
	if msg, ok := result["message"].(string); ok {
		s.Contains(msg, "must provide storageaccounttype or skuName")
	}
}

// TestCanCreateAzureAnyAccountStorageType asserts that a PVC referencing a
// StorageClass that has either storageaccounttype or skuName set can be
// successfully created.
func (s *PVCTestSuite) TestCanCreateAzureAnyAccountStorageType() {
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

	// Try with storageaccounttype.
	sc1Name := s.createStorageClassNorman(httpClient,
		namegen.AppendRandomString("sc-"),
		"kubernetes.io/azure-disk",
		map[string]any{"storageaccounttype": "asdf"},
	)

	result, status := s.post(httpClient, s.pvcURL(project.ID), map[string]any{
		"name":           namegen.AppendRandomString("pvc-"),
		"storageClassId": sc1Name,
		"namespaceId":    ns.Name,
		"accessModes":    []string{"ReadWriteOnce"},
		"resources": map[string]any{
			"requests": map[string]any{"storage": "30Gi"},
		},
	})
	s.Truef(status >= 200 && status < 300,
		"unexpected status %d creating PVC with storageaccounttype: %v", status, result)

	// Try with skuName.
	sc2Name := s.createStorageClassNorman(httpClient,
		namegen.AppendRandomString("sc-"),
		"kubernetes.io/azure-disk",
		map[string]any{"skuName": "asdf"},
	)

	result, status = s.post(httpClient, s.pvcURL(project.ID), map[string]any{
		"name":           namegen.AppendRandomString("pvc-"),
		"storageClassId": sc2Name,
		"namespaceId":    ns.Name,
		"accessModes":    []string{"ReadWriteOnce"},
		"resources": map[string]any{
			"requests": map[string]any{"storage": "30Gi"},
		},
	})
	s.Truef(status >= 200 && status < 300,
		"unexpected status %d creating PVC with skuName: %v", status, result)
}

// TestCanCreatePVCNoStorageNoVol asserts that a PVC with no storage class and
// no volume reference can be created and begins in the "pending" state.
func (s *PVCTestSuite) TestCanCreatePVCNoStorageNoVol() {
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

	result, status := s.post(httpClient, s.pvcURL(project.ID), map[string]any{
		"name":        namegen.AppendRandomString("pvc-"),
		"namespaceId": ns.Name,
		"accessModes": []string{"ReadWriteOnce"},
		"resources": map[string]any{
			"requests": map[string]any{"storage": "30Gi"},
		},
	})

	s.Truef(status >= 200 && status < 300,
		"unexpected status %d creating PVC: %v", status, result)
	s.NotNil(result)
	s.Equal("pending", result["state"])
}

func TestPVC(t *testing.T) {
	suite.Run(t, new(PVCTestSuite))
}

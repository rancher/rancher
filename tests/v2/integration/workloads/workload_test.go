package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	extdeployments "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/deployments"
	"github.com/rancher/rancher/tests/v2/integration/actions/namespaces"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

type WorkloadTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *WorkloadTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *WorkloadTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

// httpClient returns an authenticated HTTP client for making calls to
// the Norman project API.
func (s *WorkloadTestSuite) httpClient() *http.Client {
	httpClient, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	s.Require().NoError(err)
	return httpClient
}

// workloadURL returns the base Norman project workload URL for the given project.
func (s *WorkloadTestSuite) workloadURL(project *management.Project) string {
	return fmt.Sprintf("https://%s/v3/project/%s/workloads",
		s.client.WranglerContext.RESTConfig.Host, project.ID)
}

// portEntry mirrors the Rancher Norman port representation for workload containers.
type portEntry struct {
	Kind          string `json:"kind"`
	SourcePort    int    `json:"sourcePort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
}

// TestDeploymentCreationKubectl asserts that a Deployment created directly
// via the Kubernetes API appears in the Norman workload API with the correct
// port mapping (hostPort translated to sourcePort + kind=HostPort).
func (s *WorkloadTestSuite) TestDeploymentCreationKubectl() {
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

	deploymentName := namegen.AppendRandomString("dep-")
	template := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.7.9",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
							HostPort:      8099,
						},
					},
				},
			},
		},
	}

	dep, err := extdeployments.CreateDeployment(client, "local", deploymentName, ns.Name, template, 1)
	s.Require().NoError(err)
	s.Require().NotNil(dep)

	// Poll the Norman workload API until the deployment appears and the port
	// mapping has been translated from hostPort to sourcePort + kind.
	httpClient := s.httpClient()
	listURL := fmt.Sprintf("%s?namespaceId=%s", s.workloadURL(project), ns.Name)

	type containerEntry struct {
		Ports []portEntry `json:"ports"`
	}
	type workloadEntry struct {
		Name       string           `json:"name"`
		Containers []containerEntry `json:"containers"`
	}
	type workloadList struct {
		Data []workloadEntry `json:"data"`
	}

	var gotPort portEntry
	s.Require().Eventually(func() bool {
		resp, err := httpClient.Get(listURL)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false
		}
		var list workloadList
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			return false
		}
		for _, wl := range list.Data {
			if wl.Name != deploymentName {
				continue
			}
			if len(wl.Containers) == 0 || len(wl.Containers[0].Ports) == 0 {
				return false
			}
			gotPort = wl.Containers[0].Ports[0]
			return true
		}
		return false
	}, 30*time.Second, 2*time.Second,
		"workload %s not found in Norman API for project %s", deploymentName, project.ID)

	s.Equal("HostPort", gotPort.Kind)
	s.Equal(8099, gotPort.SourcePort)
}

// TestWorkloadPortKinds asserts that workloads created via the Norman project
// API correctly store the port kind and sourcePort for each port type
// (HostPort, NodePort, LoadBalancer, ClusterIP).
func (s *WorkloadTestSuite) TestWorkloadPortKinds() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	project, err := client.Management.Project.Create(&management.Project{
		ClusterID: "local",
		Name:      namegen.AppendRandomString("project-"),
	})
	s.Require().NoError(err)

	portTests := []portEntry{
		{SourcePort: 776, ContainerPort: 80, Kind: "HostPort", Protocol: "TCP"},
		{SourcePort: 777, ContainerPort: 80, Kind: "NodePort", Protocol: "TCP"},
		{SourcePort: 778, ContainerPort: 80, Kind: "LoadBalancer", Protocol: "TCP"},
		{SourcePort: 779, ContainerPort: 80, Kind: "ClusterIP", Protocol: "TCP"},
	}

	httpClient := s.httpClient()
	workloadBaseURL := s.workloadURL(project)

	for _, port := range portTests {
		ns, err := namespaces.CreateNamespace(client, namegen.AppendRandomString("ns-"), "", map[string]string{}, map[string]string{}, project)
		s.Require().NoError(err)

		body := map[string]any{
			"name":        namegen.AppendRandomString("workload-"),
			"namespaceId": ns.Name,
			"scale":       1,
			"containers": []any{
				map[string]any{
					"name":  "one",
					"image": "nginx",
					"ports": []any{port},
				},
			},
		}
		bodyBytes, err := json.Marshal(body)
		s.Require().NoError(err)

		resp, err := httpClient.Post(workloadBaseURL, "application/json", bytes.NewReader(bodyBytes))
		s.Require().NoError(err)
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		s.Require().NoError(err)
		s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
			"unexpected status %d for port kind %s: %s", resp.StatusCode, port.Kind, string(respBody))

		type containerEntry struct {
			Ports []portEntry `json:"ports"`
		}
		var workload struct {
			Containers []containerEntry `json:"containers"`
		}
		s.Require().NoError(json.Unmarshal(respBody, &workload))
		s.Require().NotEmptyf(workload.Containers, "expected containers in workload response for kind %s", port.Kind)
		ports := workload.Containers[0].Ports
		s.Require().NotEmptyf(ports, "expected ports in container for kind %s", port.Kind)

		s.Equalf(port.Kind, ports[0].Kind, "port kind mismatch")
		s.Equalf(port.ContainerPort, ports[0].ContainerPort, "containerPort mismatch for kind %s", port.Kind)
		s.Equalf(port.SourcePort, ports[0].SourcePort, "sourcePort mismatch for kind %s", port.Kind)
	}
}

// serviceURL returns the base Norman project service URL for the given project.
func (s *WorkloadTestSuite) serviceURL(project *management.Project) string {
	return fmt.Sprintf("https://%s/v3/project/%s/services",
		s.client.WranglerContext.RESTConfig.Host, project.ID)
}

// dockerCredentialURL returns the Norman project dockerCredential URL.
func (s *WorkloadTestSuite) dockerCredentialURL(project *management.Project) string {
	return fmt.Sprintf("https://%s/v3/project/%s/dockerCredentials",
		s.client.WranglerContext.RESTConfig.Host, project.ID)
}

// createWorkload creates a workload via the Norman project API and returns the parsed response body.
func (s *WorkloadTestSuite) createWorkload(httpClient *http.Client, project *management.Project, body map[string]any) map[string]any {
	bodyBytes, err := json.Marshal(body)
	s.Require().NoError(err)
	resp, err := httpClient.Post(s.workloadURL(project), "application/json", bytes.NewReader(bodyBytes))
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d creating workload: %s", resp.StatusCode, string(respBody))
	var result map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &result))
	return result
}

// updateWorkload updates a workload via PUT to the Norman project API.
func (s *WorkloadTestSuite) updateWorkload(httpClient *http.Client, project *management.Project, workloadID string, body map[string]any) map[string]any {
	bodyBytes, err := json.Marshal(body)
	s.Require().NoError(err)
	url := fmt.Sprintf("%s/%s", s.workloadURL(project), workloadID)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(bodyBytes))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d updating workload: %s", resp.StatusCode, string(respBody))
	var result map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &result))
	return result
}

// TestWorkloadImageChangePrivateRegistry asserts that when a workload image is
// updated to reference a different registry, the correct docker credential is
// automatically selected as the imagePullSecret.
func (s *WorkloadTestSuite) TestWorkloadImageChangePrivateRegistry() {
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

	// Create a docker credential for index.docker.io.
	registry1Name := namegen.AppendRandomString("reg-")
	reg1Body := map[string]any{
		"name": registry1Name,
		"registries": map[string]any{
			"index.docker.io": map[string]any{
				"username": "testuser",
				"password": "foobarbaz",
			},
		},
	}
	reg1Bytes, err := json.Marshal(reg1Body)
	s.Require().NoError(err)
	resp, err := httpClient.Post(s.dockerCredentialURL(project), "application/json", bytes.NewReader(reg1Bytes))
	s.Require().NoError(err)
	io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d creating registry1", resp.StatusCode)

	// Create a docker credential for quay.io.
	registry2Name := namegen.AppendRandomString("reg-")
	reg2Body := map[string]any{
		"name": registry2Name,
		"registries": map[string]any{
			"quay.io": map[string]any{
				"username": "testuser",
				"password": "foobarbaz",
			},
		},
	}
	reg2Bytes, err := json.Marshal(reg2Body)
	s.Require().NoError(err)
	resp, err = httpClient.Post(s.dockerCredentialURL(project), "application/json", bytes.NewReader(reg2Bytes))
	s.Require().NoError(err)
	io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d creating registry2", resp.StatusCode)

	// Create a workload using the docker.io image — registry1 should be auto-selected.
	wl := s.createWorkload(httpClient, project, map[string]any{
		"name":        namegen.AppendRandomString("workload-"),
		"namespaceId": ns.Name,
		"scale":       1,
		"containers": []any{
			map[string]any{"name": "one", "image": "testuser/testimage"},
		},
	})
	pullSecrets, ok := wl["imagePullSecrets"].([]any)
	s.Require().True(ok)
	s.Require().Len(pullSecrets, 1)
	secret0 := pullSecrets[0].(map[string]any)
	s.Equal(registry1Name, secret0["name"])

	// Update the workload to use a quay.io image — registry2 should now be selected.
	wlID := wl["id"].(string)
	updated := s.updateWorkload(httpClient, project, wlID, map[string]any{
		"containers": []any{
			map[string]any{"name": "one", "image": "quay.io/testuser/testimage"},
		},
	})
	containers, ok := updated["containers"].([]any)
	s.Require().True(ok)
	s.Require().NotEmpty(containers)
	container0 := containers[0].(map[string]any)
	s.Equal("quay.io/testuser/testimage", container0["image"])

	pullSecrets, ok = updated["imagePullSecrets"].([]any)
	s.Require().True(ok)
	s.Require().Len(pullSecrets, 1)
	s.Equal(registry2Name, pullSecrets[0].(map[string]any)["name"])
}

// TestWorkloadPortsChange asserts that changing container ports on a workload
// correctly updates the backing ClusterIP service's cluster IP field.
func (s *WorkloadTestSuite) TestWorkloadPortsChange() {
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

	// Create workload with no ports — expect a headless service (no cluster IP).
	workloadName := namegen.AppendRandomString("workload-")
	wl := s.createWorkload(httpClient, project, map[string]any{
		"name":        workloadName,
		"namespaceId": ns.Name,
		"scale":       1,
		"containers":  []any{map[string]any{"name": "one", "image": "nginx"}},
	})
	wlID := wl["id"].(string)

	svcListURL := fmt.Sprintf("%s?name=%s&kind=ClusterIP", s.serviceURL(project), workloadName)
	waitForService := func(check func(svc map[string]any) bool) map[string]any {
		var svc map[string]any
		s.Require().Eventually(func() bool {
			resp, err := httpClient.Get(svcListURL)
			if err != nil {
				return false
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var list struct {
				Data []map[string]any `json:"data"`
			}
			if json.Unmarshal(body, &list) != nil || len(list.Data) == 0 {
				return false
			}
			svc = list.Data[0]
			return check(svc)
		}, 30*time.Second, time.Second, "timed out waiting for service state")
		return svc
	}

	// Headless service: clusterIp should be absent or empty.
	svc := waitForService(func(svc map[string]any) bool {
		_, hasKey := svc["clusterIp"]
		return !hasKey || svc["clusterIp"] == nil || svc["clusterIp"] == ""
	})
	s.True(svc["clusterIp"] == nil || svc["clusterIp"] == "")
	s.Equal("ClusterIP", svc["kind"])

	// Update workload with a ClusterIP port — cluster IP should be assigned.
	s.updateWorkload(httpClient, project, wlID, map[string]any{
		"namespaceId": ns.Name,
		"scale":       1,
		"containers": []any{map[string]any{
			"name":  "one",
			"image": "nginx",
			"ports": []any{map[string]any{
				"sourcePort":    "0",
				"containerPort": "80",
				"kind":          "ClusterIP",
				"protocol":      "TCP",
			}},
		}},
	})
	svc = waitForService(func(svc map[string]any) bool {
		return svc["clusterIp"] != nil && svc["clusterIp"] != ""
	})
	s.NotEmpty(svc["clusterIp"])

	// Update workload removing ports — cluster IP should be reset.
	s.updateWorkload(httpClient, project, wlID, map[string]any{
		"namespaceId": ns.Name,
		"scale":       1,
		"containers": []any{map[string]any{
			"name":  "one",
			"image": "nginx",
			"ports": []any{},
		}},
	})
	svc = waitForService(func(svc map[string]any) bool {
		return svc["clusterIp"] == nil || svc["clusterIp"] == ""
	})
	s.True(svc["clusterIp"] == nil || svc["clusterIp"] == "")
}

// TestWorkloadProbes asserts that liveness and readiness probes on a workload
// container are persisted and can be updated via the Norman project API.
func (s *WorkloadTestSuite) TestWorkloadProbes() {
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

	container := map[string]any{
		"name":  "one",
		"image": "nginx",
		"livenessProbe": map[string]any{
			"failureThreshold":    3,
			"initialDelaySeconds": 10,
			"periodSeconds":       2,
			"successThreshold":    1,
			"tcp":                 false,
			"timeoutSeconds":      2,
			"host":                "localhost",
			"path":                "/healthcheck",
			"port":                80,
			"scheme":              "HTTP",
		},
		"readinessProbe": map[string]any{
			"failureThreshold":    3,
			"initialDelaySeconds": 10,
			"periodSeconds":       2,
			"successThreshold":    1,
			"timeoutSeconds":      2,
			"tcp":                 true,
			"host":                "localhost",
			"port":                80,
		},
	}

	wl := s.createWorkload(httpClient, project, map[string]any{
		"name":        namegen.AppendRandomString("workload-"),
		"namespaceId": ns.Name,
		"scale":       1,
		"containers":  []any{container},
	})

	getProbeHost := func(wlResp map[string]any, probe string) string {
		containers := wlResp["containers"].([]any)
		s.Require().NotEmpty(containers)
		c := containers[0].(map[string]any)
		p := c[probe].(map[string]any)
		return p["host"].(string)
	}
	s.Equal("localhost", getProbeHost(wl, "livenessProbe"))
	s.Equal("localhost", getProbeHost(wl, "readinessProbe"))

	container["livenessProbe"].(map[string]any)["host"] = "updatedhost"
	container["readinessProbe"].(map[string]any)["host"] = "updatedhost"

	wlID := wl["id"].(string)
	updated := s.updateWorkload(httpClient, project, wlID, map[string]any{
		"namespaceId": ns.Name,
		"scale":       1,
		"containers":  []any{container},
	})

	s.Equal("updatedhost", getProbeHost(updated, "livenessProbe"))
	s.Equal("updatedhost", getProbeHost(updated, "readinessProbe"))
}

// TestWorkloadScheduling asserts that the scheduler field on a workload is
// persisted and can be updated via the Norman project API.
func (s *WorkloadTestSuite) TestWorkloadScheduling() {
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

	wl := s.createWorkload(httpClient, project, map[string]any{
		"name":        namegen.AppendRandomString("workload-"),
		"namespaceId": ns.Name,
		"scale":       1,
		"scheduling":  map[string]any{"scheduler": "some-scheduler"},
		"containers":  []any{map[string]any{"name": "one", "image": "nginx"}},
	})

	getScheduler := func(wlResp map[string]any) string {
		scheduling := wlResp["scheduling"].(map[string]any)
		return scheduling["scheduler"].(string)
	}
	s.Equal("some-scheduler", getScheduler(wl))

	wlID := wl["id"].(string)
	updated := s.updateWorkload(httpClient, project, wlID, map[string]any{
		"namespaceId": ns.Name,
		"scale":       1,
		"scheduling":  map[string]any{"scheduler": "test-scheduler"},
		"containers":  []any{map[string]any{"name": "one", "image": "nginx"}},
	})
	s.Equal("test-scheduler", getScheduler(updated))
}

// TestStatefulSetWorkloadVolumeMountSubpath asserts that the Norman project API
// rejects workload creation and update requests where a volumeMount subPath is
// either an absolute path or contains "..".
func (s *WorkloadTestSuite) TestStatefulSetWorkloadVolumeMountSubpath() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	project, err := client.Management.Project.Create(&management.Project{
		ClusterID: "local",
		Name:      namegen.AppendRandomString("project-"),
	})
	s.Require().NoError(err)

	httpClient := s.httpClient()

	statefulSetConfig := map[string]any{
		"podManagementPolicy":  "OrderedReady",
		"revisionHistoryLimit": 10,
		"strategy":             "RollingUpdate",
		"type":                 "statefulSetConfig",
	}
	volumes := []any{map[string]any{
		"name": "vol1",
		"persistentVolumeClaim": map[string]any{
			"persistentVolumeClaimId": "default: myvolume",
			"readOnly":                false,
			"type":                    "persistentVolumeClaimVolumeSource",
		},
		"type": "volume",
	}}

	makeContainers := func(subPath string) []any {
		return []any{map[string]any{
			"name":  "mystatefulset",
			"image": "ubuntu:xenial",
			"volumeMounts": []any{map[string]any{
				"name":      "vol1",
				"mountPath": "var/lib/mysql",
				"subPath":   subPath,
			}},
		}}
	}

	postWorkload := func(containers []any) int {
		body := map[string]any{
			"name":              namegen.AppendRandomString("wl-"),
			"namespaceId":       "default",
			"scale":             1,
			"containers":        containers,
			"statefulSetConfig": statefulSetConfig,
			"volumes":           volumes,
		}
		bodyBytes, err := json.Marshal(body)
		s.Require().NoError(err)
		resp, err := httpClient.Post(s.workloadURL(project), "application/json", bytes.NewReader(bodyBytes))
		s.Require().NoError(err)
		io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp.StatusCode
	}

	// Absolute subPath should be rejected with 422.
	s.Equal(http.StatusUnprocessableEntity, postWorkload(makeContainers("/mysql")))
	// subPath containing ".." should be rejected with 422.
	s.Equal(http.StatusUnprocessableEntity, postWorkload(makeContainers("../mysql")))

	// Valid subPath — workload creation should succeed.
	name := namegen.AppendRandomString("wl-")
	validBody := map[string]any{
		"name":              name,
		"namespaceId":       "default",
		"scale":             1,
		"containers":        makeContainers("mysql"),
		"statefulSetConfig": statefulSetConfig,
		"volumes":           volumes,
	}
	validBytes, err := json.Marshal(validBody)
	s.Require().NoError(err)
	resp, err := httpClient.Post(s.workloadURL(project), "application/json", bytes.NewReader(validBytes))
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d creating valid workload: %s", resp.StatusCode, string(respBody))

	var wlResp map[string]any
	s.Require().NoError(json.Unmarshal(respBody, &wlResp))
	wlID := wlResp["id"].(string)

	// Update with invalid subPaths should also be rejected.
	s.Equal(http.StatusUnprocessableEntity, func() int {
		return s.updateWorkloadStatus(httpClient, project, wlID, map[string]any{
			"namespaceId":       "default",
			"scale":             1,
			"containers":        makeContainers("/mysql"),
			"statefulSetConfig": statefulSetConfig,
			"volumes":           volumes,
		})
	}())
	s.Equal(http.StatusUnprocessableEntity, func() int {
		return s.updateWorkloadStatus(httpClient, project, wlID, map[string]any{
			"namespaceId":       "default",
			"scale":             1,
			"containers":        makeContainers("../mysql"),
			"statefulSetConfig": statefulSetConfig,
			"volumes":           volumes,
		})
	}())
}

// updateWorkloadStatus performs a PUT on a workload and returns only the HTTP status code.
func (s *WorkloadTestSuite) updateWorkloadStatus(httpClient *http.Client, project *management.Project, workloadID string, body map[string]any) int {
	bodyBytes, err := json.Marshal(body)
	s.Require().NoError(err)
	url := fmt.Sprintf("%s/%s", s.workloadURL(project), workloadID)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(bodyBytes))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	s.Require().NoError(err)
	io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp.StatusCode
}

// TestWorkloadRedeploy asserts that the redeploy action sets the
// cattle.io/timestamp annotation on the workload.
func (s *WorkloadTestSuite) TestWorkloadRedeploy() {
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

	wl := s.createWorkload(httpClient, project, map[string]any{
		"name":        namegen.AppendRandomString("workload-"),
		"namespaceId": ns.Name,
		"scale":       1,
		"containers":  []any{map[string]any{"name": "one", "image": "nginx"}},
	})

	wlID := wl["id"].(string)

	// Trigger the redeploy action.
	actionURL := fmt.Sprintf("%s/%s?action=redeploy", s.workloadURL(project), wlID)
	resp, err := httpClient.Post(actionURL, "application/json", bytes.NewReader([]byte("{}")))
	s.Require().NoError(err)
	io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d for redeploy action", resp.StatusCode)

	// Poll until the cattle.io/timestamp annotation appears on the workload.
	getURL := fmt.Sprintf("%s/%s", s.workloadURL(project), wlID)
	s.Require().Eventually(func() bool {
		resp, err := httpClient.Get(getURL)
		if err != nil {
			return false
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var wlResp map[string]any
		if json.Unmarshal(body, &wlResp) != nil {
			return false
		}
		annotations, ok := wlResp["annotations"].(map[string]any)
		if !ok {
			return false
		}
		ts, ok := annotations["cattle.io/timestamp"]
		return ok && ts != nil && ts != ""
	}, 30*time.Second, time.Second, "timed out waiting for cattle.io/timestamp annotation after redeploy")
}

// TestWorkloadActionReadOnly asserts that a read-only project member receives a
// 404 when attempting the rollback action, while a project-member succeeds.
func (s *WorkloadTestSuite) TestWorkloadActionReadOnly() {
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

	// Create read-only user.
	enabled := true
	roPW := password.GenerateUserPassword("testpass-")
	roUser, err := users.CreateUserWithRole(client, &management.User{
		Username: namegen.AppendRandomString("rouser-"),
		Password: roPW,
		Name:     "rouser",
		Enabled:  &enabled,
	}, "user")
	s.Require().NoError(err)
	roUser.Password = roPW

	// Create project-member user.
	memberPW := password.GenerateUserPassword("testpass-")
	memberUser, err := users.CreateUserWithRole(client, &management.User{
		Username: namegen.AppendRandomString("memberuser-"),
		Password: memberPW,
		Name:     "memberuser",
		Enabled:  &enabled,
	}, "user")
	s.Require().NoError(err)
	memberUser.Password = memberPW

	// Bind read-only user to project.
	_, err = client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		UserID:         roUser.ID,
		RoleTemplateID: "read-only",
		ProjectID:      project.ID,
	})
	s.Require().NoError(err)

	// Bind project-member user to project.
	_, err = client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		UserID:         memberUser.ID,
		RoleTemplateID: "project-member",
		ProjectID:      project.ID,
	})
	s.Require().NoError(err)

	httpClient := s.httpClient()

	// Admin creates the workload.
	workloadName := namegen.AppendRandomString("workload-")
	wl := s.createWorkload(httpClient, project, map[string]any{
		"name":        workloadName,
		"namespaceId": ns.Name,
		"scale":       1,
		"containers": []any{map[string]any{
			"name":  "foo",
			"image": "rancher/mirrored-library-nginx:1.21.1-alpine",
			"env":   []any{map[string]any{"name": "FOO_KEY", "value": "FOO_VALUE"}},
		}},
	})
	wlID := wl["id"].(string)

	// Update workload once to produce a revision that can be rolled back.
	s.updateWorkload(httpClient, project, wlID, map[string]any{
		"namespaceId": ns.Name,
		"scale":       1,
		"containers": []any{map[string]any{
			"name":  "foo",
			"image": "rancher/mirrored-library-nginx:1.21.1-alpine",
			"env":   []any{map[string]any{"name": "BAR_KEY", "value": "BAR_VALUE"}},
		}},
	})

	// Wait for the workload to appear.
	listURL := fmt.Sprintf("%s?namespaceId=%s", s.workloadURL(project), ns.Name)
	s.Require().Eventually(func() bool {
		resp, err := httpClient.Get(listURL)
		if err != nil {
			return false
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var list struct {
			Data []map[string]any `json:"data"`
		}
		if json.Unmarshal(body, &list) != nil {
			return false
		}
		for _, w := range list.Data {
			if w["id"] == wlID {
				return true
			}
		}
		return false
	}, 30*time.Second, time.Second, "timed out waiting for workload to appear in list")

	// Fetch a real replicaSet ID from the workload's revisions link.
	wlRevisionsURL := fmt.Sprintf("%s/%s/revisions", s.workloadURL(project), wlID)
	var replicaSetID string
	s.Require().Eventually(func() bool {
		resp, err := httpClient.Get(wlRevisionsURL)
		if err != nil {
			return false
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var list struct {
			Data []map[string]any `json:"data"`
		}
		if json.Unmarshal(body, &list) != nil || len(list.Data) == 0 {
			return false
		}
		id, ok := list.Data[0]["id"].(string)
		if !ok || id == "" {
			return false
		}
		replicaSetID = id
		return true
	}, 30*time.Second, time.Second, "timed out waiting for workload revision to appear")

	rollbackURL := fmt.Sprintf("%s/%s?action=rollback", s.workloadURL(project), wlID)
	rollbackBody, err := json.Marshal(map[string]any{"replicaSetId": replicaSetID})
	s.Require().NoError(err)

	roClient, err := client.AsUser(roUser)
	s.Require().NoError(err)
	roHTTP, err := rest.HTTPClientFor(roClient.WranglerContext.RESTConfig)
	s.Require().NoError(err)

	// Read-only user attempting rollback should receive 404.
	roResp, err := roHTTP.Post(rollbackURL, "application/json", bytes.NewReader(rollbackBody))
	s.Require().NoError(err)
	io.ReadAll(roResp.Body)
	roResp.Body.Close()
	s.Equal(http.StatusNotFound, roResp.StatusCode)

	memberClient, err := client.AsUser(memberUser)
	s.Require().NoError(err)
	memberHTTP, err := rest.HTTPClientFor(memberClient.WranglerContext.RESTConfig)
	s.Require().NoError(err)

	// Project-member user attempting rollback should succeed (2xx).
	memberResp, err := memberHTTP.Post(rollbackURL, "application/json", bytes.NewReader(rollbackBody))
	s.Require().NoError(err)
	memberRespBody, _ := io.ReadAll(memberResp.Body)
	memberResp.Body.Close()
	s.Truef(memberResp.StatusCode >= 200 && memberResp.StatusCode < 300,
		"expected 2xx for project-member rollback, got %d: %s", memberResp.StatusCode, string(memberRespBody))
}

func TestWorkload(t *testing.T) {
	suite.Run(t, new(WorkloadTestSuite))
}

// hpaURL returns the base Norman project HPA URL for the given project.
func (s *WorkloadTestSuite) hpaURL(project *management.Project) string {
	return fmt.Sprintf("https://%s/v3/project/%s/horizontalPodAutoscalers",
		s.client.WranglerContext.RESTConfig.Host, project.ID)
}

// TestHPA asserts that a HorizontalPodAutoscaler can be created via the Norman
// project API with multiple metric types (Resource, Pods, External, Object),
// and that it appears in the HPA list with the expected state.
func (s *WorkloadTestSuite) TestHPA() {
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

	// Create a workload via the Norman project API.
	workloadBody := map[string]any{
		"name":        namegen.AppendRandomString("workload-"),
		"namespaceId": ns.Name,
		"scale":       1,
		"containers": []any{
			map[string]any{
				"name":  "one",
				"image": "nginx",
				"resources": map[string]any{
					"requests": "100m",
				},
			},
		},
	}
	workloadBytes, err := json.Marshal(workloadBody)
	s.Require().NoError(err)

	resp, err := httpClient.Post(s.workloadURL(project), "application/json", bytes.NewReader(workloadBytes))
	s.Require().NoError(err)
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d creating workload: %s", resp.StatusCode, string(respBody))

	var workloadResp struct {
		ID string `json:"id"`
	}
	s.Require().NoError(json.Unmarshal(respBody, &workloadResp))
	s.Require().NotEmpty(workloadResp.ID)

	// Create an HPA referencing the workload with multiple metric types.
	hpaBody := map[string]any{
		"name":        namegen.AppendRandomString("hpa-"),
		"namespaceId": ns.Name,
		"maxReplicas": 10,
		"workloadId":  workloadResp.ID,
		"metrics": []any{
			map[string]any{
				"name": "cpu",
				"type": "Resource",
				"target": map[string]any{
					"type":        "Utilization",
					"utilization": "50",
				},
			},
			map[string]any{
				"name": "pods-test",
				"type": "Pods",
				"target": map[string]any{
					"type":         "AverageValue",
					"averageValue": "50",
				},
			},
			map[string]any{
				"name": "pods-external",
				"type": "External",
				"target": map[string]any{
					"type":  "Value",
					"value": "50",
				},
			},
			map[string]any{
				"describedObject": map[string]any{
					"apiVersion": "extensions/v1beta1",
					"kind":       "Ingress",
					"name":       "test",
				},
				"name": "object-test",
				"type": "Object",
				"target": map[string]any{
					"type":  "Value",
					"value": "50",
				},
			},
		},
	}
	hpaBytes, err := json.Marshal(hpaBody)
	s.Require().NoError(err)

	resp, err = httpClient.Post(s.hpaURL(project), "application/json", bytes.NewReader(hpaBytes))
	s.Require().NoError(err)
	respBody, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Truef(resp.StatusCode >= 200 && resp.StatusCode < 300,
		"unexpected status %d creating HPA: %s", resp.StatusCode, string(respBody))

	// List HPAs and verify exactly one exists in the initializing state.
	listURL := fmt.Sprintf("%s?namespaceId=%s", s.hpaURL(project), ns.Name)
	resp, err = httpClient.Get(listURL)
	s.Require().NoError(err)
	respBody, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var hpaList struct {
		Data []struct {
			State string `json:"state"`
		} `json:"data"`
	}
	s.Require().NoError(json.Unmarshal(respBody, &hpaList))
	s.Require().Len(hpaList.Data, 1)
	s.Equal("initializing", hpaList.Data[0].State)
}

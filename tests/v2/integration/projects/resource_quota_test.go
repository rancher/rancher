package integration

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/resourcequotas"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/defaults"
	steveResourceQuotas "github.com/rancher/shepherd/extensions/resourcequotas"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/rancher/shepherd/pkg/wait"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	resourceQuotaNamespaceName  = "ns1"
	resourceQuotaNamespaceName2 = "ns2"
	localClusterID              = "local"
)

type ResourceQuotaSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *ResourceQuotaSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *ResourceQuotaSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *ResourceQuotaSuite) TestCreateNamespaceWithQuotaInProject() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	projectLimit := &management.ResourceQuotaLimit{
		LimitsCPU: "500m",
	}
	namespaceDefaultLimit := &management.ResourceQuotaLimit{
		LimitsCPU: "200m",
	}

	projectConfig := &management.Project{
		ClusterID: "local",
		Name:      "TestProject",
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: projectLimit,
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: namespaceDefaultLimit,
		},
	}
	testProject, err := client.Management.Project.Create(projectConfig)
	s.Require().NoError(err)

	namespace, err := namespaces.CreateNamespace(client, resourceQuotaNamespaceName, "", map[string]string{}, map[string]string{}, testProject)
	s.Require().NoError(err)
	s.Require().NotNil(namespace)

	quotas, err := resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().NotNil(quotas)
	s.Require().Lenf(quotas.Items, 1, "Expected 1 quota in a new namespace, but got %d", len(quotas.Items))

	resourceList := quotas.Items[0].Spec.Hard
	want := v1.ResourceList{
		v1.ResourceLimitsCPU: resource.MustParse("200m"),
	}
	s.Require().Equal(want, resourceList)
	s.Require().NoError(err)
}

func (s *ResourceQuotaSuite) TestCreateNamespaceWithOverriddenQuotaInProject() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	projectConfig := &management.Project{
		ClusterID: "local",
		Name:      "TestProject",
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{
				LimitsCPU: "500m",
			},
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{
				LimitsCPU: "200m",
			},
		},
	}
	testProject, err := client.Management.Project.Create(projectConfig)
	s.Require().NoError(err)

	annotations1 := map[string]string{
		"field.cattle.io/resourceQuota": "{\"limit\":{\"limitsCpu\":\"190m\"}}",
	}
	namespace, err := namespaces.CreateNamespace(client, resourceQuotaNamespaceName, "", map[string]string{}, annotations1, testProject)
	s.Require().NoError(err)
	s.Require().NotNil(namespace)

	annotations2 := map[string]string{
		"field.cattle.io/resourceQuota": "{\"limit\":{\"limitsCpu\":\"400m\", \"configMaps\":\"50\"}}",
	}
	namespace, err = namespaces.CreateNamespace(client, resourceQuotaNamespaceName2, "", map[string]string{}, annotations2, testProject)
	s.Require().NoError(err)
	s.Require().NotNil(namespace)

	quotas, err := resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().NotNil(quotas)
	s.Require().Lenf(quotas.Items, 1, "Expected 1 quota in %s, but got %d", resourceQuotaNamespaceName, len(quotas.Items))

	resourceList := quotas.Items[0].Spec.Hard
	want := v1.ResourceList{
		v1.ResourceLimitsCPU: resource.MustParse("190m"),
	}
	s.Require().Equal(want, resourceList)
	s.Require().NoError(err)

	quotas, err = resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName2, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().NotNil(quotas)
	s.Require().Lenf(quotas.Items, 1, "Expected 1 quota in %s, but got %d", resourceQuotaNamespaceName2, len(quotas.Items))

	resourceList = quotas.Items[0].Spec.Hard
	want = v1.ResourceList{
		v1.ResourceLimitsCPU: resource.MustParse("0"),
	}
	s.Require().Equal(want, resourceList)
	s.Require().NoError(err)
}

func (s *ResourceQuotaSuite) TestRemoveQuotaFromProjectWithNamespacePropagation() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	dynamicClient, err := client.GetRancherDynamicClient()
	s.Require().NoError(err)

	projectLimit := &management.ResourceQuotaLimit{
		LimitsCPU:  "500m",
		ConfigMaps: "10",
	}
	namespaceDefaultLimit := &management.ResourceQuotaLimit{
		LimitsCPU:  "200m",
		ConfigMaps: "5",
	}

	projectConfig := &management.Project{
		ClusterID: "local",
		Name:      "TestProject",
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: projectLimit,
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: namespaceDefaultLimit,
		},
	}
	testProject, err := client.Management.Project.Create(projectConfig)
	s.Require().NoError(err)

	namespace, err := namespaces.CreateNamespace(client, resourceQuotaNamespaceName, "", map[string]string{}, map[string]string{}, testProject)
	s.Require().NoError(err)
	s.Require().NotNil(namespace)

	testProject.ResourceQuota.Limit.LimitsCPU = ""
	testProject.NamespaceDefaultResourceQuota.Limit.LimitsCPU = ""

	_, err = client.Management.Project.Replace(testProject)
	s.Require().NoError(err)

	quotas, err := resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
	s.Require().NoError(err)

	// Allow the controller to update the resource quotas after the project has been updated.
	quotaName := quotas.Items[0].Name
	quotaID := fmt.Sprintf("%s/%s", resourceQuotaNamespaceName, quotaName)
	err = steveResourceQuotas.CheckResourceActiveState(client, quotaID)
	s.Require().NoError(err)

	// Wait a little while for the resourcequota to be updated.
	// The quota controller sometimes gets conflict error trying to update the namespace annotation and needs time to retry.
	want := v1.ResourceList{
		v1.ResourceConfigMaps: resource.MustParse("5"),
	}
	var resourceList v1.ResourceList
	err = kwait.Poll(500*time.Millisecond, 10*time.Second, func() (done bool, err error) {
		quotas, err = resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		if quotas == nil {
			return false, nil
		}
		if len(quotas.Items) != 1 {
			return false, fmt.Errorf("expected 1 quota in the namespace, but got %d", len(quotas.Items))
		}
		resourceList = quotas.Items[0].Spec.Hard
		if !reflect.DeepEqual(want, resourceList) {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		// if it was a timeout error, check why it never succeeded so it can be reported as output
		s.Require().NotNil(quotas)
		s.Require().Lenf(quotas.Items, 1, "Expected 1 quota in the namespace, but got %d", len(quotas.Items))
		s.Require().Equal(want, resourceList, "Expected the CPU limits to be removed, but config maps limit to remain")
	}
	s.Require().NoError(err)

	// Now remove the last resource limit from the project.
	testProject.ResourceQuota.Limit.ConfigMaps = ""
	testProject.NamespaceDefaultResourceQuota.Limit.ConfigMaps = ""

	watchInterface, err := dynamicClient.Resource(resourcequotas.ResourceQuotaGroupVersionResource).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + quotaName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	s.Require().NoError(err)

	_, err = client.Management.Project.Replace(testProject)
	s.Require().NoError(err)

	err = wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
		if event.Type == watch.Error {
			return false, fmt.Errorf("there was an error deleting cluster")
		} else if event.Type == watch.Deleted {
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)

	quotas, err = resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().NotNil(quotas)
	s.Require().Lenf(quotas.Items, 0, "Expected no quotas in the namespace, but got %d", len(quotas.Items))
}

func (s *ResourceQuotaSuite) TestAddQuotaFromProjectWithNamespacePropagation() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	projectLimit := &management.ResourceQuotaLimit{
		LimitsCPU: "500m",
	}
	namespaceDefaultLimit := &management.ResourceQuotaLimit{
		LimitsCPU: "200m",
	}

	projectConfig := &management.Project{
		ClusterID: "local",
		Name:      "TestProject",
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: projectLimit,
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: namespaceDefaultLimit,
		},
	}
	testProject, err := client.Management.Project.Create(projectConfig)
	s.Require().NoError(err)

	namespace, err := namespaces.CreateNamespace(client, resourceQuotaNamespaceName, "", map[string]string{}, map[string]string{}, testProject)
	s.Require().NoError(err)
	s.Require().NotNil(namespace)

	testProject.ResourceQuota.Limit.Secrets = "20"
	testProject.NamespaceDefaultResourceQuota.Limit.Secrets = "10"

	_, err = client.Management.Project.Replace(testProject)
	s.Require().NoError(err)

	quotas, err := resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
	s.Require().NoError(err)

	// Allow the controller to update the resource quotas after the project has been updated.
	quotaName := quotas.Items[0].Name
	quotaID := fmt.Sprintf("%s/%s", resourceQuotaNamespaceName, quotaName)
	err = steveResourceQuotas.CheckResourceActiveState(client, quotaID)
	s.Require().NoError(err)

	// Wait a little while for the resourcequota to be updated.
	// The quota controller sometimes gets conflict error trying to update the namespace annotation and needs time to retry.
	want := v1.ResourceList{
		v1.ResourceLimitsCPU: resource.MustParse("200m"),
		v1.ResourceSecrets:   resource.MustParse("10"),
	}
	var resourceList v1.ResourceList
	err = kwait.Poll(500*time.Millisecond, 10*time.Second, func() (done bool, err error) {
		quotas, err = resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		if quotas == nil {
			return false, nil
		}
		if len(quotas.Items) != 1 {
			return false, fmt.Errorf("expected 1 quota in the namespace, but got %d", len(quotas.Items))
		}
		resourceList = quotas.Items[0].Spec.Hard
		if !reflect.DeepEqual(want, resourceList) {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		// if it was a timeout error, check why it never succeeded so it can be reported as output
		s.Require().NotNil(quotas)
		s.Require().Lenf(quotas.Items, 1, "Expected 1 quota in the namespace, but got %d", len(quotas.Items))
		s.Require().Equal(want, resourceList, "Expected the secrets limits to be added")
	}
	s.Require().NoError(err)
}

func TestResourceQuotaTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceQuotaSuite))
}

//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package ingress

import (
	"fmt"
	"strings"
	"testing"
	"time"

	normantypes "github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/v2/actions/ingresses"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/pods"
	"github.com/rancher/rancher/tests/v2/actions/services"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	extensionsingress "github.com/rancher/shepherd/extensions/ingresses"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/util/intstr"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	nginxImage          = "nginx"
	DeploymentSteveType = "apps.deployment"
	ingressPath         = "/index.html"
)

type IngressTestSuite struct {
	suite.Suite
	client    *rancher.Client
	session   *session.Session
	cluster   *management.Cluster
	namespace *corev1.Namespace
	workload  *v1.SteveAPIObject
}

func (i *IngressTestSuite) SetupSuite() {
	time.Sleep(5 * time.Second)
	log.Info("Initializing Ingress Test Suite")

	i.session = session.NewSession()
	client, err := rancher.NewClient("", i.session)
	if err != nil {
		log.Fatalf("Suite setup failed - client creation error: %v", err)
	}
	i.client = client

	clusterName := client.RancherConfig.ClusterName
	if clusterName == "" {
		log.Fatal("Suite setup failed - cluster name not set in configuration")
	}

	clusterID, err := clusters.GetClusterIDByName(i.client, clusterName)
	if err != nil {
		log.Fatalf("Suite setup failed - error getting cluster ID: %v", err)
	}

	i.cluster, err = i.client.Management.Cluster.ByID(clusterID)
	if err != nil {
		log.Fatalf("Suite setup failed - error getting cluster: %v", err)
	}

	log.Infof("Suite setup complete - using cluster: %s (%s)", clusterName, clusterID)
}

func (i *IngressTestSuite) TearDownTest() {
	testName := i.T().Name()
	log.Infof("Starting cleanup for test: %s", testName)

	if i.client == nil || i.cluster == nil {
		log.Warnf("Skipping cleanup for %s - client or cluster not initialized", testName)
		return
	}

	steveClient, err := i.client.Steve.ProxyDownstream(i.cluster.ID)
	if err != nil {
		log.Errorf("Failed to get downstream client for cleanup in %s: %v", testName, err)
		return
	}

	// Safe cleanup even if namespace is nil
	if err := cleanupResources(steveClient, i.namespace); err != nil {
		log.Errorf("Initial cleanup failed for %s: %v", testName, err)
		log.Info("Attempting secondary cleanup after delay")
		time.Sleep(5 * time.Second)
		if err := cleanupResources(steveClient, i.namespace); err != nil {
			log.Errorf("Secondary cleanup also failed for %s: %v", testName, err)
		}
	}

	// Reset suite variables
	i.namespace = nil
	i.workload = nil

	log.Infof("Cleanup completed for test: %s", testName)
}

func (i *IngressTestSuite) TestIngressFields() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()
	log.Info("Starting ingress fields validation test")

	steveClient, _, namespace, err := setupTest(i.client, i.cluster.ID, "ingress fields")
	if err != nil {
		log.Fatalf("Test setup failed: %v", err)
	}

	i.namespace = namespace

	schemas, err := steveClient.SteveType("schema").List(nil)
	if err != nil {
		log.Fatalf("Failed to retrieve schemas: %v", err)
	}

	ingressSchema := findIngressSchema(schemas)
	if ingressSchema == nil {
		log.Fatal("Ingress schema not found in the cluster")
	}

	attributes, ok := ingressSchema.JSONResp["attributes"].(map[string]interface{})
	if !ok {
		log.Fatal("Failed to parse ingress schema attributes")
	}

	log.Info("Beginning schema validation checks")
	validateSchemaVerbs(i.T(), attributes)
	fields := validateSchemaColumns(i.T(), attributes)
	validateExpectedFields(i.T(), fields)
	validateNamespaceScope(i.T(), attributes)

	log.Info("Ingress fields validation completed successfully")
}

func (i *IngressTestSuite) TestIngressCreation() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()
	log.Info("Starting ingress creation test")

	steveClient, project, namespace, err := setupTest(i.client, i.cluster.ID, "ingress creation")
	if err != nil {
		log.Fatalf("Test setup failed: %v", err)
	}

	workload, err := createWorkload(i.client, i.cluster.ID, namespace.Name, 1)
	if err != nil {
		log.Fatalf("Workload creation failed: %v", err)
	}

	i.workload = &v1.SteveAPIObject{
		Resource: normantypes.Resource{
			ID:   workload.Name,
			Type: DeploymentSteveType,
		},
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.Name + "-svc",
			Namespace: namespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: workload.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{{
				Port:       ingresses.IngressTestPort,
				TargetPort: intstr.FromInt(ingresses.IngressTestPort),
			}},
		},
	}

	createdService, err := services.CreateService(steveClient, *service)
	if err != nil {
		log.Fatalf("Service creation failed: %v", err)
	}
	log.Infof("Service created successfully: %s", createdService.Name)

	initialIngressName := namegen.AppendRandomString("test-ingress")
	host := generateUniqueHost(i.T(), "creation")
	path := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypeExact, ingressPath, createdService.Name, ingresses.IngressTestPort)
	ingressTemplate := extensionsingress.NewIngressTemplate(initialIngressName, namespace.Name, host, []networkingv1.HTTPIngressPath{path})

	ingress, err := extensionsingress.CreateIngress(steveClient, initialIngressName, ingressTemplate)
	if err != nil {
		log.Fatalf("Ingress creation failed: %v", err)
	}
	log.Infof("Ingress created successfully: %s", ingress.Name)

	log.Info("Waiting for ingress to become active")
	ingresses.WaitForIngressToBeActive(i.client, project.ClusterID, namespace.Name, ingress.Name)

	log.Info("Validating ingress configuration")
	ingresses.ValidateIngress(i.T(), steveClient, ingress, host, ingressPath, project.ClusterID)

	log.Infof("Ingress creation test completed successfully for: %s", ingress.Name)
}

func (i *IngressTestSuite) TestIngressWithMultipleTargets() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()

	steveClient, project, namespace, err := setupTest(i.client, i.cluster.ID, "ingress multiple targets")
	require.NoError(i.T(), err)

	log.Info("Creating first workload")
	workload1, err := createWorkload(i.client, i.cluster.ID, namespace.Name, 1)
	require.NoError(i.T(), err)

	log.Info("Creating first service")
	service1 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload1.Name + "-svc",
			Namespace: namespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: workload1.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{{
				Port:       ingresses.IngressTestPort,
				TargetPort: intstr.FromInt(ingresses.IngressTestPort),
			}},
		},
	}
	createdService1, err := services.CreateService(steveClient, *service1)
	require.NoError(i.T(), err)

	log.Info("Creating second workload")
	workload2, err := createWorkload(i.client, i.cluster.ID, namespace.Name, 1)
	require.NoError(i.T(), err)

	log.Info("Creating second service")
	service2 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload2.Name + "-svc",
			Namespace: namespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: workload2.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{{
				Port:       ingresses.IngressTestPort,
				TargetPort: intstr.FromInt(ingresses.IngressTestPort),
			}},
		},
	}
	createdService2, err := services.CreateService(steveClient, *service2)
	require.NoError(i.T(), err)

	i.workload = &v1.SteveAPIObject{
		Resource: normantypes.Resource{
			ID:   workload1.Name,
			Type: DeploymentSteveType,
		},
	}

	host := generateUniqueHost(i.T(), "multi-target")
	path := "/"
	ingressName := namegen.AppendRandomString("test-ingress")

	path1 := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, path, createdService1.Name, ingresses.IngressTestPort)
	path2 := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, path, createdService2.Name, ingresses.IngressTestPort)

	ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, host, []networkingv1.HTTPIngressPath{path1, path2})

	log.Info("Creating ingress with multiple targets")
	ingress, err := extensionsingress.CreateIngress(steveClient, ingressName, ingressTemplate)
	require.NoError(i.T(), err)

	log.Info("Waiting for ingress to be active")
	ingresses.WaitForIngressToBeActive(i.client, project.ClusterID, namespace.Name, ingress.Name)

	log.Info("Validating ingress rules")
	updatedIngress, err := steveClient.SteveType(ingress.Type).ByID(ingress.ID)
	require.NoError(i.T(), err)

	var k8sIngress networkingv1.Ingress
	err = v1.ConvertToK8sType(updatedIngress, &k8sIngress)
	require.NoError(i.T(), err)

	require.Len(i.T(), k8sIngress.Spec.Rules, 1, "Should have one rule")
	require.Equal(i.T(), host, k8sIngress.Spec.Rules[0].Host)
	require.Len(i.T(), k8sIngress.Spec.Rules[0].HTTP.Paths, 2, "Should have two paths")
	require.ElementsMatch(i.T(), []string{createdService1.Name, createdService2.Name},
		[]string{
			k8sIngress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name,
			k8sIngress.Spec.Rules[0].HTTP.Paths[1].Backend.Service.Name,
		})
}

func (i *IngressTestSuite) TestIngressEditPath() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()

	steveClient, project, namespace, err := setupTest(i.client, i.cluster.ID, "ingress edit path")
	require.NoError(i.T(), err)

	log.Info("Creating workload")
	workload, err := createWorkload(i.client, i.cluster.ID, namespace.Name, 1)
	require.NoError(i.T(), err)

	i.workload = &v1.SteveAPIObject{
		Resource: normantypes.Resource{
			ID:   workload.Name,
			Type: DeploymentSteveType,
		},
	}

	log.Info("Creating service")
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.Name + "-svc",
			Namespace: namespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: workload.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{{
				Port:       ingresses.IngressTestPort,
				TargetPort: intstr.FromInt(ingresses.IngressTestPort),
			}},
		},
	}
	createdService, err := services.CreateService(steveClient, *service)
	require.NoError(i.T(), err)

	host := generateUniqueHost(i.T(), "edit-path")
	path1 := "/name.html"
	path2 := "/service1.html"
	ingressName := namegen.AppendRandomString("test-ingress")

	initialPath := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, path1, createdService.Name, ingresses.IngressTestPort)
	ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, host, []networkingv1.HTTPIngressPath{initialPath})

	log.Info("Creating initial ingress")
	ingress, err := extensionsingress.CreateIngress(steveClient, ingressName, ingressTemplate)
	require.NoError(i.T(), err)

	log.Info("Waiting for ingress to be active")
	ingresses.WaitForIngressToBeActive(i.client, project.ClusterID, namespace.Name, ingress.Name)

	log.Info("Validating initial ingress")
	ingresses.ValidateIngress(i.T(), steveClient, ingress, host, path1, project.ClusterID)

	log.Info("Updating ingress path")
	updatedPath := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, path2, createdService.Name, ingresses.IngressTestPort)
	updatedTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, host, []networkingv1.HTTPIngressPath{updatedPath})

	updatedIngress, err := ingresses.UpdateIngress(steveClient, ingress, &updatedTemplate)
	require.NoError(i.T(), err)

	log.Info("Waiting for updated ingress to be active")
	ingresses.WaitForIngressToBeActive(i.client, project.ClusterID, namespace.Name, updatedIngress.Name)

	log.Info("Validating updated ingress")
	ingresses.ValidateIngress(i.T(), steveClient, ingress, host, path2, project.ClusterID)
}

func (i *IngressTestSuite) TestIngressEditAddMoreRules() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()

	steveClient, project, namespace, err := setupTest(i.client, i.cluster.ID, "ingress add rules")
	require.NoError(i.T(), err)

	log.Info("Creating workload")
	workload, err := createWorkload(i.client, i.cluster.ID, namespace.Name, 1)
	require.NoError(i.T(), err)

	i.workload = &v1.SteveAPIObject{
		Resource: normantypes.Resource{
			ID:   workload.Name,
			Type: DeploymentSteveType,
		},
	}

	log.Info("Creating service")
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.Name + "-svc",
			Namespace: namespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: workload.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{{
				Port:       ingresses.IngressTestPort,
				TargetPort: intstr.FromInt(ingresses.IngressTestPort),
			}},
		},
	}
	createdService, err := services.CreateService(steveClient, *service)
	require.NoError(i.T(), err)

	host1 := generateUniqueHost(i.T(), "add-rules-1")
	host2 := generateUniqueHost(i.T(), "add-rules-2")
	path := "/name.html"
	ingressName := namegen.AppendRandomString("test-ingress")

	initialPath := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, path, createdService.Name, ingresses.IngressTestPort)
	ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, host1, []networkingv1.HTTPIngressPath{initialPath})

	log.Info("Creating initial ingress")
	ingress, err := extensionsingress.CreateIngress(steveClient, ingressName, ingressTemplate)
	require.NoError(i.T(), err)

	log.Info("Waiting for ingress to be active")
	ingresses.WaitForIngressToBeActive(i.client, project.ClusterID, namespace.Name, ingress.Name)

	log.Info("Validating initial ingress")
	ingresses.ValidateIngress(i.T(), steveClient, ingress, host1, path, project.ClusterID)

	log.Info("Adding additional rule to ingress")

	additionalPath := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, path, createdService.Name, ingresses.IngressTestPort)

	updatedTemplate := ingressTemplate.DeepCopy()
	updatedTemplate.Spec.Rules = append(updatedTemplate.Spec.Rules, networkingv1.IngressRule{
		Host: host2,
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{additionalPath},
			},
		},
	})

	updatedIngress, err := ingresses.UpdateIngress(steveClient, ingress, updatedTemplate)
	require.NoError(i.T(), err)

	log.Info("Waiting for updated ingress to be active")
	ingresses.WaitForIngressToBeActive(i.client, project.ClusterID, namespace.Name, updatedIngress.Name)

	log.Info("Validating multiple rules")
	i.validateMultipleRules(updatedIngress, []string{host1, host2}, path)
}

func (i *IngressTestSuite) TestIngressScaleUpTarget() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()

	steveClient, project, namespace, err := setupTest(i.client, i.cluster.ID, "ingress scale up")
	require.NoError(i.T(), err)

	log.Info("Creating initial workload")
	workload, err := createWorkload(i.client, i.cluster.ID, namespace.Name, 1)
	require.NoError(i.T(), err)

	i.workload = &v1.SteveAPIObject{
		Resource: normantypes.Resource{
			ID:   workload.Name,
			Type: DeploymentSteveType,
		},
	}

	log.Info("Creating service")
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.Name + "-svc",
			Namespace: namespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: workload.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{{
				Port:       ingresses.IngressTestPort,
				TargetPort: intstr.FromInt(ingresses.IngressTestPort),
			}},
		},
	}
	createdService, err := services.CreateService(steveClient, *service)
	require.NoError(i.T(), err)

	host := generateUniqueHost(i.T(), "scale-up")
	path := "/"
	ingressName := namegen.AppendRandomString("test-ingress")

	ingressPath := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, path, createdService.Name, ingresses.IngressTestPort)
	ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, host, []networkingv1.HTTPIngressPath{ingressPath})

	log.Info("Creating ingress")
	ingress, err := extensionsingress.CreateIngress(steveClient, ingressName, ingressTemplate)
	require.NoError(i.T(), err)

	log.Info("Waiting for ingress to be active")
	ingresses.WaitForIngressToBeActive(i.client, project.ClusterID, namespace.Name, ingress.Name)

	log.Info("Validating initial ingress")
	ingresses.ValidateIngress(i.T(), steveClient, ingress, host, path, project.ClusterID)

	log.Info("Scaling up workload")
	updatedWorkload, err := updateDeploymentWorkload(i.client, project.ClusterID, namespace.Name, workload.Name, 4, ingresses.IngressTestImage)
	require.NoError(i.T(), err)

	log.Info("Waiting for scaled workload to be ready")
	err = pods.WaitForReadyPods(i.client, project.ClusterID, namespace.Name, updatedWorkload.Name, 4)
	require.NoError(i.T(), err)

	log.Info("Validating ingress after scale up")
	ingresses.ValidateIngress(i.T(), steveClient, ingress, host, path, project.ClusterID)
}

func (i *IngressTestSuite) validateMultipleRules(ingress *v1.SteveAPIObject, hosts []string, path string) {
	var k8sIngress networkingv1.Ingress
	err := v1.ConvertToK8sType(ingress, &k8sIngress)
	require.NoError(i.T(), err)

	require.Equal(i.T(), len(hosts), len(k8sIngress.Spec.Rules), "Number of rules should match the number of hosts")

	for idx, host := range hosts {
		rule := k8sIngress.Spec.Rules[idx]
		require.Equal(i.T(), host, rule.Host, "Host should match for rule %d", idx)
		require.NotNil(i.T(), rule.HTTP, "HTTP rule should not be nil for rule %d", idx)
		require.Len(i.T(), rule.HTTP.Paths, 1, "There should be one path for rule %d", idx)
		require.Equal(i.T(), path, rule.HTTP.Paths[0].Path, "Path should match for rule %d", idx)
	}

}

func (i *IngressTestSuite) TestIngressUpgradeTarget() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()

	steveClient, project, namespace, err := setupTest(i.client, i.cluster.ID, "ingress upgrade")
	require.NoError(i.T(), err)

	log.Info("Creating initial workload")
	workload, err := createWorkload(i.client, i.cluster.ID, namespace.Name, 1)
	require.NoError(i.T(), err)

	i.workload = &v1.SteveAPIObject{
		Resource: normantypes.Resource{
			ID:   workload.Name,
			Type: DeploymentSteveType,
		},
	}

	log.Info("Creating service")
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.Name + "-svc",
			Namespace: namespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: workload.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{{
				Port:       ingresses.IngressTestPort,
				TargetPort: intstr.FromInt(ingresses.IngressTestPort),
			}},
		},
	}
	createdService, err := services.CreateService(steveClient, *service)
	require.NoError(i.T(), err)

	host := generateUniqueHost(i.T(), "upgrade")
	path := "/"
	ingressName := namegen.AppendRandomString("test-ingress")

	ingressPath := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, path, createdService.Name, ingresses.IngressTestPort)
	ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, host, []networkingv1.HTTPIngressPath{ingressPath})

	log.Info("Creating ingress")
	ingress, err := extensionsingress.CreateIngress(steveClient, ingressName, ingressTemplate)
	require.NoError(i.T(), err)

	log.Info("Waiting for ingress to be active")
	ingresses.WaitForIngressToBeActive(i.client, project.ClusterID, namespace.Name, ingress.Name)

	log.Info("Validating initial ingress")
	ingresses.ValidateIngress(i.T(), steveClient, ingress, host, path, project.ClusterID)

	log.Info("Upgrading workload")
	updatedWorkload, err := updateDeploymentWorkload(i.client, project.ClusterID, namespace.Name, workload.Name, 2, ingresses.IngressTestImage)
	require.NoError(i.T(), err)

	log.Info("Waiting for upgraded workload to be ready")
	err = pods.WaitForReadyPods(i.client, project.ClusterID, namespace.Name, updatedWorkload.Name, 2)
	require.NoError(i.T(), err)

	log.Info("Validating ingress after upgrade")
	ingresses.ValidateIngress(i.T(), steveClient, ingress, host, path, project.ClusterID)
}

func (i *IngressTestSuite) TestIngressRuleWithOnlyPath() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()

	steveClient, project, namespace, err := setupTest(i.client, i.cluster.ID, "ingress only path")

	require.NoError(i.T(), err)

	log.Info("Creating workload")
	workload, err := createWorkload(i.client, i.cluster.ID, namespace.Name, 1)
	require.NoError(i.T(), err)

	i.workload = &v1.SteveAPIObject{
		Resource: normantypes.Resource{
			ID:   workload.Name,
			Type: DeploymentSteveType,
		},
	}

	log.Info("Creating service")
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.Name + "-svc",
			Namespace: namespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: workload.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{{
				Port:       ingresses.IngressTestPort,
				TargetPort: intstr.FromInt(ingresses.IngressTestPort),
			}},
		},
	}
	createdService, err := services.CreateService(steveClient, *service)
	require.NoError(i.T(), err)

	uniquePath := fmt.Sprintf("/test-%s-%s-%s",
		strings.ToLower(strings.ReplaceAll(i.T().Name(), "/", "-")),
		namespace.Name,
		namegen.AppendRandomString(""))

	ingressName := namegen.AppendRandomString("test-ingress")
	uniqueHost := generateUniqueHost(i.T(), "path-only")

	ingressPath := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, uniquePath, createdService.Name, ingresses.IngressTestPort)

	ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, uniqueHost, []networkingv1.HTTPIngressPath{ingressPath})

	ingressTemplate.ObjectMeta.Annotations = map[string]string{
		"nginx.ingress.kubernetes.io/rewrite-target": "/",
	}

	ingress, err := extensionsingress.CreateIngress(steveClient, ingressName, ingressTemplate)
	require.NoError(i.T(), err)

	log.Info("Waiting for ingress to be active")
	ingresses.WaitForIngressToBeActive(i.client, project.ClusterID, namespace.Name, ingress.Name)

	log.Info("Validating ingress with unique path")
	ingresses.ValidateIngress(i.T(), steveClient, ingress, uniqueHost, uniquePath, project.ClusterID)
}

func (i *IngressTestSuite) TestIngressRuleWithOnlyHost() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()

	steveClient, project, namespace, err := setupTest(i.client, i.cluster.ID, "ingress only host")
	require.NoError(i.T(), err)

	log.Info("Creating workload")
	workload, err := createWorkload(i.client, i.cluster.ID, namespace.Name, 1)
	require.NoError(i.T(), err)

	i.workload = &v1.SteveAPIObject{
		Resource: normantypes.Resource{
			ID:   workload.Name,
			Type: DeploymentSteveType,
		},
	}

	log.Info("Creating service")
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.Name + "-svc",
			Namespace: namespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: workload.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{{
				Port:       ingresses.IngressTestPort,
				TargetPort: intstr.FromInt(ingresses.IngressTestPort),
			}},
		},
	}
	createdService, err := services.CreateService(steveClient, *service)
	require.NoError(i.T(), err)

	host := generateUniqueHost(i.T(), "only-host")
	path := "/"
	ingressName := namegen.AppendRandomString("test-ingress")

	ingressPath := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, path, createdService.Name, ingresses.IngressTestPort)

	ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, host, []networkingv1.HTTPIngressPath{ingressPath})

	log.Info("Creating ingress with host")
	ingress, err := extensionsingress.CreateIngress(steveClient, ingressName, ingressTemplate)
	require.NoError(i.T(), err)

	log.Info("Waiting for ingress to be active")
	ingresses.WaitForIngressToBeActive(i.client, project.ClusterID, namespace.Name, ingress.Name)

	log.Info("Validating ingress with host")
	ingresses.ValidateIngress(i.T(), steveClient, ingress, host, path, project.ClusterID)
}

func (i *IngressTestSuite) TestIngressIPDomain() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()

	steveClient, project, namespace, err := setupTest(i.client, i.cluster.ID, "ingress IP domain")
	require.NoError(i.T(), err)

	log.Info("Creating workload")
	workload, err := createWorkload(i.client, i.cluster.ID, namespace.Name, 1)
	require.NoError(i.T(), err)

	i.workload = &v1.SteveAPIObject{
		Resource: normantypes.Resource{
			ID:   workload.Name,
			Type: DeploymentSteveType,
		},
	}

	log.Info("Creating service")
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.Name + "-svc",
			Namespace: namespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: workload.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{{
				Port:       ingresses.IngressTestPort,
				TargetPort: intstr.FromInt(ingresses.IngressTestPort),
			}},
		},
	}
	createdService, err := services.CreateService(steveClient, *service)
	require.NoError(i.T(), err)

	ingressName := namegen.AppendRandomString("test-ingress")
	expectedDomain := getIngressIPDomain()
	host := fmt.Sprintf("%s.%s.%s", ingressName, namespace.Name, expectedDomain)
	path := "/name.html"

	log.Infof("Creating ingress with host: %s", host)
	ingressPath := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, path, createdService.Name, ingresses.IngressTestPort)
	ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, host, []networkingv1.HTTPIngressPath{ingressPath})

	ingress, err := extensionsingress.CreateIngress(steveClient, ingressName, ingressTemplate)
	require.NoError(i.T(), err)

	log.Info("Waiting for ingress to be active")
	ingresses.WaitForIngressToBeActive(i.client, project.ClusterID, namespace.Name, ingress.Name)

	log.Info("Validating ingress")
	var k8sIngress networkingv1.Ingress
	err = v1.ConvertToK8sType(ingress, &k8sIngress)
	require.NoError(i.T(), err)

	validateIngressIPDomain(i.T(), &k8sIngress)
	require.Equal(i.T(), path, k8sIngress.Spec.Rules[0].HTTP.Paths[0].Path)
}

func (i *IngressTestSuite) TestIngressRulesSameHostPortPath() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()

	steveClient, project, namespace, err := setupTest(i.client, i.cluster.ID, "ingress same host port path")
	require.NoError(i.T(), err)

	log.Info("Creating first workload")
	workload1, err := createWorkload(i.client, i.cluster.ID, namespace.Name, 1)
	require.NoError(i.T(), err)

	i.workload = &v1.SteveAPIObject{
		Resource: normantypes.Resource{
			ID:   workload1.Name,
			Type: DeploymentSteveType,
		},
	}

	log.Info("Creating first service")
	service1 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload1.Name + "-svc",
			Namespace: namespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: workload1.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{{
				Port:       ingresses.IngressTestPort,
				TargetPort: intstr.FromInt(ingresses.IngressTestPort),
			}},
		},
	}
	createdService1, err := services.CreateService(steveClient, *service1)
	require.NoError(i.T(), err)

	log.Info("Creating second workload")
	workload2, err := createWorkload(i.client, i.cluster.ID, namespace.Name, 1)
	require.NoError(i.T(), err)

	log.Info("Creating second service")
	service2 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload2.Name + "-svc",
			Namespace: namespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: workload2.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{{
				Port:       ingresses.IngressTestPort,
				TargetPort: intstr.FromInt(ingresses.IngressTestPort),
			}},
		},
	}
	createdService2, err := services.CreateService(steveClient, *service2)
	require.NoError(i.T(), err)

	host := generateUniqueHost(i.T(), "same-host")
	path := "/"
	ingressName := namegen.AppendRandomString("test-ingress")

	path1 := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, path, createdService1.Name, ingresses.IngressTestPort)
	path2 := extensionsingress.NewIngressPathTemplate(networkingv1.PathTypePrefix, path, createdService2.Name, ingresses.IngressTestPort)

	log.Info("Creating ingress wtemplates with both ports")
	ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, host, []networkingv1.HTTPIngressPath{path1, path2})

	log.Info("Creating ingress with same host, port and path")
	ingress, err := extensionsingress.CreateIngress(steveClient, ingressName, ingressTemplate)
	require.NoError(i.T(), err)

	log.Info("Waiting for ingress to be active")
	ingresses.WaitForIngressToBeActive(i.client, project.ClusterID, namespace.Name, ingress.Name)

	log.Info("Verifying ingress rules")
	ingressObj, err := steveClient.SteveType("networking.k8s.io.ingress").ByID(fmt.Sprintf("%s/%s", namespace.Name, ingress.Name))
	require.NoError(i.T(), err)

	var k8sIngress networkingv1.Ingress
	err = v1.ConvertToK8sType(ingressObj, &k8sIngress)
	require.NoError(i.T(), err)

	require.Len(i.T(), k8sIngress.Spec.Rules, 1)
	require.Equal(i.T(), host, k8sIngress.Spec.Rules[0].Host)
	require.Len(i.T(), k8sIngress.Spec.Rules[0].HTTP.Paths, 2)
}

func TestIngressTestSuite(t *testing.T) {
	suite.Run(t, new(IngressTestSuite))
}

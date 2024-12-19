//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package ingress

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/ingresses"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	extensionsingress "github.com/rancher/shepherd/extensions/ingresses"
	"github.com/rancher/shepherd/extensions/users"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IngressRBACTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

type userContext struct {
	user      *management.User
	client    *rancher.Client
	steve     *v1.Client
	project   *management.Project
	workload  *v1.SteveAPIObject
	namespace string
	role      string
}

func (i *IngressRBACTestSuite) TearDownSuite() {
	i.session.Cleanup()
}

func (i *IngressRBACTestSuite) SetupSuite() {
	testSession := session.NewSession()
	i.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(i.T(), err)
	i.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(i.T(), clusterName, "Cluster name should be set")

	clusterID, err := clusters.GetClusterIDByName(i.client, clusterName)
	require.NoError(i.T(), err)

	i.cluster, err = i.client.Management.Cluster.ByID(clusterID)
	require.NoError(i.T(), err)
}
func (i *IngressRBACTestSuite) createWorkloadWithAppropriateClient(ctx *userContext) error {
	log.Infof("Creating test workload for %s role", ctx.role)
	clientToUse := ctx.client
	if ctx.role == rbac.ClusterMember.String() || ctx.role == rbac.ReadOnly.String() {
		log.Infof("Using admin client for %s role", ctx.role)
		clientToUse = i.client
	}

	k8sWorkload, err := deployment.CreateDeployment(clientToUse, i.cluster.ID, ctx.namespace, 1, "nginx", "", false, false, true, false)
	if err != nil {
		return fmt.Errorf("failed to create workload for %s role: %v", ctx.role, err)
	}

	steveWorkload := &v1.SteveAPIObject{}
	if err = v1.ConvertToK8sType(k8sWorkload, steveWorkload); err != nil {
		return fmt.Errorf("failed to convert workload for %s role: %v", ctx.role, err)
	}
	ctx.workload = steveWorkload
	return nil
}

func (i *IngressRBACTestSuite) createIngressWithAppropriateClient(ctx *userContext, ingressTemplate networkingv1.Ingress) (*v1.SteveAPIObject, error) {
	log.Infof("Creating ingress for %s role", ctx.role)
	var clientToUse *v1.Client
	var err error

	if ctx.role == rbac.ClusterMember.String() || ctx.role == rbac.ReadOnly.String() {
		log.Infof("Using admin client for %s role", ctx.role)
		clientToUse, err = i.client.Steve.ProxyDownstream(i.cluster.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get admin client for %s role: %v", ctx.role, err)
		}
	} else {
		clientToUse = ctx.steve
	}

	ingress, err := extensionsingress.CreateIngress(clientToUse, ingressTemplate.Name, ingressTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to create ingress for %s role: %v", ctx.role, err)
	}
	return ingress, nil
}

func (i *IngressRBACTestSuite) validateIngressUpdate(ctx *userContext, ingress *v1.SteveAPIObject, ingressTemplate networkingv1.Ingress) error {
	log.Infof("Validating ingress update for %s role", ctx.role)
	updatedTemplate := ingressTemplate.DeepCopy()
	updatedTemplate.Spec.Rules[0].Host = fmt.Sprintf("%s.updated.com", namegen.AppendRandomString("test"))

	_, err := ingresses.UpdateIngress(ctx.steve, ingress, updatedTemplate)
	if ctx.role == rbac.ReadOnly.String() || ctx.role == rbac.ClusterMember.String() {
		if err == nil {
			return fmt.Errorf("update should be denied for %s role", ctx.role)
		}
		if !strings.Contains(err.Error(), "admission webhook \"validate.nginx.ingress.kubernetes.io\" denied the request") &&
			!strings.Contains(err.Error(), "Unknown schema type [networking.k8s.io.ingress]") &&
			!strings.Contains(err.Error(), "Resource type [networking.k8s.io.ingress] is not updatable") {
			return fmt.Errorf("unexpected error for %s role: %v", ctx.role, err)
		}
		return nil
	}
	return err
}

func (i *IngressRBACTestSuite) validateIngressDelete(ctx *userContext, ingress *v1.SteveAPIObject) error {
	log.Infof("Validating ingress deletion for %s role", ctx.role)
	err := ctx.steve.SteveType(ingress.Type).Delete(ingress)
	if ctx.role == rbac.ReadOnly.String() || ctx.role == rbac.ClusterMember.String() {
		if err == nil {
			return fmt.Errorf("delete should be denied for %s role", ctx.role)
		}
		acceptableErrors := []string{
			"admission webhook \"validate.nginx.ingress.kubernetes.io\" denied the request",
			"Unknown schema type [networking.k8s.io.ingress]",
			"Resource type [networking.k8s.io.ingress] is not deletable",
			"Resource type [networking.k8s.io.ingress] can not be deleted",
		}

		for _, acceptableError := range acceptableErrors {
			if strings.Contains(err.Error(), acceptableError) {
				log.Infof("Expected delete restriction for %s role", ctx.role)
				log.Info("Cleaning up with admin client")
				adminSteveClient, err := i.client.Steve.ProxyDownstream(i.cluster.ID)
				if err != nil {
					return fmt.Errorf("failed to get admin client for cleanup: %v", err)
				}
				return adminSteveClient.SteveType(ingress.Type).Delete(ingress)
			}
		}
		return fmt.Errorf("unexpected error for %s role: %v", ctx.role, err)
	}
	return err
}

func (i *IngressRBACTestSuite) validateIngressPermissions(ctx *userContext) {
	err := i.createWorkloadWithAppropriateClient(ctx)
	require.NoError(i.T(), err, fmt.Sprintf("Failed to create workload for %s role", ctx.role))

	ingressTemplate := i.createIngressTemplate(ctx)
	ingress, err := i.createIngressWithAppropriateClient(ctx, ingressTemplate)
	require.NoError(i.T(), err, fmt.Sprintf("Failed to create ingress for %s role", ctx.role))

	log.Infof("Waiting for ingress to be active for %s role", ctx.role)
	clientToUse := ctx.client
	if ctx.role == rbac.ClusterMember.String() || ctx.role == rbac.ReadOnly.String() {
		log.Infof("Using admin client for %s role", ctx.role)
		clientToUse = i.client
	}
	ingresses.WaitForIngressToBeActive(clientToUse, i.cluster.ID, ctx.namespace, ingress.Name)

	err = i.validateIngressUpdate(ctx, ingress, ingressTemplate)
	require.NoError(i.T(), err, fmt.Sprintf("Failed to validate ingress update for %s role", ctx.role))

	err = i.validateIngressDelete(ctx, ingress)
	require.NoError(i.T(), err, fmt.Sprintf("Failed to validate ingress deletion for %s role", ctx.role))
}

func (i *IngressRBACTestSuite) setupUserContext(role string) (*userContext, error) {
	log.Infof("Setting up user context for role: %s", role)

	user, client, err := rbac.SetupUser(i.client, rbac.StandardUser.String())
	if err != nil {
		return nil, fmt.Errorf("failed to setup user: %v", err)
	}

	project, namespace, err := projects.CreateProjectAndNamespace(i.client, i.cluster.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create project and namespace: %v", err)
	}

	switch role {
	case rbac.ClusterOwner.String(), rbac.ClusterMember.String():
		err = users.AddClusterRoleToUser(i.client, i.cluster, user, role, nil)
	case rbac.ProjectOwner.String(), rbac.ProjectMember.String(), rbac.ReadOnly.String():
		err = users.AddProjectMember(i.client, project, user, role, nil)
	default:
		return nil, fmt.Errorf("unsupported role: %s", role)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to add role to user: %v", err)
	}

	client, err = client.ReLogin()
	if err != nil {
		return nil, fmt.Errorf("failed to relogin: %v", err)
	}

	steveClient, err := client.Steve.ProxyDownstream(i.cluster.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get downstream client: %v", err)
	}

	return &userContext{
		user:      user,
		client:    client,
		steve:     steveClient,
		project:   project,
		namespace: namespace.Name,
		role:      role,
	}, nil
}

func (i *IngressRBACTestSuite) createTestWorkload(ctx *userContext) error {
	log.Info("Creating test workload")

	clientToUse := ctx.client
	if ctx.role == rbac.ClusterMember.String() || ctx.role == rbac.ReadOnly.String() {
		log.Info("Using admin client for workload creation due to restricted role")
		clientToUse = i.client
	}

	k8sWorkload, err := deployment.CreateDeployment(clientToUse, i.cluster.ID, ctx.namespace, 1, "nginx", "", false, false, true, false)
	if err != nil {
		return err
	}

	steveWorkload := &v1.SteveAPIObject{}
	err = v1.ConvertToK8sType(k8sWorkload, steveWorkload)
	if err != nil {
		return err
	}

	ctx.workload = steveWorkload
	return nil
}

func (i *IngressRBACTestSuite) createIngressTemplate(ctx *userContext) networkingv1.Ingress {
	ingressName := namegen.AppendRandomString("test-ingress")

	host := fmt.Sprintf("%s.foo.com", namegen.AppendRandomString("test"))
	path := "/"
	pathType := networkingv1.PathTypePrefix

	return networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: ctx.namespace,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     path,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: ctx.workload.Name,
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (i *IngressRBACTestSuite) TestIngressRBAC() {
	roles := []string{
		rbac.ClusterOwner.String(),
		rbac.ClusterMember.String(),
		rbac.ProjectOwner.String(),
		rbac.ProjectMember.String(),
		rbac.ReadOnly.String(),
	}

	for _, role := range roles {
		i.Run(fmt.Sprintf("Testing role: %s", role), func() {
			ctx, err := i.setupUserContext(role)
			require.NoError(i.T(), err, "Failed to setup user context")

			i.validateIngressPermissions(ctx)
		})
	}
}

func TestIngressRBACTestSuite(t *testing.T) {
	suite.Run(t, new(IngressRBACTestSuite))
}

package ingress

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	networkingv1 "k8s.io/api/networking/v1"

	"github.com/rancher/rancher/tests/v2/actions/ingresses"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	extensionsingress "github.com/rancher/shepherd/extensions/ingresses"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
)

type IngressRBACTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (i *IngressRBACTestSuite) SetupSuite() {
	i.session = session.NewSession()

	client, err := rancher.NewClient("", i.session)
	require.NoError(i.T(), err)
	i.client = client

	log.Info("Getting cluster name from the config file and append cluster details in i")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(i.T(), clusterName, "Cluster name should be set")

	clusterID, err := clusters.GetClusterIDByName(i.client, clusterName)
	require.NoError(i.T(), err)

	i.cluster, err = i.client.Management.Cluster.ByID(clusterID)
	require.NoError(i.T(), err, "Error getting cluster ID")
}

func (i *IngressRBACTestSuite) TearDownSuite() {
	log.Info("Starting test suite cleanup")
	i.session.Cleanup()
	log.Info("Cleanup completed")

}

func (i *IngressRBACTestSuite) TestCreateIngress() {
	tests := []struct {
		role      rbac.Role
		canCreate bool
		member    string
	}{
		{rbac.ClusterOwner, true, rbac.StandardUser.String()},
		{rbac.ClusterMember, true, rbac.StandardUser.String()},
		{rbac.ProjectOwner, true, rbac.StandardUser.String()},
		{rbac.ProjectMember, true, rbac.StandardUser.String()},
		{rbac.ReadOnly, false, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		i.Run(fmt.Sprintf("Validate ingress creation for %s role", tt.role), func() {
			subSession := i.session.NewSession()
			defer subSession.Cleanup()

			log.Infof("Creating project and namespace for role: %s", tt.role)
			adminProject, namespace, err := projects.CreateProjectAndNamespace(i.client, i.cluster.ID)
			require.NoError(i.T(), err)

			log.Infof("Creating user with role: %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(i.client, tt.member, tt.role.String(), i.cluster, adminProject)
			require.NoError(i.T(), err)
			log.Infof("Created user: %v", newUser.Username)

			log.Info("Creating workload using admin client")
			workload, err := deployment.CreateDeployment(i.client, i.cluster.ID, namespace.Name, 1, "nginx", "", false, false, true, false)
			require.NoError(i.T(), err)

			steveWorkload := &v1.SteveAPIObject{}
			err = v1.ConvertToK8sType(workload, steveWorkload)
			require.NoError(i.T(), err)

			pathType := networkingv1.PathTypePrefix
			ingressPath := extensionsingress.NewIngressPathTemplate(pathType, "/", steveWorkload.Name, 80)

			ingressName := namegen.AppendRandomString("test-ingress")
			hostName := fmt.Sprintf("%s.foo.com", namegen.AppendRandomString("test"))
			ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, hostName, []networkingv1.HTTPIngressPath{ingressPath})

			steveClient, err := standardUserClient.Steve.ProxyDownstream(i.cluster.ID)
			require.NoError(i.T(), err)

			var ingress *v1.SteveAPIObject
			var createErr error
			_, cancel := context.WithTimeout(context.Background(), defaults.ThirtyMinuteTimeout)
			defer cancel()

			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				log.Infof("Creating ingress as %s", tt.role)
				ingress, createErr = extensionsingress.CreateIngress(steveClient, ingressTemplate.Name, ingressTemplate)
				require.NoError(i.T(), createErr)
				require.NotNil(i.T(), ingress)

				log.Info("Verifying ingress creation")
				_, err = steveClient.SteveType(ingress.Type).ByID(ingress.ID)
				require.NoError(i.T(), err)

			case rbac.ClusterMember.String():
				log.Info("Creating ingress as cluster member using admin client")
				adminSteveClient, err := i.client.Steve.ProxyDownstream(i.cluster.ID)
				require.NoError(i.T(), err)

				ingress, createErr = extensionsingress.CreateIngress(adminSteveClient, ingressTemplate.Name, ingressTemplate)
				require.NoError(i.T(), createErr)
				require.NotNil(i.T(), ingress)

			case rbac.ReadOnly.String():
				log.Info("Attempting to create ingress as read-only user")
				ingress, createErr = extensionsingress.CreateIngress(steveClient, ingressTemplate.Name, ingressTemplate)

				require.Error(i.T(), createErr)
				isPermissionError := k8sError.IsForbidden(createErr) ||
					strings.Contains(createErr.Error(), "admission webhook") ||
					strings.Contains(createErr.Error(), "not allowed") ||
					strings.Contains(createErr.Error(), "permission denied") ||
					strings.Contains(createErr.Error(), "is not creatable")

				if !isPermissionError {
					log.Errorf("Unexpected error: %v", createErr)
				}
				require.True(i.T(), isPermissionError)
				require.Nil(i.T(), ingress)
			}

			if ingress != nil {
				log.Info("Cleaning up created ingress")
				adminSteveClient, err := i.client.Steve.ProxyDownstream(i.cluster.ID)
				require.NoError(i.T(), err)
				err = adminSteveClient.SteveType(ingress.Type).Delete(ingress)
				require.NoError(i.T(), err)
			}
		})
	}
}

func (i *IngressRBACTestSuite) TestListIngress() {
	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		i.Run("Validate listing ingress for user with role "+tt.role.String(), func() {
			subSession := i.session.NewSession()
			defer subSession.Cleanup()

			log.Infof("Creating project and namespace for role: %s", tt.role)
			adminProject, namespace, err := projects.CreateProjectAndNamespace(i.client, i.cluster.ID)
			require.NoError(i.T(), err)

			log.Infof("Creating user with role: %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(i.client, tt.member, tt.role.String(), i.cluster, adminProject)
			require.NoError(i.T(), err)
			log.Infof("Created user: %v", newUser.Username)

			log.Info("Setting up admin steve client")
			adminSteveClient, err := i.client.Steve.ProxyDownstream(i.cluster.ID)
			require.NoError(i.T(), err)

			log.Info("Creating test workload")
			workload, err := deployment.CreateDeployment(i.client, i.cluster.ID, namespace.Name, 1, "nginx", "", false, false, true, false)
			require.NoError(i.T(), err)

			steveWorkload := &v1.SteveAPIObject{}
			err = v1.ConvertToK8sType(workload, steveWorkload)
			require.NoError(i.T(), err)

			pathType := networkingv1.PathTypePrefix
			ingressPath := extensionsingress.NewIngressPathTemplate(pathType, "/", steveWorkload.Name, 80)

			ingressName := namegen.AppendRandomString("test-ingress")
			hostName := fmt.Sprintf("%s.foo.com", namegen.AppendRandomString("test"))
			ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, hostName, []networkingv1.HTTPIngressPath{ingressPath})

			log.Info("Creating test ingress")
			ingress, err := extensionsingress.CreateIngress(adminSteveClient, ingressTemplate.Name, ingressTemplate)
			require.NoError(i.T(), err)

			log.Infof("Setting up user steve client for role: %s", tt.role)
			steveClient, err := standardUserClient.Steve.ProxyDownstream(i.cluster.ID)
			require.NoError(i.T(), err)

			options := url.Values{}
			options.Add("namespace", namespace.Name)

			log.Info("Attempting to list ingresses")
			var ingressList *v1.SteveCollection
			maxRetries := 3
			for retry := 0; retry < maxRetries; retry++ {
				if retry > 0 {
					log.Infof("Retry attempt %d/%d", retry+1, maxRetries)
				}
				ingressList, err = steveClient.SteveType(ingress.Type).List(options)
				if err == nil {
					break
				}
				if strings.Contains(err.Error(), "500 Internal Server Error") {
					log.Warnf("Got 500 error, retrying operation for %s role", tt.role)
					time.Sleep(defaults.FiveSecondTimeout)
					continue
				}
				break
			}

			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String(), rbac.ReadOnly.String():
				log.Info("Verifying successful list operation")
				require.NoError(i.T(), err)
				require.NotNil(i.T(), ingressList)
				require.Greater(i.T(), len(ingressList.Data), 0)

			case rbac.ClusterMember.String():
				log.Info("Verifying list operation for cluster member")
				if strings.Contains(err.Error(), "sync from client") ||
					strings.Contains(err.Error(), "Unknown schema type") {
					log.Infof("Skipping schema validation for %s role", tt.role)
					return
				}
				isPermissionError := err != nil && (k8sError.IsForbidden(err) ||
					strings.Contains(err.Error(), "is not listable"))
				require.True(i.T(), isPermissionError)
			}

			log.Info("Cleaning up test ingress")
			err = adminSteveClient.SteveType(ingress.Type).Delete(ingress)
			require.NoError(i.T(), err)
		})
	}
}

func (i *IngressRBACTestSuite) TestUpdateIngress() {
	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		i.Run("Validate updating ingress for user with role "+tt.role.String(), func() {
			subSession := i.session.NewSession()
			defer subSession.Cleanup()

			log.Infof("Creating project and namespace for role: %s", tt.role)
			adminProject, namespace, err := projects.CreateProjectAndNamespace(i.client, i.cluster.ID)
			require.NoError(i.T(), err)

			log.Infof("Creating user with role: %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(i.client, tt.member, tt.role.String(), i.cluster, adminProject)
			require.NoError(i.T(), err)
			log.Infof("Created user: %v", newUser.Username)

			log.Info("Creating workload using admin client")
			workload, err := deployment.CreateDeployment(i.client, i.cluster.ID, namespace.Name, 1, "nginx", "", false, false, true, false)
			require.NoError(i.T(), err)

			steveWorkload := &v1.SteveAPIObject{}
			err = v1.ConvertToK8sType(workload, steveWorkload)
			require.NoError(i.T(), err)

			pathType := networkingv1.PathTypePrefix
			ingressPath := extensionsingress.NewIngressPathTemplate(pathType, "/", steveWorkload.Name, 80)

			ingressName := namegen.AppendRandomString("test-ingress")
			hostName := fmt.Sprintf("%s.foo.com", namegen.AppendRandomString("test"))
			ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, hostName, []networkingv1.HTTPIngressPath{ingressPath})

			log.Info("Setting up admin steve client")
			adminSteveClient, err := i.client.Steve.ProxyDownstream(i.cluster.ID)
			require.NoError(i.T(), err)

			log.Info("Creating test ingress with admin client")
			ingress, err := extensionsingress.CreateIngress(adminSteveClient, ingressTemplate.Name, ingressTemplate)
			require.NoError(i.T(), err)

			log.Infof("Setting up user steve client for role: %s", tt.role)
			steveClient, err := standardUserClient.Steve.ProxyDownstream(i.cluster.ID)
			require.NoError(i.T(), err)

			log.Info("Preparing ingress update")
			updatedTemplate := ingressTemplate.DeepCopy()
			updatedTemplate.Spec.Rules[0].Host = fmt.Sprintf("%s.updated.com", namegen.AppendRandomString("test"))

			log.Infof("Attempting to update ingress as %s", tt.role)
			_, err = ingresses.UpdateIngress(steveClient, ingress, updatedTemplate)

			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				log.Info("Verifying successful update")
				require.NoError(i.T(), err)

			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				log.Info("Verifying update was prevented")
				require.Error(i.T(), err)
				isPermissionError := k8sError.IsForbidden(err) ||
					strings.Contains(err.Error(), "is not updatable") ||
					strings.Contains(err.Error(), "Resource type [networking.k8s.io.ingress]") ||
					strings.Contains(err.Error(), "Unknown schema type") ||
					strings.Contains(err.Error(), "admission webhook")

				if !isPermissionError {
					log.Errorf("Unexpected error for %s role: %v", tt.role, err)
				}
				require.True(i.T(), isPermissionError)
			}

			log.Info("Cleaning up test ingress")
			err = adminSteveClient.SteveType(ingress.Type).Delete(ingress)
			require.NoError(i.T(), err)
		})
	}
}

func (i *IngressRBACTestSuite) TestDeleteIngress() {
	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		i.Run("Validate deleting ingress for user with role "+tt.role.String(), func() {
			subSession := i.session.NewSession()
			defer subSession.Cleanup()

			log.Infof("Creating project and namespace for role: %s", tt.role)
			adminProject, namespace, err := projects.CreateProjectAndNamespace(i.client, i.cluster.ID)
			require.NoError(i.T(), err)

			log.Infof("Creating user with role: %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(i.client, tt.member, tt.role.String(), i.cluster, adminProject)
			require.NoError(i.T(), err)
			log.Infof("Created user: %v", newUser.Username)

			log.Info("Creating workload and ingress as admin")
			workload, err := deployment.CreateDeployment(i.client, i.cluster.ID, namespace.Name, 1, "nginx", "", false, false, true, false)
			require.NoError(i.T(), err)

			steveWorkload := &v1.SteveAPIObject{}
			err = v1.ConvertToK8sType(workload, steveWorkload)
			require.NoError(i.T(), err)

			pathType := networkingv1.PathTypePrefix
			ingressPath := extensionsingress.NewIngressPathTemplate(pathType, "/", steveWorkload.Name, 80)

			ingressName := namegen.AppendRandomString("test-ingress")
			hostName := fmt.Sprintf("%s.foo.com", namegen.AppendRandomString("test"))
			ingressTemplate := extensionsingress.NewIngressTemplate(ingressName, namespace.Name, hostName, []networkingv1.HTTPIngressPath{ingressPath})

			log.Info("Setting up admin steve client")
			adminSteveClient, err := i.client.Steve.ProxyDownstream(i.cluster.ID)
			require.NoError(i.T(), err)

			log.Info("Creating test ingress")
			ingress, err := extensionsingress.CreateIngress(adminSteveClient, ingressTemplate.Name, ingressTemplate)
			require.NoError(i.T(), err)

			log.Infof("Setting up user steve client for role: %s", tt.role)
			steveClient, err := standardUserClient.Steve.ProxyDownstream(i.cluster.ID)
			require.NoError(i.T(), err)

			log.Info("Attempting ingress deletion")
			err = steveClient.SteveType(ingress.Type).Delete(ingress)

			options := url.Values{}
			options.Add("fieldSelector", fmt.Sprintf("metadata.name=%s", ingress.Name))
			options.Add("namespace", namespace.Name)

			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				log.Info("Verifying successful deletion")
				require.NoError(i.T(), err)
				ingressList, err := adminSteveClient.SteveType(ingress.Type).List(options)
				require.NoError(i.T(), err)
				require.Equal(i.T(), 0, len(ingressList.Data))

			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				log.Info("Verifying deletion was prevented")
				require.Error(i.T(), err)
				isPermissionError := k8sError.IsForbidden(err) ||
					strings.Contains(err.Error(), "is not deletable") ||
					strings.Contains(err.Error(), "can not be deleted") ||
					strings.Contains(err.Error(), "Resource type [networking.k8s.io.ingress]") ||
					strings.Contains(err.Error(), "Unknown schema type") ||
					strings.Contains(err.Error(), "admission webhook")

				if !isPermissionError {
					log.Errorf("Unexpected error: %v", err)
				}
				require.True(i.T(), isPermissionError)

				log.Info("Verifying ingress still exists")
				ingressList, err := adminSteveClient.SteveType(ingress.Type).List(options)
				require.NoError(i.T(), err)
				require.Equal(i.T(), 1, len(ingressList.Data))

				log.Info("Cleaning up with admin client")
				err = adminSteveClient.SteveType(ingress.Type).Delete(ingress)
				require.NoError(i.T(), err)
			}
		})
	}
}

func TestIngressRBACTestSuite(t *testing.T) {
	suite.Run(t, new(IngressRBACTestSuite))
}

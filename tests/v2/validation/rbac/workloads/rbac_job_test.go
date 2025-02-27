//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package workloads

import (
	"context"
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/rancher/tests/v2/actions/workloads/job"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

type RbacJobTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (rj *RbacJobTestSuite) TearDownSuite() {
	rj.session.Cleanup()
}

func (rj *RbacJobTestSuite) SetupSuite() {
	rj.session = session.NewSession()

	client, err := rancher.NewClient("", rj.session)
	require.NoError(rj.T(), err)
	rj.client = client

	log.Info("Getting cluster name from the config file and append cluster details in rj")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rj.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rj.client, clusterName)
	require.NoError(rj.T(), err, "Error getting cluster ID")
	rj.cluster, err = rj.client.Management.Cluster.ByID(clusterID)
	require.NoError(rj.T(), err)
}

func (rj *RbacJobTestSuite) createPodTemplate() corev1.PodTemplateSpec {
	containerName := namegen.AppendRandomString("testcontainer")
	pullPolicy := corev1.PullAlways

	containerTemplate := workloads.NewContainer(
		containerName,
		rbac.ImageName,
		pullPolicy,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil, nil, nil,
	)

	podTemplate := workloads.NewPodTemplate(
		[]corev1.Container{containerTemplate},
		[]corev1.Volume{},
		[]corev1.LocalObjectReference{},
		nil, nil,
	)

	return podTemplate
}

func (rj *RbacJobTestSuite) TestCreateJob() {
	subSession := rj.session.NewSession()
	defer subSession.Cleanup()

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
		rj.Run("Validate job creation as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rj.client, rj.cluster.ID)
			assert.NoError(rj.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project with role %s", tt.role)
			_, userClient, err := rbac.AddUserWithRoleToCluster(rj.client, tt.member, tt.role.String(), rj.cluster, adminProject)
			assert.NoError(rj.T(), err)

			log.Infof("As a %v, create a job", tt.role.String())
			podTemplate := rj.createPodTemplate()
			_, err = job.CreateJob(userClient, rj.cluster.ID, namespace.Name, podTemplate, false)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rj.T(), err, "failed to create job")
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rj.T(), err)
				assert.True(rj.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rj *RbacJobTestSuite) TestListJob() {
	subSession := rj.session.NewSession()
	defer subSession.Cleanup()

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
		rj.Run("Validate listing job as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rj.client, rj.cluster.ID)
			assert.NoError(rj.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project with role %s", tt.role)
			_, userClient, err := rbac.AddUserWithRoleToCluster(rj.client, tt.member, tt.role.String(), rj.cluster, adminProject)
			assert.NoError(rj.T(), err)

			log.Infof("As a %v, create a job in the namespace %v", rbac.Admin, namespace.Name)
			podTemplate := rj.createPodTemplate()
			createdJob, err := job.CreateJob(rj.client, rj.cluster.ID, namespace.Name, podTemplate, true)
			assert.NoError(rj.T(), err, "failed to create job")

			log.Infof("As a %v, list the job", tt.role.String())
			standardUserContext, err := userClient.WranglerContext.DownStreamClusterWranglerContext(rj.cluster.ID)
			assert.NoError(rj.T(), err)
			jobList, err := standardUserContext.Batch.Job().List(namespace.Name, metav1.ListOptions{})
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String(), rbac.ReadOnly.String():
				assert.NoError(rj.T(), err, "failed to list job")
				assert.Equal(rj.T(), len(jobList.Items), 1)
				assert.Equal(rj.T(), jobList.Items[0].Name, createdJob.Name)
			case rbac.ClusterMember.String():
				assert.Error(rj.T(), err)
				assert.True(rj.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rj *RbacJobTestSuite) TestUpdateJob() {
	subSession := rj.session.NewSession()
	defer subSession.Cleanup()

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
		rj.Run("Validate updating job as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rj.client, rj.cluster.ID)
			assert.NoError(rj.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project with role %s", tt.role)
			_, userClient, err := rbac.AddUserWithRoleToCluster(rj.client, tt.member, tt.role.String(), rj.cluster, adminProject)
			assert.NoError(rj.T(), err)

			log.Infof("As a %v, create a job in the namespace %v", rbac.Admin, namespace.Name)
			podTemplate := rj.createPodTemplate()
			createdJob, err := job.CreateJob(rj.client, rj.cluster.ID, namespace.Name, podTemplate, true)
			assert.NoError(rj.T(), err, "failed to create job")

			log.Infof("As a %v, update the job %s with a new label.", tt.role.String(), createdJob.Name)
			adminContext, err := rj.client.WranglerContext.DownStreamClusterWranglerContext(rj.cluster.ID)
			assert.NoError(rj.T(), err)
			standardUserContext, err := userClient.WranglerContext.DownStreamClusterWranglerContext(rj.cluster.ID)
			assert.NoError(rj.T(), err)

			latestJob, err := adminContext.Batch.Job().Get(namespace.Name, createdJob.Name, metav1.GetOptions{})
			assert.NoError(rj.T(), err, "Failed to list job.")

			if latestJob.Labels == nil {
				latestJob.Labels = make(map[string]string)
			}
			latestJob.Labels["updated"] = "true"

			_, err = standardUserContext.Batch.Job().Update(latestJob)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rj.T(), err, "failed to update job")
				updatedJob, err := standardUserContext.Batch.Job().Get(namespace.Name, createdJob.Name, metav1.GetOptions{})
				assert.NoError(rj.T(), err, "Failed to list the job after updating labels.")
				assert.Equal(rj.T(), "true", updatedJob.Labels["updated"], "job label update failed.")
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rj.T(), err)
				assert.True(rj.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rj *RbacJobTestSuite) TestDeleteJob() {
	subSession := rj.session.NewSession()
	defer subSession.Cleanup()

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
		rj.Run("Validate deleting job as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rj.client, rj.cluster.ID)
			assert.NoError(rj.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project with role %s", tt.role)
			_, userClient, err := rbac.AddUserWithRoleToCluster(rj.client, tt.member, tt.role.String(), rj.cluster, adminProject)
			assert.NoError(rj.T(), err)

			log.Infof("As a %v, create a job in the namespace %v", rbac.Admin, namespace.Name)
			podTemplate := rj.createPodTemplate()
			createdJob, err := job.CreateJob(rj.client, rj.cluster.ID, namespace.Name, podTemplate, true)
			assert.NoError(rj.T(), err, "failed to create job")

			log.Infof("As a %v, delete the job", tt.role.String())
			standardUserContext, err := userClient.WranglerContext.DownStreamClusterWranglerContext(rj.cluster.ID)
			assert.NoError(rj.T(), err)
			err = standardUserContext.Batch.Job().Delete(namespace.Name, createdJob.Name, &metav1.DeleteOptions{})
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rj.T(), err, "failed to delete job")
				err := kwait.PollUntilContextTimeout(context.Background(), defaults.FiveSecondTimeout, defaults.OneMinuteTimeout, false, func(ctx context.Context) (done bool, pollErr error) {
					updatedJobList, pollErr := standardUserContext.Batch.Job().List(namespace.Name, metav1.ListOptions{})
					if pollErr != nil {
						return false, fmt.Errorf("failed to list jobs: %w", pollErr)
					}

					if len(updatedJobList.Items) == 0 {
						return true, nil
					}
					return false, nil
				})
				assert.NoError(rj.T(), err)
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rj.T(), err)
				assert.True(rj.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rj *RbacJobTestSuite) TestCrudJobAsClusterMember() {
	subSession := rj.session.NewSession()
	defer subSession.Cleanup()

	role := rbac.ClusterMember.String()
	log.Infof("Create a standard user.")
	user, userClient, err := rbac.SetupUser(rj.client, rbac.StandardUser.String())
	require.NoError(rj.T(), err)

	log.Infof("Add the user to the downstream cluster with role %s", role)
	err = users.AddClusterRoleToUser(rj.client, rj.cluster, user, role, nil)
	require.NoError(rj.T(), err)

	log.Infof("As a %v, create a project and a namespace in the project.", role)
	_, namespace, err := projects.CreateProjectAndNamespace(userClient, rj.cluster.ID)
	require.NoError(rj.T(), err)

	log.Infof("As a %v, create a job in the namespace %v", role, namespace.Name)
	podTemplate := rj.createPodTemplate()
	createdJob, err := job.CreateJob(userClient, rj.cluster.ID, namespace.Name, podTemplate, true)
	require.NoError(rj.T(), err, "failed to create job")

	log.Infof("As a %v, list the job", role)
	standardUserContext, err := userClient.WranglerContext.DownStreamClusterWranglerContext(rj.cluster.ID)
	assert.NoError(rj.T(), err)
	jobList, err := standardUserContext.Batch.Job().List(namespace.Name, metav1.ListOptions{})
	require.NoError(rj.T(), err, "failed to list jobs")
	require.Equal(rj.T(), len(jobList.Items), 1)
	require.Equal(rj.T(), jobList.Items[0].Name, createdJob.Name)

	log.Infof("As a %v, update the job %s with a new label.", role, createdJob.Name)
	latestJob, err := standardUserContext.Batch.Job().Get(namespace.Name, createdJob.Name, metav1.GetOptions{})
	assert.NoError(rj.T(), err, "Failed to get the latest job.")

	if latestJob.Labels == nil {
		latestJob.Labels = make(map[string]string)
	}
	latestJob.Labels["updated"] = "true"

	_, err = standardUserContext.Batch.Job().Update(latestJob)
	require.NoError(rj.T(), err, "failed to update job")
	updatedJobList, err := standardUserContext.Batch.Job().List(namespace.Name, metav1.ListOptions{})
	require.NoError(rj.T(), err, "Failed to list the job after updating labels.")
	require.Equal(rj.T(), "true", updatedJobList.Items[0].Labels["updated"], "job label update failed.")

	log.Infof("As a %v, delete the job", role)
	err = standardUserContext.Batch.Job().Delete(namespace.Name, createdJob.Name, &metav1.DeleteOptions{})
	require.NoError(rj.T(), err, "failed to delete job")
	err = kwait.PollUntilContextTimeout(context.Background(), defaults.FiveSecondTimeout, defaults.OneMinuteTimeout, false, func(ctx context.Context) (done bool, pollErr error) {
		updatedJobList, pollErr := standardUserContext.Batch.Job().List(namespace.Name, metav1.ListOptions{})
		if pollErr != nil {
			return false, fmt.Errorf("failed to list jobs: %w", pollErr)
		}

		if len(updatedJobList.Items) == 0 {
			return true, nil
		}
		return false, nil
	})
	require.NoError(rj.T(), err)
}

func TestRbacJobTestSuite(t *testing.T) {
	suite.Run(t, new(RbacJobTestSuite))
}

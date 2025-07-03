package project_cluster

import (
	"reflect"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeClientset "k8s.io/client-go/kubernetes/fake"
)

const clusterID = "test-cluster"

func TestEnqueueCrtbsOnProjectCreation(t *testing.T) {
	existingCrtbs := []*v3.ClusterRoleTemplateBinding{
		{ObjectMeta: v1.ObjectMeta{
			Name:      "crtb-1",
			Namespace: clusterID,
		}},
		{ObjectMeta: v1.ObjectMeta{
			Name:      "crtb-2",
			Namespace: clusterID,
		}},
	}

	ctrl := gomock.NewController(t)

	crtbLister := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
	crtbLister.EXPECT().List(gomock.Any(), gomock.Any()).Return(existingCrtbs, nil)

	crtbClient := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
	crtbClient.EXPECT().Enqueue(gomock.Any(), gomock.Any()).Return().AnyTimes()

	c := projectLifecycle{
		crtbLister: crtbLister,
		crtbClient: crtbClient,
	}

	newProject := v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-project",
			Namespace: clusterID,
		},
	}
	c.enqueueCrtbs(&newProject)
	assert.Equal(t, len(existingCrtbs), 2)
}

func TestReconcileProjectCreatorRTBRespectsUserPrincipalName(t *testing.T) {
	var prtbs []*v3.ProjectRoleTemplateBinding

	ctrl := gomock.NewController(t)

	prtbLister := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
	prtbLister.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	prtbClient := fake.NewMockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
	prtbClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
		prtbs = append(prtbs, obj)
		return obj, nil
	}).AnyTimes()

	projects := fake.NewMockControllerInterface[*v3.Project, *v3.ProjectList](ctrl)
	projects.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.Project) (*v3.Project, error) {
		return obj, nil
	}).AnyTimes()

	lifecycle := &projectLifecycle{
		prtbLister: prtbLister,
		prtbClient: prtbClient,
		projects:   projects,
	}

	userPrincipalName := "keycloak_user@12345"

	project := &v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Name:      "p-abcdef",
			Namespace: clusterID,
			Annotations: map[string]string{
				CreatorIDAnnotation:             "u-abcdef",
				creatorPrincipalNameAnnotation:  userPrincipalName,
				roleTemplatesRequiredAnnotation: `{"created":["project-owner"],"required":["project-owner"]}`,
			},
		},
	}

	obj, err := lifecycle.reconcileProjectCreatorRTB(project, clusterID)
	require.NoError(t, err)
	require.NotNil(t, obj)

	require.Len(t, prtbs, 1)
	assert.Equal(t, "creator-project-owner", prtbs[0].Name)
	assert.Equal(t, clusterID, prtbs[0].Namespace)
	assert.Equal(t, clusterID+":p-abcdef", prtbs[0].ProjectName)
	assert.Equal(t, "", prtbs[0].UserName)
	assert.Equal(t, userPrincipalName, prtbs[0].UserPrincipalName)
}

func TestReconcileProjectCreatorRTBNoCreatorRBAC(t *testing.T) {
	// When NoCreatorRBACAnnotation is set, nothing in the lifecycle will be called
	lifecycle := &projectLifecycle{}
	project := &v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				NoCreatorRBACAnnotation: "true",
			},
		},
	}
	obj, err := lifecycle.reconcileProjectCreatorRTB(project, clusterID)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

type fakeSystemAccountManager struct {
	removedIDs []string
	removeErr  error
}

func (f *fakeSystemAccountManager) RemoveSystemAccount(projectID string) error {
	f.removedIDs = append(f.removedIDs, projectID)
	return f.removeErr
}

// Test deletion of system account created for this project when Sync is called with DeletionTimeStamp
func TestSyncDeleteSystemUser(t *testing.T) {
	sysMgr := &fakeSystemAccountManager{}
	// Create a project with a deletion timestamp.
	now := v1.NewTime(time.Now())
	proj := &v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Name:              "p-deleted",
			Namespace:         clusterID,
			DeletionTimestamp: &now,
		},
	}

	// Instantiate lifecycle with the fake system account manager.
	lifecycle := &projectLifecycle{
		systemAccountManager: sysMgr,
	}

	// Use a key of format "clusterID/projectID".
	key := clusterID + "/p-deleted"
	obj, err := lifecycle.Sync(key, proj)
	require.NoError(t, err)
	require.Nil(t, obj)
	// Check that RemoveSystemAccount was called with the correct projectID.
	assert.Contains(t, sysMgr.removedIDs, "p-deleted")
}

// TestCreateAndUpdated verifies that the Create and Updated methods return the original object because they're currently no-op
func TestCreateAndUpdated(t *testing.T) {
	lifecycle := &projectLifecycle{}
	proj := &v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-project",
		},
	}
	created, err := lifecycle.Create(proj)
	require.NoError(t, err)
	require.True(t, reflect.DeepEqual(created, proj), "Create should return the original object")

	updated, err := lifecycle.Updated(proj)
	require.NoError(t, err)
	require.True(t, reflect.DeepEqual(updated, proj), "Updated should return the original object")
}

func TestRemove(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: "p-remove",
		},
	}
	// Create a fake clientset using the fake package.
	clientset := fakeClientset.NewSimpleClientset(namespace)
	fakeNSClient := clientset.CoreV1().Namespaces()

	lifecycle := &projectLifecycle{
		nsClient: fakeNSClient,
	}
	project := &v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Name:      "p-remove",
			Namespace: "test-namespace",
		},
	}
	Obj, err := lifecycle.Remove(project)
	// Since the namespace exists in the cache, a GET & DELETE call should be recorded
	require.Len(t, clientset.Fake.Actions(), 2, "expected exactly one action to be recorded")
	require.NoError(t, err)
	require.NotNil(t, Obj)
	// Since Remove returns the original project after deleting, assert the returned object equals original project
	assert.Equal(t, project, Obj)
}

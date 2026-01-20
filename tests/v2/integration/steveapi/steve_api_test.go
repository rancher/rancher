package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/api/scheme"
	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/features"
	kubenamespaces "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/secrets"
	stevesecrets "github.com/rancher/rancher/tests/v2/integration/actions/secrets"
	"github.com/rancher/rancher/tests/v2/integration/actions/serviceaccounts"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	clientv1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"

	"github.com/rancher/rancher/tests/v2/integration/actions/namespaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

const (
	labelKey          = "test-label"
	labelGTEKey       = "test-label-gte"
	continueToken     = "nondeterministictoken"
	revisionNum       = "nondeterministicint"
	fakeTestID        = "nondeterministicid"
	defautlUrlString  = "https://rancherurl/"
	steveAPITestLabel = "test.cattle.io/steveapi"
)

var (
	testID                     = namegenerator.RandStringLower(5)
	userEnabled                = true
	impersonationNamespace     = "cattle-impersonation-system"
	impersonationSABase        = "cattle-impersonation-"
	urlRegex                   = regexp.MustCompile(`https://([\w.:]+)/`)
	continueReg                = regexp.MustCompile(`(continue=)[\w]+(%3D){0,2}`)
	revisionReg                = regexp.MustCompile(`(revision=)[\d]+`)
	testLabelReg               = regexp.MustCompile(`(labelSelector=test.cattle.io%2Fsteveapi%3D)[\w]+`)
	projectTag                 = regexp.MustCompile(`(test-prj-[1-9])`)
	namespaceTag               = regexp.MustCompile(`(test-ns-[1-9])`)
	namespaceSecretManagerRole = rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace-secret-manager",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs: []string{
					"get",
					"list",
				},
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"secrets",
				},
			},
		},
	}
	mixedSecretUserRole = rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mixed-secret-user",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs: []string{
					"get",
					"list",
				},
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"secrets",
				},
				ResourceNames: []string{
					"test1",
					"test2",
				},
			},
		},
	}
	testUsers = map[string][]interface{}{
		"user-a": {
			management.ProjectRoleTemplateBinding{
				RoleTemplateID: "project-owner",
				ProjectID:      "test-prj-1",
			},
		},
		"user-b": {
			rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "namespace-secret-manager",
					Namespace: "test-ns-1",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "namespace-secret-manager",
				},
			},
		},
		"user-c": {
			rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-secret-user",
					Namespace: "test-ns-1",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "mixed-secret-user",
				},
			},
			rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-secret-user",
					Namespace: "test-ns-2",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "mixed-secret-user",
				},
			},
			rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-secret-user",
					Namespace: "test-ns-3",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "mixed-secret-user",
				},
			},
		},
		"user-d": {
			management.ProjectRoleTemplateBinding{
				RoleTemplateID: "project-owner",
				ProjectID:      "test-prj-1",
			},
			management.ProjectRoleTemplateBinding{
				RoleTemplateID: "project-owner",
				ProjectID:      "test-prj-2",
			},
			rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "namespace-secret-manager",
					Namespace: "test-ns-8",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "namespace-secret-manager",
				},
			},
			rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "namespace-secret-manager",
					Namespace: "test-ns-9",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     "namespace-secret-manager",
				},
			},
		},
		"user-e": {
			management.ClusterRoleTemplateBinding{
				RoleTemplateID: "cluster-owner",
			},
		},
	}
	namespaceMap = map[string]string{
		"test-ns-1": "",
		"test-ns-2": "",
		"test-ns-3": "",
		"test-ns-4": "",
		"test-ns-5": "",
		"test-ns-6": "",
		"test-ns-7": "",
		"test-ns-8": "",
		"test-ns-9": "",
	}
	projectMap = map[string]*management.Project{
		"test-prj-1": nil,
		"test-prj-2": nil,
	}
	projectNamespaceMap = map[string]string{
		"test-ns-1": "test-prj-1",
		"test-ns-2": "test-prj-1",
		"test-ns-3": "test-prj-1",
		"test-ns-4": "test-prj-1",
		"test-ns-5": "test-prj-1",
		"test-ns-6": "test-prj-2",
		"test-ns-7": "test-prj-2",
		"test-ns-8": "",
		"test-ns-9": "",
	}
)

type steveAPITestSuite struct {
	suite.Suite
	client            *rancher.Client
	session           *session.Session
	clusterID         string
	userClients       map[string]*rancher.Client
	lastContinueToken string
	lastRevision      string
}

type LocalSteveAPITestSuite struct {
	steveAPITestSuite
}

func (s *LocalSteveAPITestSuite) TestExtensionAPIServer() {
	restConfig := newExtensionAPIRestConfig(s.client.RancherConfig, s.clusterID, s.client.RancherConfig.AdminToken)
	discClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	require.NoError(s.T(), err)

	groups, err := discClient.ServerGroups()
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), len(groups.Groups), 0)

	v2Document, err := discClient.OpenAPISchema()
	require.NoError(s.T(), err)
	require.NotNil(s.T(), v2Document)

	v3Client := discClient.OpenAPIV3()
	paths, err := v3Client.Paths()
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), len(paths), 0)

	// No auth
	unauthRestConfig := newExtensionAPIRestConfig(s.client.RancherConfig, s.clusterID, "")
	unauthDiscClient, err := discovery.NewDiscoveryClientForConfig(unauthRestConfig)
	require.NoError(s.T(), err)

	_, err = unauthDiscClient.ServerGroups()
	require.Error(s.T(), err)
	require.True(s.T(), apierrors.IsForbidden(err))

	_, err = unauthDiscClient.OpenAPISchema()
	require.Error(s.T(), err)
	require.True(s.T(), apierrors.IsForbidden(err))

	unauthV3Client := unauthDiscClient.OpenAPIV3()
	_, err = unauthV3Client.Paths()
	require.Error(s.T(), err)
	require.True(s.T(), apierrors.IsForbidden(err))

}

func (s *LocalSteveAPITestSuite) TestExtensionAPIServerAuthorization() {
	restConfig := newExtensionAPIRestConfig(s.client.RancherConfig, s.clusterID, s.client.RancherConfig.AdminToken)
	client, err := rest.HTTPClientFor(restConfig)
	require.NoError(s.T(), err)

	tests := []struct {
		path               string
		expectedStatusCode int
	}{
		{
			path:               "/openapi/v2",
			expectedStatusCode: http.StatusOK,
		},
		{
			path:               "/openapi/v3",
			expectedStatusCode: http.StatusOK,
		},
		{
			path:               "/openapi/v3/version",
			expectedStatusCode: http.StatusOK,
		},
		{
			path:               "/metrics",
			expectedStatusCode: http.StatusForbidden,
		},
		{
			path:               "/healthz",
			expectedStatusCode: http.StatusForbidden,
		},
		{
			path:               "/readyz",
			expectedStatusCode: http.StatusForbidden,
		},
		{
			path:               "/livez",
			expectedStatusCode: http.StatusForbidden,
		},
		{
			path:               "/version",
			expectedStatusCode: http.StatusForbidden,
		},
	}

	for _, test := range tests {
		name := strings.ReplaceAll(test.path, "/", "_")
		s.T().Run(name, func(t *testing.T) {
			resp, err := client.Get(fmt.Sprintf("%s/%s", restConfig.Host, test.path))
			require.NoError(t, err)
			require.Equal(t, test.expectedStatusCode, resp.StatusCode)
		})
	}
}

func (s *LocalSteveAPITestSuite) TestExtensionAPIServerCreateRequests() {
	client, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	require.NoError(s.T(), err)

	tests := []struct {
		name string
		path string
		body io.Reader

		expectedCode int
	}{
		{
			name: "create kubeconfig",
			path: "/v1/ext.cattle.io.kubeconfig",
			body: strings.NewReader(`
			{
				"apiVersion":"ext.cattle.io/v1",
				"kind":"kubeconfig",
				"metadata": {
					"name": "test-kubeconfig"
				},
				"spec": {
					"clusters": ["local"],
					"currentContent": "local",
					"description": "kubeconfig for testing new kubeconfigs",
					"ttl": 100
				}
			}`),
			expectedCode: http.StatusCreated,
		},
		{
			name: "create self user",
			path: "/v1/ext.cattle.io.selfusers",
			body: strings.NewReader(`
			{
				"apiVersion":"ext.cattle.io/v1",
				"kind":"selfuser"
			}`),
			expectedCode: http.StatusCreated,
		},
	}

	for _, test := range tests {
		s.T().Run(test.name, func(t *testing.T) {
			resp, err := client.Post(
				fmt.Sprintf("https://%s%s", s.client.WranglerContext.RESTConfig.Host, test.path),
				"application/json",
				test.body,
			)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedCode, resp.StatusCode)
		})
	}
}

func (s *LocalSteveAPITestSuite) TestExtensionAPIServerUpdateRequests() {
	client, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	require.NoError(s.T(), err)

	kubeconfig := s.createKubeconfig(client)

	tests := []struct {
		name         string
		path         string
		kubeconfig   extv1.Kubeconfig
		expectedCode int
	}{
		{
			name: "update kubeconfig",
			path: "/v1/ext.cattle.io.kubeconfig/" + kubeconfig.Name,
			kubeconfig: extv1.Kubeconfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:            kubeconfig.Name,
					ResourceVersion: kubeconfig.ResourceVersion,
				},
				Spec: extv1.KubeconfigSpec{
					Clusters:       kubeconfig.Spec.Clusters,
					CurrentContext: kubeconfig.Spec.CurrentContext,
					Description:    "kubeconfig updated",
					TTL:            kubeconfig.Spec.TTL,
				},
			},
			expectedCode: http.StatusOK,
		},
		{
			name: "update non-existant kubeconfig",
			path: "/v1/ext.cattle.io/kubeconfig/does-not-exist",
			kubeconfig: extv1.Kubeconfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:            kubeconfig.Name,
					ResourceVersion: kubeconfig.ResourceVersion,
				},
				Spec: extv1.KubeconfigSpec{
					Clusters:       kubeconfig.Spec.Clusters,
					CurrentContext: kubeconfig.Spec.CurrentContext,
					Description:    "kubeconfig updated",
					TTL:            kubeconfig.Spec.TTL,
				},
			},
			expectedCode: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		s.T().Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.kubeconfig)
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("https://%s%s", s.client.WranglerContext.RESTConfig.Host, test.path), bytes.NewBuffer(data))
			require.NoError(t, err)

			resp, err := client.Do(req)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedCode, resp.StatusCode)
		})
	}
}

func (s *LocalSteveAPITestSuite) TestExtensionAPIServerDeleteRequests() {
	client, err := rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
	require.NoError(s.T(), err)

	kubeconfig := s.createKubeconfig(client)

	tests := []struct {
		name         string
		path         string
		expectedCode int
	}{
		{
			name:         "delete kubeconfig",
			path:         "/v1/ext.cattle.io.kubeconfig/" + kubeconfig.Name,
			expectedCode: http.StatusNoContent,
		},
		{
			name:         "delete non-existant kubeconfig",
			path:         "/v1/ext.cattle.io/kubeconfig/does-not-exist",
			expectedCode: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		s.T().Run(test.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("https://%s%s", s.client.WranglerContext.RESTConfig.Host, test.path), nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedCode, resp.StatusCode)
		})
	}
}

type DownstreamSteveAPITestSuite struct {
	steveAPITestSuite
}

func (s *steveAPITestSuite) TearDownSuite() {
	s.session.Cleanup()
}

// Tests that everything in /ext returns 404 since the Downstream cluster shouldn't be served
func (s *DownstreamSteveAPITestSuite) TestExtensionAPIServer() {
	restConfig := newExtensionAPIRestConfig(s.client.RancherConfig, s.clusterID, s.client.RancherConfig.AdminToken)
	discClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	require.NoError(s.T(), err)

	_, err = discClient.ServerGroups()
	require.Error(s.T(), err)
	require.True(s.T(), apierrors.IsNotFound(err))

	_, err = discClient.OpenAPISchema()
	require.Error(s.T(), err)
	require.True(s.T(), apierrors.IsNotFound(err))

	v3Client := discClient.OpenAPIV3()
	_, err = v3Client.Paths()
	require.Error(s.T(), err)
	require.True(s.T(), apierrors.IsNotFound(err))
}

func (s *LocalSteveAPITestSuite) SetupSuite() {
	s.steveAPITestSuite.setupSuite("local")
}

func (s *DownstreamSteveAPITestSuite) SetupSuite() {
	s.steveAPITestSuite.setupSuite("")
}

func (s *steveAPITestSuite) setupSuite(clusterName string) {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)
	s.client = client

	s.userClients = make(map[string]*rancher.Client)

	if clusterName == "" {
		clusterName = s.client.RancherConfig.ClusterName
	}
	s.clusterID, err = clusters.GetClusterIDByName(client, clusterName)
	require.NoError(s.T(), err)

	mgmtCluster, err := client.Management.Cluster.ByID(s.clusterID)
	require.NoError(s.T(), err)

	// create projects
	for p := range projectMap {
		project, err := s.client.Management.Project.Create(&management.Project{
			ClusterID: s.clusterID,
			Name:      p,
		})
		require.NoError(s.T(), err)
		projectMap[p] = project
	}

	userID, err := users.GetUserIDByName(client, "admin")
	require.NoError(s.T(), err)

	impersonationSA := impersonationSABase + userID
	err = serviceaccounts.IsServiceAccountReady(client, s.clusterID, impersonationNamespace, impersonationSA)
	require.NoError(s.T(), err)

	// create project namespaces
	for n := range namespaceMap {
		if projectMap[projectNamespaceMap[n]] == nil {
			continue
		}
		name := namegenerator.AppendRandomString(n)
		_, err := namespaces.CreateNamespace(client, name, "", nil, nil, projectMap[projectNamespaceMap[n]])
		require.NoError(s.T(), err)
		namespaceMap[n] = name
	}
	// create non project namespaces
	for n := range namespaceMap {
		if projectMap[projectNamespaceMap[n]] != nil {
			continue
		}
		name := namegenerator.AppendRandomString(n)
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		dynamicClient, err := client.GetDownStreamClusterClient(s.clusterID)
		require.NoError(s.T(), err)
		namespaceResource := dynamicClient.Resource(kubenamespaces.NamespaceGroupVersionResource)
		resp, err := namespaceResource.Create(context.TODO(), unstructured.MustToUnstructured(ns), metav1.CreateOptions{})
		require.NoError(s.T(), err)
		s.client.Session.RegisterCleanupFunc(func() error {
			err := namespaceResource.Delete(context.TODO(), resp.GetName(), metav1.DeleteOptions{})
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		})
		err = scheme.Scheme.Convert(resp, ns, resp.GroupVersionKind())
		require.NoError(s.T(), err)
		err = wait.Poll(time.Second, time.Minute, func() (done bool, err error) {
			ns, _ := kubenamespaces.GetNamespaceByName(s.client, s.clusterID, ns.Name)
			if ns != nil {
				return true, nil
			}
			return false, nil
		})
		require.NoError(s.T(), err)
		namespaceMap[n] = name
	}

	// create resources in all namespaces
	for name, n := range namespaceMap {
		for i := 1; i <= 5; i++ {
			if i > 2 && (projectNamespaceMap[name] == "test-prj-2" || projectNamespaceMap[name] == "") {
				break
			}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("test%d", i),
				},
			}
			// Test sorting secrets on metadata.fields[2] (# of secret keys)
			if name == "test-ns-1" {
				numKeys := 0
				if i == 1 {
					numKeys = 15
				} else if i == 2 {
					numKeys = 23
				} else if i == 3 {
					numKeys = 7
				}
				if numKeys > 0 {
					obj := map[string][]byte{}
					for j := 1; j <= numKeys; j++ {
						obj[fmt.Sprintf("k%d-%d", i, j)] = []byte("whatever")
					}
					secret.Data = obj
				}
			}
			labels := map[string]string{steveAPITestLabel: testID}
			if i == 2 {
				labels[labelKey] = "2"
			}
			if i >= 3 {
				labels[labelGTEKey] = "3"
			}
			secret.ObjectMeta.SetLabels(labels)
			if i == 4 && name == "test-ns-2" {
				// test4 in namespace test-ns-2 has this annotation
				annotations := map[string]string{"management.cattle.io/project-scoped-secret-copy": "spuds"}
				secret.ObjectMeta.SetAnnotations(annotations)
			}
			err := retryRequest(func() error {
				_, err := secrets.CreateSecretForCluster(s.client, secret, s.clusterID, n)
				if apierrors.IsAlreadyExists(err) {
					return nil
				}
				return err
			})
			require.NoError(s.T(), err)
		}
	}

	// create test roles in all namespaces
	for _, n := range namespaceMap {
		role := namespaceSecretManagerRole
		role.Namespace = n
		err := retryRequest(func() error {
			_, err = rbac.CreateRole(s.client, s.clusterID, &role)
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return err
		})
		require.NoError(s.T(), err)
		role = mixedSecretUserRole
		role.Namespace = n
		err = retryRequest(func() error {
			_, err = rbac.CreateRole(s.client, s.clusterID, &role)
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return err
		})
		require.NoError(s.T(), err)
	}

	// create users and assign access
	for user, access := range testUsers {
		username := namegenerator.AppendRandomString(user)
		password := password.GenerateUserPassword("testpass")
		userObj := &management.User{
			Username: username,
			Password: password,
			Name:     username,
			Enabled:  &userEnabled,
		}
		userObj, err := s.client.Management.User.Create(userObj)
		require.NoError(s.T(), err)
		userObj.Password = password
		// users either have access to a whole project or to select namespaces or resources in a project
		for _, binding := range access {
			switch b := binding.(type) {
			case management.ClusterRoleTemplateBinding:
				err = users.AddClusterRoleToUser(client, mgmtCluster, userObj, b.RoleTemplateID, nil)
				require.NoError(s.T(), err)
			case management.ProjectRoleTemplateBinding:
				err = users.AddProjectMember(client, projectMap[b.ProjectID], userObj, b.RoleTemplateID, nil)
				require.NoError(s.T(), err)
			case rbacv1.RoleBinding:
				_ = users.AddClusterRoleToUser(client, mgmtCluster, userObj, "cluster-member", nil)
				subject := rbacv1.Subject{
					Kind: "User",
					Name: userObj.ID,
				}
				err := retryRequest(func() error {
					_, err = rbac.CreateRoleBinding(s.client, s.clusterID, namegenerator.AppendRandomString(b.Name), namespaceMap[b.Namespace], b.RoleRef.Name, subject)
					if apierrors.IsAlreadyExists(err) {
						return nil
					}
					return err
				})
				require.NoError(s.T(), err)
			}
		}
		s.userClients[user], err = s.client.AsUser(userObj)
		require.NoError(s.T(), err)
	}
}

type listTestType struct {
	description    string
	user           string
	namespace      string
	query          string
	expect         []map[string]string
	expectExcludes bool
	expectContains bool
	expectSummary []clientv1.SteveAPISummaryItem
}

// TEST LIST
var SQLOnlyListTests = []listTestType{
	// user-a
	{
		description: "user:user-a,namespace:none,query:summary=metadata.namespace,metadata.name",
		user:        "user-a",
		namespace:   "",
		query:       "summary=metadata.namespace,metadata.name",
		expect: []map[string]string{
			{"name": "test1", "namespace": "test-ns-1"},
			{"name": "test2", "namespace": "test-ns-1"},
			{"name": "test3", "namespace": "test-ns-1"},
			{"name": "test4", "namespace": "test-ns-1"},
			{"name": "test5", "namespace": "test-ns-1"},
			{"name": "test1", "namespace": "test-ns-2"},
			{"name": "test2", "namespace": "test-ns-2"},
			{"name": "test3", "namespace": "test-ns-2"},
			{"name": "test4", "namespace": "test-ns-2"},
			{"name": "test5", "namespace": "test-ns-2"},
			{"name": "test1", "namespace": "test-ns-3"},
			{"name": "test2", "namespace": "test-ns-3"},
			{"name": "test3", "namespace": "test-ns-3"},
			{"name": "test4", "namespace": "test-ns-3"},
			{"name": "test5", "namespace": "test-ns-3"},
			{"name": "test1", "namespace": "test-ns-4"},
			{"name": "test2", "namespace": "test-ns-4"},
			{"name": "test3", "namespace": "test-ns-4"},
			{"name": "test4", "namespace": "test-ns-4"},
			{"name": "test5", "namespace": "test-ns-4"},
			{"name": "test1", "namespace": "test-ns-5"},
			{"name": "test2", "namespace": "test-ns-5"},
			{"name": "test3", "namespace": "test-ns-5"},
			{"name": "test4", "namespace": "test-ns-5"},
			{"name": "test5", "namespace": "test-ns-5"},
		},
		expectSummary: []clientv1.SteveAPISummaryItem{
			clientv1.SteveAPISummaryItem{
				Property: "metadata.name",
				Counts: map[string]int{
					"test1":5, "test2":5, "test3":5, "test4":5, "test5":5,
				},
			},
			clientv1.SteveAPISummaryItem{
				Property: "metadata.namespace",
				Counts: map[string]int{
					"test-ns-1":5, "test-ns-2":5, "test-ns-3":5, "test-ns-4":5, "test-ns-5":5,
				},
			},
		},
	},
	{
		description: "user:user-a,namespace:none,query:pagesize=8&summary=metadata.name,metadata.namespace&sort=metadata.namespace,metadata.name",
		user:        "user-a",
		namespace:   "",
		query:       "pagesize=8&summary=metadata.name,metadata.namespace&sort=metadata.namespace,metadata.name",
		expect: []map[string]string{
			{"name": "test1", "namespace": "test-ns-1"},
			{"name": "test2", "namespace": "test-ns-1"},
			{"name": "test3", "namespace": "test-ns-1"},
			{"name": "test4", "namespace": "test-ns-1"},
			{"name": "test5", "namespace": "test-ns-1"},
			{"name": "test1", "namespace": "test-ns-2"},
			{"name": "test2", "namespace": "test-ns-2"},
			{"name": "test3", "namespace": "test-ns-2"},
		},
		expectSummary: []clientv1.SteveAPISummaryItem{
			clientv1.SteveAPISummaryItem{
				Property: "metadata.name",
				Counts: map[string]int{
					"test1":2, "test2":2, "test3":2, "test4":1, "test5":1,
				},
			},
			clientv1.SteveAPISummaryItem{
				Property: "metadata.namespace",
				Counts: map[string]int{
					"test-ns-1":5, "test-ns-2":3,
				},
			},
		},
	},
	{
		description: "user:user-a,namespace:none,query:filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&summary=metadata.name",
		user:        "user-a",
		namespace:   "",
		query:       "filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&summary=metadata.name",
		expect: []map[string]string{
			{"name": "test5"},
			{"name": "test5"},
			{"name": "test5"},
			{"name": "test5"},
			{"name": "test5"},
			{"name": "test4"},
		},
		expectSummary: []clientv1.SteveAPISummaryItem{
			clientv1.SteveAPISummaryItem{
				Property: "metadata.name",
				Counts: map[string]int{
					"test4": 1,
					"test5": 5,
				},
			},
		},
	},
	{
		description: "user:user-a,namespace:test-ns-2,query:filter=metadata.labels.test-label=2&summary=metadata.namespace,metadata.name",
		user:        "user-a",
		namespace:   "test-ns-2",
		query:       "filter=metadata.labels.test-label=2&summary=metadata.namespace,metadata.name",
		expect: []map[string]string{
			{"name": "test2", "namespace": "test-ns-2"},
		},
		expectSummary: []clientv1.SteveAPISummaryItem{
			clientv1.SteveAPISummaryItem{
				Property: "metadata.name",
				Counts: map[string]int{
					"test2":1,
				},
			},
			clientv1.SteveAPISummaryItem{
				Property: "metadata.namespace",
				Counts: map[string]int{
					"test-ns-2":1,
				},
			},
		},
	},
	{
		description: "user:user-a,namespace:none,query:summary=metadata.state.name",
		user:        "user-a",
		namespace:   "none",
		query:       "summary=metadata.state.name",
		expect: []map[string]string{
			{"name": "test1", "namespace": "test-ns-1"},
			{"name": "test2", "namespace": "test-ns-1"},
			{"name": "test3", "namespace": "test-ns-1"},
			{"name": "test4", "namespace": "test-ns-1"},
			{"name": "test5", "namespace": "test-ns-1"},
			{"name": "test1", "namespace": "test-ns-2"},
			{"name": "test2", "namespace": "test-ns-2"},
			{"name": "test3", "namespace": "test-ns-2"},
			{"name": "test4", "namespace": "test-ns-2"},
			{"name": "test5", "namespace": "test-ns-2"},
			{"name": "test1", "namespace": "test-ns-3"},
			{"name": "test2", "namespace": "test-ns-3"},
			{"name": "test3", "namespace": "test-ns-3"},
			{"name": "test4", "namespace": "test-ns-3"},
			{"name": "test5", "namespace": "test-ns-3"},
			{"name": "test1", "namespace": "test-ns-4"},
			{"name": "test2", "namespace": "test-ns-4"},
			{"name": "test3", "namespace": "test-ns-4"},
			{"name": "test4", "namespace": "test-ns-4"},
			{"name": "test5", "namespace": "test-ns-4"},
			{"name": "test1", "namespace": "test-ns-5"},
			{"name": "test2", "namespace": "test-ns-5"},
			{"name": "test3", "namespace": "test-ns-5"},
			{"name": "test4", "namespace": "test-ns-5"},
			{"name": "test5", "namespace": "test-ns-5"},
		},
		expectSummary: []clientv1.SteveAPISummaryItem{
			clientv1.SteveAPISummaryItem{
				Property: "metadata.state.name",
				Counts: map[string]int{
					"active":25,
				},
			},
		},
	},
	{
		// non-sql can't handle annotations and label fields that contain "."s because it splits on them
		description: "user:user-a,namespace:test-ns-2,query:filter=metadata.annotations[management.cattle.io/project-scoped-secret-copy]=spuds",
		user:        "user-a",
		namespace:   "test-ns-2",
		query:       "filter=metadata.annotations[management.cattle.io/project-scoped-secret-copy]=spuds",
		expect: []map[string]string{
			{"name": "test4", "namespace": "test-ns-2"},
		},
	},
	{
		// non-sql doesn't handle the '>' greater-than operator
		description: "user:user-a,namespace:test-ns-1,query:filter=metadata.fields[2]>0&sort=metadata.fields[2]",
		user:        "user-a",
		namespace:   "test-ns-1",
		query:       "filter=metadata.fields[2]>0&sort=metadata.fields[2]",
		expect: []map[string]string{
			{"name": "test3", "namespace": "test-ns-1"},
			{"name": "test1", "namespace": "test-ns-1"},
			{"name": "test2", "namespace": "test-ns-1"},
		},
	},
}
// TEST LIST
var nonSQLListTests = []listTestType{
		// user-a
	{
		description: "user:user-a,namespace:none,query:limit=8",
		user:        "user-a",
		namespace:   "",
		query:       "limit=8",
		expect: []map[string]string{
			{"name": "test1", "namespace": "test-ns-1"},
			{"name": "test2", "namespace": "test-ns-1"},
			{"name": "test3", "namespace": "test-ns-1"},
			{"name": "test4", "namespace": "test-ns-1"},
			{"name": "test5", "namespace": "test-ns-1"},
			{"name": "test1", "namespace": "test-ns-2"},
			{"name": "test2", "namespace": "test-ns-2"},
			{"name": "test3", "namespace": "test-ns-2"},
		},
	},
	{
		description: "user:user-a,namespace:none,query:limit=8&continue=" + continueToken,
		user:        "user-a",
		namespace:   "",
		query:       "limit=8&continue=" + continueToken,
		expect: []map[string]string{
			{"name": "test4", "namespace": "test-ns-2"},
			{"name": "test5", "namespace": "test-ns-2"},
			{"name": "test1", "namespace": "test-ns-3"},
			{"name": "test2", "namespace": "test-ns-3"},
			{"name": "test3", "namespace": "test-ns-3"},
			{"name": "test4", "namespace": "test-ns-3"},
			{"name": "test5", "namespace": "test-ns-3"},
			{"name": "test1", "namespace": "test-ns-4"},
		},
	},
	{
		description: "user:user-a,namespace:test-ns-1,query:limit=3",
		user:        "user-a",
		namespace:   "test-ns-1",
		query:       "limit=3",
		expect: []map[string]string{
			{"name": "test1", "namespace": "test-ns-1"},
			{"name": "test2", "namespace": "test-ns-1"},
			{"name": "test3", "namespace": "test-ns-1"},
		},
	},
	{
		description: "user:user-a,namespace:test-ns-1,query:limit=3&continue=" + continueToken,
		user:        "user-a",
		namespace:   "test-ns-1",
		query:       "limit=3&continue=" + continueToken,
		expect: []map[string]string{
			{"name": "test4", "namespace": "test-ns-1"},
			{"name": "test5", "namespace": "test-ns-1"},
		},
	},
	{
		description: "user:user-a,namespace:none,query:filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&limit=20",
		user:        "user-a",
		namespace:   "",
		query:       "filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&limit=20",
		// limit is applied BEFORE filter and pagesize, which is why not all test5 secrets appear in the result
		expect: []map[string]string{
			{"name": "test5"},
			{"name": "test5"},
			{"name": "test5"},
			{"name": "test5"},
			{"name": "test4"},
			{"name": "test4"},
		},
	},
	{
		description: "user:user-a,namespace:none,query:filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=2&revision=" + revisionNum + "&limit=20",
		user:        "user-a",
		namespace:   "",
		query:       "filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=2&revision=" + revisionNum + "&limit=20",
		expect: []map[string]string{
			{"name": "test4"},
			{"name": "test4"},
			{"name": "test3"},
			{"name": "test3"},
			{"name": "test3"},
			{"name": "test3"},
		},
	},
	{
		description: "user:user-a,namespace:none,query:filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=1&limit=20&continue=" + continueToken,
		user:        "user-a",
		namespace:   "",
		query:       "filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=1&limit=20&continue=" + continueToken,
		// the remaining chunk is all from test-ns-5
		expect: []map[string]string{
			{"name": "test5", "namespace": "test-ns-5"},
			{"name": "test4", "namespace": "test-ns-5"},
			{"name": "test3", "namespace": "test-ns-5"},
		},
	},
	{
		description: "user:user-b,namespace:none,query:limit=3",
		user:        "user-b",
		namespace:   "",
		query:       "limit=3",
		expect: []map[string]string{
			{"name": "test1", "namespace": "test-ns-1"},
			{"name": "test2", "namespace": "test-ns-1"},
			{"name": "test3", "namespace": "test-ns-1"},
		},
	},
	{
		description: "user:user-b,namespace:none,query:limit=3&continue=" + continueToken,
		user:        "user-b",
		namespace:   "",
		query:       "limit=3&continue=" + continueToken,
		expect: []map[string]string{
			{"name": "test4", "namespace": "test-ns-1"},
			{"name": "test5", "namespace": "test-ns-1"},
		},
	},
	{
		description: "user:user-b,namespace:test-ns-1,query:limit=3",
		user:        "user-b",
		namespace:   "test-ns-1",
		query:       "limit=3",
		expect: []map[string]string{
			{"name": "test1", "namespace": "test-ns-1"},
			{"name": "test2", "namespace": "test-ns-1"},
			{"name": "test3", "namespace": "test-ns-1"},
		},
	},
	{
		description: "user:user-b,namespace:test-ns-1,query:limit=3&continue=" + continueToken,
		user:        "user-b",
		namespace:   "test-ns-1",
		query:       "limit=3&continue=" + continueToken,
		expect: []map[string]string{
			{"name": "test4", "namespace": "test-ns-1"},
			{"name": "test5", "namespace": "test-ns-1"},
		},
	},
	{
		description: "user:user-b,namespace:test-ns-5,query:limit=3",
		user:        "user-b",
		namespace:   "test-ns-5",
		query:       "limit=3",
		expect:      []map[string]string{},
	},
	{
		description: "user:user-c,namespace:none,query:limit=3",
		user:        "user-c",
		namespace:   "",
		query:       "limit=3",
		expect: []map[string]string{
			{"name": "test1", "namespace": "test-ns-1"},
			{"name": "test2", "namespace": "test-ns-1"},
			{"name": "test1", "namespace": "test-ns-2"},
		},
	},
	{
		description: "user:user-c,namespace:none,query:limit=3&continue=" + continueToken,
		user:        "user-c",
		namespace:   "",
		query:       "limit=3&continue=" + continueToken,
		expect: []map[string]string{
			{"name": "test2", "namespace": "test-ns-2"},
			{"name": "test1", "namespace": "test-ns-3"},
			{"name": "test2", "namespace": "test-ns-3"},
		},
	},
	{
		description: "user:user-c,namespace:test-ns-1,query:limit=3",
		user:        "user-c",
		namespace:   "test-ns-1",
		query:       "limit=3",
		expect: []map[string]string{
			{"name": "test1", "namespace": "test-ns-1"},
			{"name": "test2", "namespace": "test-ns-1"},
		},
	},
	{
		description: "user:user-c,namespace:test-ns-5,query:limit=3",
		user:        "user-c",
		namespace:   "test-ns-5",
		query:       "limit=3",
		expect:      []map[string]string{},
	},
}

func (s *steveAPITestSuite) TestList() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()
	usingSQLCache := features.UISQLCache.Enabled()
	relativeDateRx := regexp.MustCompile(`^(\d+[smhd])+$`)
	containsNamespaceTag := regexp.MustCompile(`(%2Fsteveapi%5D~)[a-z]+`)
	containsSortNamespace := regexp.MustCompile(`sort=.*metadata.namespace\b`)
	containsSortName := regexp.MustCompile(`sort=.*metadata.name\b`)
	containsReverseOrderSortName := regexp.MustCompile(`sort=.*-metadata.name\b`)
	replacementNamespaceTag := "${1}MYTAG"

	// TEST LIST
	tests := []listTestType{
		// user-a
		{
			description: "user:user-a,namespace:none,query:none",
			user:        "user-a",
			namespace:   "",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:labelSelector=test-label=2",
			user:        "user-a",
			namespace:   "",
			query:       "labelSelector=test-label=2",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-2,query:labelSelector=test-label=2",
			user:        "user-a",
			namespace:   "test-ns-2",
			query:       "labelSelector=test-label=2",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-2"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:none",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-5,query:none",
			user:        "user-a",
			namespace:   "test-ns-5",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-a",
			namespace:   "",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:fieldSelector=metadata.name=test1",
			user:        "user-a",
			namespace:   "",
			query:       "fieldSelector=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-2,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-a",
			namespace:   "test-ns-2",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:fieldSelector=metadata.name=test1",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:filter=metadata.name=test1",
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:filter=metadata.name=test6",
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.name=test6",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:filter=metadata.name=test1",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "filter=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:filter=metadata.name=test1,metadata.namespace=test-ns-1",
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.name=test1,metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:filter=metadata.labels.test-label-gte=3,metadata.labels.test-label=2&filter=metadata.namespace=test-ns-1",
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3,metadata.labels.test-label=2&filter=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:filter=metadata.name!=test1",
			user:        "user-a",
			namespace:   "",
			query:       "filter=metadata.name!=test1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-2,query:filter=metadata.annotations[management.cattle.io/project-scoped-secret-copy]=potatoes",
			user:        "user-a",
			namespace:   "test-ns-2",
			query:       "filter=metadata.annotations[management.cattle.io/project-scoped-secret-copy]=potatoes",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-a,namespace:none,query:sort=metadata.name",
			user:        "user-a",
			namespace:   "",
			query:       "sort=metadata.name",
			expect: []map[string]string{
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:sort=-metadata.name",
			user:        "user-a",
			namespace:   "",
			query:       "sort=-metadata.name",
			expect: []map[string]string{
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test5"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test4"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test3"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:sort=metadata.name,metadata.namespace",
			user:        "user-a",
			namespace:   "",
			query:       "sort=metadata.name,metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-5"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:sort=metadata.name,-metadata.namespace",
			user:        "user-a",
			namespace:   "",
			query:       "sort=metadata.name,-metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:sort=metadata.name",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "sort=metadata.name",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:sort=-metadata.name",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "sort=-metadata.name",
			expect: []map[string]string{
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:pagesize=8",
			user:        "user-a",
			namespace:   "",
			query:       "pagesize=8",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
			},
		},
		{
			description: "user:user-a,namespace:none,query:pagesize=8&page=2&revision=" + revisionNum,
			user:        "user-a",
			namespace:   "",
			query:       "pagesize=8&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:pagesize=3",
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "pagesize=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-a,namespace:test-ns-1,query:pagesize=3&page=2&revision=" + revisionNum,
			user:        "user-a",
			namespace:   "test-ns-1",
			query:       "pagesize=3&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},

		// user-b
		{
			description: "user:user-b,namespace:none,query:none",
			user:        "user-b",
			namespace:   "",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:none",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-5,query:none",
			user:        "user-b",
			namespace:   "test-ns-5",
			query:       "",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:labelSelector=test-label=2",
			user:        "user-b",
			namespace:   "",
			query:       "labelSelector=test-label=2",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:labelSelector=test-label=2",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "labelSelector=test-label=2",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-2,query:labelSelector=test-label=2",
			user:        "user-b",
			namespace:   "test-ns-2",
			query:       "labelSelector=test-label=2",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-b",
			namespace:   "",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:fieldSelector=metadata.namespace=test-ns-2",
			user:        "user-b",
			namespace:   "",
			query:       "fieldSelector=metadata.namespace=test-ns-2",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:fieldSelector=metadata.name=test1",
			user:        "user-b",
			namespace:   "",
			query:       "fieldSelector=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-2,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-b",
			namespace:   "test-ns-2",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-2",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.namespace=test-ns-2",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:fieldSelector=metadata.name=test1",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-2,query:fieldSelector=metadata.name=test1",
			user:        "user-b",
			namespace:   "test-ns-2",
			query:       "fieldSelector=metadata.name=test1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:filter=metadata.name=test1",
			user:        "user-b",
			namespace:   "",
			query:       "filter=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:filter=metadata.name=test1",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "filter=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:filter=metadata.name=test6",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "filter=metadata.name=test6",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:filter=metadata.name=test1,metadata.namespace=test-ns-1",
			user:        "user-b",
			namespace:   "",
			query:       "filter=metadata.name=test1,metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:filter=metadata.labels.test-label-gte=3,metadata.labels.test-label=2&filter=metadata.namespace=test-ns-1",
			user:        "user-b",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3,metadata.labels.test-label=2&filter=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:filter=metadata.name!=test1",
			user:        "user-b",
			namespace:   "",
			query:       "filter=metadata.name!=test1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:sort=metadata.name",
			user:        "user-b",
			namespace:   "",
			query:       "sort=metadata.name",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:sort=-metadata.name",
			user:        "user-b",
			namespace:   "",
			query:       "sort=-metadata.name",
			expect: []map[string]string{
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:sort=metadata.name,metadata.namespace",
			user:        "user-b",
			namespace:   "",
			query:       "sort=metadata.name,metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:sort=-metadata.name,metadata.namespace",
			user:        "user-b",
			namespace:   "",
			query:       "sort=-metadata.name,metadata.namespace",
			expect: []map[string]string{
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:sort=metadata.name",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "sort=metadata.name",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:sort=-metadata.name",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "sort=-metadata.name",
			expect: []map[string]string{
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-5,query:sort=metadata.name",
			user:        "user-b",
			namespace:   "test-ns-5",
			query:       "sort=metadata.name",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:pagesize=3",
			user:        "user-b",
			namespace:   "",
			query:       "pagesize=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:pagesize=3&page=2&revision=" + revisionNum,
			user:        "user-b",
			namespace:   "",
			query:       "pagesize=3&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:pagesize=3",
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "pagesize=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-1,query:pagesize=3&page=2&revision=" + revisionNum,
			user:        "user-b",
			namespace:   "test-ns-1",
			query:       "pagesize=3&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:test-ns-5,query:pagesize=3",
			user:        "user-b",
			namespace:   "test-ns-5",
			query:       "pagesize=3",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-b,namespace:none,query:filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2",
			user:        "user-b",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2",
			expect: []map[string]string{
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-b,namespace:none,query:filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2&page=2&revision=" + revisionNum,
			user:        "user-b",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test3", "namespace": "test-ns-1"},
			},
		},

		// user-c
		{
			description: "user:user-c,namespace:none,query:none",
			user:        "user-c",
			namespace:   "",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:none",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-5,query:none",
			user:        "user-c",
			namespace:   "test-ns-5",
			query:       "",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:labelSelector=test-label=2",
			user:        "user-c",
			namespace:   "",
			query:       "labelSelector=test-label=2",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:labelSelector=test-label=2",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "labelSelector=test-label=2",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-5,query:labelSelector=test-label=2",
			user:        "user-c",
			namespace:   "test-ns-5",
			query:       "labelSelector=test-label=2",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-c",
			namespace:   "",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:fieldSelector=metadata.namespace=test-ns-2",
			user:        "user-c",
			namespace:   "",
			query:       "fieldSelector=metadata.namespace=test-ns-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:fieldSelector=metadata.namespace=test-ns-5",
			user:        "user-c",
			namespace:   "",
			query:       "fieldSelector=metadata.namespace=test-ns-5",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:fieldSelector=metadata.name=test1",
			user:        "user-c",
			namespace:   "",
			query:       "fieldSelector=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:fieldSelector=metadata.name=test5",
			user:        "user-c",
			namespace:   "",
			query:       "fieldSelector=metadata.name=test5",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-2,query:fieldSelector=metadata.namespace=test-ns-1",
			user:        "user-c",
			namespace:   "test-ns-2",
			query:       "fieldSelector=metadata.namespace=test-ns-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-2",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.namespace=test-ns-2",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.name=test1",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-5,query:fieldSelector=metadata.name=test1",
			user:        "user-c",
			namespace:   "test-ns-5",
			query:       "fieldSelector=metadata.name=test1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.name=test5",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "fieldSelector=metadata.name=test5",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:filter=metadata.name=test1",
			user:        "user-c",
			namespace:   "",
			query:       "filter=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:filter=metadata.name=test1",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "filter=metadata.name=test1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:filter=metadata.name=test3",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "filter=metadata.name=test3",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:filter=metadata.name=test1,metadata.namespace=test-ns-1",
			user:        "user-c",
			namespace:   "",
			query:       "filter=metadata.name=test1,metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:filter=metadata.name!=test1",
			user:        "user-c",
			namespace:   "",
			query:       "filter=metadata.name!=test1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:filter=metadata.labels.test-label-gte=3,metadata.labels.test-label=2&filter=metadata.namespace=test-ns-1",
			user:        "user-c",
			namespace:   "",
			query:       "filter=metadata.labels.test-label-gte=3,metadata.labels.test-label=2&filter=metadata.namespace=test-ns-1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:sort=metadata.name",
			user:        "user-c",
			namespace:   "",
			query:       "sort=metadata.name",
			expect: []map[string]string{
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:sort=-metadata.name",
			user:        "user-c",
			namespace:   "",
			query:       "sort=-metadata.name",
			expect: []map[string]string{
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test2"},
				{"name": "test1"},
				{"name": "test1"},
				{"name": "test1"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:sort=metadata.name,metadata.namespace",
			user:        "user-c",
			namespace:   "",
			query:       "sort=metadata.name,metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:sort=metadata.name,-metadata.namespace",
			user:        "user-c",
			namespace:   "",
			query:       "sort=metadata.name,-metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:sort=metadata.name",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "sort=metadata.name",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:sort=-metadata.name",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "sort=-metadata.name",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:sort=metadata.name,metadata.namespace",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "sort=metadata.name,metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:sort=metadata.name,-metadata.namespace",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "sort=metadata.name,-metadata.namespace",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-5,query:sort=metadata.name",
			user:        "user-c",
			namespace:   "test-ns-5",
			query:       "sort=metadata.name",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:pagesize=3",
			user:        "user-c",
			namespace:   "",
			query:       "pagesize=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:pagesize=3&page=2&revision=" + revisionNum,
			user:        "user-c",
			namespace:   "",
			query:       "pagesize=3&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-1,query:pagesize=3",
			user:        "user-c",
			namespace:   "test-ns-1",
			query:       "pagesize=3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-c,namespace:test-ns-5,query:pagesize=3",
			user:        "user-c",
			namespace:   "test-ns-5",
			query:       "pagesize=3",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-c,namespace:none,query:filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1",
			user:        "user-c",
			namespace:   "",
			query:       "filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1",
			expect: []map[string]string{
				{"name": "test2", "namespace": "test-ns-3"},
			},
		},
		{
			description: "user:user-c,namespace:none,query:filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1&page=2&revision=" + revisionNum,
			user:        "user-c",
			namespace:   "",
			query:       "filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1&page=2&revision=" + revisionNum,
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-3"},
			},
		},

		// user-d
		{
			description: "user:user-d,namespace:none,query:none",
			user:        "user-d",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces=test-prj-2",
			user:        "user-d",
			query:       "projectsornamespaces=test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces=test-prj-1,test-prj-2",
			user:        "user-d",
			query:       "projectsornamespaces=test-prj-1,test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces=test-ns-1",
			user:        "user-d",
			query:       "projectsornamespaces=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces=test-ns-1,test-ns-2",
			user:        "user-d",
			query:       "projectsornamespaces=test-ns-1,test-ns-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces=test-prj-2,test-ns-2,test-ns-3",
			user:        "user-d",
			query:       "projectsornamespaces=test-prj-2,test-ns-2,test-ns-3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces=test-ns-8,test-ns-9",
			user:        "user-d",
			query:       "projectsornamespaces=test-ns-8,test-ns-9",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces!=test-prj-1",
			user:        "user-d",
			query:       "projectsornamespaces!=test-prj-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces!=test-prj-1,test-prj-2",
			user:        "user-d",
			query:       "projectsornamespaces!=test-prj-1,test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
		},
		{
			description: "user:user-d,namespace:none,query:projectsornamespaces!=test-prj-1,test-ns-6,test-ns-8",
			user:        "user-d",
			query:       "projectsornamespaces!=test-prj-1,test-ns-6,test-ns-8",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
		},
		{
			description: "user:user-d,namespace:test-ns-6,query:none",
			user:        "user-d",
			namespace:   "test-ns-6",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
			},
		},
		{
			description: "user:user-d,namespace:test-ns-6,query:projectsornamespaces=test-prj-2",
			user:        "user-d",
			namespace:   "test-ns-6",
			query:       "projectsornamespaces=test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
			},
		},
		{
			description: "user:user-d,namespace:test-ns-6,query:projectsornamespaces=test-prj-2",
			user:        "user-d",
			namespace:   "test-ns-6",
			query:       "projectsornamespaces=test-prj-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-d,namespace:test-ns-1,query:projectsornamespaces=test-ns-1,test-ns-2,-test-prj-2,test-ns-7",
			user:        "user-d",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces=test-ns-1,test-ns-2,test-prj-2,test-ns-7",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-d,namespace:test-ns-1,query:projectsornamespaces!=test-prj-1",
			user:        "user-d",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces!=test-prj-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-d,namespace:test-ns-1,query:projectsornamespaces!=test-prj-1,test-prj-2",
			user:        "user-d",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces!=test-prj-1,test-prj-2",
			expect:      []map[string]string{},
		},

		// user-e
		{
			description: "user:user-e,namespace:none,query:none",
			user:        "user-e",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
			expectContains: true,
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces=test-prj-2",
			user:        "user-e",
			query:       "projectsornamespaces=test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces=test-prj-1,test-prj-2",
			user:        "user-e",
			query:       "projectsornamespaces=test-prj-1,test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces=test-ns-1",
			user:        "user-e",
			query:       "projectsornamespaces=test-ns-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces=test-ns-1,test-ns-2",
			user:        "user-e",
			query:       "projectsornamespaces=test-ns-1,test-ns-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
			},
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces=test-prj-2,test-ns-2,test-ns-3",
			user:        "user-e",
			query:       "projectsornamespaces=test-prj-2,test-ns-2,test-ns-3",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces=test-ns-8,test-ns-9",
			user:        "user-e",
			query:       "projectsornamespaces=test-ns-8,test-ns-9",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
				{"name": "test1", "namespace": "test-ns-9"},
				{"name": "test2", "namespace": "test-ns-9"},
			},
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces!=test-prj-1",
			user:        "user-e",
			query:       "projectsornamespaces!=test-prj-1",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
			},
			expectExcludes: true,
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces!=test-prj-1,test-prj-2",
			user:        "user-e",
			query:       "projectsornamespaces!=test-prj-1,test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-7"},
				{"name": "test2", "namespace": "test-ns-7"},
			},
			expectExcludes: true,
		},
		{
			description: "user:user-e,namespace:none,query:projectsornamespaces!=test-prj-1,test-ns-6,test-ns-8",
			user:        "user-e",
			query:       "projectsornamespaces!=test-prj-1,test-ns-6,test-ns-8",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
				{"name": "test1", "namespace": "test-ns-2"},
				{"name": "test2", "namespace": "test-ns-2"},
				{"name": "test3", "namespace": "test-ns-2"},
				{"name": "test4", "namespace": "test-ns-2"},
				{"name": "test5", "namespace": "test-ns-2"},
				{"name": "test1", "namespace": "test-ns-3"},
				{"name": "test2", "namespace": "test-ns-3"},
				{"name": "test3", "namespace": "test-ns-3"},
				{"name": "test4", "namespace": "test-ns-3"},
				{"name": "test5", "namespace": "test-ns-3"},
				{"name": "test1", "namespace": "test-ns-4"},
				{"name": "test2", "namespace": "test-ns-4"},
				{"name": "test3", "namespace": "test-ns-4"},
				{"name": "test4", "namespace": "test-ns-4"},
				{"name": "test5", "namespace": "test-ns-4"},
				{"name": "test1", "namespace": "test-ns-5"},
				{"name": "test2", "namespace": "test-ns-5"},
				{"name": "test3", "namespace": "test-ns-5"},
				{"name": "test4", "namespace": "test-ns-5"},
				{"name": "test5", "namespace": "test-ns-5"},
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
				{"name": "test1", "namespace": "test-ns-8"},
				{"name": "test2", "namespace": "test-ns-8"},
			},
			expectExcludes: true,
		},
		{
			description: "user:user-e,namespace:test-ns-6,query:none",
			user:        "user-e",
			namespace:   "test-ns-6",
			query:       "",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
			},
		},
		{
			description: "user:user-e,namespace:test-ns-6,query:projectsornamespaces=test-prj-2",
			user:        "user-e",
			namespace:   "test-ns-6",
			query:       "projectsornamespaces=test-prj-2",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-6"},
				{"name": "test2", "namespace": "test-ns-6"},
			},
		},
		{
			description: "user:user-e,namespace:test-ns-6,query:projectsornamespaces=test-prj-1",
			user:        "user-e",
			namespace:   "test-ns-6",
			query:       "projectsornamespaces=test-prj-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-e,namespace:test-ns-1,query:projectsornamespaces=test-ns-1,test-ns-2,test-prj-2,test-ns-7",
			user:        "user-e",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces=test-ns-1,test-ns-2,test-prj-2,test-ns-7",
			expect: []map[string]string{
				{"name": "test1", "namespace": "test-ns-1"},
				{"name": "test2", "namespace": "test-ns-1"},
				{"name": "test3", "namespace": "test-ns-1"},
				{"name": "test4", "namespace": "test-ns-1"},
				{"name": "test5", "namespace": "test-ns-1"},
			},
		},
		{
			description: "user:user-e,namespace:test-ns-1,query:projectsornamespaces!=test-prj-1",
			user:        "user-e",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces!=test-prj-1",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-e,namespace:test-ns-1,query:projectsornamespaces!=test-prj-1,test-prj-2",
			user:        "user-e",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces!=test-prj-1,test-prj-2",
			expect:      []map[string]string{},
		},
		{
			description: "user:user-e,namespace:test-ns-1,query:projectsornamespaces!=test-prj-1,test-ns-2,test-ns-8",
			user:        "user-e",
			namespace:   "test-ns-1",
			query:       "projectsornamespaces!=test-prj-1,test-ns-2,test-ns-8",
			expect:      []map[string]string{},
		},
	}
	if !usingSQLCache {
		tests = append(tests, nonSQLListTests...)
	} else {
		tests = append(tests, SQLOnlyListTests...)
		// map labelSelector and fieldSelector params to the VAI equivalents
		// ensure metadata.namespace tests are doing partial matching because
		// the actual namespaces are given an `auto` prefix and a random suffix
		for i, test := range tests {
			query := test.query
			parts := strings.Split(query, "&")
			changed := false
			for j, part := range parts {
				subparts := strings.Split(part, "=")
				if subparts[0] == "labelSelector" {
					parts[j] = fmt.Sprintf("filter=metadata.labels[%s]=%s", subparts[1], subparts[2])
					changed = true
				} else if subparts[0] == "fieldSelector" {
					op := "="
					if subparts[1] == "metadata.namespace" {
						// Use the partial-match operator because actual namespaces have a random prefix and suffix
						op = "~"
					}
					parts[j] = fmt.Sprintf("filter=%s%s%s", subparts[1], op, subparts[2])
					changed = true
				} else if subparts[0] == "filter" {
					if strings.Contains(part, "metadata.namespace=") {
						// No need to break the filter down into sub-filters because in the test suite we don't
						// have any VALUES that match 'metadata.namespace='
						changed = true
						parts[j] = strings.ReplaceAll(part, "metadata.namespace=", "metadata.namespace~")
					}
				}
			}
			if changed {
				query = strings.Join(parts, "&")
				tests[i].query = query
			}
		}
	}

	var csvWriter *csv.Writer
	var jsonDir string
	if s.clusterID == "local" {
		var fp *os.File
		var err error
		csvWriter, fp, jsonDir, err = setUpResults()
		defer fp.Close()
		defer func() {
			csvWriter.Flush()
			require.NoError(s.T(), csvWriter.Error())
		}()
		require.NoError(s.T(), err)
	}

	for _, test := range tests {
		s.Run(test.description, func() {
			userClient := s.userClients[test.user]

			client, err := userClient.Steve.ProxyDownstream(s.clusterID)
			require.NoError(s.T(), err)
			var secretClient clientv1.SteveOperations
			secretClient = client.SteveType(stevesecrets.SecretSteveType)
			if test.namespace != "" {
				secretClient = secretClient.(*clientv1.SteveClient).NamespacedSteveClient(namespaceMap[test.namespace])
			}
			query, err := url.ParseQuery(test.query)
			require.NoError(s.T(), err)
			if _, ok := query["sort"]; !ok && s.clusterID != "local" {
				// k8s does not guarantee any particular order but usually returns results sorted by namespace and name.
				// k3d seems to have its own ideas, so we can't rely on a consistent order when testing on the downstream cluster.
				query["sort"] = []string{"metadata.namespace,metadata.name"}
			}
			if _, ok := query["continue"]; ok {
				query["continue"] = []string{s.lastContinueToken}
			}
			if fs, ok := query["fieldSelector"]; ok {
				if strings.Contains(fs[0], "metadata.namespace") {
					fieldParts := strings.Split(fs[0], "=")
					ns := namespaceMap[fieldParts[1]]
					query["fieldSelector"] = []string{"metadata.namespace=" + ns}
				}
			}
			key := "projectsornamespaces"
			projectsOrNamespaces, ok := query[key]
			if !ok {
				key += "!"
				projectsOrNamespaces = query[key]
			}
			if len(projectsOrNamespaces) != 0 {
				groups := projectTag.FindAllStringSubmatch(projectsOrNamespaces[0], -1)
				for _, g := range groups {
					name := string(g[1])
					projectID := projectMap[name].ID
					projectID = strings.Split(projectID, ":")[1]
					projectsOrNamespaces[0] = strings.ReplaceAll(projectsOrNamespaces[0], name, projectID)
				}
				groups = namespaceTag.FindAllStringSubmatch(projectsOrNamespaces[0], -1)
				for _, g := range groups {
					name := string(g[1])
					projectsOrNamespaces[0] = strings.ReplaceAll(projectsOrNamespaces[0], name, namespaceMap[name])
				}
				query[key] = projectsOrNamespaces
			}
			if _, ok := query["revision"]; ok {
				query["revision"] = []string{s.lastRevision}
			}
			if usingSQLCache {
				query["filter"] = append(query["filter"], fmt.Sprintf("metadata.labels[%s]~%s", steveAPITestLabel, testID))
			} else {
				query["labelSelector"] = append(query["labelSelector"], steveAPITestLabel+"="+testID)
			}
			secretList, err := secretClient.List(query)
			require.NoError(s.T(), err)

			if secretList.Continue != "" {
				s.lastContinueToken = secretList.Continue
			}
			s.lastRevision = secretList.Revision

			if test.expectContains {
				s.assertListContains(test.expect, secretList.Data)
			} else if test.expectExcludes {
				s.assertListExcludes(test.expect, secretList.Data)
			} else {
				s.assertListIsEqual(test.expect, secretList.Data)
			}
			if test.expectSummary != nil {
				require.NotNil(s.T(), secretList.Summary)
				s.assertSummariesMatch(test.expectSummary, secretList.Summary)
			}

			// Write human-readable request and response examples
			if s.clusterID == "local" {
				curlURL, err := getCurlURL(client, test.namespace, test.query)
				require.NoError(s.T(), err)
				if containsSortName.MatchString(test.query) && !containsSortNamespace.MatchString(test.query) {
					// We're getting objects with the same name returned in random order based on namespace,
					// so save them consistently w.r.t their namespace
					multiplier := 1
					if containsReverseOrderSortName.MatchString(test.query) {
						multiplier = -1
					}
					steveAPIObjects := make([]*clientv1.SteveAPIObject, len(secretList.Data))
					for i, secret := range secretList.Data {
						steveAPIObjects[i] = &secret
					}
					isSorted := slices.IsSortedFunc(steveAPIObjects, func(x, y *clientv1.SteveAPIObject) int {
						return multiplier * strings.Compare(x.Name, y.Name)
					})
					assert.True(s.T(), isSorted, "secretList.Data is not sorted by name")
					secretList.Data = slices.SortedStableFunc(slices.Values(secretList.Data),
						func(x, y clientv1.SteveAPIObject) int {
							nameDiff := strings.Compare(x.Name, y.Name)
							if nameDiff != 0 {
								return multiplier * nameDiff
							}
							return multiplier * strings.Compare(x.Namespace, y.Namespace)
						})
				}
				for _, steveAPIObj := range secretList.Data {
					fields := steveAPIObj.Fields
					if len(fields) > 3 {
						fieldValue := fields[3].(string)
						if fieldValue != "0s" && relativeDateRx.MatchString(fieldValue) {
							fields[3] = "0s"
						}
					}
				}
				pagination := secretList.Pagination
				if pagination != nil {
					pagination.First = containsNamespaceTag.ReplaceAllString(pagination.First, replacementNamespaceTag)
					pagination.Next = containsNamespaceTag.ReplaceAllString(pagination.Next, replacementNamespaceTag)
				}

				jsonResp, err := formatJSON(secretList)
				require.NoError(s.T(), err)
				jsonFilePath := filepath.Join(jsonDir, getFileName(test.user, test.namespace, test.query))
				err = writeResp(csvWriter, test.user, curlURL, jsonFilePath, jsonResp)
				require.NoError(s.T(), err)
			}
		})
	}
}

func getFileName(user, ns, query string) string {
	if user == "" {
		user = "none"
	}
	if ns == "" {
		ns = "none"
	}
	if query == "" {
		query = "none"
	} else {
		query = strings.ReplaceAll(query, "/", "%2F")
	}
	return user + "_" + ns + "_" + query + ".json"
}

func getCurlURL(client *clientv1.Client, namespace, query string) (string, error) {
	curlURL, err := client.APIBaseClient.Ops.GetCollectionURL(stevesecrets.SecretSteveType, "GET")
	if err != nil {
		return "", err
	}
	if namespace != "" {
		curlURL += "/" + namespace
	}
	if query != "" {
		curlURL += "?" + query
	}

	curlURL = urlRegex.ReplaceAllString(curlURL, defautlUrlString)
	return curlURL, nil
}

func formatJSON(obj *clientv1.SteveCollection) ([]byte, error) {
	jsonResp, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var mapResp map[string]interface{}
	err = json.Unmarshal(jsonResp, &mapResp)
	if err != nil {
		return nil, err
	}

	mapResp["revision"] = "100"
	if _, ok := mapResp["continue"]; ok {
		mapResp["continue"] = continueToken
	}
	if pagination, ok := mapResp["pagination"].(map[string]interface{}); ok {
		if next, ok := pagination["next"].(string); ok {
			next = continueReg.ReplaceAllString(next, "${1}"+continueToken)
			next = revisionReg.ReplaceAllString(next, "${1}"+revisionNum)
			next = testLabelReg.ReplaceAllString(next, "${1}"+fakeTestID)
			pagination["next"] = next
			mapResp["pagination"] = pagination
		}
	}
	data, ok := mapResp["data"].([]interface{})
	if ok {
		for i := range data {
			delete(data[i].(map[string]interface{}), "JSONResp")
			delete(data[i].(map[string]interface{})["metadata"].(map[string]interface{}), "creationTimestamp")
			delete(data[i].(map[string]interface{})["metadata"].(map[string]interface{}), "managedFields")
			delete(data[i].(map[string]interface{})["metadata"].(map[string]interface{}), "uid")
			data[i].(map[string]interface{})["metadata"].(map[string]interface{})["labels"].(map[string]interface{})[steveAPITestLabel] = fakeTestID
			data[i].(map[string]interface{})["metadata"].(map[string]interface{})["resourceVersion"] = "1000"
		}
		mapResp["data"] = data
	}
	jsonBytes, err := json.MarshalIndent(mapResp, "", "  ")
	if err != nil {
		return nil, err
	}
	jsonString := string(jsonBytes)
	for k, v := range namespaceMap {
		jsonString = strings.ReplaceAll(jsonString, v, k)
	}
	jsonString = urlRegex.ReplaceAllString(jsonString, defautlUrlString)
	return []byte(jsonString), nil
}

func setUpResults() (*csv.Writer, *os.File, string, error) {
	outputFile := "output.csv"
	fields := []string{"user", "url", "response"}
	csvFile, err := os.OpenFile(outputFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, nil, "", err
	}
	csvWriter := csv.NewWriter(bufio.NewWriter(csvFile))
	csvWriter.Write(fields)
	if csvWriter.Error() != nil {
		return nil, csvFile, "", err
	}
	jsonDir := "json"
	err = os.MkdirAll(jsonDir, 0755)
	if err != nil {
		return nil, csvFile, "", err
	}
	return csvWriter, csvFile, jsonDir, nil
}

func writeResp(csvWriter *csv.Writer, user, url, path string, resp []byte) error {
	jsonFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	jsonFile.Write(resp)
	csvWriter.Write([]string{user, url, fmt.Sprintf("[%s](%s)", path, path)})
	return nil
}

func (s *steveAPITestSuite) TestLinks() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.Steve.ProxyDownstream(s.clusterID)
	require.NoError(s.T(), err)

	secretClient := client.SteveType(stevesecrets.SecretSteveType)

	secretObj, err := secretClient.Create(corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namegenerator.AppendRandomString("steve-secret-squirrel"),
			Namespace: namespaceMap["test-ns-1"],
		},
		Data: map[string][]byte{"foo": []byte("bar")},
	})
	require.NoError(s.T(), err)

	readObj, err := secretClient.ByID(secretObj.ID)
	require.NoError(s.T(), err)

	host := s.client.RancherConfig.Host

	id := readObj.JSONResp["id"].(string)
	expectedID := secretObj.Namespace + "/" + secretObj.Name
	links := readObj.JSONResp["links"].(map[string]any)
	expectedLinks := map[string]any{
		"patch":  fmt.Sprintf("https://%s/v1/secrets/%s", host, expectedID),
		"remove": fmt.Sprintf("https://%s/v1/secrets/%s", host, expectedID),
		"update": fmt.Sprintf("https://%s/v1/secrets/%s", host, expectedID),
		"self":   fmt.Sprintf("https://%s/v1/secrets/%s", host, expectedID),
		"view":   fmt.Sprintf("https://%s/api/v1/namespaces/%s/secrets/%s", host, secretObj.Namespace, secretObj.Name),
	}

	// delete
	err = secretClient.Delete(readObj)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), expectedID, id)
	assert.Equal(s.T(), expectedLinks, links)
}

func (s *steveAPITestSuite) TestCRUD() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.Steve.ProxyDownstream(s.clusterID)
	require.NoError(s.T(), err)

	s.Run("global", func() {
		secretClient := client.SteveType(stevesecrets.SecretSteveType)

		// create
		secretObj, err := secretClient.Create(corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      namegenerator.AppendRandomString("steve-secret-garden"),
				Namespace: namespaceMap["test-ns-1"], // need to specify the namespace for a namespaced resource if using a global endpoint ("/v1/secrets")
			},
			Data: map[string][]byte{"foo": []byte("bar")},
		})
		require.NoError(s.T(), err)

		// read
		readObj, err := secretClient.ByID(secretObj.ID)
		require.NoError(s.T(), err)
		assert.Contains(s.T(), readObj.JSONResp["data"], "foo")

		// update
		updatedSecret := secretObj.JSONResp
		updatedSecret["data"] = map[string][]byte{"lorem": []byte("ipsum")}
		secretObj, err = secretClient.Update(secretObj, &updatedSecret)
		require.NoError(s.T(), err)

		// read again
		readObj, err = secretClient.ByID(secretObj.ID)
		require.NoError(s.T(), err)
		assert.Contains(s.T(), readObj.JSONResp["data"], "lorem")

		// delete
		err = secretClient.Delete(readObj)
		require.NoError(s.T(), err)

		// read again
		readObj, err = secretClient.ByID(secretObj.ID)
		require.Error(s.T(), err)
		assert.Nil(s.T(), readObj)
	})

	s.Run("namespaced", func() {
		secretClient := client.SteveType(stevesecrets.SecretSteveType).NamespacedSteveClient(namespaceMap["test-ns-1"])

		// create
		secretObj, err := secretClient.Create(corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: namegenerator.AppendRandomString("steve-secret-six"),
				// no need to provide a namespace since using a namespaced endpoint ("/v1/secrets/test-ns-1")
			},
			Data: map[string][]byte{"foo": []byte("bar")},
		})
		require.NoError(s.T(), err)

		// read
		readObj, err := secretClient.ByID(secretObj.ID)
		require.NoError(s.T(), err)
		assert.Contains(s.T(), readObj.JSONResp["data"], "foo")

		// update
		updatedSecret := secretObj.JSONResp
		updatedSecret["data"] = map[string][]byte{"lorem": []byte("ipsum")}
		secretObj, err = secretClient.Update(secretObj, &updatedSecret)
		require.NoError(s.T(), err)

		// read again
		readObj, err = secretClient.ByID(secretObj.ID)
		require.NoError(s.T(), err)
		assert.Contains(s.T(), readObj.JSONResp["data"], "lorem")

		// delete
		err = secretClient.Delete(readObj)
		require.NoError(s.T(), err)

		// read again
		readObj, err = secretClient.ByID(secretObj.ID)
		require.Error(s.T(), err)
		assert.Nil(s.T(), readObj)
	})
}

func (s *steveAPITestSuite) assertListIsEqual(expectedList []map[string]string, receivedList []clientv1.SteveAPIObject) {
	assert.Equal(s.T(), len(expectedList), len(receivedList))
	receivedSubset := make([]map[string]string, len(receivedList))
	includeNamespace := false
	if len(expectedList) > 0 {
		_, includeNamespace = expectedList[0]["namespace"]
	}

	for i, r := range receivedList {
		vals := map[string]string{"name": r.Name}
		if includeNamespace {
			vals["namespace"] = r.Namespace
		} else {
			vals["namespace"] = ""
		}
		receivedSubset[i] = vals
	}
	expectedSubset := make([]map[string]string, len(expectedList))
	for i, w := range expectedList {
		vals := map[string]string{"name": w["name"]}
		if includeNamespace {
			vals["namespace"] = namespaceMap[w["namespace"]]
		} else {
			vals["namespace"] = ""
		}
		expectedSubset[i] = vals
	}
	assert.Equal(s.T(), expectedSubset, receivedSubset)
	length := len(expectedList)
	if length > len(receivedList) {
		length = len(receivedList)
	}
	for i := range length {
		w := expectedList[i]
		if name, ok := w["name"]; ok {
			assert.Equal(s.T(), name, receivedList[i].Name, fmt.Sprintf("diff at index %d: expecting name %q, got %q", i, name, receivedList[i].Name))
		}
		if ns, ok := w["namespace"]; includeNamespace && ok {
			assert.Equal(s.T(), namespaceMap[ns], receivedList[i].Namespace, fmt.Sprintf("diff at index %d: expecting namespace mapped:%q (raw:%q), got %q", i, namespaceMap[ns], ns, receivedList[i].Namespace))
		}
	}
}

// shepherd includes a filled `jsonResp` field in the clientv1.SteveAPISummaryItem payload,
// and it needs to be ignored
func (s *steveAPITestSuite) assertSummariesMatch(expectedSummaries []clientv1.SteveAPISummaryItem, receivedSummaries []clientv1.SteveAPISummaryItem) {
	assert.Equal(s.T(), len(expectedSummaries), len(receivedSummaries))
	fixExpectedSummaries := make([]clientv1.SteveAPISummaryItem, len(expectedSummaries))
	for i, s := range expectedSummaries {
		newS := clientv1.SteveAPISummaryItem{}
		newS.Property = s.Property
		if newS.Property == "metadata.namespace" {
			newS.Counts = make(map[string]int)
			for k, v := range s.Counts {
				newS.Counts[namespaceMap[k]] = v
			}
		} else {
			newS.Counts = s.Counts
		}
		fixExpectedSummaries[i] = newS
	}
	fixReceivedSummaries := make([]clientv1.SteveAPISummaryItem, len(receivedSummaries))
	for i, s := range receivedSummaries {
		// Drop the jsonResp
		newS := clientv1.SteveAPISummaryItem{}
		newS.Property = s.Property
		newS.Counts = s.Counts
		fixReceivedSummaries[i] = newS
	}
	assert.Equal(s.T(), fixExpectedSummaries, fixReceivedSummaries)
}

func (s *steveAPITestSuite) assertListContains(expect []map[string]string, list []clientv1.SteveAPIObject) {
	assert.GreaterOrEqual(s.T(), len(list), len(expect))
	matches := true
	for _, w := range expect {
		found := false
		for _, obj := range list {
			if obj.Name == w["name"] && obj.Namespace == namespaceMap[w["namespace"]] {
				found = true
				break
			}
		}
		if !found {
			matches = false
		}
	}
	assert.True(s.T(), matches, "list did not contain expected results")
}

func (s *steveAPITestSuite) assertListExcludes(expect []map[string]string, list []clientv1.SteveAPIObject) {
	found := false
	for _, w := range expect {
		for _, obj := range list {
			if obj.Name == w["name"] && obj.Namespace == namespaceMap[w["namespace"]] {
				found = true
				break
			}
		}
		if found == true {
			break
		}
	}
	assert.False(s.T(), found, "list contained unexpected results")
}

func (s *steveAPITestSuite) createKubeconfig(client *http.Client) *extv1.Kubeconfig {
	var err error

	if client == nil {
		client, err = rest.HTTPClientFor(s.client.WranglerContext.RESTConfig)
		require.NoError(s.T(), err)
	}

	kubeconfig := &extv1.Kubeconfig{}

	resp, err := client.Post(
		fmt.Sprintf("https://%s/v1/ext.cattle.io.kubeconfig", s.client.WranglerContext.RESTConfig.Host),
		"application/json",
		strings.NewReader(`
		{
			"apiVersion": "ext.cattle.io/v1",
			"kind": "kubeconfig",
			"metadata": {
				"name": "test-kubeconfig"
			},
			"spec": {
				"clusters": ["local"],
				"currentContent": "local",
				"description": "kubeconfig for testing new kubeconfigs",
				"ttl": 100
			}
		}`),
	)
	require.NoError(s.T(), err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(s.T(), err)

	require.NoError(s.T(), json.Unmarshal(body, kubeconfig))

	return kubeconfig
}

func retryRequest(fn func() error) error {
	retriable := func(err error) bool { return strings.Contains(err.Error(), "tunnel disconnect") }
	return retry.OnError(retry.DefaultBackoff, retriable, fn)
}

func TestSteveLocal(t *testing.T) {
	suite.Run(t, new(LocalSteveAPITestSuite))
}

func TestSteveDownstream(t *testing.T) {
	// TODO: Re-enable the test when the bug is fixed
	t.Skip()
	suite.Run(t, new(DownstreamSteveAPITestSuite))
}

func newExtensionAPIRestConfig(rancherConfig *rancher.Config, clusterID string, bearerToken string) *rest.Config {
	host := fmt.Sprintf("https://%s/ext", rancherConfig.Host)
	if clusterID != "" {
		host = fmt.Sprintf("https://%s/k8s/clusters/%s/ext", rancherConfig.Host, clusterID)
	}
	return &rest.Config{
		Host:        host,
		BearerToken: bearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: *rancherConfig.Insecure,
			CAFile:   rancherConfig.CAFile,
		},
	}
}

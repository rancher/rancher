package rbac

import (
	"fmt"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/apis/management.cattle.io"
	apisV3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/resourcequota"
	coreFakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v3fakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	wfakes "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

var (
	testNamespace1 = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns-1",
			Annotations: map[string]string{
				projectIDAnnotation: "c-123xyz:p-123xyz",
			},
		},
	}
	testNamespace2 = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns-2",
			Annotations: map[string]string{
				projectIDAnnotation: "c-123xyz:p-123xyz",
			},
		},
	}
	// The following namespace does not have any projectIDAnnotation
	unrelatedNamespace = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns-unrelated",
		},
	}
)

func TestReconcileNamespaceProjectClusterRole(t *testing.T) {
	getCR := createClusterRoleForProject("p-123xyz", testNamespace1.Name, "get")
	updateGetCR := addNamespaceToClusterRole(testNamespace2.Name, "get", getCR.DeepCopy())
	manageCR := createClusterRoleForProject("p-123xyz", testNamespace1.Name, manageNSVerb)
	updateManageCR := addNamespaceToClusterRole(testNamespace2.Name, manageNSVerb, manageCR.DeepCopy())
	noResourceNameCR := createClusterRoleForProject("p-123xyz", unrelatedNamespace.Name, "get")
	tests := []struct {
		name                 string
		namespace            *corev1.Namespace
		roleVerb             []string
		setupCRController    func(*wfakes.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList], *[]*rbacv1.ClusterRole, *[]string)
		setupCRLister        func(*wfakes.MockNonNamespacedCacheInterface[*rbacv1.ClusterRole])
		crIndexer            *FakeResourceIndexer[*rbacv1.ClusterRole]
		currentRoles         []*rbacv1.ClusterRole
		wantRoles            []*rbacv1.ClusterRole
		wantDeletedRoleNames []string
		wantErr              bool
		wantErrMessage       string
	}{
		{
			name:      "create read-only role",
			namespace: &testNamespace1,
			roleVerb:  []string{"get"},
			setupCRController: func(c *wfakes.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList], finalRoles *[]*rbacv1.ClusterRole, deletedRoleNames *[]string) {
				c.EXPECT().Create(gomock.Any()).DoAndReturn(
					func(role *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
						created := getCR.DeepCopy()
						*finalRoles = append(*finalRoles, created)
						return created, nil
					})
			},
			setupCRLister: func(l *wfakes.MockNonNamespacedCacheInterface[*rbacv1.ClusterRole]) {
				l.EXPECT().Get(gomock.Any()).Return(nil, nil)
			},
			crIndexer: &FakeResourceIndexer[*rbacv1.ClusterRole]{
				index:     crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{},
				err:       nil,
			},
			wantRoles: []*rbacv1.ClusterRole{
				getCR,
			},
			wantDeletedRoleNames: []string{},
			wantErr:              false,
		},
		{
			name:      "update existing read-only role",
			namespace: &testNamespace2,
			roleVerb:  []string{"get"},
			setupCRController: func(c *wfakes.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList], finalRoles *[]*rbacv1.ClusterRole, deletedRoleNames *[]string) {
				c.EXPECT().Update(gomock.Any()).DoAndReturn(
					func(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
						updated := updateGetCR.DeepCopy()
						*finalRoles = append(*finalRoles, updated)
						return updated, nil
					}).Times(1)
			},
			setupCRLister: func(l *wfakes.MockNonNamespacedCacheInterface[*rbacv1.ClusterRole]) {
				l.EXPECT().Get("p-123xyz-namespaces-readonly").Return(getCR.DeepCopy(), nil)
			},
			crIndexer: &FakeResourceIndexer[*rbacv1.ClusterRole]{
				index: crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{
					testNamespace2.Name: {getCR.DeepCopy()},
				},
				err: nil,
			},
			wantRoles: []*rbacv1.ClusterRole{
				updateGetCR,
			},
			wantDeletedRoleNames: []string{},
			wantErr:              false,
		},
		{
			name:      "create manage-ns role",
			namespace: &testNamespace1,
			roleVerb:  []string{manageNSVerb},
			setupCRController: func(c *wfakes.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList], finalRoles *[]*rbacv1.ClusterRole, deletedRoleNames *[]string) {
				c.EXPECT().Create(gomock.Any()).DoAndReturn(
					func(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
						created := manageCR.DeepCopy()
						*finalRoles = append(*finalRoles, created)
						return created, nil
					})
			},
			setupCRLister: func(l *wfakes.MockNonNamespacedCacheInterface[*rbacv1.ClusterRole]) {
				l.EXPECT().Get(gomock.Any()).Return(nil, nil)
			},
			crIndexer: &FakeResourceIndexer[*rbacv1.ClusterRole]{
				index:     crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{},
				err:       nil,
			},
			wantRoles: []*rbacv1.ClusterRole{
				manageCR,
			},
			wantDeletedRoleNames: []string{},
			wantErr:              false,
		},
		{
			name:      "update get & create manage-ns role",
			namespace: &testNamespace2,
			roleVerb:  []string{"get", manageNSVerb},
			setupCRController: func(c *wfakes.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList], finalRoles *[]*rbacv1.ClusterRole, deletedRoleNames *[]string) {
				c.EXPECT().Create(gomock.Any()).DoAndReturn(
					func(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
						created := createClusterRoleForProject("p-123xyz", testNamespace2.Name, manageNSVerb)
						*finalRoles = append(*finalRoles, created)
						return created, nil
					})
				c.EXPECT().Update(gomock.Any()).DoAndReturn(
					func(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
						updated := updateGetCR.DeepCopy()
						*finalRoles = append(*finalRoles, updated)
						return updated, nil
					})
			},
			setupCRLister: func(l *wfakes.MockNonNamespacedCacheInterface[*rbacv1.ClusterRole]) {
				l.EXPECT().Get(gomock.Any()).Return(getCR.DeepCopy(), nil)
				l.EXPECT().Get(gomock.Any()).Return(nil, nil)
			},
			crIndexer: &FakeResourceIndexer[*rbacv1.ClusterRole]{
				index: crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{
					testNamespace1.Name: {getCR.DeepCopy()},
				},
				err: nil,
			},
			wantRoles: []*rbacv1.ClusterRole{
				updateGetCR,
				createClusterRoleForProject("p-123xyz", testNamespace2.Name, manageNSVerb),
			},
			wantDeletedRoleNames: []string{},
			wantErr:              false,
		},
		{
			name:      "update get & manage-ns role",
			namespace: &testNamespace2,
			roleVerb:  []string{"get", manageNSVerb},
			setupCRController: func(c *wfakes.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList], finalRoles *[]*rbacv1.ClusterRole, deletedRoleNames *[]string) {
				c.EXPECT().Update(gomock.Any()).DoAndReturn(
					func(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
						updated := updateGetCR.DeepCopy()
						*finalRoles = append(*finalRoles, updated)
						return updated, nil
					})
				c.EXPECT().Update(gomock.Any()).DoAndReturn(
					func(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
						updated := updateManageCR.DeepCopy()
						*finalRoles = append(*finalRoles, updated)
						return updated, nil
					})
			},
			setupCRLister: func(l *wfakes.MockNonNamespacedCacheInterface[*rbacv1.ClusterRole]) {
				l.EXPECT().Get(gomock.Any()).Return(getCR.DeepCopy(), nil)
				l.EXPECT().Get(gomock.Any()).Return(manageCR.DeepCopy(), nil)
			},
			crIndexer: &FakeResourceIndexer[*rbacv1.ClusterRole]{
				index: crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{
					testNamespace1.Name: {getCR.DeepCopy(), manageCR.DeepCopy()},
				},
				err: nil,
			},
			wantRoles: []*rbacv1.ClusterRole{
				updateGetCR,
				updateManageCR,
			},
			wantDeletedRoleNames: []string{},
			wantErr:              false,
		},
		{
			name:      "delete a role with no resourceName",
			namespace: &unrelatedNamespace,
			roleVerb:  []string{"get"},
			setupCRController: func(c *wfakes.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList], finalRoles *[]*rbacv1.ClusterRole, deletedRoleNames *[]string) {
				c.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(
					func(name string, opts *metav1.DeleteOptions) error {
						*deletedRoleNames = append(*deletedRoleNames, noResourceNameCR.Name)
						return nil
					})
			},
			setupCRLister: func(l *wfakes.MockNonNamespacedCacheInterface[*rbacv1.ClusterRole]) {},
			crIndexer: &FakeResourceIndexer[*rbacv1.ClusterRole]{
				index: crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{
					unrelatedNamespace.Name: {noResourceNameCR.DeepCopy()},
				},
				err: nil,
			},
			wantRoles: []*rbacv1.ClusterRole{},
			wantDeletedRoleNames: []string{
				noResourceNameCR.Name,
			},
			wantErr: false,
		},
		{
			name:              "indexer error",
			namespace:         &testNamespace1,
			roleVerb:          []string{"get"},
			setupCRController: nil,
			setupCRLister:     nil,
			crIndexer: &FakeResourceIndexer[*rbacv1.ClusterRole]{
				index:     crByNSIndex,
				resources: nil,
				err:       fmt.Errorf("unable to read from indexer"),
			},
			wantErr:        true,
			wantErrMessage: "unable to read from indexer",
		},
		{
			name:      "update error",
			namespace: &testNamespace2,
			roleVerb:  []string{"get"},
			setupCRController: func(c *wfakes.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList], finalRoles *[]*rbacv1.ClusterRole, deletedRoleNames *[]string) {
				c.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("unable to update"))
			},
			setupCRLister: func(l *wfakes.MockNonNamespacedCacheInterface[*rbacv1.ClusterRole]) {
				l.EXPECT().Get("p-123xyz-namespaces-readonly").Return(getCR.DeepCopy(), nil)
			},
			crIndexer: &FakeResourceIndexer[*rbacv1.ClusterRole]{
				index: crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{
					testNamespace1.Name: {getCR.DeepCopy()},
				},
				err: nil,
			},
			wantErr:        true,
			wantErrMessage: "unable to update",
		},
		{
			name:      "create error",
			namespace: &testNamespace1,
			roleVerb:  []string{"get"},
			setupCRController: func(c *wfakes.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList], finalRoles *[]*rbacv1.ClusterRole, deletedRoleNames *[]string) {
				c.EXPECT().Create(gomock.Any()).Return(nil, fmt.Errorf("unable to create"))
			},
			setupCRLister: func(l *wfakes.MockNonNamespacedCacheInterface[*rbacv1.ClusterRole]) {
				l.EXPECT().Get(gomock.Any()).Return(nil, nil)
			},
			crIndexer: &FakeResourceIndexer[*rbacv1.ClusterRole]{
				index:     crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{},
				err:       nil,
			},
			wantErr:        true,
			wantErrMessage: "unable to create",
		},
		{
			name:      "delete error",
			namespace: &unrelatedNamespace,
			roleVerb:  []string{"get"},
			setupCRController: func(c *wfakes.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList], finalRoles *[]*rbacv1.ClusterRole, deletedRoleNames *[]string) {
				c.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(fmt.Errorf("unable to delete"))
			},
			setupCRLister: nil,
			crIndexer: &FakeResourceIndexer[*rbacv1.ClusterRole]{
				index: crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{
					unrelatedNamespace.Name: {noResourceNameCR.DeepCopy()},
				},
				err: nil,
			},
			wantErr:        true,
			wantErrMessage: "unable to delete",
		},
		{
			name:              "get error",
			namespace:         &testNamespace1,
			roleVerb:          []string{"get"},
			setupCRController: nil,
			setupCRLister: func(l *wfakes.MockNonNamespacedCacheInterface[*rbacv1.ClusterRole]) {
				l.EXPECT().Get(gomock.Any()).Return(nil, fmt.Errorf("unable to get"))
			},
			crIndexer: &FakeResourceIndexer[*rbacv1.ClusterRole]{
				index:     crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{},
				err:       nil,
			},
			wantErr:        true,
			wantErrMessage: "unable to get",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCRController := wfakes.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			var finalRoles []*rbacv1.ClusterRole
			var deletedRoleNames []string
			if tc.setupCRController != nil {
				tc.setupCRController(mockCRController, &finalRoles, &deletedRoleNames)
			}

			mockCRLister := wfakes.NewMockNonNamespacedCacheInterface[*rbacv1.ClusterRole](ctrl)
			if tc.setupCRLister != nil {
				tc.setupCRLister(mockCRLister)
			}

			mockNSLifecycle := &nsLifecycle{
				m: &manager{
					crIndexer:    tc.crIndexer,
					crLister:     mockCRLister,
					clusterRoles: mockCRController,
				},
			}

			for _, verb := range tc.roleVerb {
				err := mockNSLifecycle.reconcileNamespaceProjectClusterRole(tc.namespace, verb)
				if tc.wantErr {
					if assert.Error(t, err, "expected error but got none") && tc.wantErrMessage != "" {
						assert.EqualError(t, err, tc.wantErrMessage)
					}
				} else {
					assert.NoError(t, err, "expected no error but got one")
				}
			}
			assert.ElementsMatch(t, tc.wantRoles, finalRoles, "expected roles to match")
			assert.ElementsMatch(t, tc.wantDeletedRoleNames, deletedRoleNames, "expected deleted roles to match")
		})
	}
}

func TestCreateProjectNSRole(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	type testCase struct {
		description string
		verb        string
		namespace   string
		projectName string
		crSetup     func()
		startingCR  *rbacv1.ClusterRole
		expectedCR  *rbacv1.ClusterRole
		createError error
		expectedErr string
	}
	testCases := []testCase{
		{
			description: "create get role",
			verb:        "get",
			projectName: "p-123xyz",
			expectedCR: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-readonly",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-readonly",
					},
				},
			},
		},
		{
			description: "create edit role",
			verb:        "*",
			projectName: "p-123xyz",
			expectedCR: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						Verbs:         []string{"manage-namespaces"},
						Resources:     []string{"projects"},
						ResourceNames: []string{"p-123xyz"},
					},
				},
			},
		},
		{
			description: "do not change role if already exists and return AlreadyExists error",
			verb:        "*",
			projectName: "p-123xyz",
			expectedCR: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						Verbs:         []string{"manage-namespaces"},
						Resources:     []string{"projects"},
						ResourceNames: []string{"p-123xyz"},
					},
				},
			},
			startingCR: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						Verbs:         []string{"manage-namespaces"},
						Resources:     []string{"projects"},
						ResourceNames: []string{"p-123xyz"},
					},
				},
			},
			expectedErr: `roletemplates.management.cattle.io "p-123xyz-namespaces-edit" already exists`,
		},
		{
			description: "test should return non-AlreadyExists error",
			verb:        "*",
			projectName: "p-123xyz",
			createError: errors.NewInternalError(fmt.Errorf("some error")),
			expectedErr: "Internal error occurred: some error",
		},
	}
	for _, test := range testCases {
		clusterRoles := map[string]*rbacv1.ClusterRole{}
		if test.startingCR != nil {
			clusterRoles = map[string]*rbacv1.ClusterRole{
				test.startingCR.Name: test.startingCR,
			}
		}

		m := newManager(withClusterRoles(clusterRoles, &clientErrs{createError: test.createError}, ctrl), func(m *manager) {
			clusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			clusterRoles.EXPECT().Create(gomock.Any()).DoAndReturn(
				func(In1 *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
					if test.expectedErr != "" {
						return nil, fmt.Errorf("%v", test.expectedErr)
					}
					return In1, nil
				},
			).Times(1)
			m.clusterRoles = clusterRoles
		})

		roleName := fmt.Sprintf(projectNSGetClusterRoleNameFmt, test.projectName, projectNSVerbToSuffix[test.verb])
		err := m.createProjectNSRole(roleName, test.verb, test.namespace, test.projectName)

		if test.expectedErr != "" {
			assert.ErrorContains(t, err, test.expectedErr, test.description)
		} else {
			assert.NoError(t, err)
		}
	}
}

func createClusterRoleForProject(projectName string, namespace string, verb string) *rbacv1.ClusterRole {
	cr := createBaseClusterRoleForProject(projectName, verb)
	return addNamespaceToClusterRole(namespace, verb, cr)
}

func createBaseClusterRoleForProject(projectName string, verb string) *rbacv1.ClusterRole {
	roleName := createRoleName(projectName, verb)
	newCR := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Annotations: map[string]string{
				projectNSAnn: roleName,
			},
		},
	}
	return newCR.DeepCopy()
}

func addNamespaceToClusterRole(namespace string, verb string, clusterRole *rbacv1.ClusterRole) *rbacv1.ClusterRole {
	if clusterRole.Rules == nil {
		clusterRole.Rules = []rbacv1.PolicyRule{}
	}
	foundIdx := -1
	// aggregate to a single rule for all NS if we can
	for i, rule := range clusterRole.Rules {
		hasApiGroup := false
		for _, apiGroup := range rule.APIGroups {
			if apiGroup == "" {
				hasApiGroup = true
				break
			}
		}
		if !hasApiGroup {
			continue
		}

		hasNamespaces := false
		for _, resource := range rule.Resources {
			if resource == "namespaces" {
				hasNamespaces = true
				break
			}
		}
		if !hasNamespaces {
			continue
		}
		foundIdx = i
	}
	if foundIdx >= 0 {
		clusterRole.Rules[foundIdx].ResourceNames = append(clusterRole.Rules[foundIdx].ResourceNames, namespace)
		return clusterRole
	}
	rule := rbacv1.PolicyRule{
		APIGroups:     []string{""},
		Verbs:         []string{verb},
		Resources:     []string{"namespaces"},
		ResourceNames: []string{namespace},
	}
	clusterRole.Rules = append(clusterRole.Rules, rule)

	return clusterRole
}

func addManagePermissionToClusterRole(projectName string, clusterRole *rbacv1.ClusterRole) *rbacv1.ClusterRole {
	if clusterRole.Rules == nil {
		clusterRole.Rules = []rbacv1.PolicyRule{}
	}
	rule := rbacv1.PolicyRule{
		APIGroups:     []string{management.GroupName},
		Verbs:         []string{manageNSVerb},
		Resources:     []string{apisV3.ProjectResourceName},
		ResourceNames: []string{projectName},
	}
	clusterRole.Rules = append(clusterRole.Rules, rule)
	return clusterRole
}

func createRoleName(projectName string, verb string) string {
	return fmt.Sprintf(projectNSGetClusterRoleNameFmt, projectName, projectNSVerbToSuffix[verb])
}

type FakeResourceIndexer[T runtime.Object] struct {
	cache.Store
	resources map[string][]T
	err       error
	index     string
}

func (d *FakeResourceIndexer[T]) Index(indexName string, obj interface{}) ([]interface{}, error) {
	return nil, nil
}

func (d *FakeResourceIndexer[T]) IndexKeys(indexName, indexKey string) ([]string, error) {
	return []string{}, nil
}

func (d *FakeResourceIndexer[T]) ListIndexFuncValues(indexName string) []string {
	return []string{}
}

func (d *FakeResourceIndexer[T]) ByIndex(indexName, indexKey string) ([]interface{}, error) {
	if indexName != d.index {
		return nil, fmt.Errorf("dummy indexer only supports %s for index name, %s given", d.index, indexName)
	}
	if d.err != nil {
		return nil, d.err
	}

	resources := d.resources[indexKey]
	var interfaces []interface{}
	for _, resource := range resources {
		interfaces = append(interfaces, resource)
	}

	return interfaces, nil
}

func (d *FakeResourceIndexer[T]) GetIndexers() cache.Indexers {
	return nil
}

func (d *FakeResourceIndexer[T]) AddIndexers(newIndexers cache.Indexers) error {
	return nil
}

func TestAsyncCleanupRBAC_NamespaceDeleted(t *testing.T) {
	tests := []struct {
		name                  string
		indexedRoles          []*rbacv1.ClusterRole
		nsGetCalls            int
		nsGetTerminatingCalls int
	}{
		{
			name:                  "namespace already deleted",
			nsGetCalls:            1,
			nsGetTerminatingCalls: 0,
		},
		{
			name:                  "namespace still terminating need to wait",
			nsGetCalls:            2,
			nsGetTerminatingCalls: 1,
		},
	}

	namespaceName := "test-namespace"
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			timesCalled := 0
			nsTerminatingCount := 0

			namespaceListerMock := &coreFakes.NamespaceListerMock{
				GetFunc: func(namespace string, name string) (*corev1.Namespace, error) {
					timesCalled++
					if nsTerminatingCount < test.nsGetTerminatingCalls {
						nsTerminatingCount++
						return &corev1.Namespace{
							TypeMeta: metav1.TypeMeta{},
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-namespace",
							},
							Status: corev1.NamespaceStatus{
								Phase: corev1.NamespaceTerminating,
							},
						}, nil
					}
					// indicate deleted namespace
					return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
				},
			}

			indexedRoles := []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "*"),
			}
			indexer := FakeResourceIndexer[*rbacv1.ClusterRole]{
				index: crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{
					namespaceName: indexedRoles,
				},
				err: nil,
			}

			fakeClusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			fakeClusterRoles.EXPECT().Update(gomock.Any()).DoAndReturn(
				func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
					return in, nil
				},
			).AnyTimes()
			fakeClusterRoles.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(
				func(name string, options *metav1.DeleteOptions) error {
					return nil
				},
			).AnyTimes()

			fakeLister := wfakes.NewMockNonNamespacedCacheInterface[*rbacv1.ClusterRole](ctrl)
			fakeLister.EXPECT().Get(gomock.Any()).DoAndReturn(
				func(int1 string) (*rbacv1.ClusterRole, error) {
					return &rbacv1.ClusterRole{}, nil
				},
			).AnyTimes()
			nsLifecycle := &nsLifecycle{
				m: &manager{
					nsLister:     namespaceListerMock,
					crLister:     fakeLister,
					crIndexer:    &indexer,
					clusterRoles: fakeClusterRoles,
				},
			}

			nsLifecycle.asyncCleanupRBAC(namespaceName)

			waitForCondition(t, func() bool {
				return timesCalled == test.nsGetCalls
			}, 15*time.Second, time.Second)
		})
	}
}

func waitForCondition(t *testing.T, condition func() bool, timeout time.Duration, interval time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("condition not met within %v", timeout)
}

func TestEnsurePRTBAddToNamespace(t *testing.T) {
	const namespaceName = "test-ns"
	ctrl := gomock.NewController(t)
	tests := []struct {
		name                string
		indexedRoles        []*rbacv1.ClusterRole
		currentRoles        []*rbacv1.ClusterRole
		indexedPRTBs        []*v3.ProjectRoleTemplateBinding
		projectNSAnnotation string
		indexerError        error
		updateError         error
		createError         error
		deleteError         error
		getError            error

		wantError    string
		wantHasPRTBs bool
	}{
		{
			name:                "update namespace with missing namespace",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "*"),
				addNamespaceToClusterRole("otherNamespace", "get", createClusterRoleForProject("p-123abc", namespaceName, "get")),
			},
			indexedPRTBs: []*v3.ProjectRoleTemplateBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "test-prtb",
					},
					UserName:         "test-user",
					ProjectName:      "test-cluster:test-project",
					RoleTemplateName: "test-rt",
				},
			},
			wantHasPRTBs: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			crIndexer := &FakeResourceIndexer[*rbacv1.ClusterRole]{
				resources: map[string][]*rbacv1.ClusterRole{
					namespaceName: test.indexedRoles,
				},
				index: crByNSIndex,
				err:   test.indexerError,
			}
			prtbIndexer := &FakeResourceIndexer[*v3.ProjectRoleTemplateBinding]{
				resources: map[string][]*v3.ProjectRoleTemplateBinding{
					"c-123xyz:p-123xyz": test.indexedPRTBs,
				},
				err:   test.indexerError,
				index: prtbByProjectIndex,
			}

			rtLister := wfakes.NewMockNonNamespacedCacheInterface[*v3.RoleTemplate](ctrl)
			rtLister.EXPECT().Get(gomock.Any()).DoAndReturn(
				func(name string) (*v3.RoleTemplate, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    "management.cattle.io",
						Resource: "roletemplates",
					}, name)
				},
			)
			lifecycle := nsLifecycle{
				m: &manager{
					crIndexer:   crIndexer,
					prtbIndexer: prtbIndexer,
					rtLister:    rtLister,
				},
				rq: &resourcequota.SyncController{
					ProjectLister: &v3fakes.ProjectListerMock{
						GetFunc: func(namespace string, name string) (*v3.Project, error) {
							return nil, apierrors.NewNotFound(schema.GroupResource{
								Group:    "management.cattle.io",
								Resource: "projects",
							}, name)
						},
					},
				},
			}
			hasPRTBs, err := lifecycle.ensurePRTBAddToNamespace(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
					Annotations: map[string]string{
						projectIDAnnotation: test.projectNSAnnotation,
					},
				},
			})
			assert.Equal(t, test.wantHasPRTBs, hasPRTBs)
			if test.wantError != "" {
				assert.ErrorContains(t, err, test.wantError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

package globalroles

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apisv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	rbacFakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/rancher/wrangler/v2/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

var (
	inheritedTestGr = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inherit-test-gr",
		},
		InheritedClusterRoles: []string{"cluster-owner"},
	}
	noInheritTestGR = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "noinherit-test-gr",
		},
		InheritedClusterRoles: []string{},
	}
	missingInheritTestGR = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "missing-test-gr",
		},
	}
	purgeTestGR = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "purge-inherit-test-gr",
		},
		InheritedClusterRoles: []string{"already-exists", "missing",
			"wrong-cluster-name", "wrong-user-name", "wrong-group-name",
			"deleting", "duplicate"},
	}
	notLocalCluster = v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-local",
		},
	}
	errorCluster = v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "error",
		},
	}
	localCluster = v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "local",
		},
	}
)

type testStateChanges struct {
	t                *testing.T
	createdCRTBs     []*v3.ClusterRoleTemplateBinding
	createdCRBs      []*v1.ClusterRoleBinding
	deletedCRTBNames []string
}
type testState struct {
	crtbCacheMock     *fake.MockCacheInterface[*v3.ClusterRoleTemplateBinding]
	grListerMock      *fakes.GlobalRoleListerMock
	crbListerMock     *rbacFakes.ClusterRoleBindingListerMock
	clusterListerMock *fakes.ClusterListerMock
	crtbClientMock    *fakes.ClusterRoleTemplateBindingInterfaceMock
	crbClientMock     *rbacFakes.ClusterRoleBindingInterfaceMock
	stateChanges      *testStateChanges
}

func TestCreateUpdate(t *testing.T) {
	// right now, create and update have the same input/output, so they are tested in the same way
	t.Parallel()
	readPodRule := v1.PolicyRule{
		APIGroups: []string{""},
		Resources: []string{"pods"},
		Verbs:     []string{"get", "list", "watch"},
	}
	readOnlyRoleName := "read-only"
	userName := "test-user"
	gr := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gr",
			UID:  "5678",
		},
		Rules:                 []v1.PolicyRule{readPodRule},
		InheritedClusterRoles: []string{readOnlyRoleName},
	}

	grb := v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-grb",
			UID:  "1234",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: apisv3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
			Kind:       apisv3.GlobalRoleBindingGroupVersionKind.Kind,
		},
		GlobalRoleName: gr.Name,
		UserName:       userName,
	}
	grbOwnerRef := metav1.OwnerReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       "GlobalRoleBinding",
		Name:       "test-grb",
		UID:        types.UID("1234"),
	}

	addAnnotation := func(grb *v3.GlobalRoleBinding) *v3.GlobalRoleBinding {
		newGRB := grb.DeepCopy()
		newGRB.Annotations = map[string]string{
			"authz.management.cattle.io/crb-name": "cattle-globalrolebinding-" + grb.Name,
		}
		return newGRB
	}

	tests := []struct {
		name            string
		stateSetup      func(testState)
		stateAssertions func(testStateChanges)
		inputBinding    *v3.GlobalRoleBinding
		wantBinding     *v3.GlobalRoleBinding
		wantError       bool
	}{
		{
			name: "success on both cluster and global permissions",
			stateSetup: func(state testState) {
				// mocks for both cluster and global permissions
				state.grListerMock.GetFunc = func(namespace, name string) (*v3.GlobalRole, error) {
					if name == gr.Name && namespace == "" {
						return &gr, nil
					}
					return nil, fmt.Errorf("not found")
				}
				// mocks for just cluster permissions
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					return nil
				}
				state.clusterListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Cluster, error) {
					return []*v3.Cluster{&notLocalCluster, &localCluster}, nil
				}

				// mocks for just global permissions
				state.crbListerMock.GetFunc = func(namespace, name string) (*v1.ClusterRoleBinding, error) {
					return nil, fmt.Errorf("not found")
				}
				state.crbClientMock.CreateFunc = func(crb *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
					state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
					return crb, nil
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 1)
				require.Len(stateChanges.t, stateChanges.createdCRBs, 1)
				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 0)

				// cluster assertions
				crtb := stateChanges.createdCRTBs[0]
				require.Equal(stateChanges.t, "not-local", crtb.ClusterName)
				require.Equal(stateChanges.t, "not-local", crtb.Namespace)
				require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
				require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
				require.Equal(stateChanges.t, readOnlyRoleName, crtb.RoleTemplateName)
				require.Equal(stateChanges.t, userName, crtb.UserName)
				require.Equal(stateChanges.t, "", crtb.GroupPrincipalName)
				require.Len(stateChanges.t, crtb.OwnerReferences, 1)
				require.Equal(stateChanges.t, grbOwnerRef, crtb.OwnerReferences[0])

				// global assertions
				crb := stateChanges.createdCRBs[0]
				bindingName := "cattle-globalrolebinding-" + grb.Name
				roleName := "cattle-globalrole-" + gr.Name
				require.Equal(stateChanges.t, bindingName, crb.Name)
				require.Len(stateChanges.t, crb.OwnerReferences, 1)
				require.Equal(stateChanges.t, grbOwnerRef, crb.OwnerReferences[0])
				require.Equal(stateChanges.t, v1.RoleRef{Name: roleName, Kind: "ClusterRole"}, crb.RoleRef)
				require.Equal(stateChanges.t, v1.Subject{Name: userName, Kind: "User", APIGroup: v1.GroupName}, crb.Subjects[0])
				require.Equal(stateChanges.t, "true", crb.Labels["authz.management.cattle.io/globalrolebinding"])

			},
			inputBinding: grb.DeepCopy(),
			wantBinding:  addAnnotation(&grb),
			wantError:    false,
		},
		{
			name: "success on global, failure on cluster",
			stateSetup: func(state testState) {
				// mocks for both cluster and global permissions
				state.grListerMock.GetFunc = func(namespace, name string) (*v3.GlobalRole, error) {
					if name == gr.Name && namespace == "" {
						return &gr, nil
					}
					return nil, fmt.Errorf("not found")
				}
				// mocks for just cluster permissions
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					return nil
				}
				state.clusterListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Cluster, error) {
					return nil, fmt.Errorf("server not available")
				}

				// mocks for just global permissions
				state.crbListerMock.GetFunc = func(namespace, name string) (*v1.ClusterRoleBinding, error) {
					return nil, fmt.Errorf("not found")
				}
				state.crbClientMock.CreateFunc = func(crb *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
					state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
					return crb, nil
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 0)
				require.Len(stateChanges.t, stateChanges.createdCRBs, 1)
				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 0)

				// global assertions
				crb := stateChanges.createdCRBs[0]
				bindingName := "cattle-globalrolebinding-" + grb.Name
				roleName := "cattle-globalrole-" + gr.Name
				require.Equal(stateChanges.t, bindingName, crb.Name)
				ownerRef := crb.OwnerReferences[0]
				require.Equal(stateChanges.t, "management.cattle.io/v3", ownerRef.APIVersion)
				require.Equal(stateChanges.t, "GlobalRoleBinding", ownerRef.Kind)
				require.Equal(stateChanges.t, "test-grb", ownerRef.Name)
				require.Equal(stateChanges.t, types.UID("1234"), ownerRef.UID)
				require.Equal(stateChanges.t, v1.RoleRef{Name: roleName, Kind: "ClusterRole"}, crb.RoleRef)
				require.Equal(stateChanges.t, v1.Subject{Name: userName, Kind: "User", APIGroup: v1.GroupName}, crb.Subjects[0])
				require.Equal(stateChanges.t, "true", crb.Labels["authz.management.cattle.io/globalrolebinding"])

			},
			inputBinding: grb.DeepCopy(),
			wantBinding:  addAnnotation(&grb),
			wantError:    true,
		},
		{
			name: "success on cluster, failure on global",
			stateSetup: func(state testState) {
				// mocks for both cluster and global permissions
				state.grListerMock.GetFunc = func(namespace, name string) (*v3.GlobalRole, error) {
					if name == gr.Name && namespace == "" {
						return &gr, nil
					}
					return nil, fmt.Errorf("not found")
				}
				// mocks for just cluster permissions
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					return nil
				}
				state.clusterListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Cluster, error) {
					return []*v3.Cluster{&notLocalCluster, &localCluster}, nil
				}

				// mocks for just global permissions
				state.crbListerMock.GetFunc = func(namespace, name string) (*v1.ClusterRoleBinding, error) {
					return nil, fmt.Errorf("not found")
				}
				state.crbClientMock.CreateFunc = func(crb *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
					state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
					return nil, fmt.Errorf("server not available")
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 1)
				require.Len(stateChanges.t, stateChanges.createdCRBs, 1)
				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 0)

				// cluster assertions
				crtb := stateChanges.createdCRTBs[0]
				require.Equal(stateChanges.t, "not-local", crtb.ClusterName)
				require.Equal(stateChanges.t, "not-local", crtb.Namespace)
				require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
				require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
				require.Equal(stateChanges.t, readOnlyRoleName, crtb.RoleTemplateName)
				require.Equal(stateChanges.t, userName, crtb.UserName)
				require.Equal(stateChanges.t, "", crtb.GroupPrincipalName)
				require.Len(stateChanges.t, crtb.OwnerReferences, 1)
				require.Equal(stateChanges.t, grbOwnerRef, crtb.OwnerReferences[0])

				// global assertions
				crb := stateChanges.createdCRBs[0]
				bindingName := "cattle-globalrolebinding-" + grb.Name
				roleName := "cattle-globalrole-" + gr.Name
				require.Equal(stateChanges.t, bindingName, crb.Name)
				require.Len(stateChanges.t, crb.OwnerReferences, 1)
				require.Equal(stateChanges.t, grbOwnerRef, crb.OwnerReferences[0])
				require.Equal(stateChanges.t, v1.RoleRef{Name: roleName, Kind: "ClusterRole"}, crb.RoleRef)
				require.Equal(stateChanges.t, v1.Subject{Name: userName, Kind: "User", APIGroup: v1.GroupName}, crb.Subjects[0])
				require.Equal(stateChanges.t, "true", crb.Labels["authz.management.cattle.io/globalrolebinding"])

			},
			inputBinding: grb.DeepCopy(),
			wantBinding:  &grb,
			wantError:    true,
		},
		{
			name: "failure on cluster and global",
			stateSetup: func(state testState) {
				// mocks for both cluster and global permissions
				state.grListerMock.GetFunc = func(namespace, name string) (*v3.GlobalRole, error) {
					return nil, fmt.Errorf("not found")
				}
				// mocks for just cluster permissions
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					return nil
				}

				// mocks for just global permissions
				state.crbListerMock.GetFunc = func(namespace, name string) (*v1.ClusterRoleBinding, error) {
					return nil, fmt.Errorf("not found")
				}
				state.crbClientMock.CreateFunc = func(crb *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
					state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
					return nil, fmt.Errorf("server not available")
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 0)
				require.Len(stateChanges.t, stateChanges.createdCRBs, 1)
				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 0)

				// global assertions
				crb := stateChanges.createdCRBs[0]
				bindingName := "cattle-globalrolebinding-" + grb.Name
				roleName := "cattle-globalrole-" + gr.Name
				require.Equal(stateChanges.t, bindingName, crb.Name)
				require.Len(stateChanges.t, crb.OwnerReferences, 1)
				require.Equal(stateChanges.t, grbOwnerRef, crb.OwnerReferences[0])
				require.Equal(stateChanges.t, v1.RoleRef{Name: roleName, Kind: "ClusterRole"}, crb.RoleRef)
				require.Equal(stateChanges.t, v1.Subject{Name: userName, Kind: "User", APIGroup: v1.GroupName}, crb.Subjects[0])
				require.Equal(stateChanges.t, "true", crb.Labels["authz.management.cattle.io/globalrolebinding"])

			},
			inputBinding: grb.DeepCopy(),
			wantBinding:  &grb,
			wantError:    true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			grbLifecycle := globalRoleBindingLifecycle{}
			testFuncs := []func(*v3.GlobalRoleBinding) (runtime.Object, error){grbLifecycle.Create, grbLifecycle.Updated}
			for _, testFunc := range testFuncs {
				ctrl := gomock.NewController(t)
				crtbCacheMock := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
				grListerMock := fakes.GlobalRoleListerMock{}
				crbListerMock := rbacFakes.ClusterRoleBindingListerMock{}
				clusterListerMock := fakes.ClusterListerMock{}
				crtbClientMock := fakes.ClusterRoleTemplateBindingInterfaceMock{}
				crbClientMock := rbacFakes.ClusterRoleBindingInterfaceMock{}
				stateChanges := testStateChanges{
					t:                t,
					createdCRTBs:     []*v3.ClusterRoleTemplateBinding{},
					createdCRBs:      []*v1.ClusterRoleBinding{},
					deletedCRTBNames: []string{},
				}
				state := testState{
					crtbCacheMock:     crtbCacheMock,
					grListerMock:      &grListerMock,
					crbListerMock:     &crbListerMock,
					clusterListerMock: &clusterListerMock,
					crtbClientMock:    &crtbClientMock,
					crbClientMock:     &crbClientMock,
					stateChanges:      &stateChanges,
				}
				if test.stateSetup != nil {
					test.stateSetup(state)
				}
				grbLifecycle.grLister = &grListerMock
				grbLifecycle.crbLister = &crbListerMock
				grbLifecycle.crtbCache = crtbCacheMock
				grbLifecycle.clusterLister = &clusterListerMock
				grbLifecycle.crtbClient = &crtbClientMock
				grbLifecycle.crbClient = &crbClientMock
				res, resErr := testFunc(test.inputBinding)
				require.Equal(t, test.wantBinding, res)
				if test.wantError {
					require.Error(t, resErr)
				} else {
					require.NoError(t, resErr)
				}
				if test.stateAssertions != nil {
					test.stateAssertions(*state.stateChanges)
				}
			}
		})
	}
}

func Test_crtbGrbOwnerIndexer(t *testing.T) {
	t.Parallel()
	grbOwnedCRTB := &v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crtb-grb-alread-exists",
			Labels: map[string]string{
				grbOwnerLabel: "test-grb",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
					UID:        "1234",
					Name:       "other-grb",
				},
			},
			GenerateName: "crtb-grb-",
			Namespace:    "some-cluster",
		},
		RoleTemplateName:   "already-exists",
		ClusterName:        "other-cluster",
		UserName:           "test-user",
		GroupPrincipalName: "",
	}
	keys, err := crtbGrbOwnerIndexer(grbOwnedCRTB)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, "other-cluster/test-grb", keys[0])

	noLabelCRTB := &v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crtb-grb-alread-exists",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
					UID:        "1234",
					Name:       "other-grb",
				},
			},
			GenerateName: "crtb-grb-",
			Namespace:    "some-cluster",
		},
		RoleTemplateName:   "already-exists",
		ClusterName:        "other-cluster",
		UserName:           "test-user",
		GroupPrincipalName: "",
	}
	keys, err = crtbGrbOwnerIndexer(noLabelCRTB)
	require.NoError(t, err)
	require.Len(t, keys, 0)

	standardCRTB := &v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "crtb-grb-123xyz",
			Namespace: "some-cluster",
		},
		RoleTemplateName:   "already-exists",
		ClusterName:        "some-cluster",
		UserName:           "test-user",
		GroupPrincipalName: "",
	}
	keys, err = crtbGrbOwnerIndexer(standardCRTB)
	require.NoError(t, err)
	require.Len(t, keys, 0)
}

func Test_reconcileClusterPermissions(t *testing.T) {
	t.Parallel()
	grListerGetFunc := func(_ string, name string) (*v3.GlobalRole, error) {
		switch name {
		case inheritedTestGr.Name:
			return &inheritedTestGr, nil
		case noInheritTestGR.Name:
			return &noInheritTestGR, nil
		case missingInheritTestGR.Name:
			return &missingInheritTestGR, nil
		case purgeTestGR.Name:
			return &purgeTestGR, nil
		default:
			return nil, fmt.Errorf("not found")
		}
	}

	tests := []struct {
		name            string
		stateSetup      func(state testState)
		inputObject     *v3.GlobalRoleBinding
		stateAssertions func(stateChanges testStateChanges)
		wantError       bool
	}{
		{
			name: "no inherited roles",
			stateSetup: func(state testState) {
				state.grListerMock.GetFunc = grListerGetFunc
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					return nil
				}
				state.clusterListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Cluster, error) {
					return []*v3.Cluster{&notLocalCluster, &localCluster}, nil
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 0)
				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 0)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: noInheritTestGR.Name,
				UserName:       "test-user",
			},
			wantError: false,
		},
		{
			name: "missing inherited roles",
			stateSetup: func(state testState) {
				state.grListerMock.GetFunc = grListerGetFunc
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					return nil
				}
				state.clusterListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Cluster, error) {
					return []*v3.Cluster{&notLocalCluster, &localCluster}, nil
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 0)
				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 0)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: missingInheritTestGR.Name,
				UserName:       "test-user",
			},
			wantError: false,
		},
		{
			name: "inherited cluster roles",
			stateSetup: func(state testState) {
				state.grListerMock.GetFunc = grListerGetFunc
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					return nil
				}
				state.clusterListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Cluster, error) {
					return []*v3.Cluster{&notLocalCluster, &localCluster}, nil
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 1)
				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 0)
				crtb := stateChanges.createdCRTBs[0]
				require.Equal(stateChanges.t, "not-local", crtb.ClusterName)
				require.Equal(stateChanges.t, "not-local", crtb.Namespace)
				require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
				require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
				require.Equal(stateChanges.t, "cluster-owner", crtb.RoleTemplateName)
				require.Equal(stateChanges.t, "test-user", crtb.UserName)
				require.Equal(stateChanges.t, "", crtb.GroupPrincipalName)
				require.Len(stateChanges.t, crtb.OwnerReferences, 1)
				ownerRef := crtb.OwnerReferences[0]
				require.Equal(stateChanges.t, "management.cattle.io/v3", ownerRef.APIVersion)
				require.Equal(stateChanges.t, "GlobalRoleBinding", ownerRef.Kind)
				require.Equal(stateChanges.t, "test-grb", ownerRef.Name)
				require.Equal(stateChanges.t, types.UID("1234"), ownerRef.UID)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: false,
		},
		{
			name: "inherited cluster roles, purge innacurate roles",
			stateSetup: func(state testState) {
				state.grListerMock.GetFunc = grListerGetFunc
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crtb-grb-already-exists-local",
							Labels: map[string]string{
								grbOwnerLabel: "test-grb",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "management.cattle.io/v3",
									Kind:       "GlobalRoleBinding",
									UID:        "1234",
									Name:       "test-grb",
								},
							},
							GenerateName: "crtb-grb-",
							Namespace:    "local",
						},
						RoleTemplateName:   "already-exists",
						ClusterName:        "local",
						UserName:           "test-user",
						GroupPrincipalName: "",
					},
				}, nil)
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crtb-grb-already-exists",
							Labels: map[string]string{
								grbOwnerLabel: "test-grb",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "management.cattle.io/v3",
									Kind:       "GlobalRoleBinding",
									UID:        "1234",
									Name:       "test-grb",
								},
							},
							GenerateName: "crtb-grb-",
							Namespace:    "not-local",
						},
						RoleTemplateName:   "already-exists",
						ClusterName:        "not-local",
						UserName:           "test-user",
						GroupPrincipalName: "",
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crtb-grb-wrong-cluster-name",
							Labels: map[string]string{
								grbOwnerLabel: "test-grb",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "management.cattle.io/v3",
									Kind:       "GlobalRoleBinding",
									UID:        "1234",
									Name:       "test-grb",
								},
							},
							GenerateName: "crtb-grb-",
							Namespace:    "not-local",
						},
						RoleTemplateName:   "wrong-cluster-name",
						ClusterName:        "does-not-exist",
						UserName:           "test-user",
						GroupPrincipalName: "",
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crtb-grb-wrong-user-name",
							Labels: map[string]string{
								grbOwnerLabel: "test-grb",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "management.cattle.io/v3",
									Kind:       "GlobalRoleBinding",
									UID:        "1234",
									Name:       "test-grb",
								},
							},
							GenerateName: "crtb-grb-",
							Namespace:    "not-local",
						},
						RoleTemplateName:   "wrong-user-name",
						ClusterName:        "not-local",
						UserName:           "wrong-user",
						GroupPrincipalName: "",
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crtb-grb-wrong-group-name",
							Labels: map[string]string{
								grbOwnerLabel: "test-grb",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "management.cattle.io/v3",
									Kind:       "GlobalRoleBinding",
									UID:        "1234",
									Name:       "test-grb",
								},
							},
							GenerateName: "crtb-grb-",
							Namespace:    "not-local",
						},
						RoleTemplateName:   "wrong-group-name",
						ClusterName:        "not-local",
						UserName:           "test-user",
						GroupPrincipalName: "wrong-group",
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crtb-grb-deleting",
							Labels: map[string]string{
								grbOwnerLabel: "test-grb",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "management.cattle.io/v3",
									Kind:       "GlobalRoleBinding",
									UID:        "1234",
									Name:       "test-grb",
								},
							},
							GenerateName:      "crtb-grb-",
							Namespace:         "not-local",
							DeletionTimestamp: &metav1.Time{Time: time.Now()},
						},
						RoleTemplateName:   "deleting",
						ClusterName:        "not-local",
						UserName:           "test-user",
						GroupPrincipalName: "",
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crtb-grb-duplicate",
							Labels: map[string]string{
								grbOwnerLabel: "test-grb",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "management.cattle.io/v3",
									Kind:       "GlobalRoleBinding",
									UID:        "1234",
									Name:       "test-grb",
								},
							},
							GenerateName: "crtb-grb-",
							Namespace:    "not-local",
						},
						RoleTemplateName:   "duplicate",
						ClusterName:        "not-local",
						UserName:           "test-user",
						GroupPrincipalName: "",
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crtb-grb-duplicate-2",
							Labels: map[string]string{
								grbOwnerLabel: "test-grb",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "management.cattle.io/v3",
									Kind:       "GlobalRoleBinding",
									UID:        "1234",
									Name:       "test-grb",
								},
							},
							GenerateName: "crtb-grb-",
							Namespace:    "not-local",
						},
						RoleTemplateName:   "duplicate",
						ClusterName:        "not-local",
						UserName:           "test-user",
						GroupPrincipalName: "",
					},
				}, nil).Times(2)
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					return nil
				}

				state.clusterListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Cluster, error) {
					return []*v3.Cluster{&notLocalCluster, &localCluster}, nil
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 5)
				var roleTemplateNames []string
				for _, crtb := range stateChanges.createdCRTBs {
					roleTemplateNames = append(roleTemplateNames, crtb.RoleTemplateName)
					require.Equal(stateChanges.t, "not-local", crtb.ClusterName)
					require.Equal(stateChanges.t, "not-local", crtb.Namespace)
					require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
					require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
					require.Equal(stateChanges.t, "test-user", crtb.UserName)
					require.Equal(stateChanges.t, "", crtb.GroupPrincipalName)
					require.Len(stateChanges.t, crtb.OwnerReferences, 1)
					ownerRef := crtb.OwnerReferences[0]
					require.Equal(stateChanges.t, "management.cattle.io/v3", ownerRef.APIVersion)
					require.Equal(stateChanges.t, "GlobalRoleBinding", ownerRef.Kind)
					require.Equal(stateChanges.t, "test-grb", ownerRef.Name)
					require.Equal(stateChanges.t, types.UID("1234"), ownerRef.UID)
				}
				require.Contains(stateChanges.t, roleTemplateNames, "missing")
				require.Contains(stateChanges.t, roleTemplateNames, "wrong-cluster-name")
				require.Contains(stateChanges.t, roleTemplateNames, "wrong-user-name")
				require.Contains(stateChanges.t, roleTemplateNames, "wrong-group-name")
				require.Contains(stateChanges.t, roleTemplateNames, "deleting")

				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 6)
				require.Contains(stateChanges.t, stateChanges.deletedCRTBNames, "crtb-grb-wrong-cluster-name")
				require.Contains(stateChanges.t, stateChanges.deletedCRTBNames, "crtb-grb-wrong-user-name")
				require.Contains(stateChanges.t, stateChanges.deletedCRTBNames, "crtb-grb-wrong-group-name")
				require.Contains(stateChanges.t, stateChanges.deletedCRTBNames, "crtb-grb-deleting")
				require.Contains(stateChanges.t, stateChanges.deletedCRTBNames, "crtb-grb-duplicate-2")
				require.Contains(stateChanges.t, stateChanges.deletedCRTBNames, "crtb-grb-already-exists-local")
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: purgeTestGR.Name,
				UserName:       "test-user",
			},
			wantError: false,
		},
		{
			name: "cluster lister error",
			stateSetup: func(state testState) {
				state.grListerMock.GetFunc = grListerGetFunc
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					return nil
				}
				state.clusterListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Cluster, error) {
					return nil, fmt.Errorf("server unavailable")
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 0)
				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 0)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: true,
		},
		{
			name: "crtb creation error",
			stateSetup: func(state testState) {
				state.grListerMock.GetFunc = grListerGetFunc
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "error/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					if crtb.ClusterName == errorCluster.Name {
						return nil, fmt.Errorf("server unavailable")
					}
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					return nil
				}
				state.clusterListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Cluster, error) {
					return []*v3.Cluster{&errorCluster, &notLocalCluster, &localCluster}, nil
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 2)
				var clusterNames []string
				for _, crtb := range stateChanges.createdCRTBs {
					clusterNames = append(clusterNames, crtb.ClusterName)
					require.Equal(stateChanges.t, crtb.ClusterName, crtb.Namespace)
					require.Equal(stateChanges.t, "cluster-owner", crtb.RoleTemplateName)
					require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
					require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
					require.Equal(stateChanges.t, "test-user", crtb.UserName)
					require.Equal(stateChanges.t, "", crtb.GroupPrincipalName)
					require.Len(stateChanges.t, crtb.OwnerReferences, 1)
					ownerRef := crtb.OwnerReferences[0]
					require.Equal(stateChanges.t, "management.cattle.io/v3", ownerRef.APIVersion)
					require.Equal(stateChanges.t, "GlobalRoleBinding", ownerRef.Kind)
					require.Equal(stateChanges.t, "test-grb", ownerRef.Name)
					require.Equal(stateChanges.t, types.UID("1234"), ownerRef.UID)
				}
				require.Contains(stateChanges.t, clusterNames, notLocalCluster.Name)
				require.Contains(stateChanges.t, clusterNames, errorCluster.Name)
				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 0)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: true,
		},
		{
			name: "indexer error",
			stateSetup: func(state testState) {
				state.grListerMock.GetFunc = grListerGetFunc
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "error/test-grb").Return(nil, fmt.Errorf("indexer error")).Times(2)
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					return nil
				}
				state.clusterListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Cluster, error) {
					return []*v3.Cluster{&errorCluster, &notLocalCluster, &localCluster}, nil
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 1)
				crtb := stateChanges.createdCRTBs[0]
				require.Equal(stateChanges.t, "not-local", crtb.Namespace)
				require.Equal(stateChanges.t, "not-local", crtb.ClusterName)
				require.Equal(stateChanges.t, "cluster-owner", crtb.RoleTemplateName)
				require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
				require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
				require.Equal(stateChanges.t, "test-user", crtb.UserName)
				require.Equal(stateChanges.t, "", crtb.GroupPrincipalName)
				require.Len(stateChanges.t, crtb.OwnerReferences, 1)
				ownerRef := crtb.OwnerReferences[0]
				require.Equal(stateChanges.t, "management.cattle.io/v3", ownerRef.APIVersion)
				require.Equal(stateChanges.t, "GlobalRoleBinding", ownerRef.Kind)
				require.Equal(stateChanges.t, "test-grb", ownerRef.Name)
				require.Equal(stateChanges.t, types.UID("1234"), ownerRef.UID)

				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 0)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: true,
		},
		{
			name: "crtb delete error",
			stateSetup: func(state testState) {
				state.grListerMock.GetFunc = grListerGetFunc
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:         "crtb-grb-delete-local",
							GenerateName: "crtb-grb-",
							Namespace:    "local",
						},
						RoleTemplateName:   "not-valid",
						ClusterName:        "local",
						UserName:           "test-user",
						GroupPrincipalName: "",
					},
				}, nil)
				state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "error/test-grb").Return([]*v3.ClusterRoleTemplateBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:         "crtb-grb-delete",
							GenerateName: "crtb-grb-",
							Namespace:    "error",
						},
						RoleTemplateName:   "not-valid",
						ClusterName:        "error",
						UserName:           "test-user",
						GroupPrincipalName: "",
					},
				}, nil).Times(2)
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					if name == "crtb-grb-delete" || name == "crtb-grb-delete-local" {
						return fmt.Errorf("server unavailable")
					}
					return nil
				}
				state.clusterListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Cluster, error) {
					return []*v3.Cluster{&errorCluster, &localCluster}, nil
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 1)
				crtb := stateChanges.createdCRTBs[0]
				require.Equal(stateChanges.t, "error", crtb.Namespace)
				require.Equal(stateChanges.t, "error", crtb.ClusterName)
				require.Equal(stateChanges.t, "cluster-owner", crtb.RoleTemplateName)
				require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
				require.Equal(stateChanges.t, "test-grb", crtb.Labels[grbOwnerLabel])
				require.Equal(stateChanges.t, "test-user", crtb.UserName)
				require.Equal(stateChanges.t, "", crtb.GroupPrincipalName)
				require.Len(stateChanges.t, crtb.OwnerReferences, 1)
				ownerRef := crtb.OwnerReferences[0]
				require.Equal(stateChanges.t, "management.cattle.io/v3", ownerRef.APIVersion)
				require.Equal(stateChanges.t, "GlobalRoleBinding", ownerRef.Kind)
				require.Equal(stateChanges.t, "test-grb", ownerRef.Name)
				require.Equal(stateChanges.t, types.UID("1234"), ownerRef.UID)

				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 2)
				require.Contains(stateChanges.t, stateChanges.deletedCRTBNames, "crtb-grb-delete")
				require.Contains(stateChanges.t, stateChanges.deletedCRTBNames, "crtb-grb-delete-local")
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: true,
		},
		{
			name: "no global role",
			stateSetup: func(state testState) {
				state.grListerMock.GetFunc = grListerGetFunc
				state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
					return crtb, nil
				}
				state.crtbClientMock.DeleteNamespacedFunc = func(_ string, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedCRTBNames = append(state.stateChanges.deletedCRTBNames, name)
					return nil
				}
				state.clusterListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Cluster, error) {
					return []*v3.Cluster{&errorCluster, &notLocalCluster, &localCluster}, nil
				}
			},
			stateAssertions: func(stateChanges testStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdCRTBs, 0)
				require.Len(stateChanges.t, stateChanges.deletedCRTBNames, 0)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: "error",
				UserName:       "test-user",
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			crtbCacheMock := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
			grListerMock := fakes.GlobalRoleListerMock{}
			clusterListerMock := fakes.ClusterListerMock{}
			crtbClientMock := fakes.ClusterRoleTemplateBindingInterfaceMock{}
			state := testState{
				crtbCacheMock:     crtbCacheMock,
				grListerMock:      &grListerMock,
				clusterListerMock: &clusterListerMock,
				crtbClientMock:    &crtbClientMock,
				stateChanges: &testStateChanges{
					t:                t,
					createdCRTBs:     []*v3.ClusterRoleTemplateBinding{},
					deletedCRTBNames: []string{},
				},
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			grbLifecycle := globalRoleBindingLifecycle{
				grLister:      &grListerMock,
				crtbCache:     crtbCacheMock,
				clusterLister: &clusterListerMock,
				crtbClient:    &crtbClientMock,
			}
			resErr := grbLifecycle.reconcileClusterPermissions(test.inputObject)
			if test.wantError {
				require.Error(t, resErr)
			} else {
				require.NoError(t, resErr)
			}
			if test.stateAssertions != nil {
				test.stateAssertions(*state.stateChanges)
			}
		})
	}

}

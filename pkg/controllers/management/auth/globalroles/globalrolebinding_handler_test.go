package globalroles

import (
	"fmt"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apisv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	normanv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	rbacFakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	namespacedRulesGRB = v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespacedRulesGRB",
		},
		UserName:       "username",
		GlobalRoleName: "namespacedRulesGR",
	}
)

type grbTestStateChanges struct {
	t                *testing.T
	createdCRTBs     []*v3.ClusterRoleTemplateBinding
	createdCRBs      []*rbacv1.ClusterRoleBinding
	deletedCRTBNames []string
	createdRBs       map[string]*rbacv1.RoleBinding
	deletedRBsNames  map[string]struct{}
	fwhCalled        bool
}
type grbTestState struct {
	crtbCacheMock     *fake.MockCacheInterface[*v3.ClusterRoleTemplateBinding]
	grListerMock      *fakes.GlobalRoleListerMock
	crbListerMock     *rbacFakes.ClusterRoleBindingListerMock
	clusterListerMock *fakes.ClusterListerMock
	crtbClientMock    *fakes.ClusterRoleTemplateBindingInterfaceMock
	crbClientMock     *rbacFakes.ClusterRoleBindingInterfaceMock
	nsCacheMock       *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]
	rListerMock       *rbacFakes.RoleListerMock
	rbListerMock      *rbacFakes.RoleBindingListerMock
	rbClientMock      *rbacFakes.RoleBindingInterfaceMock
	fwhMock           *fleetPermissionsHandlerMock
	stateChanges      *grbTestStateChanges
}

func TestCreateUpdate(t *testing.T) {
	// right now, create and update have the same input/output, so they are tested in the same way
	t.Parallel()

	readOnlyRoleName := "read-only"
	userName := "test-user"
	gr := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gr",
			UID:  "5678",
		},
		Rules:                 []rbacv1.PolicyRule{readPodPolicyRule},
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
		stateSetup      func(grbTestState)
		stateAssertions func(grbTestStateChanges)
		inputBinding    *v3.GlobalRoleBinding
		wantBinding     *v3.GlobalRoleBinding
		wantError       bool
	}{
		{
			name: "success on both cluster and global permissions",
			stateSetup: func(state grbTestState) {
				// mocks for both cluster and global permissions
				state.grListerMock.GetFunc = func(namespace, name string) (*v3.GlobalRole, error) {
					if name == gr.Name && namespace == "" {
						return &gr, nil
					}
					return nil, fmt.Errorf("not found")
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
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
				state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
					return nil, fmt.Errorf("not found")
				}
				state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
					return crb, nil
				}

				// mocks for fleet workspace permissions
				state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding) error {
					state.stateChanges.fwhCalled = true
					return nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
				require.Equal(stateChanges.t, rbacv1.RoleRef{Name: roleName, Kind: "ClusterRole"}, crb.RoleRef)
				require.Equal(stateChanges.t, rbacv1.Subject{Name: userName, Kind: "User", APIGroup: rbacv1.GroupName}, crb.Subjects[0])
				require.Equal(stateChanges.t, "true", crb.Labels["authz.management.cattle.io/globalrolebinding"])

				// fleet workspace assertions
				require.Equal(stateChanges.t, true, stateChanges.fwhCalled)
			},
			inputBinding: grb.DeepCopy(),
			wantBinding:  addAnnotation(&grb),
			wantError:    false,
		},
		{
			name: "success on global, failure on cluster",
			stateSetup: func(state grbTestState) {
				// mocks for both cluster and global permissions
				state.grListerMock.GetFunc = func(namespace, name string) (*v3.GlobalRole, error) {
					if name == gr.Name && namespace == "" {
						return &gr, nil
					}
					return nil, fmt.Errorf("not found")
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
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
				state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
					return nil, fmt.Errorf("not found")
				}
				state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
					return crb, nil
				}

				// mocks for fleet workspace permissions
				state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding) error {
					state.stateChanges.fwhCalled = true
					return nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
				require.Equal(stateChanges.t, rbacv1.RoleRef{Name: roleName, Kind: "ClusterRole"}, crb.RoleRef)
				require.Equal(stateChanges.t, rbacv1.Subject{Name: userName, Kind: "User", APIGroup: rbacv1.GroupName}, crb.Subjects[0])
				require.Equal(stateChanges.t, "true", crb.Labels["authz.management.cattle.io/globalrolebinding"])

				// fleet workspace assertions
				require.Equal(stateChanges.t, true, stateChanges.fwhCalled)
			},
			inputBinding: grb.DeepCopy(),
			wantBinding:  addAnnotation(&grb),
			wantError:    true,
		},
		{
			name: "success on cluster, failure on global",
			stateSetup: func(state grbTestState) {
				// mocks for both cluster and global permissions
				state.grListerMock.GetFunc = func(namespace, name string) (*v3.GlobalRole, error) {
					if name == gr.Name && namespace == "" {
						return &gr, nil
					}
					return nil, fmt.Errorf("not found")
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
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
				state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
					return nil, fmt.Errorf("not found")
				}
				state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
					return nil, fmt.Errorf("server not available")
				}

				// mocks for fleet workspace permissions
				state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding) error {
					state.stateChanges.fwhCalled = true
					return nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
				require.Equal(stateChanges.t, rbacv1.RoleRef{Name: roleName, Kind: "ClusterRole"}, crb.RoleRef)
				require.Equal(stateChanges.t, rbacv1.Subject{Name: userName, Kind: "User", APIGroup: rbacv1.GroupName}, crb.Subjects[0])
				require.Equal(stateChanges.t, "true", crb.Labels["authz.management.cattle.io/globalrolebinding"])

				// fleet workspace assertions
				require.Equal(stateChanges.t, true, stateChanges.fwhCalled)
			},
			inputBinding: grb.DeepCopy(),
			wantBinding:  &grb,
			wantError:    true,
		},
		{
			name: "failure on cluster and global",
			stateSetup: func(state grbTestState) {
				// mocks for both cluster and global permissions
				state.grListerMock.GetFunc = func(namespace, name string) (*v3.GlobalRole, error) {
					return nil, fmt.Errorf("not found")
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
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
				state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
					return nil, fmt.Errorf("not found")
				}
				state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
					return nil, fmt.Errorf("server not available")
				}

				// mocks for fleet workspace permissions
				state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding) error {
					state.stateChanges.fwhCalled = true
					return nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
				require.Equal(stateChanges.t, rbacv1.RoleRef{Name: roleName, Kind: "ClusterRole"}, crb.RoleRef)
				require.Equal(stateChanges.t, rbacv1.Subject{Name: userName, Kind: "User", APIGroup: rbacv1.GroupName}, crb.Subjects[0])
				require.Equal(stateChanges.t, "true", crb.Labels["authz.management.cattle.io/globalrolebinding"])

				// fleet workspace assertions
				require.Equal(stateChanges.t, true, stateChanges.fwhCalled)
			},
			inputBinding: grb.DeepCopy(),
			wantBinding:  &grb,
			wantError:    true,
		},
		{
			name: "success on cluster and global permissions, failure on fleet workspace permissions",
			stateSetup: func(state grbTestState) {
				// mocks for both cluster and global permissions
				state.grListerMock.GetFunc = func(namespace, name string) (*v3.GlobalRole, error) {
					if name == gr.Name && namespace == "" {
						return &gr, nil
					}
					return nil, fmt.Errorf("not found")
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
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
				state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
					return nil, fmt.Errorf("not found")
				}
				state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
					return crb, nil
				}

				// mocks for fleet workspace permissions
				state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding) error {
					state.stateChanges.fwhCalled = true
					return fmt.Errorf("unavailable")
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
				require.Equal(stateChanges.t, rbacv1.RoleRef{Name: roleName, Kind: "ClusterRole"}, crb.RoleRef)
				require.Equal(stateChanges.t, rbacv1.Subject{Name: userName, Kind: "User", APIGroup: rbacv1.GroupName}, crb.Subjects[0])
				require.Equal(stateChanges.t, "true", crb.Labels["authz.management.cattle.io/globalrolebinding"])

				// fleet workspace assertions
				require.Equal(stateChanges.t, true, stateChanges.fwhCalled)
			},
			inputBinding: grb.DeepCopy(),
			wantBinding:  addAnnotation(&grb),
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
				rbListerMock := rbacFakes.RoleBindingListerMock{}
				fphMock := fleetPermissionsHandlerMock{}

				stateChanges := grbTestStateChanges{
					t:                t,
					createdCRTBs:     []*v3.ClusterRoleTemplateBinding{},
					createdCRBs:      []*rbacv1.ClusterRoleBinding{},
					deletedCRTBNames: []string{},
				}
				state := grbTestState{
					crtbCacheMock:     crtbCacheMock,
					grListerMock:      &grListerMock,
					crbListerMock:     &crbListerMock,
					clusterListerMock: &clusterListerMock,
					crtbClientMock:    &crtbClientMock,
					crbClientMock:     &crbClientMock,
					rbListerMock:      &rbListerMock,
					fwhMock:           &fphMock,
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
				grbLifecycle.roleBindingLister = &rbListerMock
				grbLifecycle.fleetPermissionsHandler = &fphMock
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
		stateSetup      func(state grbTestState)
		inputObject     *v3.GlobalRoleBinding
		stateAssertions func(stateChanges grbTestStateChanges)
		wantError       bool
	}{
		{
			name: "no inherited roles",
			stateSetup: func(state grbTestState) {
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
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
			stateSetup: func(state grbTestState) {
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
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
			stateSetup: func(state grbTestState) {
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
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
			stateSetup: func(state grbTestState) {
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
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
			stateSetup: func(state grbTestState) {
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
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
			stateSetup: func(state grbTestState) {
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
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
			stateSetup: func(state grbTestState) {
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
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
			stateSetup: func(state grbTestState) {
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
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
			stateSetup: func(state grbTestState) {
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
			stateAssertions: func(stateChanges grbTestStateChanges) {
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
			state := grbTestState{
				crtbCacheMock:     crtbCacheMock,
				grListerMock:      &grListerMock,
				clusterListerMock: &clusterListerMock,
				crtbClientMock:    &crtbClientMock,
				stateChanges: &grbTestStateChanges{
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

func Test_reconcileNamespacedPermissions(t *testing.T) {
	t.Parallel()

	activeNamespace := &corev1.Namespace{
		Status: corev1.NamespaceStatus{
			Phase: corev1.NamespaceActive,
		},
	}
	terminatingNamespace := &corev1.Namespace{
		Status: corev1.NamespaceStatus{
			Phase: corev1.NamespaceTerminating,
		},
	}

	tests := []struct {
		name              string
		stateSetup        func(grbTestState)
		stateAssertions   func(grbTestStateChanges)
		globalRoleBinding *v3.GlobalRoleBinding
		wantError         bool
	}{
		{
			name: "global role not found",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return nil, fmt.Errorf("error")
				}
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "getting namespace fails",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, fmt.Errorf("error"))

				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
				}
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "namespace is nil",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(nil, nil)

				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
				}
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "getting roleBinding fails",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rbListerMock.GetFunc = func(_, _ string) (*rbacv1.RoleBinding, error) {
					return nil, fmt.Errorf("error")
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
				}
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "creating roleBinding fails",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rbListerMock.GetFunc = func(_, _ string) (*rbacv1.RoleBinding, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleBindingGroupVersionKind.Group,
						Resource: normanv1.RoleBindingGroupVersionResource.Resource,
					}, "")
				}
				state.rbClientMock.CreateFunc = func(roleBinding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					return nil, fmt.Errorf("error")
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
				}
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "roleBindings get created",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					return nil, nil
				}
				state.rbListerMock.GetFunc = func(_, _ string) (*rbacv1.RoleBinding, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleBindingGroupVersionKind.Group,
						Resource: normanv1.RoleBindingGroupVersionResource.Resource,
					}, "")
				}
				state.rbClientMock.CreateFunc = func(roleBinding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					state.stateChanges.createdRBs[roleBinding.Name] = roleBinding
					roleBinding.UID = ""
					return roleBinding, nil
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdRBs, 2)

				roleBinding, ok := stateChanges.createdRBs["namespacedRulesGRB-namespace1"]
				require.True(stateChanges.t, ok)
				require.Equal(stateChanges.t, "namespace1", roleBinding.ObjectMeta.Namespace)
				require.Equal(stateChanges.t, "namespacedRulesGR-namespace1", roleBinding.RoleRef.Name)
				require.Equal(stateChanges.t, "Role", roleBinding.RoleRef.Kind)
				require.Equal(stateChanges.t, "namespacedRulesGRB", roleBinding.Labels[grbOwnerLabel])
				subject := rbacv1.Subject{
					Kind:     "User",
					Name:     "username",
					APIGroup: rbacv1.GroupName,
				}
				require.Len(stateChanges.t, roleBinding.Subjects, 1)
				require.Equal(stateChanges.t, subject, roleBinding.Subjects[0])

				roleBinding, ok = stateChanges.createdRBs["namespacedRulesGRB-namespace2"]
				require.True(stateChanges.t, ok)
				require.Equal(stateChanges.t, "namespace2", roleBinding.ObjectMeta.Namespace)
				require.Equal(stateChanges.t, "namespacedRulesGR-namespace2", roleBinding.RoleRef.Name)
				require.Equal(stateChanges.t, "Role", roleBinding.RoleRef.Kind)
				require.Len(stateChanges.t, roleBinding.Subjects, 1)
				require.Equal(stateChanges.t, subject, roleBinding.Subjects[0])
				require.Equal(stateChanges.t, "namespacedRulesGRB", roleBinding.Labels[grbOwnerLabel])
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "roleBindings don't get created in a terminating namespace",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(terminatingNamespace, nil)

				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					return nil, nil
				}
				state.rbListerMock.GetFunc = func(_, _ string) (*rbacv1.RoleBinding, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleBindingGroupVersionKind.Group,
						Resource: normanv1.RoleBindingGroupVersionResource.Resource,
					}, "")
				}
				state.rbClientMock.CreateFunc = func(roleBinding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					state.stateChanges.createdRBs[roleBinding.Name] = roleBinding
					roleBinding.UID = ""
					return roleBinding, nil
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
				require.Len(stateChanges.t, stateChanges.createdRBs, 0)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "one NS not found, still creates other RB",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				first := state.nsCacheMock.EXPECT().Get(gomock.Any()).Return(activeNamespace, fmt.Errorf("error"))
				second := state.nsCacheMock.EXPECT().Get(gomock.Any()).Return(activeNamespace, nil)
				gomock.InOrder(first, second)

				state.rbListerMock.GetFunc = func(_, _ string) (*rbacv1.RoleBinding, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleBindingGroupVersionKind.Group,
						Resource: normanv1.RoleBindingGroupVersionResource.Resource,
					}, "")
				}
				state.rbClientMock.CreateFunc = func(roleBinding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					state.stateChanges.createdRBs[roleBinding.Name] = roleBinding
					roleBinding.UID = ""
					return roleBinding, nil
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
				// The second roleBindings should be created despite the first getting an error
				// Because the order is not guaranteed, we can't assert any info on the
				// created roleBinding, just that it exists
				require.Len(stateChanges.t, stateChanges.createdRBs, 1)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "delete roleBinding from terminating namespace",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get("namespace1").Return(terminatingNamespace, nil)
				state.nsCacheMock.EXPECT().Get("namespace2").Return(activeNamespace, nil)

				state.rbListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.RoleBinding, error) {
					if namespace == "namespace1" {
						roleBinding := &rbacv1.RoleBinding{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "namespacedRulesGRB-" + namespace,
								Namespace: namespace,
							},
						}
						return roleBinding, nil
					} else {
						return nil, apierrors.NewNotFound(schema.GroupResource{
							Group:    normanv1.RoleBindingGroupVersionKind.Group,
							Resource: normanv1.RoleBindingGroupVersionResource.Resource,
						}, "")
					}
				}
				state.rbClientMock.CreateFunc = func(roleBinding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					state.stateChanges.createdRBs[roleBinding.Name] = roleBinding
					roleBinding.UID = ""
					return roleBinding, nil
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedRBsNames[name] = struct{}{}
					return nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
				// in the case of a terminating namespace, the bad RoleBinding should be deleted, not updated
				require.Len(stateChanges.t, stateChanges.deletedRBsNames, 1)
				require.Contains(stateChanges.t, stateChanges.deletedRBsNames, "namespacedRulesGRB-namespace1")

				// The second RoleBinding in an active namespace should still get created
				require.Len(stateChanges.t, stateChanges.createdRBs, 1)
				roleBinding, ok := stateChanges.createdRBs["namespacedRulesGRB-namespace2"]
				require.True(stateChanges.t, ok)
				require.Equal(stateChanges.t, "namespace2", roleBinding.ObjectMeta.Namespace)
				require.Equal(stateChanges.t, "namespacedRulesGR-namespace2", roleBinding.RoleRef.Name)
				require.Equal(stateChanges.t, "Role", roleBinding.RoleRef.Kind)
				require.Equal(stateChanges.t, "namespacedRulesGRB", roleBinding.Labels[grbOwnerLabel])

			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "update roleBindings with bad roleRef name",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rbListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.RoleBinding, error) {
					roleBinding := &rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "namespacedRulesGRB-" + namespace,
							Namespace: namespace,
						},
						RoleRef: rbacv1.RoleRef{
							Name: "badRoleName",
							Kind: "Role",
						},
					}
					return roleBinding, nil
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedRBsNames[name] = struct{}{}
					return nil
				}
				state.rbClientMock.CreateFunc = func(roleBinding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					state.stateChanges.createdRBs[roleBinding.Name] = roleBinding
					roleBinding.UID = ""
					return roleBinding, nil
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
				require.Len(stateChanges.t, stateChanges.deletedRBsNames, 2)
				require.Contains(stateChanges.t, stateChanges.deletedRBsNames, "namespacedRulesGRB-namespace1")
				require.Contains(stateChanges.t, stateChanges.deletedRBsNames, "namespacedRulesGRB-namespace2")

				require.Len(stateChanges.t, stateChanges.createdRBs, 2)
				roleBinding, ok := stateChanges.createdRBs["namespacedRulesGRB-namespace1"]
				require.True(stateChanges.t, ok)
				require.Equal(stateChanges.t, "namespace1", roleBinding.ObjectMeta.Namespace)
				require.Equal(stateChanges.t, "namespacedRulesGR-namespace1", roleBinding.RoleRef.Name)
				require.Equal(stateChanges.t, "Role", roleBinding.RoleRef.Kind)
				require.Equal(stateChanges.t, "namespacedRulesGRB", roleBinding.Labels[grbOwnerLabel])

				roleBinding, ok = stateChanges.createdRBs["namespacedRulesGRB-namespace2"]
				require.True(stateChanges.t, ok)
				require.Equal(stateChanges.t, "namespace2", roleBinding.ObjectMeta.Namespace)
				require.Equal(stateChanges.t, "namespacedRulesGR-namespace2", roleBinding.RoleRef.Name)
				require.Equal(stateChanges.t, "Role", roleBinding.RoleRef.Kind)
				require.Equal(stateChanges.t, "namespacedRulesGRB", roleBinding.Labels[grbOwnerLabel])
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "update roleBindings with bad grbOwner label",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rbListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.RoleBinding, error) {
					roleBinding := &rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "namespacedRulesGRB-" + namespace,
							Namespace: namespace,
							Labels:    map[string]string{grbOwnerLabel: "bad-owner"},
						},
						RoleRef: rbacv1.RoleRef{
							Name: "namespacedRulesGR-" + namespace,
							Kind: "Role",
						},
					}
					return roleBinding, nil
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedRBsNames[name] = struct{}{}
					return nil
				}
				state.rbClientMock.CreateFunc = func(roleBinding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					state.stateChanges.createdRBs[roleBinding.Name] = roleBinding
					roleBinding.UID = ""
					return roleBinding, nil
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
				require.Len(stateChanges.t, stateChanges.deletedRBsNames, 2)
				require.Contains(stateChanges.t, stateChanges.deletedRBsNames, "namespacedRulesGRB-namespace1")
				require.Contains(stateChanges.t, stateChanges.deletedRBsNames, "namespacedRulesGRB-namespace2")

				require.Len(stateChanges.t, stateChanges.createdRBs, 2)
				roleBinding, ok := stateChanges.createdRBs["namespacedRulesGRB-namespace1"]
				require.True(stateChanges.t, ok)
				require.Equal(stateChanges.t, "namespace1", roleBinding.ObjectMeta.Namespace)
				require.Equal(stateChanges.t, "namespacedRulesGR-namespace1", roleBinding.RoleRef.Name)
				require.Equal(stateChanges.t, "Role", roleBinding.RoleRef.Kind)
				require.Equal(stateChanges.t, "namespacedRulesGRB", roleBinding.Labels[grbOwnerLabel])

				roleBinding, ok = stateChanges.createdRBs["namespacedRulesGRB-namespace2"]
				require.True(stateChanges.t, ok)
				require.Equal(stateChanges.t, "namespace2", roleBinding.ObjectMeta.Namespace)
				require.Equal(stateChanges.t, "namespacedRulesGR-namespace2", roleBinding.RoleRef.Name)
				require.Equal(stateChanges.t, "Role", roleBinding.RoleRef.Kind)
				require.Equal(stateChanges.t, "namespacedRulesGRB", roleBinding.Labels[grbOwnerLabel])
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "update roleBindings fails",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rbListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.RoleBinding, error) {
					roleBinding := &rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "namespacedRulesGRB-" + namespace,
							Namespace: namespace,
						},
					}
					return roleBinding, nil
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *metav1.DeleteOptions) error {
					return fmt.Errorf("error")
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, nil
				}
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "purge RBs that falsely claim to be owned by GRB",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rbListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.RoleBinding, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleBindingGroupVersionKind.Group,
						Resource: normanv1.RoleBindingGroupVersionResource.Resource,
					}, "")
				}
				state.rbClientMock.CreateFunc = func(roleBinding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					roleBinding.UID = "1111"
					return roleBinding, nil
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedRBsNames[name] = struct{}{}
					return nil
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					roleBindings := []*rbacv1.RoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "deleted-role",
								UID:  "2222",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "kept-role",
								UID:  "1111",
							},
						},
					}
					return roleBindings, nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
				require.Contains(stateChanges.t, stateChanges.deletedRBsNames, "deleted-role")
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "do not purge RBs that correctly claim to belong to GRB",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rbListerMock.GetFunc = func(namespace string, name string) (*rbacv1.RoleBinding, error) {
					roleRef := rbacv1.RoleRef{
						APIGroup: rbacv1.GroupName,
						Kind:     "Role",
						Name:     "namespacedRulesGR-" + namespace,
					}

					roleBinding := &rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								grbOwnerLabel: "namespacedRulesGRB",
							},
							UID: "1111",
						},
						RoleRef: roleRef,
					}
					return roleBinding, nil
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedRBsNames[name] = struct{}{}
					return nil
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					roleBindings := []*rbacv1.RoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "deleted-role",
								UID:  "2222",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "kept-role",
								UID:  "1111",
							},
						},
					}
					return roleBindings, nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
				require.Contains(stateChanges.t, stateChanges.deletedRBsNames, "deleted-role")
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "delete purged RBs fails",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rbListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.RoleBinding, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleBindingGroupVersionKind.Group,
						Resource: normanv1.RoleBindingGroupVersionResource.Resource,
					}, "")
				}
				state.rbClientMock.CreateFunc = func(roleBinding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					roleBinding.UID = "1111"
					return roleBinding, nil
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedRBsNames[name] = struct{}{}
					return fmt.Errorf("error")
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					roleBindings := []*rbacv1.RoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "deleted-role",
								UID:  "2222",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "kept-role",
								UID:  "1111",
							},
						},
					}
					return roleBindings, nil
				}
			},
			stateAssertions: func(stateChanges grbTestStateChanges) {
				require.Contains(stateChanges.t, stateChanges.deletedRBsNames, "deleted-role")
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "list RBs fails",
			stateSetup: func(state grbTestState) {
				state.grListerMock.GetFunc = func(_, _ string) (*v3.GlobalRole, error) {
					return namespacedRulesGR.DeepCopy(), nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rbListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.RoleBinding, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleBindingGroupVersionKind.Group,
						Resource: normanv1.RoleBindingGroupVersionResource.Resource,
					}, "")
				}
				state.rbClientMock.CreateFunc = func(roleBinding *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					roleBinding.UID = "1111"
					return roleBinding, nil
				}
				state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, fmt.Errorf("error")
				}
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			grbLifecycle := globalRoleBindingLifecycle{}
			grLister := fakes.GlobalRoleListerMock{}
			nsCacheMock := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
			rLister := rbacFakes.RoleListerMock{}
			rbLister := rbacFakes.RoleBindingListerMock{}
			rbClient := rbacFakes.RoleBindingInterfaceMock{}

			stateChanges := grbTestStateChanges{
				t:               t,
				createdRBs:      map[string]*rbacv1.RoleBinding{},
				deletedRBsNames: map[string]struct{}{},
			}
			state := grbTestState{
				nsCacheMock:  nsCacheMock,
				grListerMock: &grLister,
				rListerMock:  &rLister,
				rbListerMock: &rbLister,
				rbClientMock: &rbClient,
				stateChanges: &stateChanges,
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}

			grbLifecycle.nsCache = nsCacheMock
			grbLifecycle.grLister = &grLister
			grbLifecycle.roleLister = &rLister
			grbLifecycle.roleBindingLister = &rbLister
			grbLifecycle.roleBindings = &rbClient
			err := grbLifecycle.reconcileNamespacedRoleBindings(test.globalRoleBinding)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if test.stateAssertions != nil {
				test.stateAssertions(*state.stateChanges)
			}
		})
	}
}

type fleetPermissionsHandlerMock struct {
	reconcileFleetWorkspacePermissionsFunc func(globalRoleBinding *v3.GlobalRoleBinding) error
}

func (f *fleetPermissionsHandlerMock) reconcileFleetWorkspacePermissionsBindings(globalRoleBinding *v3.GlobalRoleBinding) error {
	return f.reconcileFleetWorkspacePermissionsFunc(globalRoleBinding)
}

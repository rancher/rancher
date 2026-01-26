package globalroles

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	"github.com/rancher/rancher/pkg/controllers/status"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	rbacFakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	userMocks "github.com/rancher/rancher/pkg/user/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
			UID:  "1234",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "fake-kind",
			APIVersion: "fake-version",
		},
		UserName:       "username",
		GlobalRoleName: "namespacedRulesGR",
	}
)

type grbTestStateChanges struct {
	t                *testing.T
	createdCRTBs     []*v3.ClusterRoleTemplateBinding
	createdCRBs      []*rbacv1.ClusterRoleBinding
	updatedCRBs      []*rbacv1.ClusterRoleBinding
	deletedCRTBNames []string
	createdRBs       map[string]*rbacv1.RoleBinding
	deletedRBsNames  map[string]struct{}
	fwhCalled        bool
}
type grbTestState struct {
	crtbCacheMock    *fake.MockCacheInterface[*v3.ClusterRoleTemplateBinding]
	grListerMock     *fakes.GlobalRoleListerMock
	crbListerMock    *rbacFakes.ClusterRoleBindingListerMock
	clusterCacheMock *fake.MockNonNamespacedCacheInterface[*v3.Cluster]
	crtbClientMock   *fakes.ClusterRoleTemplateBindingInterfaceMock
	crbClientMock    *rbacFakes.ClusterRoleBindingInterfaceMock
	nsCacheMock      *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]
	rListerMock      *rbacFakes.RoleListerMock
	rbListerMock     *rbacFakes.RoleBindingListerMock
	rbClientMock     *rbacFakes.RoleBindingInterfaceMock
	grbListerMock    *fake.MockNonNamespacedCacheInterface[*v3.GlobalRoleBinding]
	grbClientMock    *fake.MockNonNamespacedControllerInterface[*v3.GlobalRoleBinding, *v3.GlobalRoleBindingList]
	userManagerMock  *userMocks.MockManager
	userListerMock   *fakes.UserListerMock
	stateChanges     *grbTestStateChanges
}

/*
	func TestCreateUpdate(t *testing.T) {
		// right now, create and update have the same input/output, so they are tested in the same way
		t.Parallel()

		readOnlyRoleName := "read-only"
		testPrincipal := "testing://test-user"
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
			UserName:       userName,
			GlobalRoleName: gr.Name,
		}
		grbOwnerRef := metav1.OwnerReference{
			APIVersion: "management.cattle.io/v3",
			Kind:       "GlobalRoleBinding",
			Name:       "test-grb",
			UID:        types.UID("1234"),
		}

		addAnnotation := func(grb *v3.GlobalRoleBinding, others ...map[string]string) *v3.GlobalRoleBinding {
			newGRB := grb.DeepCopy()
			newGRB.Annotations = map[string]string{
				"authz.management.cattle.io/crb-name": "cattle-globalrolebinding-" + grb.Name,
			}
			for _, o := range others {
				for k, v := range o {
					newGRB.Annotations[k] = v
				}
			}

			return newGRB
		}

		addUserPrincipalName := func(grb *v3.GlobalRoleBinding, name string) *v3.GlobalRoleBinding {
			newGRB := grb.DeepCopy()
			newGRB.UserPrincipalName = name

			return newGRB
		}

		mockTime := func() time.Time {
			return time.Unix(0, 0)
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
					state.clusterCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()

					// mocks for just global permissions
					state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
						return nil, fmt.Errorf("not found")
					}
					state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
						state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
						return crb, nil
					}

					// mocks for fleet workspace permissions
					state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding, _ *[]metav1.Condition) error {
						state.stateChanges.fwhCalled = true
						return nil
					}
					// Allow the username specified to be queried for.
					state.userListerMock.GetFunc = func(namespace, name string) (*v3.User, error) {
						return &v3.User{
							ObjectMeta: metav1.ObjectMeta{
								Name: name,
							},
							PrincipalIDs: []string{
								testPrincipal,
							},
						}, nil
					}
					// mocks for status field
					state.grbClientMock.EXPECT().UpdateStatus(&v3.GlobalRoleBinding{
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
						Status: v3.GlobalRoleBindingStatus{
							LastUpdateTime: mockTime().Format(time.RFC3339),
							SummaryLocal:   status.SummaryCompleted,
							Summary:        status.SummaryCompleted,
							LocalConditions: []metav1.Condition{
								{
									Type:               subjectReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             subjectExists,
								},
								{
									Type:               clusterPermissionsReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             clusterPermissionsReconciled,
								},
								{
									Type:               globalRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             globalRoleBindingReconciled,
								},
								{
									Type:               namespacedRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             namespacedRoleBindingReconciled,
								},
							},
						},
					})
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
				wantBinding:  addUserPrincipalName(addAnnotation(&grb), testPrincipal),
				wantError:    false,
			},
			{
				name: "success on both cluster and global permissions - ClusterRole already exists and need to be updated with new Subject",
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
					state.clusterCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()

					// mocks for just global permissions
					state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
						return &rbacv1.ClusterRoleBinding{
							ObjectMeta: metav1.ObjectMeta{
								Name: "cattle-globalrolebinding-test-grb",
							},
						}, nil
					}
					state.crbClientMock.UpdateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
						state.stateChanges.updatedCRBs = append(state.stateChanges.updatedCRBs, crb)
						return crb, nil
					}

					// mocks for fleet workspace permissions
					state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding, _ *[]metav1.Condition) error {
						state.stateChanges.fwhCalled = true
						return nil
					}
					// Allow the username specified to be queried for.
					state.userListerMock.GetFunc = func(namespace, name string) (*v3.User, error) {
						return &v3.User{
							ObjectMeta: metav1.ObjectMeta{
								Name: name,
							},
							PrincipalIDs: []string{
								testPrincipal,
							},
						}, nil
					}
					// mocks for status field
					state.grbClientMock.EXPECT().UpdateStatus(&v3.GlobalRoleBinding{
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
						Status: v3.GlobalRoleBindingStatus{
							LastUpdateTime: mockTime().Format(time.RFC3339),
							Summary:        status.SummaryCompleted,
							SummaryLocal:   status.SummaryCompleted,
							LocalConditions: []metav1.Condition{
								{
									Type:               subjectReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             subjectExists,
								},
								{
									Type:               clusterPermissionsReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             clusterPermissionsReconciled,
								},
								{
									Type:               globalRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             globalRoleBindingReconciled,
								},
								{
									Type:               namespacedRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             namespacedRoleBindingReconciled,
								},
							},
						},
					})
				},
				stateAssertions: func(stateChanges grbTestStateChanges) {
					require.Len(stateChanges.t, stateChanges.createdCRTBs, 1)
					require.Len(stateChanges.t, stateChanges.updatedCRBs, 1)
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
					crb := stateChanges.updatedCRBs[0]
					bindingName := "cattle-globalrolebinding-" + grb.Name
					roleName := "cattle-globalrole-" + gr.Name
					require.Equal(stateChanges.t, bindingName, crb.Name)
					require.Equal(stateChanges.t, rbacv1.RoleRef{Name: roleName, Kind: "ClusterRole"}, crb.RoleRef)
					require.Equal(stateChanges.t, rbacv1.Subject{Name: userName, Kind: "User", APIGroup: rbacv1.GroupName}, crb.Subjects[0])

					// fleet workspace assertions
					require.Equal(stateChanges.t, true, stateChanges.fwhCalled)
				},
				inputBinding: grb.DeepCopy(),
				wantBinding:  addUserPrincipalName(&grb, testPrincipal),
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
					state.clusterCacheMock.EXPECT().List(labels.Everything()).Return(nil, fmt.Errorf("server not available")).AnyTimes()

					// mocks for just global permissions
					state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
						return nil, fmt.Errorf("not found")
					}
					state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
						state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
						return crb, nil
					}

					// mocks for fleet workspace permissions
					state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding, _ *[]metav1.Condition) error {
						state.stateChanges.fwhCalled = true
						return nil
					}
					// Allow the username specified to be queried for.
					state.userListerMock.GetFunc = func(namespace, name string) (*v3.User, error) {
						return &v3.User{
							ObjectMeta: metav1.ObjectMeta{
								Name: name,
							},
							PrincipalIDs: []string{
								testPrincipal,
							},
						}, nil
					}
					// mocks for status field
					state.grbClientMock.EXPECT().UpdateStatus(&v3.GlobalRoleBinding{
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
						Status: v3.GlobalRoleBindingStatus{
							LastUpdateTime: mockTime().Format(time.RFC3339),
							SummaryLocal:   status.SummaryError,
							Summary:        status.SummaryError,
							LocalConditions: []metav1.Condition{
								{
									Type:               subjectReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             subjectExists,
								},
								{
									Type:               clusterPermissionsReconciled,
									Status:             metav1.ConditionFalse,
									Message:            "server not available",
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             failedToListCluster,
								},
								{
									Type:               globalRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             globalRoleBindingReconciled,
								},
								{
									Type:               namespacedRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             namespacedRoleBindingReconciled,
								},
							},
						},
					})
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
				wantBinding:  addUserPrincipalName(addAnnotation(&grb), testPrincipal),
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
					state.clusterCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()

					// mocks for just global permissions
					state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
						return nil, fmt.Errorf("not found")
					}
					state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
						state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
						return nil, fmt.Errorf("server not available")
					}

					// mocks for fleet workspace permissions
					state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding, _ *[]metav1.Condition) error {
						state.stateChanges.fwhCalled = true
						return nil
					}
					// Allow the username specified to be queried for.
					state.userListerMock.GetFunc = func(namespace, name string) (*v3.User, error) {
						return &v3.User{
							ObjectMeta: metav1.ObjectMeta{
								Name: name,
							},
							PrincipalIDs: []string{
								testPrincipal,
							},
						}, nil
					}
					// mocks for status field
					state.grbClientMock.EXPECT().UpdateStatus(&v3.GlobalRoleBinding{
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
						Status: v3.GlobalRoleBindingStatus{
							LastUpdateTime: mockTime().Format(time.RFC3339),
							SummaryLocal:   status.SummaryError,
							Summary:        status.SummaryError,
							LocalConditions: []metav1.Condition{
								{
									Type:               subjectReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             subjectExists,
								},
								{
									Type:               clusterPermissionsReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             clusterPermissionsReconciled,
								},
								{
									Type:               globalRoleBindingReconciled,
									Status:             metav1.ConditionFalse,
									Message:            "server not available",
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             failedToCreateClusterRoleBinding,
								},
								{
									Type:               namespacedRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             namespacedRoleBindingReconciled,
								},
							},
						},
					})
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
				wantBinding:  addUserPrincipalName(&grb, testPrincipal),
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
					state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding, _ *[]metav1.Condition) error {
						state.stateChanges.fwhCalled = true
						return nil
					}
					// Allow the provided user to be looked up
					state.userListerMock.GetFunc = func(namespace, name string) (*v3.User, error) {
						return &v3.User{
							ObjectMeta: metav1.ObjectMeta{
								Name: name,
							},
							PrincipalIDs: []string{
								testPrincipal,
							},
						}, nil
					}
					// mocks for status field
					state.grbClientMock.EXPECT().UpdateStatus(&v3.GlobalRoleBinding{
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
						Status: v3.GlobalRoleBindingStatus{
							LastUpdateTime: mockTime().Format(time.RFC3339),
							SummaryLocal:   status.SummaryError,
							Summary:        status.SummaryError,
							LocalConditions: []metav1.Condition{
								{
									Type:               subjectReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             subjectExists,
								},
								{
									Type:               clusterPermissionsReconciled,
									Status:             metav1.ConditionFalse,
									Message:            "not found",
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             failedToGetGlobalRole,
								},
								{
									Type:               globalRoleBindingReconciled,
									Status:             metav1.ConditionFalse,
									Message:            "server not available",
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             failedToCreateClusterRoleBinding,
								},
								{
									Type:               namespacedRoleBindingReconciled,
									Status:             metav1.ConditionFalse,
									Message:            "not found",
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             failedToGetGlobalRole,
								},
							},
						},
					})
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
				wantBinding:  addUserPrincipalName(&grb, testPrincipal),
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
					state.clusterCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()

					// mocks for just global permissions
					state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
						return nil, fmt.Errorf("not found")
					}
					state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
						state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
						return crb, nil
					}

					// mocks for fleet workspace permissions
					state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding, _ *[]metav1.Condition) error {
						state.stateChanges.fwhCalled = true
						return fmt.Errorf("unavailable")
					}
					// Allow the provided user to be looked up
					state.userListerMock.GetFunc = func(namespace, name string) (*v3.User, error) {
						return &v3.User{
							ObjectMeta: metav1.ObjectMeta{
								Name: name,
							},
							PrincipalIDs: []string{
								testPrincipal,
							},
						}, nil
					}
					// mocks for status field
					state.grbClientMock.EXPECT().UpdateStatus(&v3.GlobalRoleBinding{
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
						Status: v3.GlobalRoleBindingStatus{
							LastUpdateTime: mockTime().Format(time.RFC3339),
							SummaryLocal:   status.SummaryCompleted,
							Summary:        status.SummaryCompleted,
							LocalConditions: []metav1.Condition{
								{
									Type:               subjectReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             subjectExists,
								},
								{
									Type:               clusterPermissionsReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             clusterPermissionsReconciled,
								},
								{
									Type:               globalRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             globalRoleBindingReconciled,
								},
								{
									Type:               namespacedRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             namespacedRoleBindingReconciled,
								},
							},
						},
					})
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
				wantBinding:  addUserPrincipalName(addAnnotation(&grb), testPrincipal),
				wantError:    true,
			},
			{
				name: "binding referencing user triggers creation of user",
				stateSetup: func(state grbTestState) {
					state.clusterCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()
					state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
						return nil, nil
					}
					state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
					state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
					state.grListerMock.GetFunc = func(namespace, name string) (*v3.GlobalRole, error) {
						if name == gr.Name && namespace == "" {
							return &gr, nil
						}
						return nil, fmt.Errorf("not found")
					}
					state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
						state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
						return crb, nil
					}
					state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
						state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
						return crtb, nil
					}
					state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
						return nil, fmt.Errorf("not found")
					}
					state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding, _ *[]metav1.Condition) error {
						state.stateChanges.fwhCalled = true
						return nil
					}
					state.userManagerMock.EXPECT().EnsureUser(
						"activedirectory_user://CN=test,CN=Users,DC=ad,DC=ians,DC=farm", "test-user").Return(
						&v3.User{ObjectMeta: metav1.ObjectMeta{Name: "test-user"}}, nil)
					// mocks for status field
					state.grbClientMock.EXPECT().UpdateStatus(&v3.GlobalRoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-grb",
							UID:  "1234",
							Annotations: map[string]string{
								"auth.cattle.io/principal-display-name": "test-user",
							},
						},
						TypeMeta: metav1.TypeMeta{
							APIVersion: apisv3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
							Kind:       apisv3.GlobalRoleBindingGroupVersionKind.Kind,
						},
						GlobalRoleName:    gr.Name,
						UserPrincipalName: "activedirectory_user://CN=test,CN=Users,DC=ad,DC=ians,DC=farm",
						Status: v3.GlobalRoleBindingStatus{
							LastUpdateTime: mockTime().Format(time.RFC3339),
							SummaryLocal:   status.SummaryCompleted,
							Summary:        status.SummaryCompleted,
							LocalConditions: []metav1.Condition{
								{
									Type:               subjectReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             subjectExists,
								},
								{
									Type:               clusterPermissionsReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             clusterPermissionsReconciled,
								},
								{
									Type:               globalRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             globalRoleBindingReconciled,
								},
								{
									Type:               namespacedRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             namespacedRoleBindingReconciled,
								},
							},
						},
					})
				},
				inputBinding: &v3.GlobalRoleBinding{
					TypeMeta: metav1.TypeMeta{
						APIVersion: apisv3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
						Kind:       apisv3.GlobalRoleBindingGroupVersionKind.Kind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-grb",
						UID:  "1234",
						Annotations: map[string]string{
							"auth.cattle.io/principal-display-name": "test-user",
						},
					},
					GlobalRoleName:    gr.Name,
					UserPrincipalName: "activedirectory_user://CN=test,CN=Users,DC=ad,DC=ians,DC=farm",
				},
				wantBinding: addUserPrincipalName(addAnnotation(&grb, map[string]string{
					"auth.cattle.io/principal-display-name": "test-user",
				}), "activedirectory_user://CN=test,CN=Users,DC=ad,DC=ians,DC=farm"),
				wantError: false,
			},
			{
				name: "binding referencing groupName and no user or userPrincipal",
				stateSetup: func(state grbTestState) {
					state.clusterCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()
					state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
						return nil, nil
					}
					state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
					state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
					state.grListerMock.GetFunc = func(namespace, name string) (*v3.GlobalRole, error) {
						if name == gr.Name && namespace == "" {
							return &gr, nil
						}
						return nil, fmt.Errorf("not found")
					}
					state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
						state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
						return crb, nil
					}
					state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
						state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
						return crtb, nil
					}
					state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
						return nil, fmt.Errorf("not found")
					}
					state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding, _ *[]metav1.Condition) error {
						state.stateChanges.fwhCalled = true
						return nil
					}
					// mocks for status field
					state.grbClientMock.EXPECT().UpdateStatus(&v3.GlobalRoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-grb",
							UID:  "1234",
						},
						TypeMeta: metav1.TypeMeta{
							APIVersion: apisv3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
							Kind:       apisv3.GlobalRoleBindingGroupVersionKind.Kind,
						},
						GlobalRoleName:     gr.Name,
						GroupPrincipalName: "activedirectory_user://CN=Users,DC=ad,DC=ians,DC=farm",
						Status: v3.GlobalRoleBindingStatus{
							LastUpdateTime: mockTime().Format(time.RFC3339),
							SummaryLocal:   status.SummaryCompleted,
							Summary:        status.SummaryCompleted,
							LocalConditions: []metav1.Condition{
								{
									Type:               subjectReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             subjectExists,
								},
								{
									Type:               clusterPermissionsReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             clusterPermissionsReconciled,
								},
								{
									Type:               globalRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             globalRoleBindingReconciled,
								},
								{
									Type:               namespacedRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             namespacedRoleBindingReconciled,
								},
							},
						},
					})
				},
				inputBinding: &v3.GlobalRoleBinding{
					TypeMeta: metav1.TypeMeta{
						APIVersion: apisv3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
						Kind:       apisv3.GlobalRoleBindingGroupVersionKind.Kind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-grb",
						UID:  "1234",
					},
					GlobalRoleName:     gr.Name,
					GroupPrincipalName: "activedirectory_user://CN=Users,DC=ad,DC=ians,DC=farm",
				},
				wantBinding: &v3.GlobalRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-grb",
						UID:  "1234",
						Annotations: map[string]string{
							"authz.management.cattle.io/crb-name": "cattle-globalrolebinding-test-grb",
						},
					},
					TypeMeta: metav1.TypeMeta{
						APIVersion: apisv3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
						Kind:       apisv3.GlobalRoleBindingGroupVersionKind.Kind,
					},
					GroupPrincipalName: "activedirectory_user://CN=Users,DC=ad,DC=ians,DC=farm",
					GlobalRoleName:     "test-gr",
				},
				wantError: false,
			},
			{
				name: "binding referencing user links to existing user",
				stateSetup: func(state grbTestState) {
					state.clusterCacheMock.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()
					state.rbListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.RoleBinding, error) {
						return nil, nil
					}
					state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
					state.crtbCacheMock.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
					state.grListerMock.GetFunc = func(namespace, name string) (*v3.GlobalRole, error) {
						if name == gr.Name && namespace == "" {
							return &gr, nil
						}
						return nil, fmt.Errorf("not found")
					}
					state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
						state.stateChanges.createdCRBs = append(state.stateChanges.createdCRBs, crb)
						return crb, nil
					}
					state.crtbClientMock.CreateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
						state.stateChanges.createdCRTBs = append(state.stateChanges.createdCRTBs, crtb)
						return crtb, nil
					}
					state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
						return nil, fmt.Errorf("not found")
					}
					state.fwhMock.reconcileFleetWorkspacePermissionsFunc = func(globalRoleBinding *v3.GlobalRoleBinding, _ *[]metav1.Condition) error {
						state.stateChanges.fwhCalled = true
						return nil
					}
					// Allow the provided user to be looked up
					state.userListerMock.GetFunc = func(namespace, name string) (*v3.User, error) {
						return &v3.User{
							ObjectMeta: metav1.ObjectMeta{
								Name: name,
							},
							PrincipalIDs: []string{
								testPrincipal,
							},
						}, nil
					}
					// mocks for status field
					state.grbClientMock.EXPECT().UpdateStatus(&v3.GlobalRoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-grb",
							UID:  "1234",
							Annotations: map[string]string{
								"auth.cattle.io/principal-display-name": "test-user",
							},
						},
						TypeMeta: metav1.TypeMeta{
							APIVersion: apisv3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
							Kind:       apisv3.GlobalRoleBindingGroupVersionKind.Kind,
						},
						GlobalRoleName: gr.Name,
						UserName:       "test-user",
						Status: v3.GlobalRoleBindingStatus{
							LastUpdateTime: mockTime().Format(time.RFC3339),
							SummaryLocal:   status.SummaryCompleted,
							Summary:        status.SummaryCompleted,
							LocalConditions: []metav1.Condition{
								{
									Type:               subjectReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             subjectExists,
								},
								{
									Type:               clusterPermissionsReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             clusterPermissionsReconciled,
								},
								{
									Type:               globalRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             globalRoleBindingReconciled,
								},
								{
									Type:               namespacedRoleBindingReconciled,
									Status:             metav1.ConditionTrue,
									LastTransitionTime: metav1.Time{Time: mockTime()},
									Reason:             namespacedRoleBindingReconciled,
								},
							},
						},
					})
				},
				inputBinding: &v3.GlobalRoleBinding{
					TypeMeta: metav1.TypeMeta{
						APIVersion: apisv3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
						Kind:       apisv3.GlobalRoleBindingGroupVersionKind.Kind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-grb",
						UID:  "1234",
						Annotations: map[string]string{
							"auth.cattle.io/principal-display-name": "test-user",
						},
					},
					GlobalRoleName: gr.Name,
					UserName:       "test-user",
				},
				wantBinding: &v3.GlobalRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-grb",
						UID:  "1234",
						Annotations: map[string]string{
							"auth.cattle.io/principal-display-name": "test-user",
							"authz.management.cattle.io/crb-name":   "cattle-globalrolebinding-test-grb",
						},
					},
					TypeMeta: metav1.TypeMeta{
						APIVersion: apisv3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
						Kind:       apisv3.GlobalRoleBindingGroupVersionKind.Kind,
					},
					GlobalRoleName:    gr.Name,
					UserName:          "test-user",
					UserPrincipalName: testPrincipal,
				},
				wantError: false,
			},
		}

		for _, test := range tests {
			test := test
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				grbLifecycle := globalRoleBindingLifecycle{}
				testFuncs := []func(*v3.GlobalRoleBinding) (runtime.Object, error){grbLifecycle.Updated, grbLifecycle.Create}
				for _, testFunc := range testFuncs {
					ctrl := gomock.NewController(t)
					crtbCacheMock := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
					grListerMock := fakes.GlobalRoleListerMock{}
					crbListerMock := rbacFakes.ClusterRoleBindingListerMock{}
					clusterCacheMock := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
					crtbClientMock := fakes.ClusterRoleTemplateBindingInterfaceMock{}
					crbClientMock := rbacFakes.ClusterRoleBindingInterfaceMock{}
					rbListerMock := rbacFakes.RoleBindingListerMock{}
					grbListerMock := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
					grbClientMock := fake.NewMockNonNamespacedControllerInterface[*v3.GlobalRoleBinding, *v3.GlobalRoleBindingList](ctrl)
					fphMock := fleetPermissionsHandlerMock{}
					grbListerMock.EXPECT().Get(test.inputBinding.Name).Return(test.inputBinding.DeepCopy(), nil)
					stateChanges := grbTestStateChanges{
						t:                t,
						createdCRTBs:     []*v3.ClusterRoleTemplateBinding{},
						createdCRBs:      []*rbacv1.ClusterRoleBinding{},
						deletedCRTBNames: []string{},
					}
					state := grbTestState{
						crtbCacheMock:    crtbCacheMock,
						grListerMock:     &grListerMock,
						crbListerMock:    &crbListerMock,
						clusterCacheMock: clusterCacheMock,
						crtbClientMock:   &crtbClientMock,
						crbClientMock:    &crbClientMock,
						rbListerMock:     &rbListerMock,
						fwhMock:          &fphMock,
						grbListerMock:    grbListerMock,
						grbClientMock:    grbClientMock,
						userManagerMock:  userMocks.NewMockManager(ctrl),
						userListerMock:   &fakes.UserListerMock{},
						stateChanges:     &stateChanges,
					}
					if test.stateSetup != nil {
						test.stateSetup(state)
					}
					grbLifecycle.grLister = &grListerMock
					grbLifecycle.crbLister = &crbListerMock
					grbLifecycle.crtbCache = crtbCacheMock
					grbLifecycle.clusterLister = clusterCacheMock
					grbLifecycle.crtbClient = &crtbClientMock
					grbLifecycle.crbClient = &crbClientMock
					grbLifecycle.roleBindingLister = &rbListerMock
					grbLifecycle.fleetPermissionsHandler = &fphMock
					grbLifecycle.grbLister = grbListerMock
					grbLifecycle.grbClient = grbClientMock
					grbLifecycle.status = status.NewStatus()
					grbLifecycle.status.TimeNow = mockTime
					grbLifecycle.userManager = state.userManagerMock
					grbLifecycle.userLister = state.userListerMock
					res, resErr := testFunc(test.inputBinding.DeepCopy())
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
*/
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

func TestReconcileClusterPermissions(t *testing.T) {
	t.Parallel()
	defaultCRTB := v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "crtb-grb-",
			Namespace:    "not-local",
			Labels: map[string]string{
				grbOwnerLabel:               "test-grb",
				controllers.K8sManagedByKey: controllers.ManagerValue,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
					Name:       "test-grb",
					UID:        "1234",
				},
			},
		},
		ClusterName:      "not-local",
		RoleTemplateName: "cluster-owner",
		UserName:         "test-user",
	}

	type controllers struct {
		crtbCache      *fake.MockCacheInterface[*v3.ClusterRoleTemplateBinding]
		crtbController *fake.MockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList]
		grCache        *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
		clusterCache   *fake.MockNonNamespacedCacheInterface[*v3.Cluster]
	}

	tests := []struct {
		name             string
		setupControllers func(controllers)
		inputObject      *v3.GlobalRoleBinding
		stateAssertions  func(stateChanges grbTestStateChanges)
		wantError        bool
	}{
		{
			name: "no inherited roles",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(noInheritTestGR.Name).Return(noInheritTestGR.DeepCopy(), nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				c.clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()
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
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(missingInheritTestGR.Name).Return(missingInheritTestGR.DeepCopy(), nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				c.clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()
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
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(inheritedTestGr.Name).Return(inheritedTestGr.DeepCopy(), nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				c.clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()
				c.crtbController.EXPECT().Create(defaultCRTB.DeepCopy()).Return(nil, nil)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: false,
		},
		{
			name: "cluster lister error",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(inheritedTestGr.Name).Return(inheritedTestGr.DeepCopy(), nil)
				c.clusterCache.EXPECT().List(labels.Everything()).Return(nil, fmt.Errorf("server unavailable"))
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
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(inheritedTestGr.Name).Return(inheritedTestGr.DeepCopy(), nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "error/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				c.clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&errorCluster, &notLocalCluster, &localCluster}, nil)
				c.crtbController.EXPECT().Create(defaultCRTB.DeepCopy()).Return(nil, nil)
				errorCRTB := defaultCRTB.DeepCopy()
				errorCRTB.Namespace = "error"
				errorCRTB.ClusterName = "error"
				c.crtbController.EXPECT().Create(errorCRTB).Return(nil, fmt.Errorf("server unavailable"))
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: true,
		},
		{
			name: "indexer error",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(inheritedTestGr.Name).Return(inheritedTestGr.DeepCopy(), nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "error/test-grb").Return(nil, fmt.Errorf("indexer error")).Times(2)
				c.clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&errorCluster, &notLocalCluster, &localCluster}, nil).AnyTimes()
				c.crtbController.EXPECT().Create(defaultCRTB.DeepCopy()).Return(nil, nil)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: true,
		},
		{
			name: "crtb delete error",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(inheritedTestGr.Name).Return(inheritedTestGr.DeepCopy(), nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{
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
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "error/test-grb").Return([]*v3.ClusterRoleTemplateBinding{
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
				c.clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&errorCluster, &localCluster}, nil).AnyTimes()
				c.crtbController.EXPECT().Delete("local", "crtb-grb-delete-local", gomock.Any()).Return(fmt.Errorf("server unavailable"))
				c.crtbController.EXPECT().Delete("error", "crtb-grb-delete", gomock.Any()).Return(fmt.Errorf("server unavailable"))
				errorCRTB := defaultCRTB.DeepCopy()
				errorCRTB.Namespace = "error"
				errorCRTB.ClusterName = "error"
				c.crtbController.EXPECT().Create(errorCRTB).Return(nil, nil)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: true,
		},
		{
			name: "no global role",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get("error").Return(nil, fmt.Errorf("error"))
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
	ctrl := gomock.NewController(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			controllers := controllers{
				grCache:        fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl),
				crtbCache:      fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl),
				clusterCache:   fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl),
				crtbController: fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl),
			}
			if test.setupControllers != nil {
				test.setupControllers(controllers)
			}

			grbLifecycle := globalRoleBindingLifecycle{
				grLister:      controllers.grCache,
				crtbCache:     controllers.crtbCache,
				clusterLister: controllers.clusterCache,
				crtbClient:    controllers.crtbController,
				status:        status.NewStatus(),
			}
			var conditions []metav1.Condition
			resErr := grbLifecycle.reconcileClusterPermissions(test.inputObject, &conditions)
			if test.wantError {
				require.Error(t, resErr)
			} else {
				require.NoError(t, resErr)
			}
		})
	}

}

func TestReconcileGlobalRoleBinding(t *testing.T) {
	t.Parallel()

	testGR := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gr",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
			},
		},
	}

	testGRWithAnnotation := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gr",
			Annotations: map[string]string{
				crNameAnnotation: "custom-cr-name",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
			},
		},
	}

	testGRB := v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-grb",
			UID:  "1234",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "management.cattle.io/v3",
			Kind:       "GlobalRoleBinding",
		},
		GlobalRoleName: "test-gr",
		UserName:       "test-user",
	}

	testGRBWithAnnotation := v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-grb",
			UID:  "1234",
			Annotations: map[string]string{
				crbNameAnnotation: "custom-crb-name",
			},
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "management.cattle.io/v3",
			Kind:       "GlobalRoleBinding",
		},
		GlobalRoleName: "test-gr",
		UserName:       "test-user",
	}

	expectedCRB := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: crbNamePrefix + "test-grb",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
					Name:       "test-grb",
					UID:        "1234",
				},
			},
			Labels: globalRoleBindingLabel,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     "test-user",
				APIGroup: rbacv1.GroupName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name: generateCRName("test-gr"),
			Kind: clusterRoleKind,
		},
	}

	type controllers struct {
		crbCache      *fake.MockNonNamespacedCacheInterface[*rbacv1.ClusterRoleBinding]
		crbController *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]
		grCache       *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
	}

	tests := []struct {
		name             string
		setupControllers func(controllers)
		inputObject      *v3.GlobalRoleBinding
		wantError        bool
		wantAnnotation   string
	}{
		{
			name: "create new clusterRoleBinding",
			setupControllers: func(c controllers) {
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(nil, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				c.crbController.EXPECT().Create(expectedCRB.DeepCopy()).Return(&expectedCRB, nil)
			},
			inputObject:    testGRB.DeepCopy(),
			wantError:      false,
			wantAnnotation: crbNamePrefix + "test-grb",
		},
		{
			name: "create new clusterRoleBinding with custom annotation",
			setupControllers: func(c controllers) {
				c.crbCache.EXPECT().Get("custom-crb-name").Return(nil, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				customCRB := expectedCRB.DeepCopy()
				customCRB.Name = "custom-crb-name"
				c.crbController.EXPECT().Create(customCRB).Return(customCRB, nil)
			},
			inputObject:    testGRBWithAnnotation.DeepCopy(),
			wantError:      false,
			wantAnnotation: "custom-crb-name",
		},
		{
			name: "create new clusterRoleBinding uses CR name from GlobalRole annotation",
			setupControllers: func(c controllers) {
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(nil, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGRWithAnnotation.DeepCopy(), nil)
				crbWithCustomCR := expectedCRB.DeepCopy()
				crbWithCustomCR.RoleRef.Name = "custom-cr-name"
				c.crbController.EXPECT().Create(crbWithCustomCR).Return(crbWithCustomCR, nil)
			},
			inputObject:    testGRB.DeepCopy(),
			wantError:      false,
			wantAnnotation: crbNamePrefix + "test-grb",
		},
		{
			name: "clusterRoleBinding creation fails",
			setupControllers: func(c controllers) {
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(nil, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				c.crbController.EXPECT().Create(expectedCRB.DeepCopy()).Return(nil, fmt.Errorf("server unavailable"))
			},
			inputObject: testGRB.DeepCopy(),
			wantError:   true,
		},
		{
			name: "clusterRoleBinding already exists no update needed",
			setupControllers: func(c controllers) {
				existingCRB := expectedCRB.DeepCopy()
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(existingCRB, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
			},
			inputObject: testGRB.DeepCopy(),
			wantError:   false,
		},
		{
			name: "update clusterRoleBinding subject",
			setupControllers: func(c controllers) {
				existingCRB := expectedCRB.DeepCopy()
				existingCRB.Subjects = []rbacv1.Subject{
					{
						Kind:     "User",
						Name:     "old-user",
						APIGroup: rbacv1.GroupName,
					},
				}
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(existingCRB, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				updatedCRB := expectedCRB.DeepCopy()
				c.crbController.EXPECT().Update(updatedCRB).Return(updatedCRB, nil)
			},
			inputObject: testGRB.DeepCopy(),
			wantError:   false,
		},
		{
			name: "update clusterRoleBinding roleRef",
			setupControllers: func(c controllers) {
				existingCRB := expectedCRB.DeepCopy()
				existingCRB.RoleRef = rbacv1.RoleRef{
					Name: "old-role",
					Kind: clusterRoleKind,
				}
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(existingCRB, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				updatedCRB := expectedCRB.DeepCopy()
				c.crbController.EXPECT().Update(updatedCRB).Return(updatedCRB, nil)
			},
			inputObject: testGRB.DeepCopy(),
			wantError:   false,
		},
		{
			name: "update clusterRoleBinding subject and roleRef",
			setupControllers: func(c controllers) {
				existingCRB := expectedCRB.DeepCopy()
				existingCRB.Subjects = []rbacv1.Subject{
					{
						Kind:     "User",
						Name:     "old-user",
						APIGroup: rbacv1.GroupName,
					},
				}
				existingCRB.RoleRef = rbacv1.RoleRef{
					Name: "old-role",
					Kind: clusterRoleKind,
				}
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(existingCRB, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				updatedCRB := expectedCRB.DeepCopy()
				c.crbController.EXPECT().Update(updatedCRB).Return(updatedCRB, nil)
			},
			inputObject: testGRB.DeepCopy(),
			wantError:   false,
		},
		{
			name: "update clusterRoleBinding fails",
			setupControllers: func(c controllers) {
				existingCRB := expectedCRB.DeepCopy()
				existingCRB.Subjects = []rbacv1.Subject{
					{
						Kind:     "User",
						Name:     "old-user",
						APIGroup: rbacv1.GroupName,
					},
				}
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(existingCRB, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				updatedCRB := expectedCRB.DeepCopy()
				c.crbController.EXPECT().Update(updatedCRB).Return(nil, fmt.Errorf("server unavailable"))
			},
			inputObject: testGRB.DeepCopy(),
			wantError:   true,
		},
		{
			name: "globalRole not found uses generated name",
			setupControllers: func(c controllers) {
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(nil, nil)
				c.grCache.EXPECT().Get("test-gr").Return(nil, apierrors.NewNotFound(schema.GroupResource{
					Group:    "management.cattle.io",
					Resource: "GlobalRole",
				}, "test-gr"))
				c.crbController.EXPECT().Create(expectedCRB.DeepCopy()).Return(&expectedCRB, nil)
			},
			inputObject:    testGRB.DeepCopy(),
			wantError:      false,
			wantAnnotation: crbNamePrefix + "test-grb",
		},
		{
			name: "group principal binding",
			setupControllers: func(c controllers) {
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(nil, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				groupCRB := expectedCRB.DeepCopy()
				groupCRB.Subjects = []rbacv1.Subject{
					{
						Kind:     "Group",
						Name:     "test-group",
						APIGroup: rbacv1.GroupName,
					},
				}
				c.crbController.EXPECT().Create(groupCRB).Return(groupCRB, nil)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
				},
				GlobalRoleName:     "test-gr",
				GroupPrincipalName: "test-group",
			},
			wantError:      false,
			wantAnnotation: crbNamePrefix + "test-grb",
		},
	}

	ctrl := gomock.NewController(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			controllers := controllers{
				crbCache:      fake.NewMockNonNamespacedCacheInterface[*rbacv1.ClusterRoleBinding](ctrl),
				crbController: fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl),
				grCache:       fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl),
			}
			if test.setupControllers != nil {
				test.setupControllers(controllers)
			}

			grbLifecycle := globalRoleBindingLifecycle{
				crbLister: controllers.crbCache,
				crbClient: controllers.crbController,
				grLister:  controllers.grCache,
				status:    status.NewStatus(),
			}
			var conditions []metav1.Condition
			resErr := grbLifecycle.reconcileGlobalRoleBinding(test.inputObject, &conditions)
			if test.wantError {
				require.Error(t, resErr)
			} else {
				require.NoError(t, resErr)
			}

			if test.wantAnnotation != "" {
				require.Equal(t, test.wantAnnotation, test.inputObject.Annotations[crbNameAnnotation])
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
	errRoleNotFound := apierrors.NewNotFound(schema.GroupResource{
		Group:    "rbac.authorization.k8s.io",
		Resource: "RoleBinding",
	}, "")
	rb1 := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespacedRulesGRB-namespace1",
			Namespace: "namespace1",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "fake-version",
					Kind:       "fake-kind",
					Name:       "namespacedRulesGRB",
					UID:        "1234",
				},
			},
			Labels: map[string]string{
				grbOwnerLabel: "namespacedRulesGRB",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "namespacedRulesGR-namespace1",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     "username",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
	rb2 := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespacedRulesGRB-namespace2",
			Namespace: "namespace2",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "fake-version",
					Kind:       "fake-kind",
					Name:       "namespacedRulesGRB",
					UID:        "1234",
				},
			},
			Labels: map[string]string{
				grbOwnerLabel: "namespacedRulesGRB",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "namespacedRulesGR-namespace2",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     "username",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
	badRB := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "badRB",
			Namespace: "namespace1",
			UID:       "666",
		},
	}

	type controllers struct {
		grCache      *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
		nsCache      *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]
		rbCache      *fake.MockCacheInterface[*rbacv1.RoleBinding]
		rbController *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]
	}
	tests := []struct {
		name              string
		setupControllers  func(controllers)
		globalRoleBinding *v3.GlobalRoleBinding
		wantError         bool
	}{
		{
			name: "global role not found",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(nil, fmt.Errorf("error"))
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "getting namespace fails",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(activeNamespace, fmt.Errorf("error")).Times(2)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "namespace is nil",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(nil, nil).Times(2)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "getting roleBinding fails",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(activeNamespace, nil).Times(2)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(nil, fmt.Errorf("error"))
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, fmt.Errorf("error"))
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "creating roleBinding fails",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(activeNamespace, nil).Times(2)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(nil, errRoleNotFound)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, errRoleNotFound)
				c.rbController.EXPECT().Create(rb1.DeepCopy()).Return(nil, fmt.Errorf("error"))
				c.rbController.EXPECT().Create(rb2.DeepCopy()).Return(nil, fmt.Errorf("error"))
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "roleBindings get created",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(activeNamespace, nil).Times(2)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(nil, errRoleNotFound)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, errRoleNotFound)
				c.rbController.EXPECT().Create(rb1.DeepCopy()).Return(rb1.DeepCopy(), nil)
				c.rbController.EXPECT().Create(rb2.DeepCopy()).Return(rb2.DeepCopy(), nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{rb1.DeepCopy(), rb2.DeepCopy()}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "roleBindings don't get created in a terminating namespace",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(terminatingNamespace, nil).Times(2)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(nil, errRoleNotFound)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, errRoleNotFound)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "one NS not found, still creates other RB",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get("namespace1").Return(terminatingNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(nil, errRoleNotFound)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, errRoleNotFound)
				c.rbController.EXPECT().Create(rb2.DeepCopy()).Return(rb2.DeepCopy(), nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{rb2.DeepCopy()}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "delete roleBinding from terminating namespace",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get("namespace1").Return(terminatingNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(badRB.DeepCopy(), nil)
				c.rbController.EXPECT().Delete("namespace1", "badRB", gomock.Any()).Return(nil)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, errRoleNotFound)
				c.rbController.EXPECT().Create(rb2.DeepCopy()).Return(rb2.DeepCopy(), nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{rb2.DeepCopy()}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "update roleBindings with bad roleRef name",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get("namespace1").Return(activeNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(badRB.DeepCopy(), nil)
				c.rbController.EXPECT().Create(rb1.DeepCopy()).Return(rb1.DeepCopy(), nil)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(badRB.DeepCopy(), nil)
				c.rbController.EXPECT().Create(rb2.DeepCopy()).Return(rb2.DeepCopy(), nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{rb1.DeepCopy(), rb2.DeepCopy()}, nil)
				c.rbController.EXPECT().Delete("namespace1", "badRB", gomock.Any()).Return(nil).Times(2)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "delete roleBindings fails",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get("namespace1").Return(activeNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(badRB.DeepCopy(), nil)
				c.rbController.EXPECT().Delete("namespace1", "badRB", gomock.Any()).Return(fmt.Errorf("error")).Times(2)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(badRB.DeepCopy(), nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "list RBs fails",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(activeNamespace, nil).Times(2)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(nil, errRoleNotFound)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, errRoleNotFound)
				c.rbController.EXPECT().Create(rb1.DeepCopy()).Return(rb1.DeepCopy(), nil)
				c.rbController.EXPECT().Create(rb2.DeepCopy()).Return(rb2.DeepCopy(), nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
	}

	ctrl := gomock.NewController(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			controllers := controllers{
				grCache:      fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl),
				nsCache:      fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl),
				rbCache:      fake.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl),
				rbController: fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl),
			}

			if test.setupControllers != nil {
				test.setupControllers(controllers)
			}

			grbLifecycle := globalRoleBindingLifecycle{
				grLister:          controllers.grCache,
				nsCache:           controllers.nsCache,
				roleBindings:      controllers.rbController,
				roleBindingLister: controllers.rbCache,
				status:            status.NewStatus(),
			}

			var conditions []metav1.Condition
			err := grbLifecycle.reconcileNamespacedRoleBindings(test.globalRoleBinding, &conditions)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type fleetPermissionsHandlerMock struct {
	reconcileFleetWorkspacePermissionsFunc func(globalRoleBinding *v3.GlobalRoleBinding, conditions *[]metav1.Condition) error
}

func (f *fleetPermissionsHandlerMock) reconcileFleetWorkspacePermissionsBindings(globalRoleBinding *v3.GlobalRoleBinding, conditions *[]metav1.Condition) error {
	return f.reconcileFleetWorkspacePermissionsFunc(globalRoleBinding, conditions)
}

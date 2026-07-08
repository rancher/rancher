package roletemplates

import (
	"reflect"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/status"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type reducedCondition struct {
	reason string
	status metav1.ConditionStatus
}

func Test_crtbHandler_reconcileSubject(t *testing.T) {
	type controllers struct {
		userMGR        *user.MockManager
		userController *fake.MockNonNamespacedControllerInterface[*v3.User, *v3.UserList]
	}
	tests := []struct {
		name             string
		binding          *v3.ClusterRoleTemplateBinding
		setupControllers func(controllers)
		want             *v3.ClusterRoleTemplateBinding
		wantErr          bool
		wantedCondition  *reducedCondition
	}{
		{
			name: "crtb has group",
			binding: &v3.ClusterRoleTemplateBinding{
				GroupName: "test-group",
			},
			wantedCondition: &reducedCondition{
				reason: subjectExists,
				status: metav1.ConditionTrue,
			},
			want: &v3.ClusterRoleTemplateBinding{
				GroupName: "test-group",
			},
		},
		{
			name: "crtb has group principal name",
			binding: &v3.ClusterRoleTemplateBinding{
				GroupPrincipalName: "test-groupprincipal",
			},
			wantedCondition: &reducedCondition{
				reason: subjectExists,
				status: metav1.ConditionTrue,
			},
			want: &v3.ClusterRoleTemplateBinding{
				GroupPrincipalName: "test-groupprincipal",
			},
		},
		{
			name: "crtb has user principal name and username",
			binding: &v3.ClusterRoleTemplateBinding{
				UserPrincipalName: "principal name",
				UserName:          "test-user",
			},
			wantedCondition: &reducedCondition{
				reason: subjectExists,
				status: metav1.ConditionTrue,
			},
			want: &v3.ClusterRoleTemplateBinding{
				UserPrincipalName: "principal name",
				UserName:          "test-user",
			},
		},
		{
			name:    "crtb has no user principal name and username",
			binding: &v3.ClusterRoleTemplateBinding{},
			wantedCondition: &reducedCondition{
				reason: crtbHasNoSubject,
				status: metav1.ConditionFalse,
			},
			want:    &v3.ClusterRoleTemplateBinding{},
			wantErr: true,
		},
		{
			name: "crtb has no username",
			binding: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"auth.cattle.io/principal-display-name": "test-name"},
				},
				UserPrincipalName: "principal-name",
			},
			setupControllers: func(c controllers) {
				c.userMGR.EXPECT().EnsureUser("principal-name", "test-name").Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				}, nil)
			},
			wantedCondition: &reducedCondition{
				reason: subjectExists,
				status: metav1.ConditionTrue,
			},
			want: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"auth.cattle.io/principal-display-name": "test-name"},
				},
				UserPrincipalName: "principal-name",
				UserName:          "test-user",
			},
		},
		{
			name: "crtb has no username error in EnsureUser",
			binding: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"auth.cattle.io/principal-display-name": "test-name"},
				},
				UserPrincipalName: "principal-name",
			},
			setupControllers: func(c controllers) {
				c.userMGR.EXPECT().EnsureUser("principal-name", "test-name").Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				}, errDefault)
			},
			wantedCondition: &reducedCondition{
				reason: failedToCreateUser,
				status: metav1.ConditionFalse,
			},
			want: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"auth.cattle.io/principal-display-name": "test-name"},
				},
				UserPrincipalName: "principal-name",
			},
			wantErr: true,
		},
		{
			name: "crtb has no user principal name",
			binding: &v3.ClusterRoleTemplateBinding{
				UserName: "test-user",
			},
			setupControllers: func(c controllers) {
				c.userController.EXPECT().Get("test-user", metav1.GetOptions{}).Return(&v3.User{
					PrincipalIDs: []string{"principal/test-user"},
				}, nil)
			},
			wantedCondition: &reducedCondition{
				reason: subjectExists,
				status: metav1.ConditionTrue,
			},
			want: &v3.ClusterRoleTemplateBinding{
				UserName:          "test-user",
				UserPrincipalName: "principal/test-user",
			},
		},
		{
			name: "crtb has no user principal name, error getting Users",
			binding: &v3.ClusterRoleTemplateBinding{
				UserName: "test-user",
			},
			setupControllers: func(c controllers) {
				c.userController.EXPECT().Get("test-user", metav1.GetOptions{}).Return(&v3.User{
					PrincipalIDs: []string{"principal/test-user"},
				}, errDefault)
			},
			wantedCondition: &reducedCondition{
				reason: failedToGetUser,
				status: metav1.ConditionFalse,
			},
			want: &v3.ClusterRoleTemplateBinding{
				UserName: "test-user",
			},
			wantErr: true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controllers := controllers{
				userMGR:        user.NewMockManager(ctrl),
				userController: fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl),
			}

			if tt.setupControllers != nil {
				tt.setupControllers(controllers)
			}
			c := &crtbHandler{
				s:              status.NewStatus(),
				userMGR:        controllers.userMGR,
				userController: controllers.userController,
			}
			localConditions := []metav1.Condition{}

			got, err := c.reconcileSubject(tt.binding, &localConditions)

			if (err != nil) != tt.wantErr {
				t.Errorf("crtbHandler.reconcileSubject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("crtbHandler.reconcileSubject() = %v, want %v", got, tt.want)
			}
			assert.Len(t, localConditions, 1)
			assert.Equal(t, tt.wantedCondition.reason, localConditions[0].Reason)
			assert.Equal(t, tt.wantedCondition.status, localConditions[0].Status)

		})
	}
}

func Test_crtbHandler_reconcileMembershipBindings(t *testing.T) {
	type controllers struct {
		rtController  *fake.MockNonNamespacedControllerInterface[*v3.RoleTemplate, *v3.RoleTemplateList]
		crbController *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]
	}
	tests := []struct {
		name             string
		crtb             *v3.ClusterRoleTemplateBinding
		setupControllers func(controllers)
		wantedCondition  *reducedCondition
		wantErr          bool
	}{
		{
			name: "error getting role template",
			crtb: &v3.ClusterRoleTemplateBinding{
				RoleTemplateName: "test-rt",
			},
			setupControllers: func(c controllers) {
				c.rtController.EXPECT().Get("test-rt", metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantedCondition: &reducedCondition{
				reason: failedToGetRoleTemplate,
				status: metav1.ConditionFalse,
			},
			wantErr: true,
		},
		{
			name: "error creating cluster membership binding",
			crtb: defaultCRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.rtController.EXPECT().Get("test-rt", metav1.GetOptions{}).Return(&v3.RoleTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "test-rt"},
				}, nil)

				c.crbController.EXPECT().Get(defaultClusterCRB.Name, metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantedCondition: &reducedCondition{
				reason: failedToCreateOrUpdateMembershipBinding,
				status: metav1.ConditionFalse,
			},
			wantErr: true,
		},
		{
			name: "cluster membership binding is created",
			crtb: defaultCRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.rtController.EXPECT().Get("test-rt", metav1.GetOptions{}).Return(&v3.RoleTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "test-rt"},
				}, nil)

				c.crbController.EXPECT().Get(defaultClusterCRB.Name, metav1.GetOptions{}).Return(nil, errNotFound)
				c.crbController.EXPECT().Create(defaultClusterCRB.DeepCopy()).Return(nil, nil)
			},
			wantedCondition: &reducedCondition{
				reason: membershipBindingExists,
				status: metav1.ConditionTrue,
			},
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controllers := controllers{
				rtController:  fake.NewMockNonNamespacedControllerInterface[*v3.RoleTemplate, *v3.RoleTemplateList](ctrl),
				crbController: fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl),
			}
			if tt.setupControllers != nil {
				tt.setupControllers(controllers)
			}
			c := &crtbHandler{
				s:             status.NewStatus(),
				rtController:  controllers.rtController,
				crbController: controllers.crbController,
			}
			localConditions := []metav1.Condition{}
			if err := c.reconcileMembershipBindings(tt.crtb, &localConditions); (err != nil) != tt.wantErr {
				t.Errorf("crtbHandler.reconcileMembershipBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Len(t, localConditions, 1)
			assert.Equal(t, tt.wantedCondition.reason, localConditions[0].Reason)
			assert.Equal(t, tt.wantedCondition.status, localConditions[0].Status)
		})
	}
}

func Test_crtbHandler_getDesiredClusterRoleBindings(t *testing.T) {
	tests := []struct {
		name               string
		crtb               *v3.ClusterRoleTemplateBinding
		setupCRBController func(*fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList])
		want               map[string]*rbacv1.ClusterRoleBinding
		wantErr            bool
	}{
		{
			name: "error getting project management plane role",
			crtb: defaultCRTB.DeepCopy(),
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt", metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error getting cluster management plane role",
			crtb: defaultCRTB.DeepCopy(),
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Get("test-rt-cluster-mgmt", metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "no cluster or project management plane role",
			crtb: defaultCRTB.DeepCopy(),
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Get("test-rt-cluster-mgmt", metav1.GetOptions{}).Return(nil, errNotFound)
			},
			want: map[string]*rbacv1.ClusterRoleBinding{},
		},
		{
			name: "found project management plane role",
			crtb: defaultCRTB.DeepCopy(),
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
				m.EXPECT().Get("test-rt-cluster-mgmt", metav1.GetOptions{}).Return(nil, errNotFound)
			},
			want: map[string]*rbacv1.ClusterRoleBinding{
				"crb-5x2rfzlbvz": {
					ObjectMeta: metav1.ObjectMeta{
						Name:   "crb-5x2rfzlbvz",
						Labels: map[string]string{"authz.cluster.cattle.io/crtb-owner": "test-crtb"},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     "test-rt-project-mgmt-aggregator",
					},
					Subjects: defaultCRB.Subjects,
				},
			},
		},
		{
			name: "found cluster management plane role",
			crtb: defaultCRTB.DeepCopy(),
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt", metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Get("test-rt-cluster-mgmt", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
			},
			want: map[string]*rbacv1.ClusterRoleBinding{
				"crb-meemnnklov": {
					ObjectMeta: metav1.ObjectMeta{
						Name:   "crb-meemnnklov",
						Labels: map[string]string{"authz.cluster.cattle.io/crtb-owner": "test-crtb"},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     "test-rt-cluster-mgmt-aggregator",
					},
					Subjects: defaultCRB.Subjects,
				},
			},
		},
		{
			name: "found both project and cluster management plane role",
			crtb: defaultCRTB.DeepCopy(),
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
				m.EXPECT().Get("test-rt-cluster-mgmt", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
			},
			want: map[string]*rbacv1.ClusterRoleBinding{
				"crb-meemnnklov": {
					ObjectMeta: metav1.ObjectMeta{
						Name:   "crb-meemnnklov",
						Labels: map[string]string{"authz.cluster.cattle.io/crtb-owner": "test-crtb"},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     "test-rt-cluster-mgmt-aggregator",
					},
					Subjects: defaultCRB.Subjects,
				},
				"crb-5x2rfzlbvz": {
					ObjectMeta: metav1.ObjectMeta{
						Name:   "crb-5x2rfzlbvz",
						Labels: map[string]string{"authz.cluster.cattle.io/crtb-owner": "test-crtb"},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     "test-rt-project-mgmt-aggregator",
					},
					Subjects: defaultCRB.Subjects,
				},
			},
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			if tt.setupCRBController != nil {
				tt.setupCRBController(crController)
			}
			c := &crtbHandler{
				s:            status.NewStatus(),
				crController: crController,
			}
			got, err := c.getDesiredClusterRoleBindings(tt.crtb)
			if (err != nil) != tt.wantErr {
				t.Errorf("crtbHandler.getDesiredClusterRoleBindings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("crtbHandler.getDesiredClusterRoleBindings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	mockTime := time.Unix(0, 0)
	oldTimeNow := timeNow
	timeNow = func() time.Time {
		return mockTime
	}
	t.Cleanup(func() {
		timeNow = oldTimeNow
	})
	ctrl := gomock.NewController(t)

	crtbSubjectExist := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LocalConditions: []v1.Condition{
				{
					Type:   subjectExists,
					Status: v1.ConditionTrue,
					Reason: subjectExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
			LastUpdateTime: mockTime.Format(time.RFC3339),
		},
	}
	crtbSubjectAndBindingExist := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LocalConditions: []v1.Condition{
				{
					Type:   subjectExists,
					Status: v1.ConditionTrue,
					Reason: subjectExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
				{
					Type:   bindingsExists,
					Status: v1.ConditionTrue,
					Reason: bindingsExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
			LastUpdateTime: mockTime.Format(time.RFC3339),
		},
	}
	crtbSubjectError := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LocalConditions: []v1.Condition{
				{
					Type:   subjectExists,
					Status: v1.ConditionFalse,
					Reason: failedToCreateUser,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
			LastUpdateTime: mockTime.Format(time.RFC3339),
		},
	}
	crtbEmptyStatus := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LastUpdateTime: mockTime.Format(time.RFC3339),
		},
	}
	crtbEmptyStatusRemoteComplete := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LastUpdateTime: mockTime.Format(time.RFC3339),
			SummaryRemote:  status.SummaryCompleted,
		},
	}
	tests := map[string]struct {
		crtb            *v3.ClusterRoleTemplateBinding
		crtbClient      func(*v3.ClusterRoleTemplateBinding) mgmtv3.ClusterRoleTemplateBindingController
		localConditions []v1.Condition
		wantErr         error
	}{
		"status updated": {
			crtb: crtbEmptyStatus.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) mgmtv3.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().UpdateStatus(&v3.ClusterRoleTemplateBinding{
					Status: v3.ClusterRoleTemplateBindingStatus{
						LocalConditions: []v1.Condition{
							{
								Type:   subjectExists,
								Status: v1.ConditionTrue,
								Reason: subjectExists,
								LastTransitionTime: v1.Time{
									Time: mockTime,
								},
							},
						},
						LastUpdateTime: mockTime.Format(time.RFC3339),
						SummaryLocal:   status.SummaryCompleted,
					},
				})

				return mock
			},
			localConditions: crtbSubjectExist.Status.LocalConditions,
		},
		"status not updated when local conditions are the same": {
			crtb: crtbSubjectExist.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) mgmtv3.ClusterRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
			},
			localConditions: crtbSubjectExist.Status.LocalConditions,
		},
		"set summary to complete when remote is complete": {
			crtb: crtbEmptyStatusRemoteComplete.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) mgmtv3.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().UpdateStatus(&v3.ClusterRoleTemplateBinding{
					Status: v3.ClusterRoleTemplateBindingStatus{
						LocalConditions: []v1.Condition{
							{
								Type:   subjectExists,
								Status: v1.ConditionTrue,
								Reason: subjectExists,
								LastTransitionTime: v1.Time{
									Time: mockTime,
								},
							},
						},
						LastUpdateTime: mockTime.Format(time.RFC3339),
						SummaryLocal:   status.SummaryCompleted,
						SummaryRemote:  status.SummaryCompleted,
						Summary:        status.SummaryCompleted,
					},
				})

				return mock
			},
			localConditions: crtbSubjectExist.Status.LocalConditions,
		},
		"set summary to error when there is an error condition": {
			crtb: crtbSubjectExist.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) mgmtv3.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().UpdateStatus(&v3.ClusterRoleTemplateBinding{
					Status: v3.ClusterRoleTemplateBindingStatus{
						LocalConditions: []v1.Condition{
							{
								Type:   subjectExists,
								Status: v1.ConditionFalse,
								Reason: failedToCreateUser,
								LastTransitionTime: v1.Time{
									Time: mockTime,
								},
							},
						},
						LastUpdateTime: mockTime.Format(time.RFC3339),
						SummaryLocal:   status.SummaryError,
						Summary:        status.SummaryError,
					},
				})

				return mock
			},
			localConditions: crtbSubjectError.Status.LocalConditions,
		},
		"status updated when a condition is removed": {
			crtb: crtbSubjectAndBindingExist.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) mgmtv3.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().UpdateStatus(&v3.ClusterRoleTemplateBinding{
					Status: v3.ClusterRoleTemplateBindingStatus{
						LocalConditions: []metav1.Condition{
							{
								Type:   subjectExists,
								Status: metav1.ConditionTrue,
								Reason: subjectExists,
								LastTransitionTime: metav1.Time{
									Time: mockTime,
								},
							},
						},
						LastUpdateTime: mockTime.Format(time.RFC3339),
						SummaryLocal:   status.SummaryCompleted,
					},
				})

				return mock
			},
			localConditions: crtbSubjectExist.Status.LocalConditions,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			crtbCache := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
			crtbCache.EXPECT().Get(test.crtb.Namespace, test.crtb.Name).Return(test.crtb, nil)
			c := crtbHandler{
				crtbClient: test.crtbClient(test.crtb),
				crtbCache:  crtbCache,
			}
			err := c.updateStatus(test.crtb, test.localConditions)
			assert.Equal(t, test.wantErr, err)
		})
	}
}

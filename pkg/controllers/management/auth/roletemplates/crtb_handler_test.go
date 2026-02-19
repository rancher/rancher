package roletemplates

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/status"
	"github.com/rancher/rancher/pkg/features"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	userMocks "github.com/rancher/rancher/pkg/user/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type reducedCondition struct {
	reason string
	status metav1.ConditionStatus
}

func Test_crtbHandler_reconcileSubject(t *testing.T) {
	type controllers struct {
		userMGR        *userMocks.MockManager
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
				userMGR:        userMocks.NewMockManager(ctrl),
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
		crController  *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]
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
			name: "error getting cluster role",
			crtb: &v3.ClusterRoleTemplateBinding{
				RoleTemplateName: "test-rt",
			},
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantedCondition: &reducedCondition{
				reason: failedToGetClusterRole,
				status: metav1.ConditionFalse,
			},
			wantErr: true,
		},
		{
			name: "error creating cluster membership binding",
			crtb: defaultCRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-rt-aggregator"},
					Rules:      []rbacv1.PolicyRule{},
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
				c.crController.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-rt-aggregator"},
					Rules:      []rbacv1.PolicyRule{},
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
				crController:  fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl),
				crbController: fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl),
			}
			if tt.setupControllers != nil {
				tt.setupControllers(controllers)
			}
			c := &crtbHandler{
				s:             status.NewStatus(),
				crController:  controllers.crController,
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

const (
	projectMGMT = "test-rt-project-mgmt-aggregator"
	clusterMGMT = "test-rt-cluster-mgmt-aggregator"
)

func Test_crtbHandler_getDesiredRoleBindings(t *testing.T) {
	defaultProject1 := v3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "project1",
		},
		Status: v3.ProjectStatus{
			BackingNamespace: "c-test-p-project1",
		},
	}
	defaultProject2 := v3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "project2",
		},
		Status: v3.ProjectStatus{
			BackingNamespace: "c-test-p-project2",
		},
	}

	type controllers struct {
		crController *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]
		projectCache *fake.MockCacheInterface[*v3.Project]
	}

	tests := []struct {
		name             string
		crtb             *v3.ClusterRoleTemplateBinding
		setupControllers func(controllers)
		want             map[string]*rbacv1.RoleBinding
		wantErr          bool
	}{
		{
			name: "error getting project management plane role",
			crtb: defaultCRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get(projectMGMT, metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error listing projects when project management plane role exists",
			crtb: defaultCRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get(projectMGMT, metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
				c.projectCache.EXPECT().List("test-cluster", gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error getting cluster management plane role",
			crtb: defaultCRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get(projectMGMT, metav1.GetOptions{}).Return(nil, errNotFound)
				c.crController.EXPECT().Get(clusterMGMT, metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "no cluster or project management plane role",
			crtb: defaultCRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get(projectMGMT, metav1.GetOptions{}).Return(nil, errNotFound)
				c.crController.EXPECT().Get(clusterMGMT, metav1.GetOptions{}).Return(nil, errNotFound)
			},
			want: map[string]*rbacv1.RoleBinding{},
		},
		{
			name: "found project management plane role with no projects",
			crtb: defaultCRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get(projectMGMT, metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
				c.projectCache.EXPECT().List("test-cluster", gomock.Any()).Return([]*v3.Project{}, nil)
				c.crController.EXPECT().Get(clusterMGMT, metav1.GetOptions{}).Return(nil, errNotFound)
			},
			want: map[string]*rbacv1.RoleBinding{},
		},
		{
			name: "found project management plane role with single project",
			crtb: defaultCRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get(projectMGMT, metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
				c.projectCache.EXPECT().List("test-cluster", gomock.Any()).Return([]*v3.Project{&defaultProject1}, nil)
				c.crController.EXPECT().Get(clusterMGMT, metav1.GetOptions{}).Return(nil, errNotFound)
			},
			want: map[string]*rbacv1.RoleBinding{
				"rb-jhe3mikle5": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rb-jhe3mikle5",
						Namespace: "c-test-p-project1",
						Labels: map[string]string{
							"authz.cluster.cattle.io/crtb-owner-test-crtb":  "true",
							"management.cattle.io/roletemplate-aggregation": "true",
						},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     projectMGMT,
					},
					Subjects: []rbacv1.Subject{defaultSubject},
				},
			},
		},
		{
			name: "found project management plane role with multiple projects",
			crtb: defaultCRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get(projectMGMT, metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
				c.projectCache.EXPECT().List("test-cluster", gomock.Any()).Return([]*v3.Project{&defaultProject1, &defaultProject2}, nil)
				c.crController.EXPECT().Get(clusterMGMT, metav1.GetOptions{}).Return(nil, errNotFound)
			},
			want: map[string]*rbacv1.RoleBinding{
				"rb-jhe3mikle5": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rb-jhe3mikle5",
						Namespace: "c-test-p-project1",
						Labels: map[string]string{
							"authz.cluster.cattle.io/crtb-owner-test-crtb":  "true",
							"management.cattle.io/roletemplate-aggregation": "true",
						},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     projectMGMT,
					},
					Subjects: []rbacv1.Subject{defaultSubject},
				},
				"rb-37o6abbhaq": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rb-37o6abbhaq",
						Namespace: "c-test-p-project2",
						Labels: map[string]string{
							"authz.cluster.cattle.io/crtb-owner-test-crtb":  "true",
							"management.cattle.io/roletemplate-aggregation": "true",
						},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     projectMGMT,
					},
					Subjects: []rbacv1.Subject{defaultSubject},
				},
			},
		},
		{
			name: "found cluster management plane role",
			crtb: defaultCRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get(projectMGMT, metav1.GetOptions{}).Return(nil, errNotFound)
				c.crController.EXPECT().Get(clusterMGMT, metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
			},
			want: map[string]*rbacv1.RoleBinding{
				"rb-lhchhtbxqn": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rb-lhchhtbxqn",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"authz.cluster.cattle.io/crtb-owner-test-crtb":  "true",
							"management.cattle.io/roletemplate-aggregation": "true",
						},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     clusterMGMT,
					},
					Subjects: []rbacv1.Subject{defaultSubject},
				},
			},
		},
		{
			name: "found both project and cluster management plane role",
			crtb: defaultCRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get(projectMGMT, metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
				c.projectCache.EXPECT().List("test-cluster", gomock.Any()).Return([]*v3.Project{&defaultProject1}, nil)
				c.crController.EXPECT().Get(clusterMGMT, metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, nil)
			},
			want: map[string]*rbacv1.RoleBinding{
				"rb-lhchhtbxqn": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rb-lhchhtbxqn",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"authz.cluster.cattle.io/crtb-owner-test-crtb":  "true",
							"management.cattle.io/roletemplate-aggregation": "true",
						},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     clusterMGMT,
					},
					Subjects: []rbacv1.Subject{defaultSubject},
				},
				"rb-jhe3mikle5": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rb-jhe3mikle5",
						Namespace: "c-test-p-project1",
						Labels: map[string]string{
							"authz.cluster.cattle.io/crtb-owner-test-crtb":  "true",
							"management.cattle.io/roletemplate-aggregation": "true",
						},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     projectMGMT,
					},
					Subjects: []rbacv1.Subject{defaultSubject},
				},
			},
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controllers := controllers{
				crController: fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl),
				projectCache: fake.NewMockCacheInterface[*v3.Project](ctrl),
			}
			if tt.setupControllers != nil {
				tt.setupControllers(controllers)
			}
			c := &crtbHandler{
				s:            status.NewStatus(),
				crController: controllers.crController,
				projectCache: controllers.projectCache,
			}
			got, err := c.getDesiredRoleBindings(tt.crtb)
			if (err != nil) != tt.wantErr {
				t.Errorf("crtbHandler.getDesiredRoleBindings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("crtbHandler.getDesiredRoleBindings() = %v, want %v", got, tt.want)
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
		},
	}
	crtbSubjectAndBindingExist := &v3.ClusterRoleTemplateBinding{
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
				{
					Type:   bindingsExists,
					Status: metav1.ConditionTrue,
					Reason: bindingsExists,
					LastTransitionTime: metav1.Time{
						Time: mockTime,
					},
				},
			},
			LastUpdateTime: mockTime.Format(time.RFC3339),
		},
	}
	crtbSubjectError := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LocalConditions: []metav1.Condition{
				{
					Type:   subjectExists,
					Status: metav1.ConditionFalse,
					Reason: failedToCreateUser,
					LastTransitionTime: metav1.Time{
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
		localConditions []metav1.Condition
		wantErr         error
	}{
		"status updated": {
			crtb: crtbEmptyStatus.DeepCopy(),
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
						LocalConditions: []metav1.Condition{
							{
								Type:   subjectExists,
								Status: metav1.ConditionFalse,
								Reason: failedToCreateUser,
								LastTransitionTime: metav1.Time{
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

func Test_crtbHandler_removeRoleBindings(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	errDefault := fmt.Errorf("error")

	listOptions := metav1.ListOptions{
		LabelSelector: "authz.cluster.cattle.io/crtb-owner-test-crtb=true,management.cattle.io/roletemplate-aggregation=true",
	}

	// Define test role bindings for reuse
	testRoleBinding1 := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rb-1",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"authz.cluster.cattle.io/crtb-owner-test-crtb":  "true",
				"management.cattle.io/roletemplate-aggregation": "true",
			},
		},
	}
	testRoleBinding2 := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rb-2",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"authz.cluster.cattle.io/crtb-owner-test-crtb":  "true",
				"management.cattle.io/roletemplate-aggregation": "true",
			},
		},
	}

	type controllers struct {
		rbController *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]
	}

	tests := []struct {
		name             string
		setupControllers func(controllers)
		wantErr          bool
		wantedConditions []reducedCondition
	}{
		{
			name: "error listing role bindings",
			setupControllers: func(c controllers) {
				c.rbController.EXPECT().List(metav1.NamespaceAll, listOptions).Return(nil, errDefault)
			},
			wantErr: true,
			wantedConditions: []reducedCondition{
				{
					reason: failedToListExistingRoleBindings,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			name: "no role bindings to remove",
			setupControllers: func(c controllers) {
				c.rbController.EXPECT().List(metav1.NamespaceAll, listOptions).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{},
				}, nil)
			},
			wantErr: false,
			wantedConditions: []reducedCondition{
				{
					reason: roleBindingDeleted,
					status: metav1.ConditionTrue,
				},
			},
		},
		{
			name: "successfully remove single role binding",
			setupControllers: func(c controllers) {
				c.rbController.EXPECT().List(metav1.NamespaceAll, listOptions).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{testRoleBinding1},
				}, nil)
				c.rbController.EXPECT().Delete("test-namespace", "rb-1", &metav1.DeleteOptions{}).Return(nil)
			},
			wantErr: false,
			wantedConditions: []reducedCondition{
				{
					reason: roleBindingDeleted,
					status: metav1.ConditionTrue,
				},
			},
		},
		{
			name: "successfully remove multiple role bindings",
			setupControllers: func(c controllers) {
				c.rbController.EXPECT().List(metav1.NamespaceAll, listOptions).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{testRoleBinding1, testRoleBinding2},
				}, nil)
				c.rbController.EXPECT().Delete("test-namespace", "rb-1", &metav1.DeleteOptions{}).Return(nil)
				c.rbController.EXPECT().Delete("test-namespace", "rb-2", &metav1.DeleteOptions{}).Return(nil)
			},
			wantErr: false,
			wantedConditions: []reducedCondition{
				{
					reason: roleBindingDeleted,
					status: metav1.ConditionTrue,
				},
			},
		},
		{
			name: "error deleting one role binding",
			setupControllers: func(c controllers) {
				c.rbController.EXPECT().List(metav1.NamespaceAll, listOptions).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{testRoleBinding1},
				}, nil)
				c.rbController.EXPECT().Delete("test-namespace", "rb-1", &metav1.DeleteOptions{}).Return(errDefault)
			},
			wantErr: true,
			wantedConditions: []reducedCondition{
				{
					reason: roleBindingDeleted,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			name: "error deleting multiple role bindings",
			setupControllers: func(c controllers) {
				c.rbController.EXPECT().List(metav1.NamespaceAll, listOptions).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{testRoleBinding1, testRoleBinding2},
				}, nil)
				c.rbController.EXPECT().Delete("test-namespace", "rb-1", &metav1.DeleteOptions{}).Return(errDefault)
				c.rbController.EXPECT().Delete("test-namespace", "rb-2", &metav1.DeleteOptions{}).Return(errDefault)
			},
			wantErr: true,
			wantedConditions: []reducedCondition{
				{
					reason: roleBindingDeleted,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			name: "partial success - one deletion succeeds, one fails",
			setupControllers: func(c controllers) {
				c.rbController.EXPECT().List(metav1.NamespaceAll, listOptions).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{testRoleBinding1, testRoleBinding2},
				}, nil)
				c.rbController.EXPECT().Delete("test-namespace", "rb-1", &metav1.DeleteOptions{}).Return(nil)
				c.rbController.EXPECT().Delete("test-namespace", "rb-2", &metav1.DeleteOptions{}).Return(errDefault)
			},
			wantErr: true,
			wantedConditions: []reducedCondition{
				{
					reason: roleBindingDeleted,
					status: metav1.ConditionFalse,
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
			if tt.setupControllers != nil {
				tt.setupControllers(controllers{
					rbController: rbController,
				})
			}

			c := crtbHandler{
				s:            status.NewStatus(),
				rbController: rbController,
			}

			crtb := v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-crtb",
				},
			}
			err := c.removeRoleBindings(&crtb)
			if (err != nil) != tt.wantErr {
				t.Errorf("crtbHandler.removeRoleBindings() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check the conditions were set correctly
			assert.Len(t, crtb.Status.LocalConditions, len(tt.wantedConditions))
			for i, wantedCondition := range tt.wantedConditions {
				assert.Equal(t, wantedCondition.reason, crtb.Status.LocalConditions[i].Reason)
				assert.Equal(t, wantedCondition.status, crtb.Status.LocalConditions[i].Status)
			}
		})
	}
}

var (
	defaultListOption = metav1.ListOptions{LabelSelector: "authz.cluster.cattle.io/crtb-owner-test-crtb=true,management.cattle.io/roletemplate-aggregation=true"}
)

func Test_deleteDownstreamClusterRoleBindings(t *testing.T) {
	tests := []struct {
		name               string
		setupCRBController func(*fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList])
		crtb               *v3.ClusterRoleTemplateBinding
		wantErr            bool
		wantedCondition    *reducedCondition
	}{
		{
			name: "error on list CRB",
			setupCRBController: func(c *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				c.EXPECT().List(defaultListOption).Return(nil, errDefault)
			},
			crtb:    defaultCRTB.DeepCopy(),
			wantErr: true,
			wantedCondition: &reducedCondition{
				reason: "FailureToListClusterRoleBindings",
				status: metav1.ConditionFalse,
			},
		},
		{
			name: "error deleting CRB",
			setupCRBController: func(c *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				c.EXPECT().List(defaultListOption).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{{ObjectMeta: metav1.ObjectMeta{Name: "crb1"}}},
				}, nil)
				c.EXPECT().Delete("crb1", &metav1.DeleteOptions{}).Return(errDefault)
			},
			crtb:    defaultCRTB.DeepCopy(),
			wantErr: true,
			wantedCondition: &reducedCondition{
				reason: "ClusterRoleBindingsDeleted",
				status: metav1.ConditionFalse,
			},
		},
		{
			name: "CRB not found on deleting",
			setupCRBController: func(c *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				c.EXPECT().List(defaultListOption).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{{ObjectMeta: metav1.ObjectMeta{Name: "crb1"}}},
				}, nil)
				c.EXPECT().Delete("crb1", &metav1.DeleteOptions{}).Return(errNotFound)
			},
			crtb: defaultCRTB.DeepCopy(),
			wantedCondition: &reducedCondition{
				reason: "ClusterRoleBindingsDeleted",
				status: metav1.ConditionTrue,
			},
		},
		{
			name: "successfully delete multiple CRBs",
			setupCRBController: func(c *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				c.EXPECT().List(defaultListOption).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						{ObjectMeta: metav1.ObjectMeta{Name: "crb1"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "crb2"}},
					},
				}, nil)
				c.EXPECT().Delete("crb1", &metav1.DeleteOptions{}).Return(nil)
				c.EXPECT().Delete("crb2", &metav1.DeleteOptions{}).Return(nil)
			},
			crtb: defaultCRTB.DeepCopy(),
			wantedCondition: &reducedCondition{
				reason: "ClusterRoleBindingsDeleted",
				status: metav1.ConditionTrue,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			crbController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl)
			if tt.setupCRBController != nil {
				tt.setupCRBController(crbController)
			}

			c := &crtbHandler{
				s: status.NewStatus(),
			}

			if err := c.deleteDownstreamClusterRoleBindings(tt.crtb, crbController); (err != nil) != tt.wantErr {
				t.Errorf("crtbHandler.deleteBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_crtbHandler_handleMigration(t *testing.T) {
	ctrl := gomock.NewController(t)

	type controllers struct {
		crtbController *fake.MockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList]
		rbController   *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]
		rbCache        *fake.MockCacheInterface[*rbacv1.RoleBinding]
	}

	tests := []struct {
		name               string
		crtb               *v3.ClusterRoleTemplateBinding
		featureFlagEnabled bool
		setupControllers   func(controllers)
		wantLabel          bool
		wantErr            bool
	}{
		{
			name: "feature flag disabled, label present - should remove label and call removeRoleBindings",
			crtb: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-crtb",
					Namespace: "test-ns",
					Labels: map[string]string{
						rbac.AggregationFeatureLabel: "true",
					},
				},
			},
			featureFlagEnabled: false,
			setupControllers: func(c controllers) {
				// Expect Update to be called with label removed
				c.crtbController.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					if _, exists := obj.Labels[rbac.AggregationFeatureLabel]; exists {
						t.Error("expected label to be removed from updated CRTB")
					}
					return obj, nil
				})
				// removeRoleBindings will be called, so mock rbController.List to return empty list
				c.rbController.EXPECT().List(gomock.Any(), gomock.Any()).Return(&rbacv1.RoleBindingList{}, nil)
			},
			wantLabel: false,
		},
		{
			name: "feature flag disabled, label absent - no-op",
			crtb: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-crtb",
					Namespace: "test-ns",
					Labels:    map[string]string{},
				},
			},
			featureFlagEnabled: false,
			wantLabel:          false,
		},
		{
			name: "feature flag enabled, label absent - should add label and call deleteLegacyRoleBindings",
			crtb: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-crtb",
					Namespace: "test-ns",
					Labels:    map[string]string{},
				},
				ClusterName: "test-cluster",
			},
			featureFlagEnabled: true,
			setupControllers: func(c controllers) {
				// deleteLegacyRoleBindings will be called
				// Mock projectCache.List to return empty list
				c.rbCache.EXPECT().GetByIndex(rbByCRTBOwnerReferenceIndex, "test-crtb").Return([]*rbacv1.RoleBinding{}, nil)
				// Mock rbController.List for cluster namespace
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
				// Expect Update to be called with label added
				c.crtbController.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					if obj.Labels[rbac.AggregationFeatureLabel] != "true" {
						t.Error("expected label to be added to updated CRTB")
					}
					return obj, nil
				})
			},
			wantLabel: true,
		},
		{
			name: "feature flag enabled, label present - no-op",
			crtb: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-crtb",
					Namespace: "test-ns",
					Labels: map[string]string{
						rbac.AggregationFeatureLabel: "true",
					},
				},
			},
			featureFlagEnabled: true,
			setupControllers:   func(c controllers) {},
			wantLabel:          true,
		},
		{
			name: "feature flag enabled, nil labels - should add label",
			crtb: &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-crtb",
					Namespace: "test-ns",
					Labels:    nil,
				},
				ClusterName: "test-cluster",
			},
			featureFlagEnabled: true,
			setupControllers: func(c controllers) {
				// deleteLegacyRoleBindings will be called
				c.rbCache.EXPECT().GetByIndex(rbByCRTBOwnerReferenceIndex, "test-crtb").Return([]*rbacv1.RoleBinding{}, nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
				// Expect Update to be called with label added
				c.crtbController.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					if obj.Labels == nil {
						t.Error("expected labels map to be initialized")
					}
					if obj.Labels[rbac.AggregationFeatureLabel] != "true" {
						t.Error("expected label to be added to updated CRTB")
					}
					return obj, nil
				})
			},
			wantLabel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := features.AggregatedRoleTemplates.Enabled()
			t.Cleanup(func() {
				features.AggregatedRoleTemplates.Set(prev)
			})
			features.AggregatedRoleTemplates.Set(tt.featureFlagEnabled)

			crtbController := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
			rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
			rbCache := fake.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl)

			if tt.setupControllers != nil {
				tt.setupControllers(controllers{
					crtbController: crtbController,
					rbController:   rbController,
					rbCache:        rbCache,
				})
			}

			h := &crtbHandler{
				crtbClient:   crtbController,
				rbController: rbController,
				rbCache:      rbCache,
				s:            status.NewStatus(),
			}

			result, err := h.handleMigration(tt.crtb)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleMigration() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantLabel {
				if result.Labels[rbac.AggregationFeatureLabel] != "true" {
					t.Error("expected label to be present and set to 'true'")
				}
			} else {
				if result != nil && result.Labels[rbac.AggregationFeatureLabel] == "true" {
					t.Error("expected label to be absent or not set to 'true'")
				}
			}
		})
	}
}

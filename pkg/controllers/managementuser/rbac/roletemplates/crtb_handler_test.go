package roletemplates

import (
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/status"
	controllersv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
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

var (
	defaultListOption = metav1.ListOptions{LabelSelector: "authz.cluster.cattle.io/crtb-owner=test-crtb"}
	defaultCRTB       = v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-crtb",
		},
		UserName:         "test-user",
		RoleTemplateName: "test-rt",
	}
	defaultCRB = rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "crb-",
			Labels:       map[string]string{"authz.cluster.cattle.io/crtb-owner": "test-crtb"},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "test-rt-aggregator",
		},
		Subjects: []rbacv1.Subject{
			{
				Namespace: "",
				Kind:      "User",
				Name:      "test-user",
				APIGroup:  "rbac.authorization.k8s.io",
			},
		},
	}
)

func Test_reconcileBindings(t *testing.T) {
	tests := []struct {
		name               string
		setupCRBController func(*fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList])
		crtb               *v3.ClusterRoleTemplateBinding
		wantedCondition    *reducedCondition
		wantErr            bool
	}{
		{
			name:    "error building cluster role binding",
			crtb:    &v3.ClusterRoleTemplateBinding{},
			wantErr: true,
			wantedCondition: &reducedCondition{
				reason: "FailureToBuildClusterRoleBinding",
				status: metav1.ConditionFalse,
			},
		},
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
			name: "error creating CRB",
			setupCRBController: func(c *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				c.EXPECT().List(defaultListOption).Return(&rbacv1.ClusterRoleBindingList{}, nil)
				c.EXPECT().Create(defaultCRB.DeepCopy()).Return(nil, errDefault)
			},
			crtb:    defaultCRTB.DeepCopy(),
			wantErr: true,
			wantedCondition: &reducedCondition{
				reason: "FailureToCreateClusterRoleBinding",
				status: metav1.ConditionFalse,
			},
		},
		{
			name: "success creating CRB",
			setupCRBController: func(c *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				c.EXPECT().List(defaultListOption).Return(&rbacv1.ClusterRoleBindingList{}, nil)
				c.EXPECT().Create(defaultCRB.DeepCopy()).Return(nil, nil)
			},
			crtb: defaultCRTB.DeepCopy(),
			wantedCondition: &reducedCondition{
				reason: "ClusterRoleBindingExists",
				status: metav1.ConditionTrue,
			},
		},
		{
			name: "CRB already exists no create needed",
			setupCRBController: func(c *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				c.EXPECT().List(defaultListOption).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{*defaultCRB.DeepCopy()},
				}, nil)
			},
			crtb: defaultCRTB.DeepCopy(),
			wantedCondition: &reducedCondition{
				reason: "ClusterRoleBindingExists",
				status: metav1.ConditionTrue,
			},
		},
		{
			name: "error deleting CRB",
			setupCRBController: func(c *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				c.EXPECT().List(defaultListOption).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "bad-crb1"},
						},
					},
				}, nil)
				c.EXPECT().Delete("bad-crb1", gomock.Any()).Return(errDefault)
			},
			crtb:    defaultCRTB.DeepCopy(),
			wantErr: true,
			wantedCondition: &reducedCondition{
				reason: "FailureToDeleteClusterRoleBinding",
				status: metav1.ConditionFalse,
			},
		},
		{
			name: "wrong CRBs exist and get deleted",
			setupCRBController: func(c *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				c.EXPECT().List(defaultListOption).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "bad-crb1"},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "bad-crb2"},
						},
					},
				}, nil)
				c.EXPECT().Delete("bad-crb1", gomock.Any()).Return(nil)
				c.EXPECT().Delete("bad-crb2", gomock.Any()).Return(nil)
				c.EXPECT().Create(defaultCRB.DeepCopy()).Return(nil, nil)
			},
			crtb: defaultCRTB.DeepCopy(),
			wantedCondition: &reducedCondition{
				reason: "ClusterRoleBindingExists",
				status: metav1.ConditionTrue,
			},
		},
		{
			name: "wrong CRBs exist and are deleted but correct CRB exists and is not created again",
			setupCRBController: func(c *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				c.EXPECT().List(defaultListOption).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "bad-crb1"},
						},
						*defaultCRB.DeepCopy(),
						{
							ObjectMeta: metav1.ObjectMeta{Name: "bad-crb2"},
						},
					},
				}, nil)
				c.EXPECT().Delete("bad-crb1", gomock.Any()).Return(nil)
				c.EXPECT().Delete("bad-crb2", gomock.Any()).Return(nil)
			},
			crtb: defaultCRTB.DeepCopy(),
			wantedCondition: &reducedCondition{
				reason: "ClusterRoleBindingExists",
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
				crbClient: crbController,
				s:         status.NewStatus(),
			}
			remoteConditions := []metav1.Condition{}

			if err := c.reconcileBindings(tt.crtb, &remoteConditions); (err != nil) != tt.wantErr {
				t.Errorf("crtbHandler.reconcileBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Len(t, remoteConditions, 1)
			assert.Equal(t, tt.wantedCondition.reason, remoteConditions[0].Reason)
			assert.Equal(t, tt.wantedCondition.status, remoteConditions[0].Status)
		})
	}
}

func Test_deleteBindings(t *testing.T) {
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
				c.EXPECT().Delete("crb1", gomock.Any()).Return(errDefault)
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
				c.EXPECT().Delete("crb1", gomock.Any()).Return(errNotFound)
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
				c.EXPECT().Delete("crb1", gomock.Any()).Return(nil)
				c.EXPECT().Delete("crb2", gomock.Any()).Return(nil)
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
				crbClient: crbController,
				s:         status.NewStatus(),
			}
			remoteConditions := []metav1.Condition{}

			if err := c.deleteBindings(tt.crtb, &remoteConditions); (err != nil) != tt.wantErr {
				t.Errorf("crtbHandler.deleteBindings() error = %v, wantErr %v", err, tt.wantErr)
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

	crtbClusterRoleBindingExists := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			RemoteConditions: []metav1.Condition{
				{
					Type:   reconcileClusterRoleBindings,
					Status: metav1.ConditionTrue,
					Reason: clusterRoleBindingExists,
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
			RemoteConditions: []metav1.Condition{
				{
					Type:   reconcileClusterRoleBindings,
					Status: metav1.ConditionFalse,
					Reason: failureToCreateClusterRoleBinding,
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
	crtbEmptyStatusLocalComplete := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LastUpdateTime: mockTime.Format(time.RFC3339),
			SummaryLocal:   status.SummaryCompleted,
		},
	}

	tests := map[string]struct {
		crtb             *v3.ClusterRoleTemplateBinding
		crtbClient       func(*v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController
		remoteConditions []metav1.Condition
		wantErr          error
	}{
		"status updated": {
			crtb: crtbEmptyStatus.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().UpdateStatus(&v3.ClusterRoleTemplateBinding{
					Status: v3.ClusterRoleTemplateBindingStatus{
						RemoteConditions: []metav1.Condition{
							{
								Type:   reconcileClusterRoleBindings,
								Status: metav1.ConditionTrue,
								Reason: clusterRoleBindingExists,
								LastTransitionTime: metav1.Time{
									Time: mockTime,
								},
							},
						},
						SummaryRemote:  status.SummaryCompleted,
						LastUpdateTime: mockTime.Format(time.RFC3339),
					},
				})

				return mock
			},
			remoteConditions: crtbClusterRoleBindingExists.Status.RemoteConditions,
		},
		"status not updated when remote conditions are the same": {
			crtb: crtbClusterRoleBindingExists.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
			},
			remoteConditions: crtbClusterRoleBindingExists.Status.RemoteConditions,
		},
		"set summary to complete when local is complete": {
			crtb: crtbEmptyStatusLocalComplete.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().UpdateStatus(&v3.ClusterRoleTemplateBinding{
					Status: v3.ClusterRoleTemplateBindingStatus{
						RemoteConditions: []metav1.Condition{
							{
								Type:   reconcileClusterRoleBindings,
								Status: metav1.ConditionTrue,
								Reason: clusterRoleBindingExists,
								LastTransitionTime: metav1.Time{
									Time: mockTime,
								},
							},
						},
						SummaryRemote:  status.SummaryCompleted,
						SummaryLocal:   status.SummaryCompleted,
						Summary:        status.SummaryCompleted,
						LastUpdateTime: mockTime.Format(time.RFC3339),
					},
				})

				return mock
			},
			remoteConditions: crtbClusterRoleBindingExists.Status.RemoteConditions,
		},
		"set summary to error when there is an error condition": {
			crtb: crtbClusterRoleBindingExists.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().UpdateStatus(&v3.ClusterRoleTemplateBinding{
					Status: v3.ClusterRoleTemplateBindingStatus{
						RemoteConditions: []metav1.Condition{
							{
								Type:   reconcileClusterRoleBindings,
								Status: metav1.ConditionFalse,
								Reason: failureToCreateClusterRoleBinding,
								LastTransitionTime: metav1.Time{
									Time: mockTime,
								},
							},
						},
						SummaryRemote:  status.SummaryError,
						Summary:        status.SummaryError,
						LastUpdateTime: mockTime.Format(time.RFC3339),
					},
				})

				return mock
			},
			remoteConditions: crtbSubjectError.Status.RemoteConditions,
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

			err := c.updateStatus(test.crtb, test.remoteConditions)

			assert.Equal(t, test.wantErr, err)
		})
	}
}

package rbac

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wfakes "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var manageNSRule = rbacv1.PolicyRule{
	APIGroups:     []string{"management.cattle.io"},
	Verbs:         []string{"manage-namespaces"},
	Resources:     []string{"projects"},
	ResourceNames: []string{"p-123xyz"},
}

func newManageNSRole(rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "p-123xyz-namespaces-manage",
		},
		Rules: rules,
	}
}

func newProjectCRLister(ctrl *gomock.Controller, getErr error, existing []*rbacv1.ClusterRole) *wfakes.MockNonNamespacedCacheInterface[*rbacv1.ClusterRole] {
	crLister := wfakes.NewMockNonNamespacedCacheInterface[*rbacv1.ClusterRole](ctrl)
	crLister.EXPECT().Get(gomock.Any()).DoAndReturn(
		func(name string) (*rbacv1.ClusterRole, error) {
			if getErr != nil {
				return nil, getErr
			}
			for _, cr := range existing {
				if cr.Name == name {
					return cr, nil
				}
			}
			return nil, apierror.NewNotFound(schema.GroupResource{
				Group:    "rbac.authorization.k8s.io",
				Resource: "ClusterRoles",
			}, name)
		},
	).AnyTimes()
	return crLister
}

func TestCreate(t *testing.T) {
	const projectName = "p-123xyz"
	ctrl := gomock.NewController(t)
	tests := []struct {
		name                 string
		existingClusterRoles []*rbacv1.ClusterRole
		getErr               error
		createErr            error
		updateErr            error

		wantErr          bool
		wantRoles        []*rbacv1.ClusterRole
		wantUpdatedRoles []*rbacv1.ClusterRole
	}{
		{
			name: "basic create",
			wantRoles: []*rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "p-123xyz-namespaces-readonly",
						Annotations: map[string]string{
							projectNSAnn: "p-123xyz-namespaces-readonly",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "p-123xyz-namespaces-edit",
						Annotations: map[string]string{
							projectNSAnn: "p-123xyz-namespaces-edit",
						},
					},
				},
				newManageNSRole(manageNSRule),
			},
		},
		{
			name:    "get error",
			getErr:  fmt.Errorf("unexpected error"),
			wantErr: true,
		},
		{
			name:      "create error",
			createErr: fmt.Errorf("unexpected error"),
			wantErr:   true,
		},
		{
			name: "roles already exist",
			existingClusterRoles: []*rbacv1.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "p-123xyz-namespaces-readonly"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "p-123xyz-namespaces-edit"}},
				newManageNSRole(manageNSRule),
			},
		},
		{
			name: "manage role is created if the other roles already exist",
			existingClusterRoles: []*rbacv1.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "p-123xyz-namespaces-readonly"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "p-123xyz-namespaces-edit"}},
			},
			wantRoles: []*rbacv1.ClusterRole{newManageNSRole(manageNSRule)},
		},
		{
			name: "manage role with extra rules is updated",
			existingClusterRoles: []*rbacv1.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "p-123xyz-namespaces-readonly"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "p-123xyz-namespaces-edit"}},
				newManageNSRole(manageNSRule, rbacv1.PolicyRule{
					APIGroups: []string{""},
					Verbs:     []string{"delete"},
					Resources: []string{"namespaces"},
				}),
			},
			wantUpdatedRoles: []*rbacv1.ClusterRole{newManageNSRole(manageNSRule)},
		},
		{
			name: "manage role update error",
			existingClusterRoles: []*rbacv1.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "p-123xyz-namespaces-readonly"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "p-123xyz-namespaces-edit"}},
				newManageNSRole(),
			},
			updateErr: fmt.Errorf("unexpected error"),
			wantErr:   true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			project := &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
			}
			var newCRs, updatedCRs []*rbacv1.ClusterRole
			clusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			clusterRoles.EXPECT().Create(gomock.Any()).DoAndReturn(
				func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
					newCRs = append(newCRs, in)
					if test.createErr != nil {
						return nil, test.createErr
					}
					return in, nil
				},
			).AnyTimes()
			clusterRoles.EXPECT().Update(gomock.Any()).DoAndReturn(
				func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
					updatedCRs = append(updatedCRs, in)
					if test.updateErr != nil {
						return nil, test.updateErr
					}
					return in, nil
				},
			).AnyTimes()
			lifecycle := pLifecycle{
				m: &manager{
					crLister:     newProjectCRLister(ctrl, test.getErr, test.existingClusterRoles),
					clusterRoles: clusterRoles,
				},
			}
			_, err := lifecycle.Create(project)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, test.wantRoles, newCRs)
				assert.ElementsMatch(t, test.wantUpdatedRoles, updatedCRs)
			}
		})
	}
}

func TestUpdated(t *testing.T) {
	const projectName = "p-123xyz"
	ctrl := gomock.NewController(t)
	editNSRule := rbacv1.PolicyRule{
		APIGroups:     []string{""},
		Verbs:         []string{"*"},
		Resources:     []string{"namespaces"},
		ResourceNames: []string{"test-ns"},
	}
	newEditNSRole := func(rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
		return &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "p-123xyz-namespaces-edit",
				Annotations: map[string]string{
					projectNSAnn: "p-123xyz-namespaces-edit",
				},
			},
			Rules: rules,
		}
	}
	tests := []struct {
		name                string
		currentClusterRoles []*rbacv1.ClusterRole
		getError            error
		updError            error
		createError         error

		wantError        bool
		wantClusterRoles []*rbacv1.ClusterRole
	}{
		{
			name:                "roles are up to date",
			currentClusterRoles: []*rbacv1.ClusterRole{newEditNSRole(editNSRule), newManageNSRole(manageNSRule)},
			wantError:           false,
		},
		{
			name:                "edit role with manage-namespaces is left unchanged",
			currentClusterRoles: []*rbacv1.ClusterRole{newEditNSRole(editNSRule, manageNSRule), newManageNSRole(manageNSRule)},
			wantError:           false,
		},
		{
			name: "missing cluster roles",
			wantClusterRoles: []*rbacv1.ClusterRole{
				newManageNSRole(manageNSRule),
				newEditNSRole(),
			},
			wantError: false,
		},
		{
			name:                "missing manage cluster role",
			currentClusterRoles: []*rbacv1.ClusterRole{newEditNSRole(editNSRule, manageNSRule)},
			wantClusterRoles:    []*rbacv1.ClusterRole{newManageNSRole(manageNSRule)},
		},
		{
			name: "manage cluster role with extra rules",
			currentClusterRoles: []*rbacv1.ClusterRole{
				newEditNSRole(editNSRule, manageNSRule),
				newManageNSRole(manageNSRule, editNSRule),
			},
			wantClusterRoles: []*rbacv1.ClusterRole{newManageNSRole(manageNSRule)},
		},
		{
			name:      "get error",
			getError:  fmt.Errorf("unexpected error"),
			wantError: true,
		},
		{
			name:        "create error",
			createError: fmt.Errorf("unexpected error"),
			wantError:   true,
		},
		{
			name:                "update error",
			currentClusterRoles: []*rbacv1.ClusterRole{newEditNSRole(editNSRule), newManageNSRole()},
			updError:            fmt.Errorf("unexpected error"),
			wantError:           true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			project := &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
			}
			var newCRs []*rbacv1.ClusterRole
			crLister := newProjectCRLister(ctrl, test.getError, test.currentClusterRoles)
			clusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			clusterRoles.EXPECT().Create(gomock.Any()).DoAndReturn(
				func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
					newCRs = append(newCRs, in)
					if test.createError != nil {
						return nil, test.createError
					}
					return in, nil
				},
			).AnyTimes()
			clusterRoles.EXPECT().Update(gomock.Any()).DoAndReturn(
				func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
					newCRs = append(newCRs, in)
					if test.updError != nil {
						return nil, test.updError
					}
					return in, nil
				},
			).AnyTimes()
			lifecycle := pLifecycle{
				m: &manager{
					crLister:     crLister,
					clusterRoles: clusterRoles,
				},
			}
			_, err := lifecycle.Updated(project)
			if test.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, test.wantClusterRoles, newCRs)
			}
		})
	}
}

package rbac

import (
	"fmt"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	fakes2 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	recursiveTestRoleTemplates = map[string]*v3.RoleTemplate{
		"recursive1": {
			RoleTemplateNames: []string{"recursive2"},
		},
		"recursive2": {
			RoleTemplateNames: []string{"recursive1"},
		},
		"non-recursive": {},
		"inherit non-recursive": {
			RoleTemplateNames: []string{"non-recursive"},
		},
	}
	createNSRoleTemplate = &v3.RoleTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "create-ns",
		},
		Builtin: true,
		Rules: []v1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"create"},
			},
		},
	}
)

type clientErrs struct {
	getError    error
	updateError error
	createError error
}

func setupManager(roleTemplates map[string]*v3.RoleTemplate, clusterRoles map[string]*v1.ClusterRole, roles map[string]*v1.Role, projects map[string]*v3.Project, crErrs, rtErrs, rErrs clientErrs) *manager {
	return &manager{
		rtLister: &fakes.RoleTemplateListerMock{
			GetFunc: func(namespace string, name string) (*v3.RoleTemplate, error) {
				if rtErrs.getError != nil {
					return nil, rtErrs.getError
				}
				rt, ok := roleTemplates[name]
				if !ok {
					return nil, errors.NewNotFound(v3.RoleTemplateGroupVersionResource.GroupResource(), name)
				}
				return rt.DeepCopy(), nil
			},
			ListFunc: func(namespace string, selector labels.Selector) ([]*v3.RoleTemplate, error) {
				rts := make([]*v3.RoleTemplate, len(roleTemplates))
				for i := range roleTemplates {
					rts = append(rts, roleTemplates[i])
				}
				return rts, nil
			},
		},
		crLister: &fakes2.ClusterRoleListerMock{
			GetFunc: func(namespace string, name string) (*v1.ClusterRole, error) {
				if crErrs.getError != nil {
					return nil, crErrs.getError
				}
				cr, ok := clusterRoles[name]
				if !ok {
					return nil, errors.NewNotFound(v3.RoleTemplateGroupVersionResource.GroupResource(), name)
				}
				return cr.DeepCopy(), nil
			},
			ListFunc: func(namespace string, selector labels.Selector) ([]*v1.ClusterRole, error) {
				crs := make([]*v1.ClusterRole, len(roleTemplates))
				for i := range clusterRoles {
					crs = append(crs, clusterRoles[i])
				}
				return crs, nil
			},
		},
		clusterRoles: &fakes2.ClusterRoleInterfaceMock{
			GetFunc: func(name string, opts metav1.GetOptions) (*v1.ClusterRole, error) {
				if crErrs.getError != nil {
					return nil, crErrs.getError
				}
				cr, ok := clusterRoles[name]
				if !ok {
					return nil, errors.NewNotFound(v3.RoleTemplateGroupVersionResource.GroupResource(), name)
				}
				return cr.DeepCopy(), nil
			},
			UpdateFunc: func(cr *v1.ClusterRole) (*v1.ClusterRole, error) {
				if crErrs.updateError != nil {
					return nil, crErrs.updateError
				}
				_, ok := clusterRoles[cr.Name]
				if !ok {
					return nil, errors.NewNotFound(v3.RoleTemplateGroupVersionResource.GroupResource(), cr.Name)
				}
				clusterRoles[cr.Name] = cr
				return clusterRoles[cr.Name].DeepCopy(), nil
			},
			CreateFunc: func(cr *v1.ClusterRole) (*v1.ClusterRole, error) {
				if crErrs.createError != nil {
					return nil, crErrs.createError
				}
				_, ok := clusterRoles[cr.Name]
				if ok {
					return nil, errors.NewAlreadyExists(v3.RoleTemplateGroupVersionResource.GroupResource(), cr.Name)
				}
				clusterRoles[cr.Name] = cr
				return clusterRoles[cr.Name].DeepCopy(), nil
			},
		},
		rLister: &fakes2.RoleListerMock{
			GetFunc: func(namespace string, name string) (*v1.Role, error) {
				if rErrs.getError != nil {
					return nil, rErrs.getError
				}
				key := fmt.Sprintf("%s:%s", namespace, name)
				r, ok := roles[key]
				if !ok {
					return nil, errors.NewNotFound(v3.RoleTemplateGroupVersionResource.GroupResource(), name)
				}
				return r.DeepCopy(), nil
			},
			ListFunc: func(namespace string, selector labels.Selector) ([]*v1.Role, error) {
				rs := make([]*v1.Role, len(roles))
				for i := range roles {
					rs = append(rs, roles[i])
				}
				return rs, nil
			},
		},
		roles: &fakes2.RoleInterfaceMock{
			UpdateFunc: func(r *v1.Role) (*v1.Role, error) {
				key := fmt.Sprintf("%s:%s", r.Namespace, r.Name)
				_, ok := roles[key]
				if ok {
					return nil, errors.NewAlreadyExists(v3.RoleTemplateGroupVersionResource.GroupResource(), key)
				}
				roles[r.Name] = r
				return roles[r.Name].DeepCopy(), nil
			},
			GetNamespacedFunc: func(namespace string, name string, opts metav1.GetOptions) (*v1.Role, error) {
				key := fmt.Sprintf("%s:%s", namespace, name)
				r, ok := roles[key]
				if !ok {
					return nil, errors.NewNotFound(v3.RoleTemplateGroupVersionResource.GroupResource(), name)
				}
				return r.DeepCopy(), nil
			},
		},
		projectLister: &fakes.ProjectListerMock{
			ListFunc: func(namespace string, selector labels.Selector) ([]*apimgmtv3.Project, error) {
				rs := make([]*v3.Project, len(projects))
				for i := range projects {
					rs = append(rs, projects[i])
				}
				return rs, nil
			},
		},
		clusterName: "testcluster",
	}
}

func Test_gatherRoles(t *testing.T) {
	m := setupManager(recursiveTestRoleTemplates, make(map[string]*v1.ClusterRole), make(map[string]*v1.Role), make(map[string]*v3.Project), clientErrs{}, clientErrs{}, clientErrs{})

	emptyRoleTemplates := make(map[string]*v3.RoleTemplate)
	type args struct {
		rt            *v3.RoleTemplate
		roleTemplates map[string]*v3.RoleTemplate
		depthCounter  int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Non-recursive role, none inherited",
			args: args{
				rt:            recursiveTestRoleTemplates["non-recursive"],
				roleTemplates: emptyRoleTemplates,
				depthCounter:  0,
			},
			wantErr: false,
		},
		{
			name: "Non-recursive role, inherits another",
			args: args{
				rt:            recursiveTestRoleTemplates["inherit non-recursive"],
				roleTemplates: emptyRoleTemplates,
				depthCounter:  0,
			},
			wantErr: false,
		},
		{
			name: "Recursive role",
			args: args{
				rt:            recursiveTestRoleTemplates["recursive1"],
				roleTemplates: emptyRoleTemplates,
				depthCounter:  0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.gatherRoles(tt.args.rt, tt.args.roleTemplates, tt.args.depthCounter)
			if tt.wantErr {
				assert.Error(t, err, "expected an error, received none")
			} else {
				assert.NoError(t, err, fmt.Sprintf("expected no err, got %v", err))
			}
		})
	}
}

func TestCompareAndUpdateClusterRole(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		clusterRole         *v1.ClusterRole
		roleTemplate        *v3.RoleTemplate
		wantNumUpdatesCalls int
	}{
		"semantic difference": {
			clusterRole: &v1.ClusterRole{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			roleTemplate: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:           []string{"get"},
						APIGroups:       []string{""},
						Resources:       []string{"pods"},
						ResourceNames:   []string{},
						NonResourceURLs: []string{},
					},
				},
			},
			wantNumUpdatesCalls: 0,
		},
		"no difference": {
			clusterRole: &v1.ClusterRole{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			roleTemplate: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			wantNumUpdatesCalls: 0,
		},
		"difference": {
			clusterRole: &v1.ClusterRole{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			roleTemplate: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get", "update"},
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			wantNumUpdatesCalls: 1,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			numUpdatesCalled := 0
			var updateParamCalled *v1.ClusterRole
			clusterRolesMock := &fakes2.ClusterRoleInterfaceMock{
				UpdateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					numUpdatesCalled++
					updateParamCalled = in1
					return &v1.ClusterRole{}, nil
				},
			}
			m := manager{clusterRoles: clusterRolesMock}
			err := m.compareAndUpdateClusterRole(test.clusterRole, test.roleTemplate)
			assert.NoError(t, err)
			assert.Equal(t, test.wantNumUpdatesCalls, numUpdatesCalled)
			if test.wantNumUpdatesCalls > 0 {
				assert.Equal(t, test.roleTemplate.Rules, updateParamCalled.Rules)
			}
		})
	}
}

func TestCompareAndUpdateNamespacedRole(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		role                *v1.Role
		roleTemplate        *v3.RoleTemplate
		wantNumUpdatesCalls int
	}{
		"semantic difference": {
			role: &v1.Role{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			roleTemplate: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:           []string{"get"},
						APIGroups:       []string{""},
						Resources:       []string{"pods"},
						ResourceNames:   []string{},
						NonResourceURLs: []string{},
					},
				},
			},
			wantNumUpdatesCalls: 0,
		},
		"no difference": {
			role: &v1.Role{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			roleTemplate: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			wantNumUpdatesCalls: 0,
		},
		"difference": {
			role: &v1.Role{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			roleTemplate: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get", "update"},
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			wantNumUpdatesCalls: 1,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			numUpdatesCalled := 0
			rolesMock := &fakes2.RoleInterfaceMock{
				UpdateFunc: func(in1 *v1.Role) (*v1.Role, error) {
					numUpdatesCalled++
					return &v1.Role{}, nil
				},
			}
			m := manager{roles: rolesMock}
			err := m.compareAndUpdateNamespacedRole(test.role, test.roleTemplate, "")
			assert.NoError(t, err)
			assert.Equal(t, numUpdatesCalled, test.wantNumUpdatesCalls)
		})
	}
}

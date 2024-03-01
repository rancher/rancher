package rbac

import (
	"fmt"
	"sync"
	"testing"

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
	listError   error
	updateError error
	createError error
}

type managerOpt func(m *manager)

func newManager(opts ...managerOpt) *manager {
	manager := &manager{
		clusterName: "testcluster",
		rtLister:    &fakes.RoleTemplateListerMock{},
		crLister:    &fakes2.ClusterRoleListerMock{},
	}

	for _, opt := range opts {
		opt(manager)
	}

	return manager
}

// withRoleTemplates setup a rtLister mock with the provided roleTemplates and errors
func withRoleTemplates(roleTemplates map[string]*v3.RoleTemplate, errs *clientErrs) managerOpt {
	if roleTemplates == nil {
		roleTemplates = map[string]*v3.RoleTemplate{}
	}

	if errs == nil {
		errs = &clientErrs{}
	}

	syncRoleTemplates := &sync.Map{}
	for k, v := range roleTemplates {
		syncRoleTemplates.Store(k, v)
	}

	return func(m *manager) {
		m.rtLister = &fakes.RoleTemplateListerMock{
			ListFunc: newListFunc[*v3.RoleTemplate](syncRoleTemplates, errs.listError),
			GetFunc: func(namespace string, name string) (*v3.RoleTemplate, error) {
				if errs.getError != nil {
					return nil, errs.getError
				}

				rtVal, ok := syncRoleTemplates.Load(name)
				if !ok {
					return nil, errors.NewNotFound(v3.RoleTemplateGroupVersionResource.GroupResource(), name)
				}

				rt := rtVal.(*v3.RoleTemplate)
				return rt.DeepCopy(), nil
			},
		}
	}
}

// withRoleTemplates setup a crLister and clusterRoles mock with the provided clusterRoles and errors
func withClusterRoles(clusterRoles map[string]*v1.ClusterRole, errs *clientErrs) managerOpt {
	if clusterRoles == nil {
		clusterRoles = map[string]*v1.ClusterRole{}
	}

	if errs == nil {
		errs = &clientErrs{}
	}

	syncClusterRoles := &sync.Map{}
	for k, v := range clusterRoles {
		syncClusterRoles.Store(k, v)
	}

	return func(m *manager) {
		m.crLister = &fakes2.ClusterRoleListerMock{
			ListFunc: newListFunc[*v1.ClusterRole](syncClusterRoles, errs.listError),
			GetFunc: func(namespace string, name string) (*v1.ClusterRole, error) {
				if errs.getError != nil {
					return nil, errs.getError
				}

				crVal, ok := syncClusterRoles.Load(name)
				if !ok {
					return nil, errors.NewNotFound(v3.RoleTemplateGroupVersionResource.GroupResource(), name)
				}

				cr := crVal.(*v1.ClusterRole)
				return cr.DeepCopy(), nil
			},
		}

		m.clusterRoles = &fakes2.ClusterRoleInterfaceMock{
			GetFunc: func(name string, opts metav1.GetOptions) (*v1.ClusterRole, error) {
				if errs.getError != nil {
					return nil, errs.getError
				}

				crVal, ok := syncClusterRoles.Load(name)
				if !ok {
					return nil, errors.NewNotFound(v3.RoleTemplateGroupVersionResource.GroupResource(), name)
				}

				cr := crVal.(*v1.ClusterRole)
				return cr.DeepCopy(), nil
			},
			UpdateFunc: func(cr *v1.ClusterRole) (*v1.ClusterRole, error) {
				if errs.updateError != nil {
					return nil, errs.updateError
				}

				_, ok := syncClusterRoles.Load(cr.Name)
				if !ok {
					return nil, errors.NewNotFound(v3.RoleTemplateGroupVersionResource.GroupResource(), cr.Name)
				}

				syncClusterRoles.Store(cr.Name, cr)
				return cr.DeepCopy(), nil
			},
			CreateFunc: func(cr *v1.ClusterRole) (*v1.ClusterRole, error) {
				if errs.createError != nil {
					return nil, errs.createError
				}

				_, ok := syncClusterRoles.Load(cr.Name)
				if ok {
					return nil, errors.NewAlreadyExists(v3.RoleTemplateGroupVersionResource.GroupResource(), cr.Name)
				}

				syncClusterRoles.Store(cr.Name, cr)
				return cr.DeepCopy(), nil
			},
		}
	}
}

func newListFunc[T any](resourceMap *sync.Map, err error) func(string, labels.Selector) ([]T, error) {
	return func(namespace string, selector labels.Selector) ([]T, error) {
		if err != nil {
			return nil, err
		}

		resourceList := []T{}
		resourceMap.Range(func(key, value any) bool {
			resourceList = append(resourceList, value.(T))
			return true
		})

		return resourceList, nil
	}
}

func Test_gatherRoles(t *testing.T) {
	m := newManager(withRoleTemplates(recursiveTestRoleTemplates, nil))

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

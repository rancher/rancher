package rbac

import (
	"fmt"
	"sync"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	wfakes "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	createError error
}

type managerOpt func(m *manager)

func newManager(opts ...managerOpt) *manager {
	manager := &manager{
		clusterName: "testcluster",
	}

	for _, opt := range opts {
		opt(manager)
	}

	return manager
}

// withRoleTemplates setup a rtLister mock with the provided roleTemplates and errors
func withRoleTemplates(roleTemplates map[string]*v3.RoleTemplate, errs *clientErrs, ctrl *gomock.Controller) managerOpt {
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
		rtLister := wfakes.NewMockNonNamespacedCacheInterface[*v3.RoleTemplate](ctrl)
		rtLister.EXPECT().List(gomock.Any()).DoAndReturn(func() ([]*v3.RoleTemplate, error) {
			if errs.listError != nil {
				return nil, errs.listError
			}

			var result []*v3.RoleTemplate
			syncRoleTemplates.Range(func(key, value interface{}) bool {
				rt := value.(*v3.RoleTemplate)
				result = append(result, rt.DeepCopy())
				return true
			})
			return result, nil
		}).AnyTimes()
		rtLister.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.RoleTemplate, error) {
			if errs.getError != nil {
				return nil, errs.getError
			}

			rtVal, ok := syncRoleTemplates.Load(name)
			if !ok {
				return nil, errors.NewNotFound(v3.Resource(v3.RoleTemplateResourceName), name)
			}

			rt := rtVal.(*v3.RoleTemplate)
			return rt.DeepCopy(), nil
		}).AnyTimes()
		m.rtLister = rtLister
	}
}

// withClusterRoles setup a crLister and clusterRoles mock with the provided clusterRoles and errors
func withClusterRoles(clusterRoles map[string]*v1.ClusterRole, errs *clientErrs, ctrl *gomock.Controller) managerOpt {
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
		crLister := wfakes.NewMockNonNamespacedCacheInterface[*v1.ClusterRole](ctrl)
		crLister.EXPECT().List(gomock.Any()).DoAndReturn(func() ([]*v1.ClusterRole, error) {
			if errs.listError != nil {
				return nil, errs.listError
			}

			var result []*v1.ClusterRole
			syncClusterRoles.Range(func(key, value interface{}) bool {
				cr := value.(*v1.ClusterRole)
				result = append(result, cr.DeepCopy())
				return true
			})
			return result, nil
		}).AnyTimes()
		crLister.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v1.ClusterRole, error) {
			if errs.getError != nil {
				return nil, errs.getError
			}

			crVal, ok := syncClusterRoles.Load(name)
			if !ok {
				return nil, errors.NewNotFound(v3.Resource(v3.RoleTemplateResourceName), name)
			}

			cr := crVal.(*v1.ClusterRole)
			return cr.DeepCopy(), nil
		}).AnyTimes()
		m.crLister = crLister
	}
}

func Test_gatherRoles(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newManager(withRoleTemplates(recursiveTestRoleTemplates, nil, ctrl))

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
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		clusterRole     *v1.ClusterRole
		roleTemplate    *v3.RoleTemplate
		clusterRoleMock func() wrbacv1.ClusterRoleController
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
			clusterRoleMock: func() wrbacv1.ClusterRoleController {
				return wfakes.NewMockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList](ctrl)
			},
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
			clusterRoleMock: func() wrbacv1.ClusterRoleController {
				return wfakes.NewMockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList](ctrl)
			},
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
			clusterRoleMock: func() wrbacv1.ClusterRoleController {
				mock := wfakes.NewMockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList](ctrl)
				mock.EXPECT().Update(&v1.ClusterRole{
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"get", "update"},
							APIGroups: []string{""},
							Resources: []string{"pods"},
						},
					},
				})

				return mock
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			m := manager{
				clusterRoles: test.clusterRoleMock(),
			}
			err := m.compareAndUpdateClusterRole(test.clusterRole, test.roleTemplate)
			assert.NoError(t, err)
		})
	}
}

func TestCreateClusterRole(t *testing.T) {
	assert.NoError(t, settings.InstallUUID.Set("test-install-uuid"))

	rules := []v1.PolicyRule{
		{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods"}},
	}
	rt := &v3.RoleTemplate{ObjectMeta: metav1.ObjectMeta{Name: "rt-test"}, Rules: rules}
	alreadyExists := errors.NewAlreadyExists(v1.Resource("clusterroles"), "rt-test")
	clusterRoleWith := func(annotations map[string]string) *v1.ClusterRole {
		return &v1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "rt-test", Annotations: annotations},
			Rules:      rules,
		}
	}
	ownAnnotations := map[string]string{
		clusterRoleOwner:            "rt-test",
		clusterRoleOwnerInstallUUID: "test-install-uuid",
	}

	tests := map[string]struct {
		setup   func(mock *wfakes.MockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList])
		wantErr bool
	}{
		"created": {
			setup: func(mock *wfakes.MockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList]) {
				mock.EXPECT().Create(clusterRoleWith(ownAnnotations))
			},
		},
		"already exists, owned without install-uuid: backfilled": {
			setup: func(mock *wfakes.MockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList]) {
				mock.EXPECT().Create(gomock.Any()).Return(nil, alreadyExists)
				mock.EXPECT().Get("rt-test", gomock.Any()).Return(clusterRoleWith(map[string]string{clusterRoleOwner: "rt-test"}), nil)
				mock.EXPECT().Update(clusterRoleWith(ownAnnotations))
			},
		},
		"already exists, foreign install-uuid: not updated": {
			setup: func(mock *wfakes.MockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList]) {
				mock.EXPECT().Create(gomock.Any()).Return(nil, alreadyExists)
				mock.EXPECT().Get("rt-test", gomock.Any()).Return(clusterRoleWith(map[string]string{
					clusterRoleOwner:            "rt-test",
					clusterRoleOwnerInstallUUID: "foreign-install-uuid",
				}), nil)
			},
		},
		"already exists, foreign install-uuid, different rules: not updated": {
			setup: func(mock *wfakes.MockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList]) {
				mock.EXPECT().Create(gomock.Any()).Return(nil, alreadyExists)
				foreign := clusterRoleWith(map[string]string{
					clusterRoleOwner:            "rt-test",
					clusterRoleOwnerInstallUUID: "foreign-install-uuid",
				})
				foreign.Rules = []v1.PolicyRule{{Verbs: []string{"*"}, APIGroups: []string{"*"}, Resources: []string{"*"}}}
				mock.EXPECT().Get("rt-test", gomock.Any()).Return(foreign, nil)
			},
		},
		"already exists, get fails: error": {
			setup: func(mock *wfakes.MockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList]) {
				mock.EXPECT().Create(gomock.Any()).Return(nil, alreadyExists)
				mock.EXPECT().Get("rt-test", gomock.Any()).Return(nil, fmt.Errorf("boom"))
			},
			wantErr: true,
		},
		"create fails: error": {
			setup: func(mock *wfakes.MockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList]) {
				mock.EXPECT().Create(gomock.Any()).Return(nil, fmt.Errorf("boom"))
			},
			wantErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mock := wfakes.NewMockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList](ctrl)
			test.setup(mock)

			m := manager{clusterRoles: mock}
			err := m.createClusterRole(rt)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCompareAndUpdateClusterRoleInstallUUIDBackfill(t *testing.T) {
	assert.NoError(t, settings.InstallUUID.Set("own-install-uuid"))

	rules := []v1.PolicyRule{
		{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods"}},
	}
	otherRules := []v1.PolicyRule{
		{Verbs: []string{"*"}, APIGroups: []string{"*"}, Resources: []string{"*"}},
	}
	rt := &v3.RoleTemplate{ObjectMeta: metav1.ObjectMeta{Name: "rt-test"}, Rules: rules}

	tests := map[string]struct {
		annotations    map[string]string
		crRules        []v1.PolicyRule
		expectedUpdate map[string]string
	}{
		"owned, no install-uuid: backfilled": {
			annotations: map[string]string{clusterRoleOwner: "rt-test"},
			expectedUpdate: map[string]string{
				clusterRoleOwner:            "rt-test",
				clusterRoleOwnerInstallUUID: "own-install-uuid",
			},
		},
		"owned, own install-uuid: no update": {
			annotations: map[string]string{
				clusterRoleOwner:            "rt-test",
				clusterRoleOwnerInstallUUID: "own-install-uuid",
			},
		},
		"owned, foreign install-uuid: not overwritten": {
			annotations: map[string]string{
				clusterRoleOwner:            "rt-test",
				clusterRoleOwnerInstallUUID: "foreign-install-uuid",
			},
		},
		"owned, foreign install-uuid, different rules: not updated": {
			annotations: map[string]string{
				clusterRoleOwner:            "rt-test",
				clusterRoleOwnerInstallUUID: "foreign-install-uuid",
			},
			crRules: otherRules,
		},
		"owned, own install-uuid, different rules: rules updated": {
			annotations: map[string]string{
				clusterRoleOwner:            "rt-test",
				clusterRoleOwnerInstallUUID: "own-install-uuid",
			},
			crRules: otherRules,
			expectedUpdate: map[string]string{
				clusterRoleOwner:            "rt-test",
				clusterRoleOwnerInstallUUID: "own-install-uuid",
			},
		},
		"not owned: not stamped": {
			annotations: map[string]string{},
		},
		"not owned: not stamped, nil map": {
			annotations: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mock := wfakes.NewMockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList](ctrl)
			if test.expectedUpdate != nil {
				mock.EXPECT().Update(&v1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "rt-test", Annotations: test.expectedUpdate},
					Rules:      rules,
				})
			}

			crRules := rules
			if test.crRules != nil {
				crRules = test.crRules
			}
			cr := &v1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{Name: "rt-test", Annotations: test.annotations},
				Rules:      crRules,
			}
			m := manager{clusterRoles: mock}
			assert.NoError(t, m.compareAndUpdateClusterRole(cr, rt))
			if test.expectedUpdate == nil {
				assert.Equal(t, test.annotations, cr.Annotations, "input must not be mutated")
				assert.Equal(t, crRules, cr.Rules, "input must not be mutated")
			}
		})
	}
}

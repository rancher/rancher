package rbac

import (
	"errors"
	"fmt"
	"io"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	typesrbacv1fakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_manager_checkForGlobalResourceRules(t *testing.T) {
	type tests struct {
		name     string
		role     *v3.RoleTemplate
		resource string
		baseRule rbacv1.PolicyRule
		want     map[string]struct{}
	}

	testCases := []tests{
		{
			name: "valid_api_group_persistentvolumes",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{""},
						Resources: []string{"persistentvolumes"},
					},
				},
			},
			resource: "persistentvolumes",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]struct{}{"put": {}},
		},
		{
			name: "invalid_api_group_persistentvolumes",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{"foo"},
						Resources: []string{"persistentvolumes"},
					},
				},
			},
			resource: "persistentvolumes",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]struct{}{},
		},
		{
			name: "valid_api_group_storageclasses",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{"storage.k8s.io"},
						Resources: []string{"storageclasses"},
					},
				},
			},
			resource: "storageclasses",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]struct{}{"put": {}},
		},
		{
			name: "invalid_api_group_storageclasses",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{"foo"},
						Resources: []string{"storageclasses"},
					},
				},
			},
			resource: "storageclasses",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]struct{}{},
		},
		{
			name: "valid_api_group_start",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{""},
						Resources: []string{"*"},
					},
				},
			},
			resource: "persistentvolumes",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]struct{}{"put": {}},
		},
		{
			name: "invalid_api_group_star",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{"foo"},
						Resources: []string{"*"},
					},
				},
			},
			resource: "persistentvolumes",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]struct{}{},
		},
		{
			name: "cluster_rule_match",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{"management.cattle.io"},
						Resources: []string{"clusters"},
					},
				},
			},
			resource: "clusters",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]struct{}{"get": {}},
		},
		{
			name: "cluster_rule_resource_names_match",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:         []string{"get"},
						APIGroups:     []string{"management.cattle.io"},
						Resources:     []string{"clusters"},
						ResourceNames: []string{"local"},
					},
				},
			},
			resource: "clusters",
			baseRule: rbacv1.PolicyRule{
				ResourceNames: []string{"local"},
			},
			want: map[string]struct{}{"get": {}},
		},
		{
			name: "cluster_rule_baserule_resource_names_no_match",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{"management.cattle.io"},
						Resources: []string{"clusters"},
					},
				},
			},
			resource: "clusters",
			baseRule: rbacv1.PolicyRule{
				ResourceNames: []string{"local"},
			},
			want: map[string]struct{}{},
		},
		{
			name: "cluster_rule_roletemplate_resource_names_no_match",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:         []string{"get"},
						APIGroups:     []string{"management.cattle.io"},
						Resources:     []string{"clusters"},
						ResourceNames: []string{"local"},
					},
				},
			},
			resource: "clusters",
			baseRule: rbacv1.PolicyRule{},
			want:     map[string]struct{}{},
		},
	}

	m := &manager{}

	for _, test := range testCases {
		got, err := m.checkForGlobalResourceRules(test.role, test.resource, test.baseRule)
		assert.Nil(t, err)
		assert.Equal(t, test.want, got, fmt.Sprintf("test %v failed", test.name))
	}
}

func Test_manager_reconcileRoleForProjectAccessToGlobalResource(t *testing.T) {
	// discard logs to avoid cluttering
	logrus.SetOutput(io.Discard)

	type args struct {
		resource string
		rtName   string
		newVerbs map[string]struct{}
		baseRule rbacv1.PolicyRule
	}

	tests := []struct {
		name             string
		crListerMock     *typesrbacv1fakes.ClusterRoleListerMock
		clusterRolesMock *typesrbacv1fakes.ClusterRoleInterfaceMock
		args             args
		want             string
		wantErr          bool
	}{
		{
			name: "non existing ClusteRole will create a new promoted ClusterRole",
			args: args{
				rtName:   "myrole",
				resource: "myresource",
				newVerbs: map[string]struct{}{"get": {}, "list": {}},
				baseRule: rbacv1.PolicyRule{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
			crListerMock: &typesrbacv1fakes.ClusterRoleListerMock{
				GetFunc: func(namespace, name string) (*v1.ClusterRole, error) {
					return nil, errNotFound
				},
			},
			clusterRolesMock: &typesrbacv1fakes.ClusterRoleInterfaceMock{
				CreateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					assert.Equal(t, &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "myrole-promoted",
						},
						Rules: []rbacv1.PolicyRule{{
							APIGroups:     []string{"management.cattle.io"},
							ResourceNames: []string{"local"},
							Resources:     []string{"myresource"},
							Verbs:         []string{"get", "list"},
						}},
					}, in1)
					return in1, nil
				},
				UpdateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					assert.Fail(t, "unexpected call to ClusterRole Update")
					return nil, nil
				},
			},
			want: "myrole-promoted",
		},
		{
			name: "existing ClusteRole will update it",
			args: args{
				rtName:   "myrole",
				resource: "myresource",
				newVerbs: map[string]struct{}{"list": {}, "delete": {}},
				baseRule: rbacv1.PolicyRule{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
			crListerMock: &typesrbacv1fakes.ClusterRoleListerMock{
				GetFunc: func(namespace, name string) (*v1.ClusterRole, error) {
					return &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "myrole-promoted",
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups:     []string{"another.cattle.io"},
								ResourceNames: []string{"foobar"},
								Resources:     []string{"nodes"},
								Verbs:         []string{"create"},
							},
							{
								APIGroups:     []string{"management.cattle.io"},
								ResourceNames: []string{"local"},
								Resources:     []string{"myresource"},
								Verbs:         []string{"get", "list"},
							},
						},
					}, nil
				},
			},
			clusterRolesMock: &typesrbacv1fakes.ClusterRoleInterfaceMock{
				CreateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					assert.Fail(t, "unexpected call to ClusterRole Create")
					return in1, nil
				},
				UpdateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					assert.Equal(t, &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "myrole-promoted",
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups:     []string{"another.cattle.io"},
								ResourceNames: []string{"foobar"},
								Resources:     []string{"nodes"},
								Verbs:         []string{"create"},
							},
							{
								APIGroups:     []string{"management.cattle.io"},
								ResourceNames: []string{"local"},
								Resources:     []string{"myresource"},
								Verbs:         []string{"delete", "list"},
							},
						},
					}, in1)
					return nil, nil
				},
			},
			want: "myrole-promoted",
		},
		{
			name: "removing verbs from policy will remove the rule",
			args: args{
				rtName:   "myrole",
				resource: "myresource",
				newVerbs: map[string]struct{}{},
				baseRule: rbacv1.PolicyRule{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
			crListerMock: &typesrbacv1fakes.ClusterRoleListerMock{
				GetFunc: func(namespace, name string) (*v1.ClusterRole, error) {
					return &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "myrole-promoted",
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups:     []string{"another.cattle.io"},
								ResourceNames: []string{"foobar"},
								Resources:     []string{"nodes"},
								Verbs:         []string{"create"},
							},
							{
								APIGroups:     []string{"management.cattle.io"},
								ResourceNames: []string{"local"},
								Resources:     []string{"myresource"},
								Verbs:         []string{"get", "list"},
							},
						},
					}, nil
				},
			},
			clusterRolesMock: &typesrbacv1fakes.ClusterRoleInterfaceMock{
				CreateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					assert.Fail(t, "unexpected call to ClusterRole Create")
					return in1, nil
				},
				UpdateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					assert.Equal(t, &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "myrole-promoted",
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups:     []string{"another.cattle.io"},
								ResourceNames: []string{"foobar"},
								Resources:     []string{"nodes"},
								Verbs:         []string{"create"},
							},
						},
					}, in1)
					return in1, nil
				},
			},
			want: "myrole-promoted",
		},
		{
			name: "reconciling non existing ClusterRole with no verbs is nop-op",
			args: args{
				rtName:   "myrole",
				resource: "myresource",
				newVerbs: map[string]struct{}{},
				baseRule: rbacv1.PolicyRule{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
			crListerMock: &typesrbacv1fakes.ClusterRoleListerMock{
				GetFunc: func(namespace, name string) (*v1.ClusterRole, error) {
					return nil, errNotFound
				},
			},
			clusterRolesMock: &typesrbacv1fakes.ClusterRoleInterfaceMock{
				CreateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					assert.Fail(t, "unexpected call to ClusterRole Create")
					return in1, nil
				},
				UpdateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					assert.Fail(t, "unexpected call to ClusterRole Update")
					return in1, nil
				},
			},
			want: "",
		},
		{
			name: "create a the role if get fails",
			args: args{
				rtName:   "myrole",
				resource: "myresource",
				newVerbs: map[string]struct{}{"get": {}, "list": {}},
				baseRule: rbacv1.PolicyRule{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
			crListerMock: &typesrbacv1fakes.ClusterRoleListerMock{
				GetFunc: func(namespace, name string) (*v1.ClusterRole, error) {
					return nil, errors.New("get failed")
				},
			},
			clusterRolesMock: &typesrbacv1fakes.ClusterRoleInterfaceMock{
				CreateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					assert.Equal(t, &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "myrole-promoted",
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups:     []string{"management.cattle.io"},
								ResourceNames: []string{"local"},
								Resources:     []string{"myresource"},
								Verbs:         []string{"get", "list"},
							},
						},
					}, in1)
					return in1, nil
				},
				UpdateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					assert.Fail(t, "unexpected call to ClusterRole Update")
					return in1, nil
				},
			},
			want: "myrole-promoted",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			manager := manager{
				crLister:     tc.crListerMock,
				clusterRoles: tc.clusterRolesMock,
			}

			got, err := manager.reconcileRoleForProjectAccessToGlobalResource(tc.args.resource, tc.args.rtName, tc.args.newVerbs, tc.args.baseRule)
			assert.Equal(t, tc.want, got)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

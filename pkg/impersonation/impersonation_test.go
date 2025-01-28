package impersonation

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	authcommon "github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

func TestImpersonatorGetUser(t *testing.T) {
	tests := []struct {
		name              string
		userInfo          user.Info
		groupName         string
		userGetFunc       func(_, _ string) (*v3.User, error)
		userAttribGetFunc func(_, _ string) (*v3.UserAttribute, error)
		want              user.Info
	}{
		{
			name:     "plain local user",
			userInfo: &user.DefaultInfo{Name: "user-abcde"},
			userGetFunc: func(_, _ string) (*v3.User, error) {
				return &v3.User{
					ObjectMeta:  metav1.ObjectMeta{Name: "user-abcde"},
					Username:    "admin",
					DisplayName: "Default Admin",
					PrincipalIDs: []string{
						"local://user-abcde",
					},
				}, nil
			},
			userAttribGetFunc: func(_, _ string) (*v3.UserAttribute, error) {
				return nil, nil
			},
			want: &user.DefaultInfo{
				UID:  "user-abcde",
				Name: "admin",
				Groups: []string{
					"system:authenticated",
					"system:cattle:authenticated",
				},
				Extra: map[string][]string{
					authcommon.UserAttributeUserName:    {"Default Admin"},
					authcommon.UserAttributePrincipalID: {"local://user-abcde"},
				},
			},
		},
		{
			name:     "local system cluster user",
			userInfo: &user.DefaultInfo{Name: "u-system"},
			userGetFunc: func(_, _ string) (*v3.User, error) {
				return &v3.User{
					ObjectMeta:  metav1.ObjectMeta{Name: "u-system"},
					DisplayName: "System account for Cluster c-abcde",
					PrincipalIDs: []string{
						"local://u-system",
					},
				}, nil
			},
			userAttribGetFunc: func(_, _ string) (*v3.UserAttribute, error) {
				return nil, nil
			},
			want: &user.DefaultInfo{
				UID: "u-system",
				Groups: []string{
					"system:authenticated",
					"system:cattle:authenticated",
				},
				Extra: map[string][]string{
					authcommon.UserAttributeUserName:    {"System account for Cluster c-abcde"},
					authcommon.UserAttributePrincipalID: {"local://u-system"},
				},
			},
		},
		{
			name:     "local system nonspecific user",
			userInfo: &user.DefaultInfo{Name: "u-system"},
			userGetFunc: func(_, _ string) (*v3.User, error) {
				return &v3.User{
					ObjectMeta: metav1.ObjectMeta{Name: "u-system"},
					PrincipalIDs: []string{
						"local://u-system",
					},
				}, nil
			},
			userAttribGetFunc: func(_, _ string) (*v3.UserAttribute, error) {
				return nil, nil
			},
			want: &user.DefaultInfo{
				UID: "u-system",
				Groups: []string{
					"system:authenticated",
					"system:cattle:authenticated",
				},
				Extra: map[string][]string{
					"principalid": {"local://u-system"},
				},
			},
		},
		{
			name: "multi auth provider user",
			userInfo: &user.DefaultInfo{
				Name: "user-abcde",
			},
			userGetFunc: func(_, _ string) (*v3.User, error) {
				return &v3.User{
					ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
					Username:   "admin",
				}, nil
			},
			userAttribGetFunc: func(_, _ string) (*v3.UserAttribute, error) {
				return &v3.UserAttribute{
					GroupPrincipals: map[string]v3.Principals{
						"github": {
							Items: []v3.Principal{
								{
									ObjectMeta: metav1.ObjectMeta{Name: "github_org://456"},
								},
								{
									ObjectMeta: metav1.ObjectMeta{Name: "github_org://123"},
								},
							},
						},
						"openldap": {
							Items: []v3.Principal{
								{
									ObjectMeta: metav1.ObjectMeta{Name: "openldap_group://cn=group1,dc=example,dc=org"},
								},
								{
									ObjectMeta: metav1.ObjectMeta{Name: "openldap_group://cn=group2,dc=example,dc=org"},
								},
								{
									ObjectMeta: metav1.ObjectMeta{Name: "openldap_group://cn=group2,dc=example,dc=org"},
								},
							},
						},
					},
					ExtraByProvider: map[string]map[string][]string{
						"github": {
							authcommon.UserAttributeUserName: {
								"user1",
							},
							authcommon.UserAttributePrincipalID: {
								"github_user://890",
							},
						},
						"openldap": {
							authcommon.UserAttributeUserName: {
								"admin",
							},
							authcommon.UserAttributePrincipalID: {
								"openldap_user://uid=user1,dc=example,dc=org",
							},
						},
					},
				}, nil
			},
			want: &user.DefaultInfo{
				UID:  "user-abcde",
				Name: "admin",
				Groups: []string{
					"github_org://123",
					"github_org://456",
					"openldap_group://cn=group1,dc=example,dc=org",
					"openldap_group://cn=group2,dc=example,dc=org",
					"system:authenticated",
					"system:cattle:authenticated",
				},
				Extra: map[string][]string{
					authcommon.UserAttributeUserName: {
						"admin",
						"user1",
					},
					authcommon.UserAttributePrincipalID: {
						"github_user://890",
						"openldap_user://uid=user1,dc=example,dc=org",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impersonator := Impersonator{
				userLister: &fakes.UserListerMock{
					GetFunc: tt.userGetFunc,
				},
				userAttributeLister: &fakes.UserAttributeListerMock{
					GetFunc: tt.userAttribGetFunc,
				},
			}
			got, err := impersonator.getUser(tt.userInfo)
			assert.Nil(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImpersonatorRulesForUser(t *testing.T) {
	impersonator := Impersonator{
		user: &user.DefaultInfo{
			UID:    "u-s857n",
			Name:   "u-s857n",
			Groups: []string{"system:authenticated", "system:cattle:authenticated"},
			Extra: map[string][]string{
				authcommon.UserAttributePrincipalID: {"local://u-s857n"},
				authcommon.UserAttributeUserName:    {"test"},
				authcommon.ExtraRequestTokenID:      {"kubeconfig-u-s857nk2bxr"},
				authcommon.ExtraRequestHost:         {"rancher.example.com"},
			},
		},
	}

	want := []rbacv1.PolicyRule{
		{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{""},
			Resources:     []string{"users"},
			ResourceNames: []string{"u-s857n"},
		},
		{
			Verbs:     []string{"impersonate"},
			APIGroups: []string{"authentication.k8s.io"},
			Resources: []string{"userextras/" + authcommon.ExtraRequestTokenID},
		},
		{
			Verbs:     []string{"impersonate"},
			APIGroups: []string{"authentication.k8s.io"},
			Resources: []string{"userextras/" + authcommon.ExtraRequestHost},
		},
		{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{""},
			Resources:     []string{"groups"},
			ResourceNames: []string{"system:authenticated", "system:cattle:authenticated"},
		},
		{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{"authentication.k8s.io"},
			Resources:     []string{"userextras/" + authcommon.UserAttributePrincipalID},
			ResourceNames: []string{"local://u-s857n"},
		},
		{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{"authentication.k8s.io"},
			Resources:     []string{"userextras/" + authcommon.UserAttributeUserName},
			ResourceNames: []string{"test"},
		},
	}

	got := impersonator.rulesForUser()
	assert.Equal(t, want, got)
}

package impersonation

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

func Test_getUser(t *testing.T) {
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
					ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
					Username:   "admin",
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
			},
		},
		{
			name: "multi auth provider user",
			userInfo: &user.DefaultInfo{
				Name: "user-abcde",
				Extra: map[string][]string{
					"username": []string{
						"user1",
						"admin",
					},
					"principalid": []string{
						"openldap_user://uid=user1,dc=example,dc=org",
						"github_user://890",
					},
				},
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
						"github": v3.Principals{
							Items: []v3.Principal{
								{
									ObjectMeta: metav1.ObjectMeta{Name: "github_org://456"},
								},
								{
									ObjectMeta: metav1.ObjectMeta{Name: "github_org://123"},
								},
							},
						},
						"openldap": v3.Principals{
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
					"username": []string{
						"admin",
						"user1",
					},
					"principalid": []string{
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

package auth

import (
	"reflect"
	"sort"
	"testing"

	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_getRBOwnerKey(t *testing.T) {
	type args struct {
		rb *v1.RoleBinding
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "get two ownerreferences",
			args: args{
				rb: &v1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "Pod",
								Name:       "example-pod",
								UID:        "12345-67890-abcdef-1",
							},
							{
								APIVersion: "v1",
								Kind:       "Pod",
								Name:       "example-pod",
								UID:        "12345-67890-abcdef-2",
							},
						},
					},
				},
			},
			want: []string{"12345-67890-abcdef-1", "12345-67890-abcdef-2"},
		},
		{
			name: "get zero ownerreferences",
			args: args{
				rb: &v1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{},
					},
				},
			},
			// slice of 0 length is considered nil.
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getRBOwnerKey(tt.args.rb); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getRBOwnerKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_rbRoleSubjectKeys(t *testing.T) {
	type args struct {
		roleName string
		subject  []v1.Subject
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "test with real examples",
			args: args{
				roleName: "rbac-role-binding-role-binding",
				subject: []v1.Subject{
					{
						Kind: "User",
						Name: "test1",
					},
					{
						Kind: "ServiceAccount",
						Name: "test1",
					},
				},
			},
			want: []string{
				"rbac-role-binding-role-binding.User.test1",
				"rbac-role-binding-role-binding.ServiceAccount.test1",
			},
		},
		{
			name: "test with empty examples",
			args: args{
				roleName: "",
				subject: []v1.Subject{
					{
						Kind: "User",
						Name: "test1",
					},
					{
						Kind: "",
						Name: "",
					},
				},
			},
			want: []string{".User.test1", ".."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rbRoleSubjectKeys(tt.args.roleName, tt.args.subject); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rbRoleSubjectKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_rbObjectKeys(t *testing.T) {
	type args struct {
		obj metav1.Object
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "get one object key",
			args: args{
				obj: &v1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example-rolebinding",
						Namespace: "default",
						UID:       "rolebinding-uid-112233",
						Labels: map[string]string{
							"app": MembershipBindingOwner,
						},
					},
					RoleRef: v1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "Role",
						Name:     "example-role",
					},
					Subjects: []v1.Subject{
						{
							Kind:      "User",
							Name:      "example-user",
							Namespace: "default",
						},
					},
				},
			},
			want: []string{
				"default/app",
			},
			wantErr: false,
		},
		{
			name: "get zero object keys",
			args: args{
				obj: &v1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example-rolebinding",
						Namespace: "default",
						UID:       "rolebinding-uid-112233",
						Labels: map[string]string{
							"app": "test-app",
						},
					},
					RoleRef: v1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "Role",
						Name:     "example-role",
					},
					Subjects: []v1.Subject{
						{
							Kind:      "User",
							Name:      "example-user",
							Namespace: "default",
						},
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "manage multiple object keys",
			args: args{
				obj: &v1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example-rolebinding",
						Namespace: "default",
						UID:       "rolebinding-uid-112233",
						Labels: map[string]string{
							"app": MembershipBindingOwner,
							"env": "test-env",
						},
					},
					RoleRef: v1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "Role",
						Name:     "example-role",
					},
					Subjects: []v1.Subject{
						{
							Kind:      "User",
							Name:      "example-user",
							Namespace: "default",
						},
					},
				},
			},
			want: []string{
				"default/app",
			},
			wantErr: false,
		},
		{
			name: "get multiple object keys",
			args: args{
				obj: &v1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example-rolebinding",
						Namespace: "default",
						UID:       "rolebinding-uid-112233",
						Labels: map[string]string{
							"app": MembershipBindingOwner,
							"env": MembershipBindingOwner,
						},
					},
					RoleRef: v1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "Role",
						Name:     "example-role",
					},
					Subjects: []v1.Subject{
						{
							Kind:      "User",
							Name:      "example-user",
							Namespace: "default",
						},
					},
				},
			},
			want: []string{
				"default/app",
				"default/env",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rbObjectKeys(tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("rbObjectKeys() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// sort values to ensure their order
			// this will make the test deterministic
			sort.Strings(got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rbObjectKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}

package auth

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_hasValidPrincipalID(t *testing.T) {
	type args struct {
		user *v3.User
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "has local PrincipalIDs",
			args: args{
				user: &v3.User{
					Username: "testuser",
					PrincipalIDs: []string{
						"ID1",
						"ID2",
						"local://testuser",
					},
				},
			},
			want: true,
		},
		{
			name: "has not local PrincipalIDs",
			args: args{
				user: &v3.User{
					Username: "testuser",
					PrincipalIDs: []string{
						"ID1",
						"ID2",
					},
				},
			},
			want: false,
		},
		{
			name: "has not PrincipalIDs",
			args: args{
				user: &v3.User{
					Username:     "testuser",
					PrincipalIDs: []string{},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasLocalPrincipalID(tt.args.user); got != tt.want {
				t.Errorf("hasValidPrincipalIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserManager := NewMockManager(ctrl)

	ul := &userLifecycle{
		userManager: mockUserManager,
	}

	tests := []struct {
		name          string
		inputUser     *v3.User
		mockSetup     func()
		expectedUser  *v3.User
		expectedError bool
	}{
		{
			name: "User without local principal IDs",
			inputUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{},
				},
				PrincipalIDs: []string{},
			},
			mockSetup: func() {},
			expectedUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{},
				},
				PrincipalIDs: []string{"local://testuser"},
			},
			expectedError: false,
		},
		{
			name: "User with creatorID annotation and successful role binding",
			inputUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					UID:         defaultCRTB.UID,
					Annotations: map[string]string{creatorIDAnn: "creator"},
				},
				PrincipalIDs: []string{},
			},
			mockSetup: func() {
				mockUserManager.EXPECT().CreateNewUserClusterRoleBinding("testuser", defaultCRTB.UID).Return(nil)
			},
			expectedUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{creatorIDAnn: "creator"},
				},
				PrincipalIDs: []string{"local://testuser"},
			},
			expectedError: false,
		},
		{
			name: "User with creatorID annotation and role binding error",
			inputUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{creatorIDAnn: "creator"},
				},
				PrincipalIDs: []string{},
			},
			mockSetup: func() {
				mockUserManager.EXPECT().CreateNewUserClusterRoleBinding("testuser", defaultCRTB.UID).Return(fmt.Errorf("role binding error"))
			},
			expectedUser:  nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			_, err := ul.Create(tt.inputUser)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

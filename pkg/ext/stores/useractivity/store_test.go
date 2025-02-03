package useractivity

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3Legacy "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStore_create(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockTokenControllerFake := wranglerfake.NewMockNonNamespacedControllerInterface[*v3Legacy.Token, *v3Legacy.TokenList](ctrl)
	uas := &Store{
		tokenController: mockTokenControllerFake,
		checker:         nil,
	}

	type args struct {
		in0          context.Context
		userActivity *ext.UserActivity
		token        *v3Legacy.Token
		user         string
		lastActivity metav1.Time
		idleMins     int
	}
	tests := []struct {
		name      string
		args      args
		mockSetup func()
		want      *ext.UserActivity
		wantErr   bool
	}{
		{
			name: "valid useractivity is created",
			args: args{
				in0: nil,
				userActivity: &ext.UserActivity{
					Spec: ext.UserActivitySpec{
						TokenId: "u-mo773yttt4",
					},
				},
				token: &v3Legacy.Token{
					ObjectMeta: v1.ObjectMeta{
						Labels: map[string]string{
							tokenUserId: "admin",
						},
					},
					UserID: "u-mo773yttt4",
				},
				user: "admin",
				lastActivity: v1.Time{
					Time: time.Date(2025, 1, 31, 16, 44, 0, 0, &time.Location{}),
				},
				idleMins: 10,
			},
			mockSetup: func() {
				// we don't care about the object returned by the Update function,
				// since we only check there are no errors.
				mockTokenControllerFake.EXPECT().Update(gomock.Any()).DoAndReturn(
					func(token *v3Legacy.Token) (*v3Legacy.Token, error) {
						return token, nil
					},
				).Times(1)
			},
			want: &ext.UserActivity{
				Spec: ext.UserActivitySpec{
					TokenId: "u-mo773yttt4",
				},
				Status: ext.UserActivityStatus{
					CurrentTimeout: time.Date(2025, 1, 31, 16, 54, 0, 0, &time.Location{}).String(),
					LastActivity:   time.Date(2025, 1, 31, 16, 44, 0, 0, &time.Location{}).String(),
				},
			},
			wantErr: false,
		},
		{
			name: "UserID is different from TokenId",
			args: args{
				in0: nil,
				userActivity: &ext.UserActivity{
					Spec: ext.UserActivitySpec{
						TokenId: "u-mo773yttt4",
					},
				},
				token: &v3Legacy.Token{
					ObjectMeta: v1.ObjectMeta{
						Labels: map[string]string{
							tokenUserId: "admin",
						},
					},
					UserID: "u-mo773yttt3",
				},
			},
			mockSetup: func() {},
			want:      nil,
			wantErr:   true,
		},
		{
			name: "token label userId is different from user",
			args: args{
				in0: nil,
				userActivity: &ext.UserActivity{
					Spec: ext.UserActivitySpec{
						TokenId: "u-mo773yttt4",
					},
				},
				token: &v3Legacy.Token{
					ObjectMeta: v1.ObjectMeta{
						Labels: map[string]string{
							tokenUserId: "admin",
						},
					},
					UserID: "u-mo773yttt4",
				},
				user: "standard-user-1",
			},
			mockSetup: func() {},
			want:      nil,
			wantErr:   true,
		},
		{
			name: "error updating token value LastIdleTimeout",
			args: args{
				in0: nil,
				userActivity: &ext.UserActivity{
					Spec: ext.UserActivitySpec{
						TokenId: "u-mo773yttt4",
					},
				},
				token: &v3Legacy.Token{
					ObjectMeta: v1.ObjectMeta{
						Labels: map[string]string{
							tokenUserId: "admin",
						},
					},
					UserID: "u-mo773yttt4",
				},
				user: "admin",
				lastActivity: v1.Time{
					Time: time.Date(2025, 1, 31, 16, 44, 0, 0, &time.Location{}),
				},
				idleMins: 10,
			},
			mockSetup: func() {
				mockTokenControllerFake.EXPECT().Update(gomock.Any()).DoAndReturn(
					func(token *v3Legacy.Token) (*v3Legacy.Token, error) {
						return nil, errors.New("some error happend")
					},
				).Times(1)
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := uas.create(tt.args.in0, tt.args.userActivity, tt.args.token, tt.args.user, tt.args.lastActivity, tt.args.idleMins)
			if (err != nil) != tt.wantErr {
				t.Errorf("Store.create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Store.create() = %v, want %v", got, tt.want)
			}
		})
	}
}
